package rbhparser

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

type testPoolKeyABI struct {
	Currency0   common.Address `abi:"currency0"`
	Currency1   common.Address `abi:"currency1"`
	Fee         *big.Int       `abi:"fee"`
	TickSpacing *big.Int       `abi:"tickSpacing"`
	Hooks       common.Address `abi:"hooks"`
}

type testExactInputABI struct {
	PoolKey          testPoolKeyABI `abi:"poolKey"`
	ZeroForOne       bool           `abi:"zeroForOne"`
	AmountIn         *big.Int       `abi:"amountIn"`
	AmountOutMinimum *big.Int       `abi:"amountOutMinimum"`
	MinHopPriceX36   *big.Int       `abi:"minHopPriceX36"`
	HookData         []byte         `abi:"hookData"`
}

type testCallABI struct {
	Target common.Address `abi:"target"`
	Value  *big.Int       `abi:"value"`
	Data   []byte         `abi:"data"`
}

type testAuthorizedCallABI struct {
	Target   common.Address `abi:"target"`
	Value    *big.Int       `abi:"value"`
	Data     []byte         `abi:"data"`
	AuthData []byte         `abi:"authData"`
}

type testMulticall3ValueABI struct {
	Target       common.Address `abi:"target"`
	AllowFailure bool           `abi:"allowFailure"`
	Value        *big.Int       `abi:"value"`
	Data         []byte         `abi:"data"`
}

type testPonsSocialsABI struct {
	Twitter   string `abi:"twitter"`
	Telegram  string `abi:"telegram"`
	Discord   string `abi:"discord"`
	Website   string `abi:"website"`
	Farcaster string `abi:"farcaster"`
}

type testPonsTokenParamsABI struct {
	Name                string             `abi:"name"`
	Symbol              string             `abi:"symbol"`
	Logo                string             `abi:"logo"`
	Description         string             `abi:"description"`
	Socials             testPonsSocialsABI `abi:"socials"`
	CreatorFeeRecipient common.Address     `abi:"creatorFeeRecipient"`
	CreatorTaxBps       uint16             `abi:"creatorTaxBps"`
	BuybackEnabled      bool               `abi:"buybackEnabled"`
	ExpectedEconomics   [32]byte           `abi:"expectedEconomics"`
	Salt                [32]byte           `abi:"salt"`
}

func buildV4IntentData(t testing.TB, key PoolKey, zeroForOne bool, amountIn, minOut *big.Int) []byte {
	t.Helper()
	tupleType, err := abi.NewType("tuple", "ExactInputSingle", []abi.ArgumentMarshaling{
		{Name: "poolKey", Type: "tuple", Components: []abi.ArgumentMarshaling{{Name: "currency0", Type: "address"}, {Name: "currency1", Type: "address"}, {Name: "fee", Type: "uint24"}, {Name: "tickSpacing", Type: "int24"}, {Name: "hooks", Type: "address"}}},
		{Name: "zeroForOne", Type: "bool"}, {Name: "amountIn", Type: "uint128"}, {Name: "amountOutMinimum", Type: "uint128"}, {Name: "minHopPriceX36", Type: "uint256"}, {Name: "hookData", Type: "bytes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	swap, err := (abi.Arguments{{Type: tupleType}}).Pack(testExactInputABI{PoolKey: testPoolKeyABI{key.Currency0, key.Currency1, new(big.Int).SetUint64(uint64(key.Fee)), big.NewInt(int64(key.TickSpacing)), key.Hooks}, ZeroForOne: zeroForOne, AmountIn: amountIn, AmountOutMinimum: minOut, MinHopPriceX36: new(big.Int), HookData: []byte{1}})
	if err != nil {
		t.Fatal(err)
	}
	planArgs := abiArguments(t, "bytes", "bytes[]")
	plan, err := planArgs.Pack([]byte{actionSwapExactInSingle}, [][]byte{swap})
	if err != nil {
		t.Fatal(err)
	}
	routerABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"execute","inputs":[{"type":"bytes"},{"type":"bytes[]"},{"type":"uint256"}],"outputs":[]}]`))
	if err != nil {
		t.Fatal(err)
	}
	data, err := routerABI.Pack("execute", []byte{commandV4Swap}, [][]byte{plan}, big.NewInt(1234))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func transactionTo(to common.Address, data []byte, value *big.Int) *gethtypes.Transaction {
	return gethtypes.NewTx(&gethtypes.DynamicFeeTx{ChainID: big.NewInt(4663), To: &to, Data: data, Value: value, Gas: 500_000, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2)})
}

func setCodeTransactionTo(to common.Address, data []byte, value *big.Int) *gethtypes.Transaction {
	return gethtypes.NewTx(&gethtypes.SetCodeTx{
		ChainID: uint256.NewInt(4663), GasTipCap: uint256.NewInt(1), GasFeeCap: uint256.NewInt(2),
		Gas: 500_000, To: to, Data: data, Value: uint256.MustFromBig(value),
	})
}

func ponsLaunchAndBuyData(t testing.TB, quote, recipient common.Address, amount *big.Int) []byte {
	t.Helper()
	paramsType := ponsParamsABIType(t)
	arrayType, err := abi.NewType("address[]", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	args, err := (abi.Arguments{{Type: paramsType}, {Type: mustABIType(t, "uint256")}, {Type: mustABIType(t, "address")}, {Type: mustABIType(t, "uint256")}, {Type: mustABIType(t, "uint256")}, {Type: mustABIType(t, "address")}, {Type: arrayType}}).Pack(testPonsParams(), big.NewInt(3), quote, amount, big.NewInt(90), recipient, []common.Address{common.HexToAddress("0x55")})
	if err != nil {
		t.Fatal(err)
	}
	return append(selectorPonsLaunchAndBuy[:], args...)
}

func ponsParamsABIType(t testing.TB) abi.Type {
	t.Helper()
	paramsType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "name", Type: "string"}, {Name: "symbol", Type: "string"}, {Name: "logo", Type: "string"}, {Name: "description", Type: "string"},
		{Name: "socials", Type: "tuple", Components: []abi.ArgumentMarshaling{{Name: "twitter", Type: "string"}, {Name: "telegram", Type: "string"}, {Name: "discord", Type: "string"}, {Name: "website", Type: "string"}, {Name: "farcaster", Type: "string"}}},
		{Name: "creatorFeeRecipient", Type: "address"}, {Name: "creatorTaxBps", Type: "uint16"}, {Name: "buybackEnabled", Type: "bool"}, {Name: "expectedEconomics", Type: "bytes32"}, {Name: "salt", Type: "bytes32"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return paramsType
}

func testPonsParams() testPonsTokenParamsABI {
	return testPonsTokenParamsABI{Name: "Pons", Symbol: "PNS", Logo: "ipfs://logo", Description: "launch", Socials: testPonsSocialsABI{Twitter: "x"}, CreatorFeeRecipient: common.HexToAddress("0x44"), CreatorTaxBps: 25, BuybackEnabled: true, ExpectedEconomics: [32]byte{1}, Salt: [32]byte{2}}
}

func mustABIType(t testing.TB, name string) abi.Type {
	t.Helper()
	typ, err := abi.NewType(name, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

func batchCallsData(t testing.TB, selector [4]byte, authorized bool, calls []testCallABI) []byte {
	t.Helper()
	components := []abi.ArgumentMarshaling{{Name: "target", Type: "address"}, {Name: "value", Type: "uint256"}, {Name: "data", Type: "bytes"}}
	var value any = calls
	if authorized {
		components = append(components, abi.ArgumentMarshaling{Name: "authData", Type: "bytes"})
		authorizedCalls := make([]testAuthorizedCallABI, len(calls))
		for index, call := range calls {
			authorizedCalls[index] = testAuthorizedCallABI{Target: call.Target, Value: call.Value, Data: call.Data}
		}
		value = authorizedCalls
	}
	tupleArray, err := abi.NewType("tuple[]", "", components)
	if err != nil {
		t.Fatal(err)
	}
	args, err := (abi.Arguments{{Type: tupleArray}}).Pack(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(selector[:], args...)
}

func multicall3ValueData(t testing.TB, calls []testMulticall3ValueABI) []byte {
	t.Helper()
	tupleArray, err := abi.NewType("tuple[]", "", []abi.ArgumentMarshaling{
		{Name: "target", Type: "address"}, {Name: "allowFailure", Type: "bool"},
		{Name: "value", Type: "uint256"}, {Name: "data", Type: "bytes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	args, err := (abi.Arguments{{Type: tupleArray}}).Pack(calls)
	if err != nil {
		t.Fatal(err)
	}
	return append(selectorAggregate3Value[:], args...)
}

func TestParseRegisteredV4BuyIntent(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0x20")
	key := PoolKey{Currency0: common.Address{}, Currency1: token, Fee: 3000, TickSpacing: 60}
	id, err := PoolID(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := parser.Registry().RegisterPool(PoolRegistration{PoolID: id, Protocol: ProtocolLong, Token: token, Quote: common.Address{}, PoolKey: key}); err != nil {
		t.Fatal(err)
	}
	sender := common.HexToAddress("0x33")
	tx := transactionTo(parser.Addresses().UniversalRouter, buildV4IntentData(t, key, true, big.NewInt(100), big.NewInt(90)), big.NewInt(100))
	intent, ok, err := parser.ParseTransactionIntent(tx, sender)
	if err != nil || !ok {
		t.Fatalf("parse: ok=%v err=%v", ok, err)
	}
	if intent.Kind != IntentBuy || intent.Protocol != ProtocolLong || intent.Sender != sender || intent.Token != token || intent.CurrencyIn != (common.Address{}) || intent.CurrencyOut != token || intent.AmountIn.Cmp(big.NewInt(100)) != 0 || intent.MinimumAmountOut.Cmp(big.NewInt(90)) != 0 || intent.PoolID != id || intent.Deadline.Uint64() != 1234 || intent.Confirmed {
		t.Fatalf("unexpected intent: %#v", intent)
	}
}

func TestUnknownV4PoolIsIgnored(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0x20")
	key := PoolKey{Currency0: common.Address{}, Currency1: token, Fee: 3000, TickSpacing: 60}
	tx := transactionTo(parser.Addresses().UniversalRouter, buildV4IntentData(t, key, true, big.NewInt(1), new(big.Int)), big.NewInt(1))
	if _, ok, err := parser.ParseTransactionIntent(tx, common.HexToAddress("0x1")); err != nil || ok {
		t.Fatalf("unknown pool parsed: ok=%v err=%v", ok, err)
	}
}

func TestParsePonsAndBagsCurveIntents(t *testing.T) {
	parser := New()
	sender := common.HexToAddress("0x11")
	recipient := common.HexToAddress("0x12")
	token := common.HexToAddress("0x13")
	quote := common.HexToAddress("0x14")
	ponsCurve := common.HexToAddress("0x15")
	bagsCurve := common.HexToAddress("0x16")
	if err := parser.Registry().RegisterCurve(CurveRegistration{Curve: ponsCurve, Protocol: ProtocolPonsV2, Token: token, Quote: quote}); err != nil {
		t.Fatal(err)
	}
	if err := parser.Registry().RegisterCurve(CurveRegistration{Curve: bagsCurve, Protocol: ProtocolBagsV2, Token: token, Quote: common.Address{}}); err != nil {
		t.Fatal(err)
	}
	ponsArgs, _ := abiArguments(t, "uint256", "uint256", "address").Pack(big.NewInt(100), big.NewInt(90), recipient)
	ponsData := append(selectorPonsBuy[:], ponsArgs...)
	intent, ok, err := parser.ParseTransactionIntent(transactionTo(ponsCurve, ponsData, new(big.Int)), sender)
	if err != nil || !ok || intent.Kind != IntentBuy || intent.AmountIn.Uint64() != 100 || intent.Recipient != recipient || intent.Quote != quote {
		t.Fatalf("Pons intent: %#v ok=%v err=%v", intent, ok, err)
	}
	bagsArgs, _ := abiArguments(t, "address", "uint256").Pack(recipient, big.NewInt(77))
	bagsData := append(selectorBagsBuyFor[:], bagsArgs...)
	intent, ok, err = parser.ParseTransactionIntent(transactionTo(bagsCurve, bagsData, big.NewInt(88)), sender)
	if err != nil || !ok || intent.Kind != IntentBuy || intent.AmountIn.Uint64() != 88 || intent.MinimumAmountOut.Uint64() != 77 || intent.Recipient != recipient {
		t.Fatalf("Bags intent: %#v ok=%v err=%v", intent, ok, err)
	}
}

func TestMalformedRouterCalldataIsRejected(t *testing.T) {
	data := make([]byte, 4+3*abiWordSize)
	copy(data, selectorExecute[:])
	for i := 4; i < 4+abiWordSize; i++ {
		data[i] = 0xff
	}
	_, _, err := New().ParseTransactionIntent(transactionTo(DefaultAddressBook().UniversalRouter, data, new(big.Int)), common.HexToAddress("0x1"))
	if err == nil {
		t.Fatal("malformed calldata accepted")
	}
}

func TestIntentProtocolAndKindFilters(t *testing.T) {
	parser := New()
	token := common.HexToAddress("0x20")
	key := PoolKey{Currency0: common.Address{}, Currency1: token, Fee: 3000, TickSpacing: 60}
	id, _ := PoolID(key)
	_ = parser.Registry().RegisterPool(PoolRegistration{PoolID: id, Protocol: ProtocolLong, Token: token, Quote: common.Address{}, PoolKey: key})
	tx := transactionTo(parser.Addresses().UniversalRouter, buildV4IntentData(t, key, true, big.NewInt(100), big.NewInt(90)), big.NewInt(100))
	sender := common.HexToAddress("0x33")
	buyLong := IntentFilter{IncludeProtocols: Protocols(ProtocolLong), IncludeKinds: IntentKinds(IntentBuy), Senders: Addresses(sender), Tokens: Addresses(token), Pools: PoolIDs(id)}
	if !parser.IsPotentialIntentWithFilter(tx, buyLong) {
		t.Fatal("valid v4 buy was rejected by early filter")
	}
	if _, ok, err := parser.ParseTransactionIntentFiltered(tx, sender, buyLong); err != nil || !ok {
		t.Fatalf("filtered parse: ok=%v err=%v", ok, err)
	}
	wrongKind := IntentFilter{IncludeKinds: IntentKinds(IntentLaunch)}
	if parser.IsPotentialIntentWithFilter(tx, wrongKind) {
		t.Fatal("v4 swap passed launch-only early filter")
	}
	if _, ok, err := parser.ParseTransactionIntentFiltered(tx, sender, wrongKind); err != nil || ok {
		t.Fatalf("wrong kind output: ok=%v err=%v", ok, err)
	}
	wrongSender := buyLong
	wrongSender.Senders = Addresses(common.HexToAddress("0x99"))
	if _, ok, err := parser.ParseTransactionIntentFiltered(tx, sender, wrongSender); err != nil || ok {
		t.Fatalf("wrong sender output: ok=%v err=%v", ok, err)
	}
}

func TestParsePonsFactoryLaunchOverloads(t *testing.T) {
	parser := New()
	sender := common.HexToAddress("0x11")
	quote := common.HexToAddress("0x12")
	for _, test := range []struct {
		name       string
		selector   [4]byte
		exemptions bool
	}{
		{"simple", selectorPonsLaunch, false},
		{"exemptions", selectorPonsLaunchExempt, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			arguments := abi.Arguments{{Type: ponsParamsABIType(t)}, {Type: mustABIType(t, "uint256")}, {Type: mustABIType(t, "address")}}
			values := []any{testPonsParams(), big.NewInt(3), quote}
			if test.exemptions {
				arguments = append(arguments, abi.Argument{Type: mustABIType(t, "address[]")})
				values = append(values, []common.Address{common.HexToAddress("0x55")})
			}
			args, err := arguments.Pack(values...)
			if err != nil {
				t.Fatal(err)
			}
			data := append(test.selector[:], args...)
			tx := transactionTo(parser.Addresses().PonsFactory, data, big.NewInt(1))
			filter := IntentFilter{IncludeProtocols: Protocols(ProtocolPonsV2), IncludeKinds: IntentKinds(IntentLaunch)}
			if !parser.IsPotentialIntentWithFilter(tx, filter) {
				t.Fatal("Pons launch rejected by early filter")
			}
			intent, ok, err := parser.ParseTransactionIntentFiltered(tx, sender, filter)
			if err != nil || !ok || intent.Protocol != ProtocolPonsV2 || intent.Kind != IntentLaunch || intent.Sender != sender || intent.Quote != quote {
				t.Fatalf("intent=%#v ok=%v err=%v", intent, ok, err)
			}
			if intent.PonsLaunch == nil || intent.PonsLaunch.Params.Symbol != "PNS" || intent.PonsLaunch.OriginalDeployer != sender {
				t.Fatalf("payload=%#v", intent.PonsLaunch)
			}
		})
	}
}

func TestKnownPonsLaunchSelectors(t *testing.T) {
	tests := []struct {
		name string
		got  [4]byte
		want [4]byte
	}{
		{"launchToken", selectorPonsLaunch, [4]byte{0xf3, 0x5a, 0xbb, 0xcf}},
		{"launchToken with exemptions", selectorPonsLaunchExempt, [4]byte{0xa7, 0x21, 0x01, 0xaf}},
		{"launchAndBuy", selectorPonsLaunchAndBuy, [4]byte{0xf8, 0x5f, 0x8e, 0x41}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("selector = 0x%x, want 0x%x", test.got, test.want)
			}
		})
	}
}

func TestParseNestedPonsLaunchAndBuy(t *testing.T) {
	parser := New()
	wallet := common.HexToAddress("0x31")
	quote := common.HexToAddress("0x32")
	innerValue := big.NewInt(7)
	inner := ponsLaunchAndBuyData(t, quote, wallet, big.NewInt(100))
	calls := []testCallABI{
		{Target: common.HexToAddress("0x99"), Value: new(big.Int), Data: []byte{1, 2, 3, 4}},
		{Target: parser.Addresses().PonsLaunchAndBuy, Value: innerValue, Data: inner},
	}
	standard := batchCallsData(t, selectorExecuteBatch, false, calls)
	authorized := batchCallsData(t, selectorExecuteBatchAuth, true, calls)
	targetArgs, err := abiArguments(t, "address", "bytes", "uint256").Pack(parser.Addresses().PonsLaunchAndBuy, inner, innerValue)
	if err != nil {
		t.Fatal(err)
	}
	target := append(selectorTargetCall[:], targetArgs...)
	targetV2 := append(selectorTargetCallV2[:], targetArgs...)
	pons7702Args, err := abiArguments(t, "bytes", "uint256", "address", "bytes", "uint256").Pack([]byte{1}, big.NewInt(2), quote, inner, innerValue)
	if err != nil {
		t.Fatal(err)
	}
	pons7702 := append(selectorPons7702Launch[:], pons7702Args...)
	executeModeArgs, err := abiArguments(t, "bytes32", "bytes").Pack([32]byte{1}, standard[4:])
	if err != nil {
		t.Fatal(err)
	}
	executeMode := append(selectorExecuteMode[:], executeModeArgs...)
	multicall := multicall3ValueData(t, []testMulticall3ValueABI{{Target: parser.Addresses().PonsLaunchAndBuy, Value: innerValue, Data: inner}})

	tests := []struct {
		name string
		tx   *gethtypes.Transaction
	}{
		{"executeBatch", setCodeTransactionTo(wallet, standard, big.NewInt(999))},
		{"authorized batch", setCodeTransactionTo(wallet, authorized, big.NewInt(999))},
		{"target envelope", transactionTo(common.HexToAddress("0x41"), target, big.NewInt(999))},
		{"target envelope v2", transactionTo(common.HexToAddress("0x41"), targetV2, big.NewInt(999))},
		{"Pons 7702 envelope", setCodeTransactionTo(wallet, pons7702, big.NewInt(999))},
		{"execute mode", setCodeTransactionTo(wallet, executeMode, big.NewInt(999))},
		{"Multicall3 value", transactionTo(common.HexToAddress("0x42"), multicall, big.NewInt(999))},
	}
	filter := IntentFilter{
		IncludeProtocols: Protocols(ProtocolPonsV2),
		IncludeKinds:     IntentKinds(IntentLaunchAndBuy),
		Contracts:        Addresses(parser.Addresses().PonsLaunchAndBuy),
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !parser.IsPotentialIntentWithFilter(test.tx, filter) {
				t.Fatal("nested launch rejected by early filter")
			}
			intent, ok, err := parser.ParseTransactionIntentFiltered(test.tx, wallet, filter)
			if err != nil || !ok {
				t.Fatalf("parse: ok=%v err=%v", ok, err)
			}
			if intent.Protocol != ProtocolPonsV2 || intent.Kind != IntentLaunchAndBuy || intent.Sender != wallet ||
				intent.Contract != parser.Addresses().PonsLaunchAndBuy || intent.Quote != quote ||
				intent.AmountIn.Cmp(big.NewInt(100)) != 0 || intent.TransactionValue.Cmp(innerValue) != 0 {
				t.Fatalf("unexpected nested intent: %#v", intent)
			}
			if intent.PonsLaunch == nil || intent.PonsLaunch.OriginalDeployer != wallet || intent.PonsLaunch.LaunchConfigID.Uint64() != 3 ||
				intent.PonsLaunch.Params.Name != "Pons" || intent.PonsLaunch.Params.CreatorTaxBPS != 25 ||
				intent.PonsLaunch.Params.Salt != (common.Hash{2}) || len(intent.PonsLaunch.SnipeTaxExemptions) != 1 {
				t.Fatalf("incomplete Pons launch payload: %#v", intent.PonsLaunch)
			}
		})
	}
}

func TestParseAtomicLaunchV2(t *testing.T) {
	parser := New()
	wallet := common.HexToAddress("0x31")
	quote := common.HexToAddress("0x32")
	tuple := make([]byte, 22*abiWordSize)
	big.NewInt(12 * abiWordSize).FillBytes(tuple[:abiWordSize])
	copy(tuple[2*abiWordSize+12:3*abiWordSize], quote.Bytes())
	args := make([]byte, abiWordSize, abiWordSize+len(tuple))
	big.NewInt(abiWordSize).FillBytes(args)
	args = append(args, tuple...)
	data := append(selectorAtomicLaunchV2[:], args...)
	tx := transactionTo(wallet, data, new(big.Int))
	filter := IntentFilter{IncludeProtocols: Protocols(ProtocolPonsV2), IncludeKinds: IntentKinds(IntentLaunch), Contracts: Addresses(parser.Addresses().PonsFactory)}
	if !parser.IsPotentialIntentWithFilter(tx, filter) {
		t.Fatal("atomic launch rejected by early filter")
	}
	intent, ok, err := parser.ParseTransactionIntentFiltered(tx, wallet, filter)
	if err != nil || !ok || intent.Kind != IntentLaunch || intent.Protocol != ProtocolPonsV2 || intent.Contract != parser.Addresses().PonsFactory || intent.Quote != quote {
		t.Fatalf("intent=%#v ok=%v err=%v", intent, ok, err)
	}
}

func TestNestedCallLimits(t *testing.T) {
	parser := New()
	wallet := common.HexToAddress("0x31")
	args := make([]byte, 2*abiWordSize)
	big.NewInt(abiWordSize).FillBytes(args[:abiWordSize])
	big.NewInt(maxNestedCalls + 1).FillBytes(args[abiWordSize:])
	data := append(selectorExecuteBatch[:], args...)
	tx := setCodeTransactionTo(wallet, data, new(big.Int))
	if parser.IsPotentialIntent(tx) {
		t.Fatal("oversized nested batch passed early filter")
	}
	if _, _, err := parser.ParseTransactionIntent(tx, wallet); err == nil {
		t.Fatal("oversized nested batch was accepted")
	}
}

func TestEventFilter(t *testing.T) {
	event := Event{Kind: EventSwap, Protocol: ProtocolLong}
	filter := EventFilter{IncludeProtocols: Protocols(ProtocolLong), IncludeKinds: EventKinds(EventSwap)}
	if !filter.Match(event) {
		t.Fatal("matching event rejected")
	}
	filter.ExcludeKinds = EventKinds(EventSwap)
	if filter.Match(event) {
		t.Fatal("excluded event accepted")
	}
}

func BenchmarkParseRegisteredV4Intent(b *testing.B) {
	parser := New()
	token := common.HexToAddress("0x20")
	key := PoolKey{Currency0: common.Address{}, Currency1: token, Fee: 3000, TickSpacing: 60}
	id, _ := PoolID(key)
	_ = parser.Registry().RegisterPool(PoolRegistration{PoolID: id, Protocol: ProtocolLong, Token: token, Quote: common.Address{}, PoolKey: key})
	tx := transactionTo(parser.Addresses().UniversalRouter, buildV4IntentData(b, key, true, big.NewInt(100), big.NewInt(90)), big.NewInt(100))
	sender := common.HexToAddress("0x33")
	b.ReportAllocs()
	for b.Loop() {
		if _, ok, err := parser.ParseTransactionIntent(tx, sender); err != nil || !ok {
			b.Fatal(err)
		}
	}
}

func BenchmarkEarlyFilterNestedPonsLaunch(b *testing.B) {
	parser := New()
	wallet := common.HexToAddress("0x31")
	inner := ponsLaunchAndBuyData(b, common.Address{}, wallet, big.NewInt(100))
	data := batchCallsData(b, selectorExecuteBatch, false, []testCallABI{{Target: parser.Addresses().PonsLaunchAndBuy, Value: big.NewInt(7), Data: inner}})
	tx := setCodeTransactionTo(wallet, data, big.NewInt(7))
	filter := IntentFilter{IncludeProtocols: Protocols(ProtocolPonsV2), IncludeKinds: IntentKinds(IntentLaunchAndBuy)}
	b.ReportAllocs()
	for b.Loop() {
		if !parser.IsPotentialIntentWithFilter(tx, filter) {
			b.Fatal("nested Pons launch rejected")
		}
	}
}

func BenchmarkParseNestedPonsLaunch(b *testing.B) {
	parser := New()
	wallet := common.HexToAddress("0x31")
	inner := ponsLaunchAndBuyData(b, common.Address{}, wallet, big.NewInt(100))
	data := batchCallsData(b, selectorExecuteBatch, false, []testCallABI{{Target: parser.Addresses().PonsLaunchAndBuy, Value: big.NewInt(7), Data: inner}})
	tx := setCodeTransactionTo(wallet, data, big.NewInt(7))
	b.ReportAllocs()
	for b.Loop() {
		if _, ok, err := parser.ParseTransactionIntent(tx, wallet); err != nil || !ok {
			b.Fatal(err)
		}
	}
}
