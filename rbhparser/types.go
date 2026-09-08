package rbhparser

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

type EventKind uint8

const (
	EventUnknown EventKind = iota
	EventLaunch
	EventPoolInitialized
	EventLiquidityModified
	EventSwap
	EventCurveBuy
	EventCurveSell
)

func (k EventKind) String() string {
	switch k {
	case EventLaunch:
		return "launch"
	case EventPoolInitialized:
		return "pool-initialized"
	case EventLiquidityModified:
		return "liquidity-modified"
	case EventSwap:
		return "swap"
	case EventCurveBuy:
		return "curve-buy"
	case EventCurveSell:
		return "curve-sell"
	default:
		return "unknown"
	}
}

type IntentKind uint8

const (
	IntentUnknown IntentKind = iota
	IntentBuy
	IntentSell
	IntentLaunch
	IntentLaunchAndBuy
)

// TransactionIntent describes requested, unconfirmed execution decoded from a
// signed transaction. AmountOut is deliberately absent because only execution
// and a receipt can establish an actual fill.
type TransactionIntent struct {
	Kind               IntentKind
	Protocol           Protocol
	TransactionHash    common.Hash
	Sender             common.Address
	Recipient          common.Address
	Contract           common.Address
	Token              common.Address
	Quote              common.Address
	CurrencyIn         common.Address
	CurrencyOut        common.Address
	PoolID             common.Hash
	PoolKey            PoolKey
	AmountIn           *big.Int
	MaximumAmountIn    *big.Int
	MinimumAmountOut   *big.Int
	RequestedAmountOut *big.Int
	TransactionValue   *big.Int
	Deadline           *big.Int
	PonsLaunch         *PonsLaunchIntent
	Confirmed          bool
}

type PonsSocials struct {
	Twitter   string
	Telegram  string
	Discord   string
	Website   string
	Farcaster string
}

type PonsTokenParams struct {
	Name                string
	Symbol              string
	Logo                string
	Description         string
	Socials             PonsSocials
	CreatorFeeRecipient common.Address
	CreatorTaxBPS       uint16
	BuybackEnabled      bool
	ExpectedEconomics   common.Hash
	Salt                common.Hash
}

// PonsLaunchIntent contains every calldata value required to derive the
// CREATE2 curve/token pair without an RPC read on the opportunity path.
type PonsLaunchIntent struct {
	Params             PonsTokenParams
	LaunchConfigID     *big.Int
	OriginalDeployer   common.Address
	SnipeTaxExemptions []common.Address
}

type Event struct {
	Kind     EventKind
	Protocol Protocol
	Data     any
	Log      gethtypes.Log
}

type PoolKey struct {
	Currency0   common.Address
	Currency1   common.Address
	Fee         uint32
	TickSpacing int32
	Hooks       common.Address
}

type PoolRegistration struct {
	PoolID   common.Hash
	Protocol Protocol
	Token    common.Address
	Quote    common.Address
	PoolKey  PoolKey
}

type CurveRegistration struct {
	Curve    common.Address
	Protocol Protocol
	Token    common.Address
	Quote    common.Address
}

type PendingPoolRegistration struct {
	PoolID   common.Hash
	Protocol Protocol
	Token    common.Address
	Quote    common.Address
}

type Launch struct {
	Protocol            Protocol
	Token               common.Address
	Quote               common.Address
	Creator             common.Address
	Curve               common.Address
	PoolID              common.Hash
	Pool                *PoolKey
	Name                string
	Symbol              string
	MetadataURI         string
	TickerKey           common.Hash
	NormalizedTicker    string
	PoolOrHook          common.Address
	PoolInitializer     common.Address
	LaunchSupply        *big.Int
	LaunchConfigID      *big.Int
	GraduationThreshold *big.Int
	TickSpacing         int32
	DeployedAt          uint64
	ReservedUntil       uint64
}

type PoolInitialized struct {
	PoolID       common.Hash
	PoolKey      PoolKey
	SqrtPriceX96 *big.Int
	Tick         int32
}

type Swap struct {
	PoolID       common.Hash
	Sender       common.Address
	Amount0      *big.Int
	Amount1      *big.Int
	SqrtPriceX96 *big.Int
	Liquidity    *big.Int
	Tick         int32
	Fee          uint32
	Registration PoolRegistration
}

type LiquidityModified struct {
	PoolID         common.Hash
	Sender         common.Address
	TickLower      int32
	TickUpper      int32
	LiquidityDelta *big.Int
	Salt           common.Hash
}

type CurveTrade struct {
	BuyerOrSeller common.Address
	Recipient     common.Address
	AmountIn      *big.Int
	AmountOut     *big.Int
	Fee           *big.Int
	Tax           *big.Int
}
