package ponsparser

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

type EventKind string

const (
	EventTokenLaunched EventKind = "TokenLaunched"
	EventCurveBuy      EventKind = "CurveBuy"
	EventCurveSell     EventKind = "CurveSell"
	EventUnknown       EventKind = "Unknown"
)

type ParsedEvent struct {
	Kind    EventKind
	Address common.Address
	Event   interface{}
	Log     gethtypes.Log
}

type GenericEvent struct {
	Name    string
	RawName string
	Values  map[string]interface{}
}

type TokenLaunched struct {
	Token               common.Address
	Curve               common.Address
	Deployer            common.Address
	PairToken           common.Address
	LaunchConfigID      *big.Int
	GraduationThreshold *big.Int
}

type CurveBuy struct {
	Buyer     common.Address
	Recipient common.Address
	QuoteIn   *big.Int
	TokensOut *big.Int
	Fee       *big.Int
	Tax       *big.Int
}

type CurveSell struct {
	Seller    common.Address
	Recipient common.Address
	TokensIn  *big.Int
	QuoteOut  *big.Int
	Fee       *big.Int
	Tax       *big.Int
}
