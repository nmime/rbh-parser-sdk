package rbhparser

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

func decodePonsLaunch(log gethtypes.Log) (Launch, error) {
	if err := validateLog(log, 4, 96); err != nil {
		return Launch{}, err
	}
	token, err := topicAddress(log.Topics[1])
	if err != nil {
		return Launch{}, err
	}
	curve, err := topicAddress(log.Topics[2])
	if err != nil {
		return Launch{}, err
	}
	if curve == (common.Address{}) {
		return Launch{}, ErrMalformedLog
	}
	creator, err := topicAddress(log.Topics[3])
	if err != nil {
		return Launch{}, err
	}
	quote, err := addressWord(log.Data, 0)
	if err != nil {
		return Launch{}, err
	}
	configID, err := uintWord(log.Data, 1, 256)
	if err != nil {
		return Launch{}, err
	}
	threshold, err := uintWord(log.Data, 2, 256)
	if err != nil {
		return Launch{}, err
	}
	return Launch{Protocol: ProtocolPonsV2, Token: token, Quote: quote, Creator: creator, Curve: curve, LaunchConfigID: configID, GraduationThreshold: threshold}, nil
}

func decodeLongLaunch(log gethtypes.Log) (Launch, error) {
	if len(log.Topics) != 4 || len(log.Data) > maxLogDataBytes {
		return Launch{}, ErrMalformedLog
	}
	poolOrHook, err := topicAddress(log.Topics[1])
	if err != nil {
		return Launch{}, err
	}
	token, err := topicAddress(log.Topics[2])
	if err != nil {
		return Launch{}, err
	}
	quote, err := topicAddress(log.Topics[3])
	if err != nil {
		return Launch{}, err
	}
	arguments := longEventABI.Events["LaunchCreated"].Inputs.NonIndexed()
	values, err := arguments.Unpack(log.Data)
	if err != nil || len(values) != 6 {
		return Launch{}, ErrMalformedLog
	}
	canonical, err := arguments.Pack(values...)
	if err != nil || !bytes.Equal(canonical, log.Data) {
		return Launch{}, ErrMalformedLog
	}
	poolInitializer, ok0 := values[0].(common.Address)
	creator, ok1 := values[1].(common.Address)
	ticker, ok2 := values[2].([32]byte)
	deployedAt, ok3 := values[3].(*big.Int)
	reservedUntil, ok4 := values[4].(*big.Int)
	normalizedTicker, ok5 := values[5].(string)
	if !ok0 || !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !deployedAt.IsUint64() || !reservedUntil.IsUint64() || deployedAt.BitLen() > 48 || reservedUntil.BitLen() > 48 {
		return Launch{}, ErrMalformedLog
	}
	return Launch{Protocol: ProtocolLong, Token: token, Quote: quote, Creator: creator, PoolOrHook: poolOrHook, PoolInitializer: poolInitializer, TickerKey: ticker, DeployedAt: deployedAt.Uint64(), ReservedUntil: reservedUntil.Uint64(), NormalizedTicker: normalizedTicker}, nil
}

func decodeO1Launch(log gethtypes.Log) (Launch, error) {
	if err := validateLog(log, 4, 96); err != nil {
		return Launch{}, err
	}
	token, err := topicAddress(log.Topics[1])
	if err != nil {
		return Launch{}, err
	}
	creator, err := topicAddress(log.Topics[3])
	if err != nil {
		return Launch{}, err
	}
	quote, err := addressWord(log.Data, 0)
	if err != nil {
		return Launch{}, err
	}
	supply, err := uintWord(log.Data, 1, 256)
	if err != nil {
		return Launch{}, err
	}
	tickSpacing, err := int24Word(log.Data, 2)
	if err != nil || tickSpacing <= 0 {
		return Launch{}, ErrMalformedLog
	}
	if log.Topics[2] == (common.Hash{}) {
		return Launch{}, ErrMalformedLog
	}
	return Launch{Protocol: ProtocolO1, Token: token, Quote: quote, Creator: creator, PoolID: log.Topics[2], LaunchSupply: supply, TickSpacing: tickSpacing}, nil
}

func decodePoolsLaunch(log gethtypes.Log) (Launch, error) {
	if err := validateLog(log, 2, 0); err != nil {
		return Launch{}, err
	}
	token, err := topicAddress(log.Topics[1])
	if err != nil {
		return Launch{}, err
	}
	return Launch{Protocol: ProtocolPoolsTrade, Token: token}, nil
}

func decodePAIRLaunch(log gethtypes.Log) (Launch, error) {
	if err := validateLog(log, 4, 224); err != nil {
		return Launch{}, err
	}
	token, err := topicAddress(log.Topics[1])
	if err != nil {
		return Launch{}, err
	}
	quote, err := topicAddress(log.Topics[2])
	if err != nil {
		return Launch{}, err
	}
	if log.Topics[3] == (common.Hash{}) {
		return Launch{}, ErrMalformedLog
	}
	for _, field := range [...]struct{ index, bits int }{{0, 256}, {1, 16}, {2, 256}, {5, 160}, {6, 256}} {
		if _, err := uintWord(log.Data, field.index, field.bits); err != nil {
			return Launch{}, err
		}
	}
	if _, err := signedWord(log.Data, 3, 24); err != nil {
		return Launch{}, err
	}
	if _, err := signedWord(log.Data, 4, 24); err != nil {
		return Launch{}, err
	}
	return Launch{Protocol: ProtocolPAIR, Token: token, Quote: quote, PoolID: log.Topics[3]}, nil
}

func decodeBagsLaunch(log gethtypes.Log, quote common.Address) (Launch, error) {
	if len(log.Topics) != 4 || len(log.Data) > maxLogDataBytes {
		return Launch{}, ErrMalformedLog
	}
	token, err := topicAddress(log.Topics[1])
	if err != nil {
		return Launch{}, err
	}
	curve, err := topicAddress(log.Topics[2])
	if err != nil {
		return Launch{}, err
	}
	if curve == (common.Address{}) {
		return Launch{}, ErrMalformedLog
	}
	creator, err := topicAddress(log.Topics[3])
	if err != nil {
		return Launch{}, err
	}
	for _, wordIndex := range [...]int{0, 1} {
		if _, err := addressWord(log.Data, wordIndex); err != nil {
			return Launch{}, err
		}
	}
	arguments := bagsEventABI.Events["TokenCreated"].Inputs.NonIndexed()
	values, err := arguments.Unpack(log.Data)
	if err != nil || len(values) != 6 {
		return Launch{}, ErrMalformedLog
	}
	canonical, err := arguments.Pack(values...)
	if err != nil || !bytes.Equal(canonical, log.Data) {
		return Launch{}, ErrMalformedLog
	}
	poolID, ok0 := values[2].([32]byte)
	name, ok1 := values[3].(string)
	symbol, ok2 := values[4].(string)
	metadataURI, ok3 := values[5].(string)
	if _, ok := values[0].(common.Address); !ok || !ok0 || !ok1 || !ok2 || !ok3 || poolID == ([32]byte{}) {
		return Launch{}, ErrMalformedLog
	}
	if _, ok := values[1].(common.Address); !ok {
		return Launch{}, ErrMalformedLog
	}
	return Launch{Protocol: ProtocolBagsV2, Token: token, Quote: quote, Creator: creator, Curve: curve, PoolID: poolID, Name: name, Symbol: symbol, MetadataURI: metadataURI}, nil
}

func decodePonsPool(log gethtypes.Log) (Launch, error) {
	if err := validateLog(log, 2, 96); err != nil {
		return Launch{}, err
	}
	token, err := addressWord(log.Data, 0)
	if err != nil {
		return Launch{}, err
	}
	quote, err := addressWord(log.Data, 1)
	if err != nil {
		return Launch{}, err
	}
	creator, err := addressWord(log.Data, 2)
	if err != nil {
		return Launch{}, err
	}
	if log.Topics[1] == (common.Hash{}) {
		return Launch{}, ErrMalformedLog
	}
	return Launch{Protocol: ProtocolPonsV2, Token: token, Quote: quote, Creator: creator, PoolID: log.Topics[1]}, nil
}

func decodeCurveTrade(log gethtypes.Log) (CurveTrade, error) {
	if err := validateLog(log, 3, 128); err != nil {
		return CurveTrade{}, err
	}
	actor, err := topicAddress(log.Topics[1])
	if err != nil {
		return CurveTrade{}, err
	}
	recipient, err := topicAddress(log.Topics[2])
	if err != nil {
		return CurveTrade{}, err
	}
	amountIn, err := uintWord(log.Data, 0, 256)
	if err != nil {
		return CurveTrade{}, err
	}
	amountOut, err := uintWord(log.Data, 1, 256)
	if err != nil {
		return CurveTrade{}, err
	}
	fee, err := uintWord(log.Data, 2, 256)
	if err != nil {
		return CurveTrade{}, err
	}
	tax, err := uintWord(log.Data, 3, 256)
	if err != nil {
		return CurveTrade{}, err
	}
	return CurveTrade{BuyerOrSeller: actor, Recipient: recipient, AmountIn: amountIn, AmountOut: amountOut, Fee: fee, Tax: tax}, nil
}

func decodeBagsCurveTrade(log gethtypes.Log, buy bool) (CurveTrade, error) {
	dataWords := 9
	if buy {
		dataWords = 10
	}
	if err := validateLog(log, 3, dataWords*32); err != nil {
		return CurveTrade{}, err
	}
	actor, err := topicAddress(log.Topics[1])
	if err != nil {
		return CurveTrade{}, err
	}
	recipient, err := topicAddress(log.Topics[2])
	if err != nil {
		return CurveTrade{}, err
	}
	values := make([]*big.Int, dataWords)
	for i := range values {
		value, err := uintWord(log.Data, i, 256)
		if err != nil {
			return CurveTrade{}, err
		}
		values[i] = value
	}
	details := &BagsCurveTradeDetails{
		GrossQuoteAmount: values[1],
		NetQuoteAmount:   values[2],
		RefundQuote:      new(big.Int),
		VaultFeeQuote:    values[4],
		CreatorFeeWETH:   values[5],
	}
	amountIn, amountOut := values[0], values[2]
	if buy {
		details.GrossQuoteAmount = values[0]
		details.NetQuoteAmount = values[1]
		details.RefundQuote = values[6]
		details.PriceQuotePerToken = values[7]
		details.VirtualTokenReserve = values[8]
		details.VirtualQuoteReserve = values[9]
		if details.RefundQuote.Cmp(details.GrossQuoteAmount) > 0 {
			return CurveTrade{}, ErrMalformedLog
		}
		amountIn = new(big.Int).Sub(details.GrossQuoteAmount, details.RefundQuote)
	} else {
		details.PriceQuotePerToken = values[6]
		details.VirtualTokenReserve = values[7]
		details.VirtualQuoteReserve = values[8]
	}
	feeSplit := new(big.Int).Add(details.VaultFeeQuote, details.CreatorFeeWETH)
	accountedGross := new(big.Int).Add(details.NetQuoteAmount, values[3])
	accountedGross.Add(accountedGross, details.RefundQuote)
	if feeSplit.Cmp(values[3]) != 0 || accountedGross.Cmp(details.GrossQuoteAmount) != 0 {
		return CurveTrade{}, ErrMalformedLog
	}
	return CurveTrade{BuyerOrSeller: actor, Recipient: recipient, AmountIn: amountIn, AmountOut: amountOut, Fee: values[3], Tax: new(big.Int), Bags: details}, nil
}
