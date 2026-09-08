package rbhparser

import (
	"bytes"
	"fmt"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// Registry stores immutable registrations. A single lock makes multi-table
// launch commits visible atomically while keeping lookups allocation-free.
type Registry struct {
	mu      sync.RWMutex
	pools   map[common.Hash]PoolRegistration
	curves  map[common.Address]CurveRegistration
	pending map[common.Hash]PendingPoolRegistration
	parent  *Registry
}

// RegistrySnapshot is a point-in-time, deterministic copy of all discovery
// state needed to resume parsing after a restart or reorg rollback.
type RegistrySnapshot struct {
	Pools        []PoolRegistration        `json:"pools"`
	Curves       []CurveRegistration       `json:"curves"`
	PendingPools []PendingPoolRegistration `json:"pending_pools"`
}

func NewRegistry() *Registry {
	return &Registry{
		pools:   make(map[common.Hash]PoolRegistration),
		curves:  make(map[common.Address]CurveRegistration),
		pending: make(map[common.Hash]PendingPoolRegistration),
	}
}

// NewOverlayRegistry returns an isolated registry whose misses fall through to
// parent. Discarding it cannot mutate confirmed discovery state.
func NewOverlayRegistry(parent *Registry) *Registry {
	overlay := NewRegistry()
	overlay.parent = parent
	return overlay
}

func NewRegistryFromSnapshot(snapshot RegistrySnapshot) (*Registry, error) {
	registry := NewRegistry()
	for _, registration := range snapshot.Curves {
		if err := registry.RegisterCurve(registration); err != nil {
			return nil, err
		}
	}
	for _, registration := range snapshot.PendingPools {
		if err := registry.RegisterPendingPool(registration); err != nil {
			return nil, err
		}
	}
	for _, registration := range snapshot.Pools {
		if err := registry.RegisterPool(registration); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *Registry) LookupPool(id common.Hash) (PoolRegistration, bool) {
	if r == nil {
		return PoolRegistration{}, false
	}
	r.mu.RLock()
	value, ok := r.pools[id]
	r.mu.RUnlock()
	if ok {
		return value, true
	}
	if r.parent != nil {
		return r.parent.LookupPool(id)
	}
	return PoolRegistration{}, false
}

func (r *Registry) LookupCurve(curve common.Address) (CurveRegistration, bool) {
	if r == nil {
		return CurveRegistration{}, false
	}
	r.mu.RLock()
	value, ok := r.curves[curve]
	r.mu.RUnlock()
	if ok {
		return value, true
	}
	if r.parent != nil {
		return r.parent.LookupCurve(curve)
	}
	return CurveRegistration{}, false
}

func (r *Registry) LookupPendingPool(id common.Hash) (PendingPoolRegistration, bool) {
	if r == nil {
		return PendingPoolRegistration{}, false
	}
	r.mu.RLock()
	if _, ok := r.pools[id]; ok {
		r.mu.RUnlock()
		return PendingPoolRegistration{}, false
	}
	value, ok := r.pending[id]
	r.mu.RUnlock()
	if ok {
		return value, true
	}
	if r.parent != nil {
		return r.parent.LookupPendingPool(id)
	}
	return PendingPoolRegistration{}, false
}

func (r *Registry) RegisterPool(registration PoolRegistration) error {
	if r == nil || !registration.Protocol.Valid() || registration.Token == (common.Address{}) || registration.Token == registration.Quote {
		return ErrInvalidRegistration
	}
	id, err := PoolID(registration.PoolKey)
	if err != nil {
		return err
	}
	if id != registration.PoolID {
		return ErrPoolIDMismatch
	}
	if registration.Token != registration.PoolKey.Currency0 && registration.Token != registration.PoolKey.Currency1 {
		return ErrInvalidRegistration
	}
	if registration.Quote != registration.PoolKey.Currency0 && registration.Quote != registration.PoolKey.Currency1 {
		return ErrInvalidRegistration
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureMapsLocked()
	if prior, ok := r.pools[registration.PoolID]; ok && prior != registration {
		return fmt.Errorf("pool %s: %w", registration.PoolID, ErrConflictingPool)
	}
	if pending, ok := r.pending[registration.PoolID]; ok && !pendingMatchesPool(pending, registration) {
		return fmt.Errorf("pool %s: %w", registration.PoolID, ErrConflictingPool)
	}
	if r.parent != nil {
		if prior, ok := r.parent.LookupPool(registration.PoolID); ok && prior != registration {
			return fmt.Errorf("pool %s: %w", registration.PoolID, ErrConflictingPool)
		}
		if pending, ok := r.parent.LookupPendingPool(registration.PoolID); ok && !pendingMatchesPool(pending, registration) {
			return fmt.Errorf("pool %s: %w", registration.PoolID, ErrConflictingPool)
		}
	}
	r.pools[registration.PoolID] = registration
	delete(r.pending, registration.PoolID)
	return nil
}

func (r *Registry) RegisterCurve(registration CurveRegistration) error {
	if r == nil || registration.Curve == (common.Address{}) || !registration.Protocol.Valid() || registration.Token == (common.Address{}) || registration.Token == registration.Quote {
		return ErrInvalidRegistration
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureMapsLocked()
	if prior, ok := r.curves[registration.Curve]; ok && prior != registration {
		return fmt.Errorf("curve %s: %w", registration.Curve, ErrConflictingCurve)
	}
	if r.parent != nil {
		if prior, ok := r.parent.LookupCurve(registration.Curve); ok && prior != registration {
			return fmt.Errorf("curve %s: %w", registration.Curve, ErrConflictingCurve)
		}
	}
	r.curves[registration.Curve] = registration
	return nil
}

func (r *Registry) RegisterPendingPool(registration PendingPoolRegistration) error {
	if r == nil || registration.PoolID == (common.Hash{}) || !registration.Protocol.Valid() || registration.Token == (common.Address{}) || registration.Token == registration.Quote {
		return ErrInvalidRegistration
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureMapsLocked()
	if pool, ok := r.pools[registration.PoolID]; ok {
		if !pendingMatchesPool(registration, pool) {
			return fmt.Errorf("pool %s: %w", registration.PoolID, ErrConflictingPool)
		}
		return nil
	}
	if prior, ok := r.pending[registration.PoolID]; ok && prior != registration {
		return fmt.Errorf("pending pool %s: %w", registration.PoolID, ErrConflictingPool)
	}
	if r.parent != nil {
		if pool, ok := r.parent.LookupPool(registration.PoolID); ok {
			if !pendingMatchesPool(registration, pool) {
				return fmt.Errorf("pool %s: %w", registration.PoolID, ErrConflictingPool)
			}
			return nil
		}
		if prior, ok := r.parent.LookupPendingPool(registration.PoolID); ok && prior != registration {
			return fmt.Errorf("pending pool %s: %w", registration.PoolID, ErrConflictingPool)
		}
	}
	r.pending[registration.PoolID] = registration
	return nil
}

func pendingMatchesPool(pending PendingPoolRegistration, pool PoolRegistration) bool {
	return pending.Protocol == pool.Protocol && pending.Token == pool.Token && (pending.Quote == (common.Address{}) || pending.Quote == pool.Quote)
}

func (r *Registry) ensureMapsLocked() {
	if r.pools == nil {
		r.pools = make(map[common.Hash]PoolRegistration)
	}
	if r.curves == nil {
		r.curves = make(map[common.Address]CurveRegistration)
	}
	if r.pending == nil {
		r.pending = make(map[common.Hash]PendingPoolRegistration)
	}
}

func (r *Registry) fork() *Registry {
	overlay := NewRegistry()
	overlay.parent = r
	return overlay
}

// commit validates a private overlay completely before publishing any entry.
func (r *Registry) commit(overlay *Registry) error {
	var pools []PoolRegistration
	var curves []CurveRegistration
	var pending []PendingPoolRegistration
	overlay.mu.RLock()
	for _, registration := range overlay.pools {
		pools = append(pools, registration)
	}
	for _, registration := range overlay.curves {
		curves = append(curves, registration)
	}
	for _, registration := range overlay.pending {
		pending = append(pending, registration)
	}
	overlay.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureMapsLocked()
	for _, registration := range pools {
		if prior, ok := r.pools[registration.PoolID]; ok && prior != registration {
			return fmt.Errorf("pool %s: %w", registration.PoolID, ErrConflictingPool)
		}
		if prior, ok := r.pending[registration.PoolID]; ok && !pendingMatchesPool(prior, registration) {
			return fmt.Errorf("pool %s: %w", registration.PoolID, ErrConflictingPool)
		}
	}
	for _, registration := range curves {
		if prior, ok := r.curves[registration.Curve]; ok && prior != registration {
			return fmt.Errorf("curve %s: %w", registration.Curve, ErrConflictingCurve)
		}
	}
	for _, registration := range pending {
		if pool, ok := r.pools[registration.PoolID]; ok && !pendingMatchesPool(registration, pool) {
			return fmt.Errorf("pool %s: %w", registration.PoolID, ErrConflictingPool)
		}
		if prior, ok := r.pending[registration.PoolID]; ok && prior != registration {
			return fmt.Errorf("pending pool %s: %w", registration.PoolID, ErrConflictingPool)
		}
	}
	for _, registration := range curves {
		r.curves[registration.Curve] = registration
	}
	for _, registration := range pools {
		r.pools[registration.PoolID] = registration
		delete(r.pending, registration.PoolID)
	}
	for _, registration := range pending {
		if _, ok := r.pools[registration.PoolID]; !ok {
			r.pending[registration.PoolID] = registration
		}
	}
	return nil
}

func (r *Registry) Pools() []PoolRegistration {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	out := make([]PoolRegistration, 0, len(r.pools))
	for _, registration := range r.pools {
		out = append(out, registration)
	}
	r.mu.RUnlock()
	return out
}

func (r *Registry) Curves() []CurveRegistration {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	out := make([]CurveRegistration, 0, len(r.curves))
	for _, registration := range r.curves {
		out = append(out, registration)
	}
	r.mu.RUnlock()
	return out
}

func (r *Registry) PendingPools() []PendingPoolRegistration {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	out := make([]PendingPoolRegistration, 0, len(r.pending))
	for _, registration := range r.pending {
		if _, registered := r.pools[registration.PoolID]; !registered {
			out = append(out, registration)
		}
	}
	r.mu.RUnlock()
	return out
}

// Snapshot copies all registry tables under one read lock. The returned
// slices are sorted so JSON snapshots and checksums remain reproducible.
func (r *Registry) Snapshot() RegistrySnapshot {
	if r == nil {
		return RegistrySnapshot{}
	}
	r.mu.RLock()
	snapshot := RegistrySnapshot{
		Pools:        make([]PoolRegistration, 0, len(r.pools)),
		Curves:       make([]CurveRegistration, 0, len(r.curves)),
		PendingPools: make([]PendingPoolRegistration, 0, len(r.pending)),
	}
	for _, registration := range r.pools {
		snapshot.Pools = append(snapshot.Pools, registration)
	}
	for _, registration := range r.curves {
		snapshot.Curves = append(snapshot.Curves, registration)
	}
	for _, registration := range r.pending {
		if _, registered := r.pools[registration.PoolID]; !registered {
			snapshot.PendingPools = append(snapshot.PendingPools, registration)
		}
	}
	r.mu.RUnlock()
	sort.Slice(snapshot.Pools, func(i, j int) bool {
		return bytes.Compare(snapshot.Pools[i].PoolID[:], snapshot.Pools[j].PoolID[:]) < 0
	})
	sort.Slice(snapshot.Curves, func(i, j int) bool {
		return bytes.Compare(snapshot.Curves[i].Curve[:], snapshot.Curves[j].Curve[:]) < 0
	})
	sort.Slice(snapshot.PendingPools, func(i, j int) bool {
		return bytes.Compare(snapshot.PendingPools[i].PoolID[:], snapshot.PendingPools[j].PoolID[:]) < 0
	})
	return snapshot
}

// Restore validates the complete snapshot before replacing live state. It is
// intended for startup hydration and fail-closed reorg recovery.
func (r *Registry) Restore(snapshot RegistrySnapshot) error {
	if r == nil {
		return ErrInvalidRegistration
	}
	candidate, err := NewRegistryFromSnapshot(snapshot)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.parent != nil {
		return ErrInvalidRegistration
	}
	r.pools = candidate.pools
	r.curves = candidate.curves
	r.pending = candidate.pending
	return nil
}
