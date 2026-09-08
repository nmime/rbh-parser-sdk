package rbhparser

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	topicPoolInitialize = eventTopic("Initialize(bytes32,address,address,uint24,int24,address,uint160,int24)")
	topicPoolSwap       = eventTopic("Swap(bytes32,address,int128,int128,uint160,uint128,int24,uint24)")
	topicPoolLiquidity  = eventTopic("ModifyLiquidity(bytes32,address,int24,int24,int256,bytes32)")
	topicPonsLaunch     = eventTopic("TokenLaunched(address,address,address,address,uint256,uint256)")
	topicPonsCurveBuy   = eventTopic("CurveBuy(address,address,uint256,uint256,uint256,uint256)")
	topicPonsCurveSell  = eventTopic("CurveSell(address,address,uint256,uint256,uint256,uint256)")
	topicPonsPool       = eventTopic("PoolRegistered(bytes32,address,address,address)")
	topicLongLaunch     = eventTopic("LaunchCreated(address,address,address,address,address,bytes32,uint48,uint48,string)")
	topicO1Launch       = eventTopic("Launched(address,bytes32,address,address,uint256,int24)")
	topicPoolsCreated   = eventTopic("TokenCreated(address)")
	topicPAIRPool       = eventTopic("PairPoolCreated(address,address,bytes32,uint256,uint16,uint256,int24,int24,uint160,uint256)")
	topicBagsCreated    = eventTopic("TokenCreated(address,address,address,address,address,bytes32,string,string,string)")
	topicBagsCurveBuy   = eventTopic("TokensBought(address,address,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256)")
	topicBagsCurveSell  = eventTopic("TokensSold(address,address,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256)")

	TopicPoolInitialize = topicPoolInitialize
	TopicPoolSwap       = topicPoolSwap
	TopicPoolLiquidity  = topicPoolLiquidity
	TopicPonsLaunch     = topicPonsLaunch
	TopicPonsCurveBuy   = topicPonsCurveBuy
	TopicPonsCurveSell  = topicPonsCurveSell
	TopicPonsPool       = topicPonsPool
	TopicLongLaunch     = topicLongLaunch
	TopicO1Launch       = topicO1Launch
	TopicPoolsCreated   = topicPoolsCreated
	TopicPAIRPool       = topicPAIRPool
	TopicBagsCreated    = topicBagsCreated
	TopicBagsCurveBuy   = topicBagsCurveBuy
	TopicBagsCurveSell  = topicBagsCurveSell
)

const longEventABIJSON = `[{"type":"event","name":"LaunchCreated","anonymous":false,"inputs":[{"name":"poolOrHook","type":"address","indexed":true},{"name":"asset","type":"address","indexed":true},{"name":"numeraire","type":"address","indexed":true},{"name":"poolInitializer","type":"address","indexed":false},{"name":"launcher","type":"address","indexed":false},{"name":"tickerKey","type":"bytes32","indexed":false},{"name":"deployedAt","type":"uint48","indexed":false},{"name":"reservedUntil","type":"uint48","indexed":false},{"name":"normalizedTicker","type":"string","indexed":false}]}]`
const bagsEventABIJSON = `[{"type":"event","name":"TokenCreated","anonymous":false,"inputs":[{"name":"token","type":"address","indexed":true},{"name":"curve","type":"address","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"feeShare","type":"address","indexed":false},{"name":"partner","type":"address","indexed":false},{"name":"poolId","type":"bytes32","indexed":false},{"name":"name","type":"string","indexed":false},{"name":"symbol","type":"string","indexed":false},{"name":"metadataURI","type":"string","indexed":false}]}]`

var (
	longEventABI = mustABI(longEventABIJSON)
	bagsEventABI = mustABI(bagsEventABIJSON)
)

func eventTopic(signature string) common.Hash { return crypto.Keccak256Hash([]byte(signature)) }

func mustABI(raw string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(raw))
	if err != nil {
		panic(err)
	}
	return parsed
}
