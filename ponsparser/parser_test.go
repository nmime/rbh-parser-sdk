package ponsparser

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var benchmarkParsedEvent ParsedEvent

func TestParseTokenLaunched(t *testing.T) {
	event := EventsABI.Events["TokenLaunched"]
	token := common.HexToAddress("0x0000000000000000000000000000000000000001")
	curve := common.HexToAddress("0x0000000000000000000000000000000000000002")
	deployer := common.HexToAddress("0x0000000000000000000000000000000000000003")
	pairToken := common.HexToAddress("0x0000000000000000000000000000000000000004")
	data, err := event.Inputs.NonIndexed().Pack(pairToken, big.NewInt(7), big.NewInt(99))
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{
		Address:     common.HexToAddress("0x00000000000000000000000000000000000000aa"),
		Topics:      []common.Hash{event.ID, common.BytesToHash(token.Bytes()), common.BytesToHash(curve.Bytes()), common.BytesToHash(deployer.Bytes())},
		Data:        data,
		BlockNumber: 42,
		TxHash:      common.HexToHash("0x1234"),
		Index:       7,
		Removed:     true,
	}
	parsed, ok, err := ParseLog(log)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected parse")
	}
	got := parsed.Event.(TokenLaunched)
	if got.Token != token || got.Curve != curve || got.Deployer != deployer || got.PairToken != pairToken {
		t.Fatalf("bad addresses: %#v", got)
	}
	if got.LaunchConfigID.Cmp(big.NewInt(7)) != 0 || got.GraduationThreshold.Cmp(big.NewInt(99)) != 0 {
		t.Fatalf("bad amounts: %#v", got)
	}
	if !parsed.Log.Removed || parsed.Log.BlockNumber != 42 || parsed.Log.TxHash != log.TxHash || parsed.Log.Index != 7 {
		t.Fatalf("raw log metadata was not preserved: %#v", parsed.Log)
	}
}

func TestParseCurveBuy(t *testing.T) {
	event := EventsABI.Events["CurveBuy"]
	buyer := common.HexToAddress("0x0000000000000000000000000000000000000011")
	recipient := common.HexToAddress("0x0000000000000000000000000000000000000012")
	data, err := event.Inputs.NonIndexed().Pack(big.NewInt(100), big.NewInt(200), big.NewInt(3), big.NewInt(4))
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{
		Topics: []common.Hash{event.ID, common.BytesToHash(buyer.Bytes()), common.BytesToHash(recipient.Bytes())},
		Data:   data,
	}
	parsed, ok, err := ParseLog(log)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected parse")
	}
	got := parsed.Event.(CurveBuy)
	if got.Buyer != buyer || got.Recipient != recipient || got.QuoteIn.Cmp(big.NewInt(100)) != 0 || got.TokensOut.Cmp(big.NewInt(200)) != 0 {
		t.Fatalf("bad event: %#v", got)
	}
}

func TestParseGenericEvent(t *testing.T) {
	event := EventsABI.Events["PoolGraduated"]
	token := common.HexToAddress("0x0000000000000000000000000000000000000021")
	data, err := event.Inputs.NonIndexed().Pack(big.NewInt(9), big.NewInt(10), big.NewInt(11))
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{
		Topics: []common.Hash{event.ID, common.BytesToHash(token.Bytes())},
		Data:   data,
	}
	parsed, ok, err := ParseLog(log)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected parse")
	}
	if parsed.Kind != EventKind("PoolGraduated") {
		t.Fatalf("unexpected kind %s", parsed.Kind)
	}
	got := parsed.Event.(GenericEvent)
	if got.Values["token"].(common.Address) != token {
		t.Fatalf("bad token %#v", got.Values)
	}
	if got.Values["positionId"].(*big.Int).Cmp(big.NewInt(9)) != 0 {
		t.Fatalf("bad position id %#v", got.Values)
	}
}

func TestParseCustomError(t *testing.T) {
	abiErr := ErrorsABI.Errors["NativeValueMismatch"]
	args, err := abiErr.Inputs.Pack(big.NewInt(1), big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	data := append(abiErr.ID[:4], args...)
	decoded, ok, err := ParseError(data)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected custom error")
	}
	if decoded.Name != "NativeValueMismatch" {
		t.Fatalf("unexpected error name %s", decoded.Name)
	}
	if decoded.Values["expected"].(*big.Int).Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("bad decoded values %#v", decoded.Values)
	}
}

func TestParsePanicAndIgnoreExportedABIMutation(t *testing.T) {
	original := ErrorsABI
	ErrorsABI = abi.ABI{}
	t.Cleanup(func() { ErrorsABI = original })

	data := append([]byte{0x4e, 0x48, 0x7b, 0x71}, common.LeftPadBytes([]byte{0x11}, 32)...)
	decoded, ok, err := ParseError(data)
	if err != nil || !ok {
		t.Fatalf("panic parse failed: decoded=%#v ok=%v err=%v", decoded, ok, err)
	}
	if decoded.Name != "Panic" || decoded.Values["reason"] != "arithmetic underflow or overflow" {
		t.Fatalf("unexpected panic: %#v", decoded)
	}

	custom := errorsContractABI.Errors["ZeroAmount"]
	decoded, ok, err = ParseError(custom.ID[:4])
	if err != nil || !ok || decoded.Name != "ZeroAmount" {
		t.Fatalf("internal error ABI was affected by export mutation: decoded=%#v ok=%v err=%v", decoded, ok, err)
	}
}

func TestParseLogRejectsNonCanonicalTopics(t *testing.T) {
	curveBuy := eventsContractABI.Events["CurveBuy"]
	data, err := curveBuy.Inputs.NonIndexed().Pack(big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4))
	if err != nil {
		t.Fatal(err)
	}
	addressTopic := common.BytesToHash(common.HexToAddress("0x1").Bytes())
	valid := types.Log{Topics: []common.Hash{curveBuy.ID, addressTopic, addressTopic}, Data: data}

	withExtra := valid
	withExtra.Topics = append(append([]common.Hash(nil), valid.Topics...), common.Hash{})
	if _, _, err := ParseLog(withExtra); err == nil {
		t.Fatal("expected extra topic to be rejected")
	}

	badAddress := valid
	badAddress.Topics = append([]common.Hash(nil), valid.Topics...)
	badAddress.Topics[1][0] = 1
	if _, _, err := ParseLog(badAddress); err == nil {
		t.Fatal("expected non-canonical address topic to be rejected")
	}

	extra := mustABI(`[{"type":"event","name":"Values","inputs":[{"name":"flag","type":"bool","indexed":true},{"name":"count","type":"uint24","indexed":true}],"anonymous":false}]`)
	event := extra.Events["Values"]
	badBool := common.Hash{}
	badBool[31] = 2
	if _, _, err := NewEventRegistry(extra).ParseExtraLog(types.Log{Topics: []common.Hash{event.ID, badBool, common.Hash{}}}); err == nil {
		t.Fatal("expected non-canonical bool topic to be rejected")
	}
	badUint := common.Hash{}
	badUint[0] = 1
	if _, _, err := NewEventRegistry(extra).ParseExtraLog(types.Log{Topics: []common.Hash{event.ID, common.HexToHash("0x1"), badUint}}); err == nil {
		t.Fatal("expected non-canonical uint24 topic to be rejected")
	}
}

func TestParseLogRejectsNonCanonicalData(t *testing.T) {
	event := eventsContractABI.Events["CurveBuy"]
	data, err := event.Inputs.NonIndexed().Pack(big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4))
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{Topics: []common.Hash{event.ID, common.Hash{}, common.Hash{}}, Data: append(data, make([]byte, 32)...)}
	if _, _, err := ParseLog(log); err == nil {
		t.Fatal("expected trailing event data to be rejected")
	}

	launch := eventsContractABI.Events["TokenLaunched"]
	launchData, err := launch.Inputs.NonIndexed().Pack(common.HexToAddress("0x1"), big.NewInt(1), big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	launchData[0] = 1
	log = types.Log{Topics: []common.Hash{launch.ID, common.Hash{}, common.Hash{}, common.Hash{}}, Data: launchData}
	if _, _, err := ParseLog(log); err == nil {
		t.Fatal("expected non-canonical address data to be rejected")
	}
}

func TestParserRejectsOversizedRecognizedData(t *testing.T) {
	event := eventsContractABI.Events["CurveBuy"]
	log := types.Log{Topics: []common.Hash{event.ID, common.Hash{}, common.Hash{}}, Data: make([]byte, MaxLogDataBytes+1)}
	if _, _, err := ParseLog(log); !errors.Is(err, ErrDataTooLarge) {
		t.Fatalf("expected ErrDataTooLarge, got %v", err)
	}

	custom := errorsContractABI.Errors["ZeroAmount"]
	data := make([]byte, MaxRevertDataBytes+1)
	copy(data, custom.ID[:4])
	if _, _, err := ParseError(data); !errors.Is(err, ErrDataTooLarge) {
		t.Fatalf("expected ErrDataTooLarge, got %v", err)
	}
}

func TestParseErrorRejectsTrailingData(t *testing.T) {
	custom := errorsContractABI.Errors["ZeroAmount"]
	if _, _, err := ParseError(append(custom.ID[:4], make([]byte, 32)...)); err == nil {
		t.Fatal("expected trailing custom error data to be rejected")
	}

	panicData := append([]byte{0x4e, 0x48, 0x7b, 0x71}, common.LeftPadBytes([]byte{0x11}, 32)...)
	if _, _, err := ParseError(append(panicData, make([]byte, 32)...)); err == nil {
		t.Fatal("expected trailing panic data to be rejected")
	}
}

func TestParseErrorStringCanonical(t *testing.T) {
	typ, err := abi.NewType("string", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	args, err := (abi.Arguments{{Type: typ}}).Pack("denied")
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte{0x08, 0xc3, 0x79, 0xa0}, args...)
	decoded, ok, err := ParseError(data)
	if err != nil || !ok || decoded.Name != "Error" || decoded.Values["reason"] != "denied" {
		t.Fatalf("decoded=%#v ok=%v err=%v", decoded, ok, err)
	}
	if _, _, err := ParseError(append(data, make([]byte, 32)...)); err == nil {
		t.Fatal("expected trailing revert data to be rejected")
	}
}

func TestErrorIndexMarksSelectorCollisionsAmbiguous(t *testing.T) {
	first := abi.NewError("First", nil)
	second := abi.NewError("Second", nil)
	copy(second.ID[:4], first.ID[:4])
	indexed := indexErrors(abi.ABI{Errors: map[string]abi.Error{"First": first, "Second": second}})
	var selector [4]byte
	copy(selector[:], first.ID[:4])
	if !indexed[selector].ambiguous {
		t.Fatal("selector collision was not marked ambiguous")
	}
}

func TestOfficialErrorSelectorsAreUnique(t *testing.T) {
	seen := make(map[[4]byte]string, len(errorsContractABI.Errors))
	for name, definition := range errorsContractABI.Errors {
		var selector [4]byte
		copy(selector[:], definition.ID[:4])
		if previous, ok := seen[selector]; ok {
			t.Fatalf("selector collision between %s and %s", previous, name)
		}
		seen[selector] = name
	}
}

func TestInternalEventABIIsIndependentFromExport(t *testing.T) {
	original := EventsABI
	EventsABI = abi.ABI{}
	t.Cleanup(func() { EventsABI = original })
	if tokenLaunchedEvent.event.ID == (common.Hash{}) || defaultEventTopics[tokenLaunchedTopic].event.ID != tokenLaunchedTopic {
		t.Fatal("exported event ABI mutation affected internal registry")
	}
}

func TestParseLogValidatedRejectsUntrustedEmitter(t *testing.T) {
	event := EventsABI.Events["CurveBuy"]
	data, err := event.Inputs.NonIndexed().Pack(big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4))
	if err != nil {
		t.Fatal(err)
	}
	curve := common.HexToAddress("0x0000000000000000000000000000000000000011")
	log := types.Log{
		Address: curve,
		Topics: []common.Hash{
			event.ID,
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000012").Bytes()),
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000013").Bytes()),
		},
		Data: data,
	}

	if _, _, err := ParseLogValidated(log, AddressAllowlist(common.HexToAddress("0x0000000000000000000000000000000000000099"))); !errors.Is(err, ErrUntrustedSource) {
		t.Fatalf("expected ErrUntrustedSource, got %v", err)
	}
	parsed, ok, err := ParseLogValidated(log, AddressAllowlist(curve))
	if err != nil || !ok || parsed.Address != curve {
		t.Fatalf("trusted log was not parsed: parsed=%#v ok=%v err=%v", parsed, ok, err)
	}
}

func TestParseLogValidatedAuthenticatesBeforeDecoding(t *testing.T) {
	event := EventsABI.Events["CurveBuy"]
	untrusted := common.HexToAddress("0x0000000000000000000000000000000000000011")
	log := types.Log{
		Address: untrusted,
		Topics: []common.Hash{
			event.ID,
			common.HexToHash("0x12"),
			common.HexToHash("0x13"),
		},
		Data: []byte{1},
	}

	if _, _, err := ParseLogValidated(log, AddressAllowlist()); !errors.Is(err, ErrUntrustedSource) {
		t.Fatalf("expected source rejection before ABI decoding, got %v", err)
	}
	if _, err := ParseLogsValidated([]types.Log{log}, AddressAllowlist()); !errors.Is(err, ErrUntrustedSource) {
		t.Fatalf("expected batch source rejection before ABI decoding, got %v", err)
	}
	receipt := &types.Receipt{Logs: []*types.Log{&log}}
	if _, err := ParseReceiptEventsValidated(receipt, AddressAllowlist()); !errors.Is(err, ErrUntrustedSource) {
		t.Fatalf("expected receipt source rejection before ABI decoding, got %v", err)
	}
	if _, _, err := ParseLogValidated(log, AddressAllowlist(untrusted)); err == nil || errors.Is(err, ErrUntrustedSource) {
		t.Fatalf("trusted malformed log should reach ABI decoding, got %v", err)
	}
}

func TestFilteredReceiptSkipsTokenTransferBeforeLaunch(t *testing.T) {
	factory := common.HexToAddress("0x00000000000000000000000000000000000000f1")
	token := common.HexToAddress("0x00000000000000000000000000000000000000a1")
	curve := common.HexToAddress("0x00000000000000000000000000000000000000a2")
	deployer := common.HexToAddress("0x00000000000000000000000000000000000000a3")
	pairToken := common.HexToAddress("0x00000000000000000000000000000000000000a4")

	transfer := EventsABI.Events["Transfer"]
	transferData, err := transfer.Inputs.NonIndexed().Pack(big.NewInt(1_000_000))
	if err != nil {
		t.Fatal(err)
	}
	transferLog := &types.Log{
		Address: token,
		Topics: []common.Hash{
			transfer.ID,
			common.Hash{},
			common.BytesToHash(deployer.Bytes()),
		},
		Data: transferData,
	}

	launched := EventsABI.Events["TokenLaunched"]
	launchData, err := launched.Inputs.NonIndexed().Pack(pairToken, big.NewInt(3), big.NewInt(50_000))
	if err != nil {
		t.Fatal(err)
	}
	launchLog := &types.Log{
		Address: factory,
		Topics: []common.Hash{
			launched.ID,
			common.BytesToHash(token.Bytes()),
			common.BytesToHash(curve.Bytes()),
			common.BytesToHash(deployer.Bytes()),
		},
		Data: launchData,
	}
	receipt := &types.Receipt{Logs: []*types.Log{transferLog, launchLog}}
	validator := func(kind EventKind, emitter common.Address) bool {
		return kind == EventTokenLaunched && emitter == factory
	}

	if _, err := ParseReceiptEventsValidated(receipt, validator); !errors.Is(err, ErrUntrustedSource) {
		t.Fatalf("strict parsing error = %v, want ErrUntrustedSource", err)
	}
	events, err := ParseReceiptEventsFiltered(receipt, validator)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != EventTokenLaunched {
		t.Fatalf("filtered events = %#v, want one TokenLaunched", events)
	}
	got := events[0].Event.(TokenLaunched)
	if got.Token != token || got.Curve != curve || got.PairToken != pairToken {
		t.Fatalf("unexpected launch event: %#v", got)
	}
}

func TestFilteredParsingStillRejectsMalformedTrustedLog(t *testing.T) {
	event := EventsABI.Events["CurveBuy"]
	trusted := common.HexToAddress("0x0000000000000000000000000000000000000011")
	log := types.Log{Address: trusted, Topics: []common.Hash{event.ID}, Data: []byte{1}}
	validator := AddressAllowlist(trusted)
	if _, _, err := ParseLogFiltered(log, validator); err == nil {
		t.Fatal("trusted malformed log must return a decoding error")
	}
	if _, err := ParseLogsFiltered([]types.Log{log}, validator); err == nil {
		t.Fatal("trusted malformed batch log must return a decoding error")
	}
}

func TestRegistryValidatedAuthenticatesBeforeDecoding(t *testing.T) {
	extra := mustABI(`[{"type":"event","name":"Extra","inputs":[{"name":"value","type":"uint256","indexed":false}],"anonymous":false}]`)
	event := extra.Events["Extra"]
	log := types.Log{
		Address: common.HexToAddress("0x0000000000000000000000000000000000000011"),
		Topics:  []common.Hash{event.ID},
		Data:    []byte{1},
	}
	if _, _, err := NewEventRegistry(extra).ParseLogValidated(log, AddressAllowlist()); !errors.Is(err, ErrUntrustedSource) {
		t.Fatalf("expected registry source rejection before ABI decoding, got %v", err)
	}
}

func TestGenericIndexedSignedInteger(t *testing.T) {
	extra := mustABI(`[{"type":"event","name":"Signed","inputs":[{"name":"value","type":"int24","indexed":true}],"anonymous":false}]`)
	event := extra.Events["Signed"]
	topic := common.Hash{}
	for i := range topic {
		topic[i] = 0xff
	}
	topic[len(topic)-1] = 0xfe

	parsed, ok, err := NewEventRegistry(extra).ParseExtraLog(types.Log{Topics: []common.Hash{event.ID, topic}})
	if err != nil || !ok {
		t.Fatalf("signed event parse failed: ok=%v err=%v", ok, err)
	}
	got := parsed.Event.(GenericEvent).Values["value"].(*big.Int)
	if got.Cmp(big.NewInt(-2)) != 0 {
		t.Fatalf("signed topic = %s, want -2", got)
	}
}

func TestOfficialEventSignatures(t *testing.T) {
	tests := []struct {
		name      string
		signature string
	}{
		{"TokenLaunched", "TokenLaunched(address,address,address,address,uint256,uint256)"},
		{"CurveBuy", "CurveBuy(address,address,uint256,uint256,uint256,uint256)"},
		{"CurveSell", "CurveSell(address,address,uint256,uint256,uint256,uint256)"},
		{"Transfer", "Transfer(address,address,uint256)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event := EventsABI.Events[tc.name]
			if event.Sig != tc.signature {
				t.Fatalf("signature = %s, want %s", event.Sig, tc.signature)
			}
			wantTopic := common.BytesToHash(crypto.Keccak256([]byte(tc.signature)))
			if !bytes.Equal(event.ID[:], wantTopic[:]) {
				t.Fatalf("topic = %s, want %s", event.ID.Hex(), wantTopic.Hex())
			}
		})
	}
}

func TestParseCurveSellAndBatchEntrypoints(t *testing.T) {
	event := eventsContractABI.Events["CurveSell"]
	seller := common.HexToAddress("0x0000000000000000000000000000000000000011")
	recipient := common.HexToAddress("0x0000000000000000000000000000000000000012")
	data, err := event.Inputs.NonIndexed().Pack(big.NewInt(100), big.NewInt(80), big.NewInt(3), big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{Topics: []common.Hash{event.ID, common.BytesToHash(seller.Bytes()), common.BytesToHash(recipient.Bytes())}, Data: data}

	parsed, ok, err := ParseLog(log)
	if err != nil || !ok {
		t.Fatalf("sell parse failed: ok=%v err=%v", ok, err)
	}
	sell := parsed.Event.(CurveSell)
	if sell.Seller != seller || sell.Recipient != recipient || sell.QuoteOut.Cmp(big.NewInt(80)) != 0 {
		t.Fatalf("unexpected sell event: %#v", sell)
	}
	logs, err := ParseLogs([]types.Log{log})
	if err != nil || len(logs) != 1 {
		t.Fatalf("batch logs: len=%d err=%v", len(logs), err)
	}
	receipt, err := ParseReceiptEvents(&types.Receipt{Logs: []*types.Log{&log, nil}})
	if err != nil || len(receipt) != 1 {
		t.Fatalf("receipt events: len=%d err=%v", len(receipt), err)
	}
	if topics := TopicsForCurveTrades(); len(topics) != 1 || len(topics[0]) != 2 || topics[0][1] != event.ID {
		t.Fatalf("unexpected curve topics: %#v", topics)
	}
	if topics := TopicsForFactoryLaunches(); len(topics) != 1 || len(topics[0]) != 1 || topics[0][0] != tokenLaunchedTopic {
		t.Fatalf("unexpected factory topics: %#v", topics)
	}
}

func TestEventRegistryParseLog(t *testing.T) {
	extra := mustABI(`[{"type":"event","name":"Extra","inputs":[{"name":"value","type":"uint256","indexed":false}],"anonymous":false}]`)
	event := extra.Events["Extra"]
	data, err := event.Inputs.NonIndexed().Pack(big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	parsed, ok, err := NewEventRegistry(extra).ParseLog(types.Log{Topics: []common.Hash{event.ID}, Data: data})
	if err != nil || !ok || parsed.Event.(GenericEvent).Values["value"].(*big.Int).Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("registry parse failed: parsed=%#v ok=%v err=%v", parsed, ok, err)
	}
}

func TestOperationalAndOwnershipEventCoverage(t *testing.T) {
	required := []string{
		"BuybackEnabledUpdated(address,bool,address)",
		"BuybackEnabledUpdated(bool)",
		"BuybackEnabledUpdated(bytes32,bool)",
		"OwnershipTransferStarted(address,address)",
		"OwnershipTransferred(address,address)",
	}
	bySignature := make(map[string]struct{}, len(EventsABI.Events))
	for _, event := range EventsABI.Events {
		bySignature[event.Sig] = struct{}{}
	}
	for _, signature := range required {
		if _, ok := bySignature[signature]; !ok {
			t.Errorf("missing official event %s", signature)
		}
		topic := common.BytesToHash(crypto.Keccak256([]byte(signature)))
		if decoder, ok := defaultEventTopics[topic]; !ok || decoder.event.Sig != signature {
			t.Errorf("official event %s is not registered for parsing", signature)
		}
	}
}

func TestParseHookBuybackEnabledUpdated(t *testing.T) {
	const signature = "BuybackEnabledUpdated(bytes32,bool)"
	var hookEventID common.Hash
	var hookEventName string
	for name, event := range EventsABI.Events {
		if event.Sig == signature {
			hookEventID = event.ID
			hookEventName = name
			break
		}
	}
	if hookEventID == (common.Hash{}) {
		t.Fatalf("event %s not found", signature)
	}
	event := EventsABI.Events[hookEventName]
	data, err := event.Inputs.NonIndexed().Pack(true)
	if err != nil {
		t.Fatal(err)
	}
	poolID := common.HexToHash("0x1234")
	parsed, ok, err := ParseLog(types.Log{Topics: []common.Hash{hookEventID, poolID}, Data: data})
	if err != nil || !ok {
		t.Fatalf("hook event parse failed: ok=%v err=%v", ok, err)
	}
	values := parsed.Event.(GenericEvent).Values
	if values["poolId"].(common.Hash) != poolID || !values["enabled"].(bool) {
		t.Fatalf("unexpected hook event values: %#v", values)
	}
}

func BenchmarkParseCurveBuy(b *testing.B) {
	event := EventsABI.Events["CurveBuy"]
	buyer := common.HexToAddress("0x0000000000000000000000000000000000000011")
	recipient := common.HexToAddress("0x0000000000000000000000000000000000000012")
	data, err := event.Inputs.NonIndexed().Pack(big.NewInt(100), big.NewInt(200), big.NewInt(3), big.NewInt(4))
	if err != nil {
		b.Fatal(err)
	}
	log := types.Log{
		Address: common.HexToAddress("0x0000000000000000000000000000000000000013"),
		Topics: []common.Hash{
			event.ID,
			common.BytesToHash(buyer.Bytes()),
			common.BytesToHash(recipient.Bytes()),
		},
		Data: data,
	}
	if _, ok, err := ParseLog(log); err != nil || !ok {
		b.Fatalf("fixture parse failed: ok=%v err=%v", ok, err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed, ok, err := ParseLog(log)
		if err != nil || !ok {
			b.Fatalf("parse failed: ok=%v err=%v", ok, err)
		}
		benchmarkParsedEvent = parsed
	}
}
