package rbhparser

import (
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

type Parser struct {
	addresses AddressBook
	registry  *Registry
}

func New() *Parser {
	return NewWithRegistry(DefaultAddressBook(), NewRegistry())
}

func NewWithRegistry(addresses AddressBook, registry *Registry) *Parser {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Parser{addresses: addresses, registry: registry}
}

func (p *Parser) Registry() *Registry {
	if p == nil {
		return nil
	}
	return p.registry
}

func (p *Parser) Addresses() AddressBook {
	if p == nil {
		return AddressBook{}
	}
	return p.addresses
}

func (p *Parser) ParseLog(log gethtypes.Log) (Event, bool, error) {
	if p == nil || p.registry == nil || log.Removed || len(log.Topics) == 0 {
		return Event{}, false, nil
	}
	topic := log.Topics[0]
	if log.Address == p.addresses.PoolManager {
		switch topic {
		case topicPoolInitialize:
			initialized, err := decodeInitialize(log)
			if err != nil {
				return Event{}, false, malformed("Initialize", err)
			}
			protocol := ProtocolUnknown
			if registration, ok := p.registry.LookupPool(initialized.PoolID); ok {
				protocol = registration.Protocol
			} else if pending, ok := p.registry.LookupPendingPool(initialized.PoolID); ok {
				registration, registrationErr := poolRegistrationFromPending(pending, initialized)
				if registrationErr != nil {
					return Event{}, false, registrationErr
				}
				if registrationErr = p.registry.RegisterPool(registration); registrationErr != nil {
					return Event{}, false, registrationErr
				}
				protocol = pending.Protocol
			}
			return Event{Kind: EventPoolInitialized, Protocol: protocol, Data: initialized, Log: log}, true, nil
		case topicPoolSwap:
			if len(log.Topics) < 2 {
				return Event{}, false, malformed("Swap", ErrMalformedLog)
			}
			registration, ok := p.registry.LookupPool(log.Topics[1])
			if !ok {
				return Event{}, false, nil
			}
			swap, err := decodeSwap(log, registration)
			if err != nil {
				return Event{}, false, malformed("Swap", err)
			}
			return Event{Kind: EventSwap, Protocol: registration.Protocol, Data: swap, Log: log}, true, nil
		case topicPoolLiquidity:
			modified, err := decodeLiquidityModified(log)
			if err != nil {
				return Event{}, false, malformed("ModifyLiquidity", err)
			}
			protocol := ProtocolUnknown
			if registration, ok := p.registry.LookupPool(modified.PoolID); ok {
				protocol = registration.Protocol
			}
			return Event{Kind: EventLiquidityModified, Protocol: protocol, Data: modified, Log: log}, true, nil
		}
	}

	if log.Address == p.addresses.PonsFactory && topic == topicPonsLaunch {
		launch, err := decodePonsLaunch(log)
		return p.launchEvent(log, ProtocolPonsV2, launch, err)
	}
	if log.Address == p.addresses.PonsMemeHook && topic == topicPonsPool {
		launch, err := decodePonsPool(log)
		return p.launchEvent(log, ProtocolPonsV2, launch, err)
	}
	if log.Address == p.addresses.LongLauncher && topic == topicLongLaunch {
		launch, err := decodeLongLaunch(log)
		return p.launchEvent(log, ProtocolLong, launch, err)
	}
	if log.Address == p.addresses.O1Factory && topic == topicO1Launch {
		launch, err := decodeO1Launch(log)
		return p.launchEvent(log, ProtocolO1, launch, err)
	}
	if log.Address == p.addresses.PoolsEntry && topic == topicPoolsCreated {
		launch, err := decodePoolsLaunch(log)
		return p.launchEvent(log, ProtocolPoolsTrade, launch, err)
	}
	if log.Address == p.addresses.PAIRLaunchpad && topic == topicPAIRPool {
		launch, err := decodePAIRLaunch(log)
		return p.launchEvent(log, ProtocolPAIR, launch, err)
	}
	if log.Address == p.addresses.BagsFactory && topic == topicBagsCreated {
		launch, err := decodeBagsLaunch(log, p.addresses.WETH)
		return p.launchEvent(log, ProtocolBagsV2, launch, err)
	}

	if topic == topicPonsCurveBuy || topic == topicPonsCurveSell || topic == topicBagsCurveBuy || topic == topicBagsCurveSell {
		registration, ok := p.registry.LookupCurve(log.Address)
		if !ok {
			return Event{}, false, nil
		}
		buy := topic == topicPonsCurveBuy || topic == topicBagsCurveBuy
		var trade CurveTrade
		var err error
		switch {
		case registration.Protocol == ProtocolPonsV2 && (topic == topicPonsCurveBuy || topic == topicPonsCurveSell):
			trade, err = decodeCurveTrade(log)
		case registration.Protocol == ProtocolBagsV2 && (topic == topicBagsCurveBuy || topic == topicBagsCurveSell):
			trade, err = decodeBagsCurveTrade(log, buy)
		default:
			return Event{}, false, nil
		}
		if err != nil {
			return Event{}, false, malformed("curve trade", err)
		}
		kind := EventCurveSell
		if buy {
			kind = EventCurveBuy
		}
		return Event{Kind: kind, Protocol: registration.Protocol, Data: trade, Log: log}, true, nil
	}
	return Event{}, false, nil
}

// ParseLogFiltered preserves registry updates while suppressing output that
// does not match filter.
func (p *Parser) ParseLogFiltered(log gethtypes.Log, filter EventFilter) (Event, bool, error) {
	if p == nil || p.registry == nil || log.Removed || len(log.Topics) == 0 {
		return Event{}, false, nil
	}
	topic := log.Topics[0]
	if log.Address == p.addresses.PoolManager && topic == topicPoolSwap {
		if len(log.Topics) < 2 {
			return p.ParseLog(log)
		}
		registration, ok := p.registry.LookupPool(log.Topics[1])
		if !ok || !filter.matchClassification(registration.Protocol, EventSwap) {
			return Event{}, false, nil
		}
	} else if topic == topicPonsCurveBuy || topic == topicPonsCurveSell || topic == topicBagsCurveBuy || topic == topicBagsCurveSell {
		registration, ok := p.registry.LookupCurve(log.Address)
		if !ok {
			return Event{}, false, nil
		}
		kind := EventCurveSell
		if topic == topicPonsCurveBuy || topic == topicBagsCurveBuy {
			kind = EventCurveBuy
		}
		if !filter.matchClassification(registration.Protocol, kind) {
			return Event{}, false, nil
		}
	}
	event, ok, err := p.ParseLog(log)
	if err != nil || !ok || !filter.Match(event) {
		return Event{}, false, err
	}
	return event, true, nil
}

func (p *Parser) launchEvent(log gethtypes.Log, protocol Protocol, launch Launch, err error) (Event, bool, error) {
	if err != nil {
		return Event{}, false, fmt.Errorf("decode %s launch: %w", protocol, err)
	}
	if launch.Protocol != protocol || launch.Token == (common.Address{}) {
		return Event{}, false, fmt.Errorf("decode %s launch: %w", protocol, ErrMalformedLog)
	}
	staged := p.registry.fork()
	if launch.Curve != (common.Address{}) {
		if err := staged.RegisterCurve(CurveRegistration{Curve: launch.Curve, Protocol: launch.Protocol, Token: launch.Token, Quote: launch.Quote}); err != nil {
			return Event{}, false, err
		}
	}
	if launch.PoolID != (common.Hash{}) {
		if err := staged.RegisterPendingPool(PendingPoolRegistration{PoolID: launch.PoolID, Protocol: launch.Protocol, Token: launch.Token, Quote: launch.Quote}); err != nil {
			return Event{}, false, err
		}
	}
	if err := p.registry.commit(staged); err != nil {
		return Event{}, false, err
	}
	return Event{Kind: EventLaunch, Protocol: launch.Protocol, Data: launch, Log: log}, true, nil
}

func (p *Parser) ParseReceipt(receipt *gethtypes.Receipt) ([]Event, error) {
	if p == nil || p.registry == nil {
		return nil, ErrInvalidRegistration
	}
	if receipt == nil {
		return nil, fmt.Errorf("nil receipt")
	}
	staged := p.registry.fork()
	stagedParser := &Parser{addresses: p.addresses, registry: staged}
	events, err := stagedParser.parseReceipt(receipt)
	if err != nil {
		return nil, err
	}
	if err := p.registry.commit(staged); err != nil {
		return nil, err
	}
	return events, nil
}

// ParseReceiptFiltered filters output after the transactional registry update,
// so hiding launch events cannot break later pool and curve attribution.
func (p *Parser) ParseReceiptFiltered(receipt *gethtypes.Receipt, filter EventFilter) ([]Event, error) {
	events, err := p.ParseReceipt(receipt)
	if err != nil {
		return nil, err
	}
	write := 0
	for _, event := range events {
		if filter.Match(event) {
			events[write] = event
			write++
		}
	}
	clear(events[write:])
	return events[:write], nil
}

func (p *Parser) parseReceipt(receipt *gethtypes.Receipt) ([]Event, error) {
	events := make([]Event, 0, len(receipt.Logs))
	launchIndexes := make([]int, 0, 4)
	initializations := make(map[common.Hash]PoolInitialized)
	initializationIndexes := make(map[common.Hash]int)

	for _, logPtr := range receipt.Logs {
		if logPtr == nil || logPtr.Removed {
			continue
		}
		if isDeferredReceiptTrade(logPtr, p.addresses.PoolManager) {
			continue
		}
		event, ok, err := p.ParseLog(*logPtr)
		if err != nil {
			return nil, fmt.Errorf("log %d: %w", logPtr.Index, err)
		}
		if !ok {
			continue
		}
		if event.Kind == EventLaunch {
			launchIndexes = append(launchIndexes, len(events))
		}
		if event.Kind == EventPoolInitialized {
			initialized := event.Data.(PoolInitialized)
			initializations[initialized.PoolID] = initialized
			initializationIndexes[initialized.PoolID] = len(events)
		}
		events = append(events, event)
	}

	for _, index := range launchIndexes {
		launch := events[index].Data.(Launch)
		initialized, found := matchInitialization(launch, initializations)
		if !found {
			continue
		}
		launch.PoolID = initialized.PoolID
		poolKey := initialized.PoolKey
		launch.Pool = &poolKey
		if launch.Quote == (common.Address{}) {
			if launch.Token == poolKey.Currency0 {
				launch.Quote = poolKey.Currency1
			} else {
				launch.Quote = poolKey.Currency0
			}
		}
		registration := PoolRegistration{PoolID: launch.PoolID, Protocol: launch.Protocol, Token: launch.Token, Quote: launch.Quote, PoolKey: poolKey}
		if err := p.registry.RegisterPool(registration); err != nil {
			return nil, err
		}
		events[index].Data = launch
		if eventIndex, ok := initializationIndexes[launch.PoolID]; ok {
			events[eventIndex].Protocol = launch.Protocol
		}
	}

	for poolID, initialized := range initializations {
		if _, exists := p.registry.LookupPool(poolID); exists {
			continue
		}
		pending, found := p.registry.LookupPendingPool(poolID)
		if !found {
			continue
		}
		registration, err := poolRegistrationFromPending(pending, initialized)
		if err != nil {
			return nil, err
		}
		if err := p.registry.RegisterPool(registration); err != nil {
			return nil, err
		}
		if eventIndex, ok := initializationIndexes[poolID]; ok {
			events[eventIndex].Protocol = pending.Protocol
		}
	}

	for index := range events {
		if events[index].Kind != EventLiquidityModified {
			continue
		}
		modified := events[index].Data.(LiquidityModified)
		if registration, ok := p.registry.LookupPool(modified.PoolID); ok {
			events[index].Protocol = registration.Protocol
		}
	}

	// Trades can be emitted before the launch contract publishes the event that
	// identifies their newly-created curve or pool. Decode them only after all
	// registrations from the receipt have been staged.
	for _, logPtr := range receipt.Logs {
		if !isDeferredReceiptTrade(logPtr, p.addresses.PoolManager) {
			continue
		}
		event, ok, err := p.ParseLog(*logPtr)
		if err != nil {
			return nil, fmt.Errorf("log %d: %w", logPtr.Index, err)
		}
		if ok {
			events = append(events, event)
		}
	}

	sort.SliceStable(events, func(i, j int) bool { return events[i].Log.Index < events[j].Log.Index })
	return events, nil
}

func isDeferredReceiptTrade(log *gethtypes.Log, poolManager common.Address) bool {
	if log == nil || log.Removed || len(log.Topics) == 0 {
		return false
	}
	topic := log.Topics[0]
	if log.Address == poolManager && topic == topicPoolSwap {
		return true
	}
	return topic == topicPonsCurveBuy || topic == topicPonsCurveSell || topic == topicBagsCurveBuy || topic == topicBagsCurveSell
}

func poolRegistrationFromPending(pending PendingPoolRegistration, initialized PoolInitialized) (PoolRegistration, error) {
	if pending.Token != initialized.PoolKey.Currency0 && pending.Token != initialized.PoolKey.Currency1 {
		return PoolRegistration{}, ErrInvalidRegistration
	}
	quote := pending.Quote
	if quote == (common.Address{}) {
		if pending.Token == initialized.PoolKey.Currency0 {
			quote = initialized.PoolKey.Currency1
		} else {
			quote = initialized.PoolKey.Currency0
		}
	}
	return PoolRegistration{PoolID: initialized.PoolID, Protocol: pending.Protocol, Token: pending.Token, Quote: quote, PoolKey: initialized.PoolKey}, nil
}

func matchInitialization(launch Launch, candidates map[common.Hash]PoolInitialized) (PoolInitialized, bool) {
	if launch.PoolID != (common.Hash{}) {
		initialized, ok := candidates[launch.PoolID]
		return initialized, ok
	}
	var match PoolInitialized
	found := false
	for _, initialized := range candidates {
		key := initialized.PoolKey
		if launch.Token != key.Currency0 && launch.Token != key.Currency1 {
			continue
		}
		if launch.Quote != (common.Address{}) && launch.Quote != key.Currency0 && launch.Quote != key.Currency1 {
			continue
		}
		if found {
			return PoolInitialized{}, false
		}
		match, found = initialized, true
	}
	return match, found
}
