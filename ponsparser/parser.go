package ponsparser

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const MaxLogDataBytes = 1 << 20

var ErrDataTooLarge = errors.New("encoded data exceeds parser limit")

var (
	tokenLaunchedEvent = newEventDecoder(eventsContractABI.Events["TokenLaunched"])
	curveBuyEvent      = newEventDecoder(eventsContractABI.Events["CurveBuy"])
	curveSellEvent     = newEventDecoder(eventsContractABI.Events["CurveSell"])
	tokenLaunchedTopic = tokenLaunchedEvent.event.ID
	curveBuyTopic      = curveBuyEvent.event.ID
	curveSellTopic     = curveSellEvent.event.ID
	TokenLaunchedTopic = tokenLaunchedTopic
	CurveBuyTopic      = curveBuyTopic
	CurveSellTopic     = curveSellTopic
	defaultEventTopics = indexEvents(eventsContractABI)
	ErrUntrustedSource = errors.New("untrusted event source")
)

type eventDecoder struct {
	event      abi.Event
	kind       EventKind
	indexed    abi.Arguments
	nonIndexed abi.Arguments
}

type EventRegistry struct {
	events map[common.Hash]eventDecoder
}

func NewEventRegistry(extraABI abi.ABI) EventRegistry {
	return EventRegistry{events: indexEvents(extraABI)}
}

type SourceValidator func(kind EventKind, address common.Address) bool

func AddressAllowlist(addresses ...common.Address) SourceValidator {
	trusted := make(map[common.Address]struct{}, len(addresses))
	for _, address := range addresses {
		trusted[address] = struct{}{}
	}
	return func(_ EventKind, address common.Address) bool {
		_, ok := trusted[address]
		return ok
	}
}

func ParseReceiptEvents(receipt *types.Receipt) ([]ParsedEvent, error) {
	return parseReceiptEvents(receipt, ParseLog)
}

func ParseReceiptEventsValidated(receipt *types.Receipt, validator SourceValidator) ([]ParsedEvent, error) {
	if validator == nil {
		return nil, errors.New("nil source validator")
	}
	return parseReceiptEvents(receipt, func(log types.Log) (ParsedEvent, bool, error) {
		return ParseLogValidated(log, validator)
	})
}

// ParseReceiptEventsFiltered parses trusted built-in events and skips known
// event signatures emitted by sources that the validator does not trust.
func ParseReceiptEventsFiltered(receipt *types.Receipt, validator SourceValidator) ([]ParsedEvent, error) {
	if validator == nil {
		return nil, errors.New("nil source validator")
	}
	return parseReceiptEvents(receipt, func(log types.Log) (ParsedEvent, bool, error) {
		return ParseLogFiltered(log, validator)
	})
}

type logParser func(types.Log) (ParsedEvent, bool, error)

func parseReceiptEvents(receipt *types.Receipt, parse logParser) ([]ParsedEvent, error) {
	if receipt == nil {
		return nil, errors.New("nil receipt")
	}
	events := make([]ParsedEvent, 0, batchCapacity(len(receipt.Logs)))
	for _, log := range receipt.Logs {
		if log == nil {
			continue
		}
		parsed, ok, err := parse(*log)
		if err != nil {
			return nil, err
		}
		if ok {
			events = append(events, parsed)
		}
	}
	return events, nil
}

func ParseLogs(logs []types.Log) ([]ParsedEvent, error) {
	return parseLogs(logs, ParseLog)
}

func ParseLogsValidated(logs []types.Log, validator SourceValidator) ([]ParsedEvent, error) {
	if validator == nil {
		return nil, errors.New("nil source validator")
	}
	return parseLogs(logs, func(log types.Log) (ParsedEvent, bool, error) {
		return ParseLogValidated(log, validator)
	})
}

// ParseLogsFiltered parses trusted built-in events and silently ignores known
// signatures from sources that the validator does not trust.
func ParseLogsFiltered(logs []types.Log, validator SourceValidator) ([]ParsedEvent, error) {
	if validator == nil {
		return nil, errors.New("nil source validator")
	}
	return parseLogs(logs, func(log types.Log) (ParsedEvent, bool, error) {
		return ParseLogFiltered(log, validator)
	})
}

func parseLogs(logs []types.Log, parse logParser) ([]ParsedEvent, error) {
	events := make([]ParsedEvent, 0, batchCapacity(len(logs)))
	for _, log := range logs {
		parsed, ok, err := parse(log)
		if err != nil {
			return nil, err
		}
		if ok {
			events = append(events, parsed)
		}
	}
	return events, nil
}

func ParseLog(log types.Log) (ParsedEvent, bool, error) {
	if len(log.Topics) == 0 {
		return ParsedEvent{}, false, nil
	}
	switch log.Topics[0] {
	case tokenLaunchedTopic:
		ev, err := parseTokenLaunched(log)
		return ParsedEvent{Kind: EventTokenLaunched, Address: log.Address, Event: ev, Log: log}, err == nil, err
	case curveBuyTopic:
		ev, err := parseCurveBuy(log)
		return ParsedEvent{Kind: EventCurveBuy, Address: log.Address, Event: ev, Log: log}, err == nil, err
	case curveSellTopic:
		ev, err := parseCurveSell(log)
		return ParsedEvent{Kind: EventCurveSell, Address: log.Address, Event: ev, Log: log}, err == nil, err
	default:
		return parseGenericLog(defaultEventTopics, log)
	}
}

func ParseLogValidated(log types.Log, validator SourceValidator) (ParsedEvent, bool, error) {
	if validator == nil {
		return ParsedEvent{}, false, errors.New("nil source validator")
	}
	kind, ok := eventKind(defaultEventTopics, log)
	if !ok {
		return ParsedEvent{}, false, nil
	}
	if err := validateSource(kind, log.Address, validator); err != nil {
		return ParsedEvent{}, false, err
	}
	return ParseLog(log)
}

// ParseLogFiltered is the non-failing filtering counterpart to
// ParseLogValidated. It returns ok=false for a recognized but untrusted source.
func ParseLogFiltered(log types.Log, validator SourceValidator) (ParsedEvent, bool, error) {
	if validator == nil {
		return ParsedEvent{}, false, errors.New("nil source validator")
	}
	kind, ok := eventKind(defaultEventTopics, log)
	if !ok || !validator(kind, log.Address) {
		return ParsedEvent{}, false, nil
	}
	return ParseLog(log)
}

func (r EventRegistry) ParseLog(log types.Log) (ParsedEvent, bool, error) {
	parsed, ok, err := ParseLog(log)
	if err != nil || ok {
		return parsed, ok, err
	}
	return r.ParseExtraLog(log)
}

func (r EventRegistry) ParseLogValidated(log types.Log, validator SourceValidator) (ParsedEvent, bool, error) {
	if validator == nil {
		return ParsedEvent{}, false, errors.New("nil source validator")
	}
	if kind, ok := eventKind(defaultEventTopics, log); ok {
		if err := validateSource(kind, log.Address, validator); err != nil {
			return ParsedEvent{}, false, err
		}
		return ParseLog(log)
	}
	kind, ok := eventKind(r.events, log)
	if !ok {
		return ParsedEvent{}, false, nil
	}
	if err := validateSource(kind, log.Address, validator); err != nil {
		return ParsedEvent{}, false, err
	}
	return r.ParseExtraLog(log)
}

func (r EventRegistry) ParseLogFiltered(log types.Log, validator SourceValidator) (ParsedEvent, bool, error) {
	if validator == nil {
		return ParsedEvent{}, false, errors.New("nil source validator")
	}
	if kind, ok := eventKind(defaultEventTopics, log); ok {
		if !validator(kind, log.Address) {
			return ParsedEvent{}, false, nil
		}
		return ParseLog(log)
	}
	kind, ok := eventKind(r.events, log)
	if !ok || !validator(kind, log.Address) {
		return ParsedEvent{}, false, nil
	}
	return r.ParseExtraLog(log)
}

func (r EventRegistry) ParseExtraLog(log types.Log) (ParsedEvent, bool, error) {
	return parseGenericLog(r.events, log)
}

func parseGenericLog(events map[common.Hash]eventDecoder, log types.Log) (ParsedEvent, bool, error) {
	if len(log.Topics) == 0 {
		return ParsedEvent{}, false, nil
	}
	decoder, ok := events[log.Topics[0]]
	if !ok {
		return ParsedEvent{}, false, nil
	}
	if err := validateLogDataSize(log.Data); err != nil {
		return ParsedEvent{}, false, err
	}
	if len(log.Topics) != len(decoder.indexed)+1 {
		return ParsedEvent{}, false, fmt.Errorf("event %s topic count mismatch: got %d, want %d", decoder.event.Name, len(log.Topics), len(decoder.indexed)+1)
	}
	values, err := decoder.nonIndexed.Unpack(log.Data)
	if err != nil {
		return ParsedEvent{}, false, err
	}
	if err := validateCanonicalArguments(decoder.nonIndexed, values, log.Data); err != nil {
		return ParsedEvent{}, false, fmt.Errorf("event %s: %w", decoder.event.Name, err)
	}
	out := make(map[string]interface{}, len(decoder.nonIndexed)+len(decoder.indexed))
	for i, input := range decoder.nonIndexed {
		out[input.Name] = values[i]
	}
	for i, input := range decoder.indexed {
		value, err := topicValue(input, log.Topics[i+1])
		if err != nil {
			return ParsedEvent{}, false, fmt.Errorf("event %s indexed topic %s: %w", decoder.event.Name, input.Name, err)
		}
		out[input.Name] = value
	}
	return ParsedEvent{
		Kind:    decoder.kind,
		Address: log.Address,
		Event:   GenericEvent{Name: decoder.event.Name, RawName: string(decoder.kind), Values: out},
		Log:     log,
	}, true, nil
}

func parseTokenLaunched(log types.Log) (TokenLaunched, error) {
	if len(log.Topics) != 4 {
		return TokenLaunched{}, fmt.Errorf("TokenLaunched topic count mismatch: got %d, want 4", len(log.Topics))
	}
	if err := validateStaticLogData(log.Data, 3*32); err != nil {
		return TokenLaunched{}, err
	}
	values, err := tokenLaunchedEvent.nonIndexed.Unpack(log.Data)
	if err != nil {
		return TokenLaunched{}, err
	}
	if err := validateCanonicalArguments(tokenLaunchedEvent.nonIndexed, values, log.Data); err != nil {
		return TokenLaunched{}, err
	}
	token, err := topicAddress(log.Topics[1])
	if err != nil {
		return TokenLaunched{}, fmt.Errorf("TokenLaunched token: %w", err)
	}
	curve, err := topicAddress(log.Topics[2])
	if err != nil {
		return TokenLaunched{}, fmt.Errorf("TokenLaunched curve: %w", err)
	}
	deployer, err := topicAddress(log.Topics[3])
	if err != nil {
		return TokenLaunched{}, fmt.Errorf("TokenLaunched deployer: %w", err)
	}
	return TokenLaunched{
		Token:               token,
		Curve:               curve,
		Deployer:            deployer,
		PairToken:           values[0].(common.Address),
		LaunchConfigID:      values[1].(*big.Int),
		GraduationThreshold: values[2].(*big.Int),
	}, nil
}

func parseCurveBuy(log types.Log) (CurveBuy, error) {
	if len(log.Topics) != 3 {
		return CurveBuy{}, fmt.Errorf("CurveBuy topic count mismatch: got %d, want 3", len(log.Topics))
	}
	if err := validateStaticLogData(log.Data, 4*32); err != nil {
		return CurveBuy{}, err
	}
	values, err := curveBuyEvent.nonIndexed.Unpack(log.Data)
	if err != nil {
		return CurveBuy{}, err
	}
	buyer, err := topicAddress(log.Topics[1])
	if err != nil {
		return CurveBuy{}, fmt.Errorf("CurveBuy buyer: %w", err)
	}
	recipient, err := topicAddress(log.Topics[2])
	if err != nil {
		return CurveBuy{}, fmt.Errorf("CurveBuy recipient: %w", err)
	}
	return CurveBuy{
		Buyer:     buyer,
		Recipient: recipient,
		QuoteIn:   values[0].(*big.Int),
		TokensOut: values[1].(*big.Int),
		Fee:       values[2].(*big.Int),
		Tax:       values[3].(*big.Int),
	}, nil
}

func parseCurveSell(log types.Log) (CurveSell, error) {
	if len(log.Topics) != 3 {
		return CurveSell{}, fmt.Errorf("CurveSell topic count mismatch: got %d, want 3", len(log.Topics))
	}
	if err := validateStaticLogData(log.Data, 4*32); err != nil {
		return CurveSell{}, err
	}
	values, err := curveSellEvent.nonIndexed.Unpack(log.Data)
	if err != nil {
		return CurveSell{}, err
	}
	seller, err := topicAddress(log.Topics[1])
	if err != nil {
		return CurveSell{}, fmt.Errorf("CurveSell seller: %w", err)
	}
	recipient, err := topicAddress(log.Topics[2])
	if err != nil {
		return CurveSell{}, fmt.Errorf("CurveSell recipient: %w", err)
	}
	return CurveSell{
		Seller:    seller,
		Recipient: recipient,
		TokensIn:  values[0].(*big.Int),
		QuoteOut:  values[1].(*big.Int),
		Fee:       values[2].(*big.Int),
		Tax:       values[3].(*big.Int),
	}, nil
}

func TopicsForFactoryLaunches() [][]common.Hash {
	return [][]common.Hash{{tokenLaunchedTopic}}
}

func TopicsForCurveTrades() [][]common.Hash {
	return [][]common.Hash{{curveBuyTopic, curveSellTopic}}
}

func filterInputs(inputs abi.Arguments, indexed bool) abi.Arguments {
	out := make(abi.Arguments, 0, len(inputs))
	for _, input := range inputs {
		if input.Indexed == indexed {
			out = append(out, input)
		}
	}
	return out
}

func topicValue(input abi.Argument, topic common.Hash) (interface{}, error) {
	switch input.Type.T {
	case abi.AddressTy:
		return topicAddress(topic)
	case abi.BoolTy:
		for _, value := range topic[:31] {
			if value != 0 {
				return nil, errors.New("non-canonical bool encoding")
			}
		}
		if topic[31] > 1 {
			return nil, errors.New("non-canonical bool encoding")
		}
		return topic[31] == 1, nil
	case abi.UintTy, abi.IntTy:
		signed := input.Type.T == abi.IntTy
		if err := validateIntegerTopic(topic, input.Type.Size, signed); err != nil {
			return nil, err
		}
		return topicInteger(topic, input.Type.Size, signed), nil
	case abi.FixedBytesTy:
		for _, value := range topic[input.Type.Size:] {
			if value != 0 {
				return nil, errors.New("non-canonical fixed-bytes encoding")
			}
		}
		return topic, nil
	default:
		return topic, nil
	}
}

func topicAddress(topic common.Hash) (common.Address, error) {
	for _, value := range topic[:12] {
		if value != 0 {
			return common.Address{}, errors.New("non-canonical address encoding")
		}
	}
	return common.BytesToAddress(topic[12:]), nil
}

func validateIntegerTopic(topic common.Hash, bits int, signed bool) error {
	if bits >= 256 {
		return nil
	}
	valueOffset := len(topic) - bits/8
	padding := byte(0)
	if signed && topic[valueOffset]&0x80 != 0 {
		padding = 0xff
	}
	for _, value := range topic[:valueOffset] {
		if value != padding {
			return errors.New("non-canonical integer encoding")
		}
	}
	return nil
}

func newEventDecoder(event abi.Event) eventDecoder {
	rawName := event.RawName
	if rawName == "" {
		rawName = event.Name
	}
	return eventDecoder{
		event:      event,
		kind:       EventKind(rawName),
		indexed:    filterInputs(event.Inputs, true),
		nonIndexed: filterInputs(event.Inputs, false),
	}
}

func indexEvents(source abi.ABI) map[common.Hash]eventDecoder {
	events := make(map[common.Hash]eventDecoder, len(source.Events))
	for _, event := range source.Events {
		current, ok := events[event.ID]
		if !ok || event.Name < current.event.Name {
			events[event.ID] = newEventDecoder(event)
		}
	}
	return events
}

func eventKind(events map[common.Hash]eventDecoder, log types.Log) (EventKind, bool) {
	if len(log.Topics) == 0 {
		return EventUnknown, false
	}
	decoder, ok := events[log.Topics[0]]
	return decoder.kind, ok
}

func validateSource(kind EventKind, address common.Address, validator SourceValidator) error {
	if !validator(kind, address) {
		return fmt.Errorf("%w: %s from %s", ErrUntrustedSource, kind, address.Hex())
	}
	return nil
}

func topicInteger(topic common.Hash, bits int, signed bool) *big.Int {
	value := new(big.Int).SetBytes(topic[:])
	if bits < 256 {
		mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
		value.And(value, mask)
	}
	if signed && value.Bit(bits-1) == 1 {
		value.Sub(value, new(big.Int).Lsh(big.NewInt(1), uint(bits)))
	}
	return value
}

func batchCapacity(length int) int {
	const initialCapacity = 256
	if length < initialCapacity {
		return length
	}
	return initialCapacity
}

func validateLogDataSize(data []byte) error {
	if len(data) > MaxLogDataBytes {
		return fmt.Errorf("%w: log data is %d bytes, maximum is %d", ErrDataTooLarge, len(data), MaxLogDataBytes)
	}
	return nil
}

func validateStaticLogData(data []byte, expected int) error {
	if err := validateLogDataSize(data); err != nil {
		return err
	}
	if len(data) != expected {
		return fmt.Errorf("non-canonical event data length: got %d, want %d", len(data), expected)
	}
	return nil
}

func validateCanonicalArguments(arguments abi.Arguments, values []interface{}, data []byte) error {
	canonical, err := arguments.Pack(values...)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, data) {
		return errors.New("non-canonical ABI data")
	}
	return nil
}
