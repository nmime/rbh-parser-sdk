package rbhparser

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

func abiArguments(t testing.TB, names ...string) abi.Arguments {
	t.Helper()
	args := make(abi.Arguments, len(names))
	for i, name := range names {
		typ, err := abi.NewType(name, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		args[i] = abi.Argument{Type: typ}
	}
	return args
}

func addressTopic(address common.Address) common.Hash { return common.BytesToHash(address.Bytes()) }

func TestNilParserPublicMethods(t *testing.T) {
	var parser *Parser
	if parser.Registry() != nil {
		t.Fatal("nil parser returned a registry")
	}
	if parser.Addresses() != (AddressBook{}) {
		t.Fatal("nil parser returned nonzero addresses")
	}
	if _, ok, err := parser.ParseLog(gethtypes.Log{Topics: []common.Hash{TopicPoolSwap}}); err != nil || ok {
		t.Fatalf("nil parser ParseLog: ok=%v err=%v", ok, err)
	}
	if _, err := parser.ParseReceipt(&gethtypes.Receipt{}); !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("nil parser ParseReceipt error = %v", err)
	}

	parser = &Parser{}
	if _, ok, err := parser.ParseLog(gethtypes.Log{Topics: []common.Hash{TopicPoolSwap}}); err != nil || ok {
		t.Fatalf("registry-less parser ParseLog: ok=%v err=%v", ok, err)
	}
	if _, err := parser.ParseReceipt(&gethtypes.Receipt{}); !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("registry-less parser ParseReceipt error = %v", err)
	}
}

func makeInitializeLog(t testing.TB, key PoolKey, index uint) gethtypes.Log {
	t.Helper()
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	data, err := abiArguments(t, "uint24", "int24", "address", "uint160", "int24").Pack(new(big.Int).SetUint64(uint64(key.Fee)), big.NewInt(int64(key.TickSpacing)), key.Hooks, big.NewInt(1).Lsh(big.NewInt(1), 96), big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	return gethtypes.Log{Address: DefaultAddressBook().PoolManager, Topics: []common.Hash{TopicPoolInitialize, id, addressTopic(key.Currency0), addressTopic(key.Currency1)}, Data: data, Index: index}
}

func makeSwapLog(t testing.TB, key PoolKey, index uint) gethtypes.Log {
	t.Helper()
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	data, err := abiArguments(t, "int128", "int128", "uint160", "uint128", "int24", "uint24").Pack(big.NewInt(100), big.NewInt(-50), big.NewInt(1).Lsh(big.NewInt(1), 96), big.NewInt(1000), big.NewInt(-4), big.NewInt(3000))
	if err != nil {
		t.Fatal(err)
	}
	return gethtypes.Log{Address: DefaultAddressBook().PoolManager, Topics: []common.Hash{TopicPoolSwap, id, addressTopic(common.HexToAddress("0x44"))}, Data: data, Index: index}
}

func makeBagsLaunchLog(t testing.TB, token, curve common.Address, poolID common.Hash, index uint) gethtypes.Log {
	t.Helper()
	data, err := bagsEventABI.Events["TokenCreated"].Inputs.NonIndexed().Pack(
		common.HexToAddress("0x61"),
		common.HexToAddress("0x62"),
		poolID,
		"Bags Test",
		"BAGS",
		"ipfs://metadata",
	)
	if err != nil {
		t.Fatal(err)
	}
	return gethtypes.Log{
		Address: DefaultAddressBook().BagsFactory,
		Topics: []common.Hash{
			TopicBagsCreated,
			addressTopic(token),
			addressTopic(curve),
			addressTopic(common.HexToAddress("0x63")),
		},
		Data:  data,
		Index: index,
	}
}

func TestParseLongLaunchInitializeAndFirstSwap(t *testing.T) {
	parser := New()
	addresses := DefaultAddressBook()
	quote := common.HexToAddress("0x10")
	token := common.HexToAddress("0x20")
	key := PoolKey{Currency0: quote, Currency1: token, Fee: 0x800000, TickSpacing: 8, Hooks: common.HexToAddress("0x4e3468951D49f2EEa976eD0D6e75fFCb44a9a544")}
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	longData, err := longEventABI.Events["LaunchCreated"].Inputs.NonIndexed().Pack(key.Hooks, common.HexToAddress("0x55"), [32]byte{1}, big.NewInt(100), big.NewInt(86500), "LONG")
	if err != nil {
		t.Fatal(err)
	}
	launchLog := gethtypes.Log{Address: addresses.LongLauncher, Topics: []common.Hash{TopicLongLaunch, addressTopic(token), addressTopic(token), addressTopic(quote)}, Data: longData, Index: 1}
	initializeLog := makeInitializeLog(t, key, 2)
	swapLog := makeSwapLog(t, key, 3)
	receipt := &gethtypes.Receipt{Logs: []*gethtypes.Log{&launchLog, &initializeLog, &swapLog}}
	events, err := parser.ParseReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3", len(events))
	}
	launch := events[0].Data.(Launch)
	if launch.Protocol != ProtocolLong || launch.Token != token || launch.Quote != quote || launch.Pool == nil || launch.PoolID != id || launch.Pool.Hooks != key.Hooks {
		t.Fatalf("unexpected launch: %#v", launch)
	}
	if events[1].Protocol != ProtocolLong || events[2].Protocol != ProtocolLong || events[2].Kind != EventSwap {
		t.Fatalf("protocol association failed: %#v", events)
	}
	registration, ok := parser.Registry().LookupPool(id)
	if !ok || registration.Protocol != ProtocolLong || registration.Token != token {
		t.Fatalf("pool was not registered: %#v", registration)
	}
}

func TestLongTopicFromWrongEmitterIsIgnored(t *testing.T) {
	log := gethtypes.Log{Address: common.HexToAddress("0xdead"), Topics: []common.Hash{TopicLongLaunch}}
	if _, ok, err := New().ParseLog(log); err != nil || ok {
		t.Fatalf("wrong emitter accepted: ok=%v err=%v", ok, err)
	}
}

func TestLongLaunchRejectsNonCanonicalData(t *testing.T) {
	arguments := longEventABI.Events["LaunchCreated"].Inputs.NonIndexed()
	data, err := arguments.Pack(
		common.HexToAddress("0x41"),
		common.HexToAddress("0x42"),
		[32]byte{1},
		big.NewInt(100),
		big.NewInt(200),
		"LONG",
	)
	if err != nil {
		t.Fatal(err)
	}
	makeLog := func(data []byte) gethtypes.Log {
		return gethtypes.Log{
			Address: DefaultAddressBook().LongLauncher,
			Topics: []common.Hash{
				TopicLongLaunch,
				addressTopic(common.HexToAddress("0x43")),
				addressTopic(common.HexToAddress("0x43")),
				addressTopic(common.HexToAddress("0x44")),
			},
			Data: data,
		}
	}

	for _, wordIndex := range []int{0, 1} {
		t.Run(fmt.Sprintf("address-word-%d", wordIndex), func(t *testing.T) {
			malformed := append([]byte(nil), data...)
			malformed[wordIndex*32] = 1
			if _, _, err := New().ParseLog(makeLog(malformed)); !errors.Is(err, ErrMalformedLog) {
				t.Fatalf("non-canonical address word error = %v", err)
			}
		})
	}

	trailing := append(append([]byte(nil), data...), make([]byte, 32)...)
	if _, _, err := New().ParseLog(makeLog(trailing)); !errors.Is(err, ErrMalformedLog) {
		t.Fatalf("trailing launch data error = %v", err)
	}
}

func TestPonsCurveRequiresRegisteredEmitter(t *testing.T) {
	parser := New()
	curve := common.HexToAddress("0x30")
	token := common.HexToAddress("0x31")
	actor := common.HexToAddress("0x32")
	recipient := common.HexToAddress("0x33")
	data, err := abiArguments(t, "uint256", "uint256", "uint256", "uint256").Pack(big.NewInt(100), big.NewInt(90), big.NewInt(5), big.NewInt(5))
	if err != nil {
		t.Fatal(err)
	}
	log := gethtypes.Log{Address: curve, Topics: []common.Hash{TopicPonsCurveBuy, addressTopic(actor), addressTopic(recipient)}, Data: data}
	if _, ok, err := parser.ParseLog(log); err != nil || ok {
		t.Fatalf("unregistered curve accepted: ok=%v err=%v", ok, err)
	}
	if err := parser.Registry().RegisterCurve(CurveRegistration{Curve: curve, Protocol: ProtocolPonsV2, Token: token, Quote: common.Address{}}); err != nil {
		t.Fatal(err)
	}
	event, ok, err := parser.ParseLog(log)
	if err != nil || !ok || event.Kind != EventCurveBuy {
		t.Fatalf("registered curve not parsed: %#v ok=%v err=%v", event, ok, err)
	}
}

func TestParseReceiptRegistersCurveBeforeEarlierTrade(t *testing.T) {
	parser := New()
	addresses := DefaultAddressBook()
	token := common.HexToAddress("0x21")
	curve := common.HexToAddress("0x22")
	creator := common.HexToAddress("0x23")
	actor := common.HexToAddress("0x24")
	recipient := common.HexToAddress("0x25")

	tradeData, err := abiArguments(t, "uint256", "uint256", "uint256", "uint256").Pack(big.NewInt(100), big.NewInt(90), big.NewInt(5), big.NewInt(5))
	if err != nil {
		t.Fatal(err)
	}
	tradeLog := gethtypes.Log{
		Address: curve,
		Topics:  []common.Hash{TopicPonsCurveBuy, addressTopic(actor), addressTopic(recipient)},
		Data:    tradeData,
		Index:   1,
	}
	launchData, err := abiArguments(t, "address", "uint256", "uint256").Pack(common.Address{}, big.NewInt(7), big.NewInt(1_000))
	if err != nil {
		t.Fatal(err)
	}
	launchLog := gethtypes.Log{
		Address: addresses.PonsFactory,
		Topics:  []common.Hash{TopicPonsLaunch, addressTopic(token), addressTopic(curve), addressTopic(creator)},
		Data:    launchData,
		Index:   2,
	}

	events, err := parser.ParseReceipt(&gethtypes.Receipt{Logs: []*gethtypes.Log{&tradeLog, &launchLog}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Kind != EventCurveBuy || events[0].Protocol != ProtocolPonsV2 || events[1].Kind != EventLaunch {
		t.Fatalf("unexpected events: %#v", events)
	}
	trade, ok := events[0].Data.(CurveTrade)
	if !ok || trade.AmountIn.Cmp(big.NewInt(100)) != 0 || trade.AmountOut.Cmp(big.NewInt(90)) != 0 {
		t.Fatalf("unexpected curve trade: %#v", events[0].Data)
	}
}

func TestParseReceiptRegistersBagsCurveBeforeEarlierTrade(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0x31")
	curve := common.HexToAddress("0x32")
	poolID := common.HexToHash("0x33")

	tradeData, err := abiArguments(t, "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256").Pack(
		big.NewInt(100), big.NewInt(90), big.NewInt(1_000), big.NewInt(10), big.NewInt(5), big.NewInt(5), new(big.Int), big.NewInt(20), big.NewInt(30), big.NewInt(40),
	)
	if err != nil {
		t.Fatal(err)
	}
	tradeLog := gethtypes.Log{
		Address: curve,
		Topics:  []common.Hash{TopicBagsCurveBuy, addressTopic(common.HexToAddress("0x34")), addressTopic(common.HexToAddress("0x35"))},
		Data:    tradeData,
		Index:   1,
	}
	launchLog := makeBagsLaunchLog(t, token, curve, poolID, 2)

	events, err := parser.ParseReceipt(&gethtypes.Receipt{Logs: []*gethtypes.Log{&tradeLog, &launchLog}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Kind != EventCurveBuy || events[0].Protocol != ProtocolBagsV2 || events[1].Kind != EventLaunch {
		t.Fatalf("unexpected events: %#v", events)
	}
	trade, ok := events[0].Data.(CurveTrade)
	if !ok || trade.AmountIn.Cmp(big.NewInt(100)) != 0 || trade.AmountOut.Cmp(big.NewInt(1_000)) != 0 || trade.Bags == nil {
		t.Fatalf("unexpected Bags curve trade: %#v", events[0].Data)
	}
}

func TestMalformedInitializeRejected(t *testing.T) {
	log := gethtypes.Log{Address: DefaultAddressBook().PoolManager, Topics: []common.Hash{TopicPoolInitialize}, Data: make([]byte, 160)}
	if _, _, err := New().ParseLog(log); err == nil {
		t.Fatal("malformed Initialize accepted")
	}
}

func TestRegistryConcurrentReadsAndIdempotentWrite(t *testing.T) {
	registry := NewRegistry()
	key := PoolKey{Currency0: common.Address{}, Currency1: common.HexToAddress("0x40"), Fee: 3000, TickSpacing: 60}
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	registration := PoolRegistration{PoolID: id, Protocol: ProtocolPoolsTrade, Token: key.Currency1, Quote: key.Currency0, PoolKey: key}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				registry.LookupPool(id)
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := registry.RegisterPool(registration); err != nil {
				t.Errorf("idempotent registration: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestRegistrySnapshotRestoreIsValidatedAndDeterministic(t *testing.T) {
	registry := NewRegistry()
	curve := CurveRegistration{Curve: common.HexToAddress("0x22"), Protocol: ProtocolPonsV2, Token: common.HexToAddress("0x33")}
	if err := registry.RegisterCurve(curve); err != nil {
		t.Fatal(err)
	}
	key := PoolKey{Currency0: common.Address{}, Currency1: common.HexToAddress("0x44"), Fee: 3000, TickSpacing: 60}
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	pool := PoolRegistration{PoolID: id, Protocol: ProtocolPoolsTrade, Token: key.Currency1, Quote: key.Currency0, PoolKey: key}
	if err := registry.RegisterPool(pool); err != nil {
		t.Fatal(err)
	}
	snapshot := registry.Snapshot()
	restored, err := NewRegistryFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if got := restored.Snapshot(); !reflect.DeepEqual(got, snapshot) {
		t.Fatalf("restored snapshot differs: got=%#v want=%#v", got, snapshot)
	}
	bad := snapshot
	bad.Pools = append([]PoolRegistration(nil), snapshot.Pools...)
	bad.Pools[0].PoolID = common.HexToHash("0xdead")
	if err := registry.Restore(bad); !errors.Is(err, ErrPoolIDMismatch) {
		t.Fatalf("invalid restore error = %v", err)
	}
	if got := registry.Snapshot(); !reflect.DeepEqual(got, snapshot) {
		t.Fatal("failed restore changed live registry")
	}
}

func TestPendingPoolConnectsLaterInitializeAndSwap(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0x50")
	quote := common.HexToAddress("0x40")
	key := PoolKey{Currency0: quote, Currency1: token, Fee: 0x800000, TickSpacing: 8, Hooks: DefaultAddressBook().BagsHook}
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := parser.Registry().RegisterPendingPool(PendingPoolRegistration{PoolID: id, Protocol: ProtocolBagsV2, Token: token, Quote: quote}); err != nil {
		t.Fatal(err)
	}
	initializeLog := makeInitializeLog(t, key, 1)
	swapLog := makeSwapLog(t, key, 2)
	events, err := parser.ParseReceipt(&gethtypes.Receipt{Logs: []*gethtypes.Log{&initializeLog, &swapLog}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Protocol != ProtocolBagsV2 || events[1].Kind != EventSwap || events[1].Protocol != ProtocolBagsV2 {
		t.Fatalf("pending pool was not connected: %#v", events)
	}
}

func TestStreamingBagsLaunchRegistersAndClearsPendingPool(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0xf000000000000000000000000000000000000001")
	quote := DefaultAddressBook().WETH
	curve := common.HexToAddress("0x70")
	key := PoolKey{Currency0: quote, Currency1: token, Fee: 0x800000, TickSpacing: 8, Hooks: DefaultAddressBook().BagsHook}
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	launchLog := makeBagsLaunchLog(t, token, curve, id, 1)
	event, ok, err := parser.ParseLog(launchLog)
	if err != nil || !ok || event.Protocol != ProtocolBagsV2 {
		t.Fatalf("parse launch: event=%#v ok=%v err=%v", event, ok, err)
	}
	if _, ok := parser.Registry().LookupCurve(curve); !ok {
		t.Fatal("curve was not registered")
	}
	if _, ok := parser.Registry().LookupPendingPool(id); !ok {
		t.Fatal("pending pool was not registered")
	}
	if _, ok, err = parser.ParseLog(makeInitializeLog(t, key, 2)); err != nil || !ok {
		t.Fatalf("parse initialize: ok=%v err=%v", ok, err)
	}
	if _, ok := parser.Registry().LookupPool(id); !ok {
		t.Fatal("pool was not registered")
	}
	if _, ok := parser.Registry().LookupPendingPool(id); ok {
		t.Fatal("registered pool remained pending")
	}
	if _, ok, err = parser.ParseLog(launchLog); err != nil || !ok {
		t.Fatalf("replay launch: ok=%v err=%v", ok, err)
	}
	if _, ok := parser.Registry().LookupPendingPool(id); ok {
		t.Fatal("launch replay recreated pending pool")
	}
}

func TestRegistryRejectsUnknownProtocol(t *testing.T) {
	registry := NewRegistry()
	err := registry.RegisterCurve(CurveRegistration{
		Curve:    common.HexToAddress("0x71"),
		Protocol: Protocol(255),
		Token:    common.HexToAddress("0x72"),
	})
	if err == nil {
		t.Fatal("unknown protocol was accepted")
	}
}

func TestRegistryConcurrentConflictHasSingleWinner(t *testing.T) {
	registry := NewRegistry()
	key := PoolKey{Currency0: common.Address{}, Currency1: common.HexToAddress("0x73"), Fee: 3000, TickSpacing: 60}
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for _, protocol := range []Protocol{ProtocolPonsV2, ProtocolPoolsTrade} {
		go func() {
			results <- registry.RegisterPool(PoolRegistration{PoolID: id, Protocol: protocol, Token: key.Currency1, Quote: key.Currency0, PoolKey: key})
		}()
	}
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("want one winner and one conflict, got %v and %v", first, second)
	}
	if first != nil && !errors.Is(first, ErrConflictingPool) || second != nil && !errors.Is(second, ErrConflictingPool) {
		t.Fatalf("unexpected conflict errors: %v and %v", first, second)
	}
	if len(registry.Pools()) != 1 {
		t.Fatalf("pool count = %d, want 1", len(registry.Pools()))
	}
}

func TestStreamingPonsLaunchRegistersCurve(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0x81")
	curve := common.HexToAddress("0x82")
	creator := common.HexToAddress("0x83")
	data, err := abiArguments(t, "address", "uint256", "uint256").Pack(common.Address{}, big.NewInt(2), big.NewInt(3))
	if err != nil {
		t.Fatal(err)
	}
	log := gethtypes.Log{Address: DefaultAddressBook().PonsFactory, Topics: []common.Hash{TopicPonsLaunch, addressTopic(token), addressTopic(curve), addressTopic(creator)}, Data: data}
	event, ok, err := parser.ParseLog(log)
	if err != nil || !ok || event.Protocol != ProtocolPonsV2 {
		t.Fatalf("parse launch: event=%#v ok=%v err=%v", event, ok, err)
	}
	registration, ok := parser.Registry().LookupCurve(curve)
	if !ok || registration.Token != token || registration.Protocol != ProtocolPonsV2 {
		t.Fatalf("curve registration = %#v, ok=%v", registration, ok)
	}
}

func TestPonsLaunchRejectsZeroCurve(t *testing.T) {
	data, err := abiArguments(t, "address", "uint256", "uint256").Pack(common.Address{}, new(big.Int), big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	log := gethtypes.Log{
		Address: DefaultAddressBook().PonsFactory,
		Topics: []common.Hash{
			TopicPonsLaunch,
			addressTopic(common.HexToAddress("0x84")),
			common.Hash{},
			addressTopic(common.HexToAddress("0x85")),
		},
		Data: data,
	}
	if _, _, err := New().ParseLog(log); !errors.Is(err, ErrMalformedLog) {
		t.Fatalf("error = %v, want ErrMalformedLog", err)
	}
}

func TestMalformedBagsLaunchDoesNotPolluteRegistry(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0x91")
	curve := common.HexToAddress("0x92")
	poolID := common.HexToHash("0x93")
	log := makeBagsLaunchLog(t, token, curve, poolID, 1)
	for i := 96; i < 128; i++ {
		log.Data[i] = 0xff
	}
	if _, _, err := parser.ParseLog(log); err == nil {
		t.Fatal("malformed dynamic Bags data accepted")
	}
	if _, ok := parser.Registry().LookupCurve(curve); ok {
		t.Fatal("malformed launch registered curve")
	}
	if _, ok := parser.Registry().LookupPendingPool(poolID); ok {
		t.Fatal("malformed launch registered pending pool")
	}
}

func TestBagsLaunchRejectsNonCanonicalAddressWords(t *testing.T) {
	for _, wordIndex := range []int{0, 1} {
		t.Run(fmt.Sprintf("word-%d", wordIndex), func(t *testing.T) {
			log := makeBagsLaunchLog(t, common.HexToAddress("0xa8"), common.HexToAddress("0xa9"), common.HexToHash("0xaa"), 1)
			log.Data[wordIndex*32] = 1
			if _, _, err := New().ParseLog(log); !errors.Is(err, ErrMalformedLog) {
				t.Fatalf("non-canonical address word error = %v", err)
			}
		})
	}
}

func TestBagsLaunchRejectsTrailingData(t *testing.T) {
	log := makeBagsLaunchLog(t, common.HexToAddress("0xab"), common.HexToAddress("0xac"), common.HexToHash("0xad"), 1)
	log.Data = append(log.Data, make([]byte, 32)...)
	if _, _, err := New().ParseLog(log); !errors.Is(err, ErrMalformedLog) {
		t.Fatalf("trailing launch data error = %v", err)
	}
}

func TestConflictingBagsLaunchDoesNotPartiallyRegisterCurve(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0xa1")
	curve := common.HexToAddress("0xa2")
	poolID := common.HexToHash("0xa3")
	if err := parser.Registry().RegisterPendingPool(PendingPoolRegistration{PoolID: poolID, Protocol: ProtocolPAIR, Token: token}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := parser.ParseLog(makeBagsLaunchLog(t, token, curve, poolID, 1)); !errors.Is(err, ErrConflictingPool) {
		t.Fatalf("error = %v, want ErrConflictingPool", err)
	}
	if _, ok := parser.Registry().LookupCurve(curve); ok {
		t.Fatal("failed launch partially registered curve")
	}
}

func TestFailedReceiptDoesNotPolluteRegistry(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0xb1")
	curve := common.HexToAddress("0xb2")
	creator := common.HexToAddress("0xb3")
	data, err := abiArguments(t, "address", "uint256", "uint256").Pack(common.Address{}, big.NewInt(1), big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	launch := gethtypes.Log{Address: DefaultAddressBook().PonsFactory, Topics: []common.Hash{TopicPonsLaunch, addressTopic(token), addressTopic(curve), addressTopic(creator)}, Data: data, Index: 1}
	malformedInitialize := gethtypes.Log{Address: DefaultAddressBook().PoolManager, Topics: []common.Hash{TopicPoolInitialize}, Index: 2}
	if _, err := parser.ParseReceipt(&gethtypes.Receipt{Logs: []*gethtypes.Log{&launch, &malformedInitialize}}); err == nil {
		t.Fatal("malformed receipt was accepted")
	}
	if _, ok := parser.Registry().LookupCurve(curve); ok {
		t.Fatal("failed receipt polluted curve registry")
	}
}

func TestFailedDeferredTradeDoesNotPolluteRegistry(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0xb4")
	curve := common.HexToAddress("0xb5")
	creator := common.HexToAddress("0xb6")
	launchData, err := abiArguments(t, "address", "uint256", "uint256").Pack(common.Address{}, big.NewInt(1), big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	malformedTrade := gethtypes.Log{
		Address: curve,
		Topics:  []common.Hash{TopicPonsCurveBuy, addressTopic(common.HexToAddress("0xb7")), addressTopic(common.HexToAddress("0xb8"))},
		Index:   1,
	}
	launch := gethtypes.Log{
		Address: DefaultAddressBook().PonsFactory,
		Topics:  []common.Hash{TopicPonsLaunch, addressTopic(token), addressTopic(curve), addressTopic(creator)},
		Data:    launchData,
		Index:   2,
	}
	if _, err := parser.ParseReceipt(&gethtypes.Receipt{Logs: []*gethtypes.Log{&malformedTrade, &launch}}); err == nil {
		t.Fatal("malformed deferred trade was accepted")
	}
	if _, ok := parser.Registry().LookupCurve(curve); ok {
		t.Fatal("failed deferred trade polluted curve registry")
	}
}

func TestConcurrentPendingAndPoolConflictLeavesConsistentWinner(t *testing.T) {
	registry := NewRegistry()
	key := PoolKey{Currency0: common.Address{}, Currency1: common.HexToAddress("0xc1"), Fee: 3000, TickSpacing: 60}
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	pool := PoolRegistration{PoolID: id, Protocol: ProtocolPoolsTrade, Token: key.Currency1, Quote: key.Currency0, PoolKey: key}
	pending := PendingPoolRegistration{PoolID: id, Protocol: ProtocolPAIR, Token: key.Currency1, Quote: key.Currency0}
	results := make(chan error, 2)
	go func() { results <- registry.RegisterPool(pool) }()
	go func() { results <- registry.RegisterPendingPool(pending) }()
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("want one winner and one conflict, got %v and %v", first, second)
	}
	_, hasPool := registry.LookupPool(id)
	_, hasPending := registry.LookupPendingPool(id)
	if hasPool == hasPending {
		t.Fatalf("inconsistent state: pool=%v pending=%v", hasPool, hasPending)
	}
}

func TestRealBagsCurveTradeFixtures(t *testing.T) {
	const actor = "0x9689992f5b5c09447f15906d8d11214944488341"
	curve := common.HexToAddress("0x0c68fe19e53a72986fa9663cadbc6334199efebc")
	parser := New()
	if err := parser.Registry().RegisterCurve(CurveRegistration{Curve: curve, Protocol: ProtocolBagsV2, Token: common.HexToAddress("0xdb89f57b36e2778d1c7a42f8cda4a5bf496e76ae"), Quote: DefaultAddressBook().WETH}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		topic     common.Hash
		data      string
		kind      EventKind
		amountIn  string
		amountOut string
		fee       string
	}{
		{"buy", TopicBagsCurveBuy, "0x00000000000000000000000000000000000000000000000000232bff5f46c000000000000000000000000000000000000000000000000000002277eae79c60000000000000000000000000000000000000000000000674a4f98dc41feaba987b0000000000000000000000000000000000000000000000000000b41477aa600000000000000000000000000000000000000000000000000000005a0a3bd5300000000000000000000000000000000000000000000000000000005a0a3bd530000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004aa7cd6100000000000000000000000000000000000000000358f20950a2170fd4b202ac0000000000000000000000000000000000000000000000001201eeda08e2fb2d", EventCurveBuy, "232bff5f46c000", "674a4f98dc41feaba987b", "b41477aa6000"},
		{"sell", TopicBagsCurveSell, "0x0000000000000000000000000000000000000000000674a4f98dc41feaba987b000000000000000000000000000000000000000000000000002277eae79c5fff0000000000000000000000000000000000000000000000000021c7707256b0000000000000000000000000000000000000000000000000000000b07a7545afff0000000000000000000000000000000000000000000000000000583d3aa2d7ff0000000000000000000000000000000000000000000000000000583d3aa2d80000000000000000000000000000000000000000000000000000000000498b12ba0000000000000000000000000000000000000000035f66ae4a2fdb2fbf6c9b2700000000000000000000000000000000000000000000000011df76ef21469b2e", EventCurveSell, "674a4f98dc41feaba987b", "21c7707256b000", "b07a7545afff"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := gethtypes.Log{Address: curve, Topics: []common.Hash{test.topic, addressTopic(common.HexToAddress(actor)), addressTopic(common.HexToAddress(actor))}, Data: common.FromHex(test.data)}
			event, ok, err := parser.ParseLog(log)
			if err != nil || !ok || event.Kind != test.kind {
				t.Fatalf("parse fixture: event=%#v ok=%v err=%v", event, ok, err)
			}
			trade := event.Data.(CurveTrade)
			if trade.AmountIn.Text(16) != test.amountIn || trade.AmountOut.Text(16) != test.amountOut || trade.Fee.Text(16) != test.fee {
				t.Fatalf("unexpected trade: %#v", trade)
			}
			if trade.Bags == nil || trade.Bags.VirtualTokenReserve.Sign() <= 0 || trade.Bags.VirtualQuoteReserve.Sign() <= 0 {
				t.Fatalf("missing Bags trade details: %#v", trade.Bags)
			}
		})
	}
}

func TestBagsCurveBuyUsesConsumedInputAfterRefund(t *testing.T) {
	curve := common.HexToAddress("0xd1")
	parser := New()
	if err := parser.Registry().RegisterCurve(CurveRegistration{Curve: curve, Protocol: ProtocolBagsV2, Token: common.HexToAddress("0xd2"), Quote: DefaultAddressBook().WETH}); err != nil {
		t.Fatal(err)
	}
	data, err := abiArguments(t, "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256").Pack(
		big.NewInt(100), big.NewInt(80), big.NewInt(1_000), big.NewInt(10), big.NewInt(5), big.NewInt(5), big.NewInt(10), big.NewInt(20), big.NewInt(30), big.NewInt(40),
	)
	if err != nil {
		t.Fatal(err)
	}
	log := gethtypes.Log{Address: curve, Topics: []common.Hash{TopicBagsCurveBuy, addressTopic(common.HexToAddress("0xd3")), addressTopic(common.HexToAddress("0xd4"))}, Data: data}
	event, ok, err := parser.ParseLog(log)
	if err != nil || !ok {
		t.Fatalf("parse Bags refund trade: ok=%v err=%v", ok, err)
	}
	trade := event.Data.(CurveTrade)
	if trade.AmountIn.Cmp(big.NewInt(90)) != 0 || trade.AmountOut.Cmp(big.NewInt(1_000)) != 0 || trade.Bags == nil || trade.Bags.GrossQuoteAmount.Cmp(big.NewInt(100)) != 0 || trade.Bags.RefundQuote.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("unexpected refunded Bags trade: %#v", trade)
	}
	badData := append([]byte(nil), data...)
	copy(badData[6*32:7*32], common.LeftPadBytes(big.NewInt(101).Bytes(), 32))
	log.Data = badData
	if _, _, err := parser.ParseLog(log); !errors.Is(err, ErrMalformedLog) {
		t.Fatalf("refund above gross input error = %v", err)
	}
	badData = append([]byte(nil), data...)
	copy(badData[3*32:4*32], common.LeftPadBytes(big.NewInt(11).Bytes(), 32))
	copy(badData[5*32:6*32], common.LeftPadBytes(big.NewInt(6).Bytes(), 32))
	log.Data = badData
	if _, _, err := parser.ParseLog(log); !errors.Is(err, ErrMalformedLog) {
		t.Fatalf("inconsistent gross accounting error = %v", err)
	}
	badData = append([]byte(nil), data...)
	copy(badData[5*32:6*32], common.LeftPadBytes(big.NewInt(6).Bytes(), 32))
	log.Data = badData
	if _, _, err := parser.ParseLog(log); !errors.Is(err, ErrMalformedLog) {
		t.Fatalf("inconsistent fee split error = %v", err)
	}
}

func TestKnownTopics(t *testing.T) {
	tests := map[string]common.Hash{
		"Initialize": topicPoolInitialize, "Swap": topicPoolSwap,
		"Pons launch": topicPonsLaunch, "Pons buy": topicPonsCurveBuy, "Pons sell": topicPonsCurveSell, "Pons pool": topicPonsPool,
		"Long": topicLongLaunch, "o1": topicO1Launch, "Pools": topicPoolsCreated, "PAIR": topicPAIRPool,
		"Bags launch": topicBagsCreated, "Bags buy": topicBagsCurveBuy, "Bags sell": topicBagsCurveSell,
	}
	want := map[string]string{
		"Initialize":  "0xdd466e674ea557f56295e2d0218a125ea4b4f0f6f3307b95f85e6110838d6438",
		"Swap":        "0x40e9cecb9f5f1f1c5b9c97dec2917b7ee92e57ba5563708daca94dd84ad7112f",
		"Pons launch": "0x8d4aad4953d0ca700d468f3753aa14432d1b35b43ec6409f051fb6aa43a89607",
		"Pons buy":    "0xec36bf571f136799e8dc0b0b8bea4b04d8bd3d43de838aab0d5fc21d4cbfc455",
		"Pons sell":   "0x8113d738abdcb6b38357e9d53a54a7157861a09031b453651f0fe7fe151f59df",
		"Pons pool":   "0x01bf263a1db1652580721573296e1a1fa70b3d4c87f61d02a69c4e1109d2d573",
		"Long":        "0xadc6f1f726f7c710f77ec06adc75f3bb964e5be19581b072c67f7b9b4039267b",
		"o1":          "0x207384e895174175cc774fe7f7457b37c382f27ebf53d37d5257b862f80eaf9c",
		"Pools":       "0x2e2b3f61b70d2d131b2a807371103cc98d51adcaa5e9a8f9c32658ad8426e74e",
		"PAIR":        "0x02f2fb2145fe2cb08b684bb8dbeed1ad361e424c5730fe9470e02b83f0890ea3",
		"Bags launch": "0x643b3b606052cbadac2f906ad0b462da99eda2a1d4f824d315d7f6edd3e4cced",
		"Bags buy":    "0x6d9c6fad0db13f6f7fca7124777996deaeb1949d0750a4874c18611ff5d436b9",
		"Bags sell":   "0x813ea2e4b7710a4562c34494d61d6e80fd2ba5105790f740fdcbf64f8b05b80d",
	}
	for name, topic := range tests {
		if topic.Hex() != want[name] {
			t.Fatalf("%s topic = %s", name, topic.Hex())
		}
	}
}

func TestKnownPonsMemeHookAddress(t *testing.T) {
	want := common.HexToAddress("0xE5e702641Ea86F4ae6cC3cDaeD2B886f976Be044")
	if got := DefaultAddressBook().PonsMemeHook; got != want {
		t.Fatalf("PonsMemeHook = %s, want %s", got, want)
	}
}

func FuzzParseLogNoPanic(f *testing.F) {
	f.Add([]byte{1, 2, 3}, uint8(2))
	launch := makeBagsLaunchLog(f, common.HexToAddress("0xf1"), common.HexToAddress("0xf2"), common.HexToHash("0xf3"), 1)
	f.Add(launch.Data, uint8(12)) // 4 topics, Bags launch mode.
	longLaunch, err := longEventABI.Events["LaunchCreated"].Inputs.NonIndexed().Pack(
		common.HexToAddress("0xf4"), common.HexToAddress("0xf5"), [32]byte{1}, big.NewInt(100), big.NewInt(200), "LONG",
	)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(longLaunch, uint8(36)) // 4 topics, Long launch mode.
	ponsLaunch, err := abiArguments(f, "address", "uint256", "uint256").Pack(common.Address{}, big.NewInt(1), big.NewInt(2))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(ponsLaunch, uint8(44)) // 4 topics, Pons launch mode.
	buyData, err := abiArguments(f, "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256").Pack(
		big.NewInt(100), big.NewInt(80), big.NewInt(1_000), big.NewInt(10), big.NewInt(5), big.NewInt(5), big.NewInt(10), big.NewInt(20), big.NewInt(30), big.NewInt(40),
	)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buyData, uint8(19)) // 3 topics, Bags buy mode.
	sellData, err := abiArguments(f, "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256", "uint256").Pack(
		big.NewInt(1_000), big.NewInt(100), big.NewInt(90), big.NewInt(10), big.NewInt(5), big.NewInt(5), big.NewInt(20), big.NewInt(30), big.NewInt(40),
	)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(sellData, uint8(27)) // 3 topics, Bags sell mode.
	f.Fuzz(func(t *testing.T, data []byte, topicCount uint8) {
		if len(data) > maxLogDataBytes+1 {
			data = data[:maxLogDataBytes+1]
		}
		count := int(topicCount % 8)
		topics := make([]common.Hash, count)
		parser := New()
		address := DefaultAddressBook().PoolManager
		if count > 0 {
			switch (topicCount / 8) % 6 {
			case 0:
				topics[0] = TopicPoolInitialize
			case 1:
				address = DefaultAddressBook().BagsFactory
				topics[0] = TopicBagsCreated
			case 2, 3:
				curve := common.HexToAddress("0xf2")
				address = curve
				if err := parser.Registry().RegisterCurve(CurveRegistration{Curve: curve, Protocol: ProtocolBagsV2, Token: common.HexToAddress("0xf1"), Quote: DefaultAddressBook().WETH}); err != nil {
					t.Fatal(err)
				}
				if (topicCount/8)%4 == 2 {
					topics[0] = TopicBagsCurveBuy
				} else {
					topics[0] = TopicBagsCurveSell
				}
			case 4:
				address = DefaultAddressBook().LongLauncher
				topics[0] = TopicLongLaunch
			case 5:
				address = DefaultAddressBook().PonsFactory
				topics[0] = TopicPonsLaunch
			}
		}
		_, _, _ = parser.ParseLog(gethtypes.Log{Address: address, Topics: topics, Data: data})
	})
}

func BenchmarkParseRegisteredSwap(b *testing.B) {
	parser := New()
	key := PoolKey{Currency0: common.Address{}, Currency1: common.HexToAddress("0x40"), Fee: 3000, TickSpacing: 60}
	id, _ := PoolID(key)
	_ = parser.Registry().RegisterPool(PoolRegistration{PoolID: id, Protocol: ProtocolPoolsTrade, Token: key.Currency1, Quote: key.Currency0, PoolKey: key})
	log := makeSwapLog(b, key, 1)
	b.ReportAllocs()
	for b.Loop() {
		if _, ok, err := parser.ParseLog(log); err != nil || !ok {
			b.Fatalf("parse failed: ok=%v err=%v", ok, err)
		}
	}
}
