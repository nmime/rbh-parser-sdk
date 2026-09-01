package ponsparser

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

const eventABIJSON = `[
	{"type":"event","name":"TokenLaunched","inputs":[{"name":"token","type":"address","indexed":true},{"name":"curve","type":"address","indexed":true},{"name":"deployer","type":"address","indexed":true},{"name":"pairToken","type":"address","indexed":false},{"name":"launchConfigId","type":"uint256","indexed":false},{"name":"graduationThreshold","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"LaunchSwept","inputs":[{"name":"token","type":"address","indexed":true},{"name":"quoteOut","type":"uint256","indexed":false},{"name":"tokenOut","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"LaunchForceSwept","inputs":[{"name":"token","type":"address","indexed":true}],"anonymous":false},
	{"type":"event","name":"CreatorFeeRecipientUpdated","inputs":[{"name":"token","type":"address","indexed":true},{"name":"previousRecipient","type":"address","indexed":true},{"name":"newRecipient","type":"address","indexed":true}],"anonymous":false},
	{"type":"event","name":"CreatorFeeRecipientChangeProposed","inputs":[{"name":"token","type":"address","indexed":true},{"name":"currentRecipient","type":"address","indexed":true},{"name":"proposedRecipient","type":"address","indexed":true},{"name":"effectiveAt","type":"uint256","indexed":false},{"name":"expiresAt","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"CreatorFeeRecipientChangeCancelled","inputs":[{"name":"token","type":"address","indexed":true},{"name":"proposedRecipient","type":"address","indexed":true}],"anonymous":false},
	{"type":"event","name":"PoolGraduated","inputs":[{"name":"token","type":"address","indexed":true},{"name":"positionId","type":"uint256","indexed":false},{"name":"tokenAmount","type":"uint256","indexed":false},{"name":"pairTokenAmount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"LaunchConfigAdded","inputs":[{"name":"id","type":"uint256","indexed":true}],"anonymous":false},
	{"type":"event","name":"LaunchConfigUpdated","inputs":[{"name":"id","type":"uint256","indexed":true}],"anonymous":false},
	{"type":"event","name":"LaunchFeeUpdated","inputs":[{"name":"launchFee","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"LaunchEnabledUpdated","inputs":[{"name":"enabled","type":"bool","indexed":false}],"anonymous":false},
	{"type":"event","name":"WhitelistedLauncherUpdated","inputs":[{"name":"launcher","type":"address","indexed":true},{"name":"enabled","type":"bool","indexed":false}],"anonymous":false},
	{"type":"event","name":"MaxCreatorTaxUpdated","inputs":[{"name":"bps","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"SnipeTaxStartBpsUpdated","inputs":[{"name":"bps","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"SnipeTaxSecondsUpdated","inputs":[{"name":"secondsWindow","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"GraduationExecutorSet","inputs":[{"name":"executor","type":"address","indexed":false}],"anonymous":false},
	{"type":"event","name":"LaunchDeployerSet","inputs":[{"name":"deployer","type":"address","indexed":false}],"anonymous":false},
	{"type":"event","name":"LaunchForwarderSet","inputs":[{"name":"forwarder","type":"address","indexed":false}],"anonymous":false},
	{"type":"event","name":"PairTokenApprovalUpdated","inputs":[{"name":"pairToken","type":"address","indexed":true},{"name":"approved","type":"bool","indexed":false}],"anonymous":false},
	{"type":"event","name":"PairTokenEconomicsUpdated","inputs":[{"name":"pairToken","type":"address","indexed":true},{"name":"phantomQuote","type":"uint256","indexed":false},{"name":"graduationThreshold","type":"uint256","indexed":false},{"name":"decimals","type":"uint8","indexed":false}],"anonymous":false},
	{"type":"event","name":"BuybackEnabledUpdated","inputs":[{"name":"token","type":"address","indexed":true},{"name":"enabled","type":"bool","indexed":false},{"name":"controller","type":"address","indexed":true}],"anonymous":false},
	{"type":"event","name":"GraduationTokensPermanentlyLocked","inputs":[{"name":"token","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"LaunchGraduationRescued","inputs":[{"name":"token","type":"address","indexed":true},{"name":"recipient","type":"address","indexed":true},{"name":"quoteAmount","type":"uint256","indexed":false},{"name":"tokenAmount","type":"uint256","indexed":false}],"anonymous":false},

	{"type":"event","name":"CurveBuy","inputs":[{"name":"buyer","type":"address","indexed":true},{"name":"recipient","type":"address","indexed":true},{"name":"quoteIn","type":"uint256","indexed":false},{"name":"tokensOut","type":"uint256","indexed":false},{"name":"fee","type":"uint256","indexed":false},{"name":"tax","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"CurveBuyRefunded","inputs":[{"name":"buyer","type":"address","indexed":true},{"name":"refund","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"CurveSell","inputs":[{"name":"seller","type":"address","indexed":true},{"name":"recipient","type":"address","indexed":true},{"name":"tokensIn","type":"uint256","indexed":false},{"name":"quoteOut","type":"uint256","indexed":false},{"name":"fee","type":"uint256","indexed":false},{"name":"tax","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"FeesSwept","inputs":[{"name":"protocolAmount","type":"uint256","indexed":false},{"name":"buybackAmount","type":"uint256","indexed":false},{"name":"creatorAmount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"FeesRescued","inputs":[{"name":"protocolRecipient","type":"address","indexed":true},{"name":"creatorRecipient","type":"address","indexed":true},{"name":"protocolAmount","type":"uint256","indexed":false},{"name":"creatorAmount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"BuybackLocked","inputs":[{"name":"quoteSpent","type":"uint256","indexed":false},{"name":"tokensLocked","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"CurveCompleted","inputs":[{"name":"recipient","type":"address","indexed":false},{"name":"quoteOut","type":"uint256","indexed":false},{"name":"tokenOut","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"Initialized","inputs":[{"name":"token","type":"address","indexed":false}],"anonymous":false},
	{"type":"event","name":"CreatorFeeRecipientUpdated","inputs":[{"name":"previousRecipient","type":"address","indexed":true},{"name":"newRecipient","type":"address","indexed":true}],"anonymous":false},
	{"type":"event","name":"BuybackEnabledUpdated","inputs":[{"name":"enabled","type":"bool","indexed":false}],"anonymous":false},
	{"type":"event","name":"AutoGraduationFailed","inputs":[{"name":"token","type":"address","indexed":true},{"name":"gasRemaining","type":"uint256","indexed":false}],"anonymous":false},

	{"type":"event","name":"PoolRegistered","inputs":[{"name":"poolId","type":"bytes32","indexed":true},{"name":"memecoin","type":"address","indexed":false},{"name":"quoteToken","type":"address","indexed":false},{"name":"creator","type":"address","indexed":false}],"anonymous":false},
	{"type":"event","name":"CreatorFeeRecipientUpdated","inputs":[{"name":"poolId","type":"bytes32","indexed":true},{"name":"previousRecipient","type":"address","indexed":true},{"name":"newRecipient","type":"address","indexed":true}],"anonymous":false},
	{"type":"event","name":"HookFeeCollected","inputs":[{"name":"poolId","type":"bytes32","indexed":true},{"name":"currency","type":"address","indexed":false},{"name":"feeAmount","type":"uint256","indexed":false},{"name":"taxAmount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"PoolFeesSwept","inputs":[{"name":"poolId","type":"bytes32","indexed":true},{"name":"protocolAmount","type":"uint256","indexed":false},{"name":"buybackAmount","type":"uint256","indexed":false},{"name":"creatorAmount","type":"uint256","indexed":false},{"name":"tokensLocked","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"PoolFeesRescued","inputs":[{"name":"poolId","type":"bytes32","indexed":true},{"name":"quoteToken","type":"address","indexed":true},{"name":"protocolAmount","type":"uint256","indexed":false},{"name":"creatorAmount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"PoolBuybackSkipped","inputs":[{"name":"poolId","type":"bytes32","indexed":true},{"name":"foldedBackQuote","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"PoolConversionSkipped","inputs":[{"name":"poolId","type":"bytes32","indexed":true},{"name":"retainedMemecoin","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"BuybackVaultSet","inputs":[{"name":"vault","type":"address","indexed":false}],"anonymous":false},
	{"type":"event","name":"ProtocolFeeShareUpdated","inputs":[{"name":"bps","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"BuybackBurnBpsUpdated","inputs":[{"name":"bps","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"HookFeeBpsUpdated","inputs":[{"name":"bps","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"MaxInternalPriceImpactUpdated","inputs":[{"name":"bps","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"ProtocolFeeRecipientUpdated","inputs":[{"name":"recipient","type":"address","indexed":false}],"anonymous":false},
	{"type":"event","name":"FeeSweepOperatorUpdated","inputs":[{"name":"operator","type":"address","indexed":false}],"anonymous":false},
	{"type":"event","name":"BuybackEnabledUpdated","inputs":[{"name":"poolId","type":"bytes32","indexed":true},{"name":"enabled","type":"bool","indexed":false}],"anonymous":false},

	{"type":"event","name":"FactorySet","inputs":[{"name":"factory","type":"address","indexed":false}],"anonymous":false},
	{"type":"event","name":"Locked","inputs":[{"name":"token","type":"address","indexed":true},{"name":"depositor","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false},{"name":"newVestingStart","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"VestingTermsSnapshotted","inputs":[{"name":"token","type":"address","indexed":true},{"name":"creatorRecipient","type":"address","indexed":true},{"name":"protocolRecipient","type":"address","indexed":true},{"name":"protocolFeeShareBps","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"Released","inputs":[{"name":"token","type":"address","indexed":true},{"name":"creatorAmount","type":"uint256","indexed":false},{"name":"protocolAmount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"CreatorRecipientUpdated","inputs":[{"name":"token","type":"address","indexed":true},{"name":"previousRecipient","type":"address","indexed":true},{"name":"newRecipient","type":"address","indexed":true}],"anonymous":false},
	{"type":"event","name":"PositionLocked","inputs":[{"name":"token","type":"address","indexed":true},{"name":"tokenId","type":"uint256","indexed":true}],"anonymous":false},
	{"type":"event","name":"TokenSupplyLocked","inputs":[{"name":"token","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"GraduationDustSwept","inputs":[{"name":"launchToken","type":"address","indexed":true},{"name":"currency","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"GraduationDustRetained","inputs":[{"name":"launchToken","type":"address","indexed":true},{"name":"currency","type":"address","indexed":true},{"name":"amount","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"OwnershipTransferStarted","inputs":[{"name":"previousOwner","type":"address","indexed":true},{"name":"newOwner","type":"address","indexed":true}],"anonymous":false},
	{"type":"event","name":"OwnershipTransferred","inputs":[{"name":"previousOwner","type":"address","indexed":true},{"name":"newOwner","type":"address","indexed":true}],"anonymous":false},
	{"type":"event","name":"Transfer","inputs":[{"name":"from","type":"address","indexed":true},{"name":"to","type":"address","indexed":true},{"name":"value","type":"uint256","indexed":false}],"anonymous":false},
	{"type":"event","name":"Approval","inputs":[{"name":"owner","type":"address","indexed":true},{"name":"spender","type":"address","indexed":true},{"name":"value","type":"uint256","indexed":false}],"anonymous":false}
]`

var eventsContractABI, EventsABI = mustABIPair(eventABIJSON)

func mustABIPair(raw string) (abi.ABI, abi.ABI) {
	return mustABI(raw), mustABI(raw)
}

func mustABI(raw string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(raw))
	if err != nil {
		panic(err)
	}
	return parsed
}
