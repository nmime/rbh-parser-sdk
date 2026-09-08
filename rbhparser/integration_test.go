package rbhparser

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const robinhoodMainnetRPCEnv = "ROBINHOOD_RPC_URL"

var robinhoodLaunchFixtures = []struct {
	name     string
	protocol Protocol
	txHash   common.Hash
}{
	{name: "pons-v2", protocol: ProtocolPonsV2, txHash: common.HexToHash("0x5bd89d6e64d66cf17fa0fa6c8f0c6137f36485b4ec2f8c53620d7678b1323e36")},
	{name: "long", protocol: ProtocolLong, txHash: common.HexToHash("0x4390d4135d71677415791d89ca840c1c9c616dbd55007812dc259707667668e2")},
	{name: "o1", protocol: ProtocolO1, txHash: common.HexToHash("0x3cd7e17ca289c38b44ee8a7bb8b144ef5fa0198cef35853d0d20ecc8e63763a6")},
	{name: "pools.trade", protocol: ProtocolPoolsTrade, txHash: common.HexToHash("0x386756ed924c54acda79f3b76804b8a8e4bf5f76501c93a60dd976a3216021be")},
	{name: "pair", protocol: ProtocolPAIR, txHash: common.HexToHash("0x81f2955becbc47ccd5c1a6214a6b65de32692c7d619ea8310d3b1fb67fbd4483")},
}

func liveClient(t *testing.T) (*ethclient.Client, context.Context, context.CancelFunc) {
	t.Helper()
	rpcURL := os.Getenv(robinhoodMainnetRPCEnv)
	if rpcURL == "" {
		t.Skipf("set %s to run Robinhood mainnet checks", robinhoodMainnetRPCEnv)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	return client, ctx, func() {
		client.Close()
		cancel()
	}
}

func TestRobinhoodProtocolDeployments(t *testing.T) {
	client, ctx, closeClient := liveClient(t)
	defer closeClient()

	addresses := DefaultAddressBook()
	deployments := map[string]common.Address{
		"pool-manager": addresses.PoolManager,
		"pons-v2":      addresses.PonsFactory,
		"long":         addresses.LongLauncher,
		"o1":           addresses.O1Factory,
		"pools.trade":  addresses.PoolsEntry,
		"pair":         addresses.PAIRLaunchpad,
		"bags-v2":      addresses.BagsFactory,
	}
	for name, address := range deployments {
		t.Run(name, func(t *testing.T) {
			code, err := client.CodeAt(ctx, address, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(code) == 0 {
				t.Fatalf("no runtime code at %s", address)
			}
		})
	}
}

func TestRobinhoodLaunchReceipts(t *testing.T) {
	client, ctx, closeClient := liveClient(t)
	defer closeClient()

	for _, fixture := range robinhoodLaunchFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			receipt, err := client.TransactionReceipt(ctx, fixture.txHash)
			if err != nil {
				t.Fatal(err)
			}
			if receipt.Status != gethtypes.ReceiptStatusSuccessful {
				t.Fatalf("fixture transaction reverted: %s", fixture.txHash)
			}
			events, err := New().ParseReceipt(receipt)
			if err != nil {
				t.Fatal(err)
			}
			launch, err := findLaunch(events, fixture.protocol)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateLiveLaunch(launch); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func findLaunch(events []Event, protocol Protocol) (Launch, error) {
	for _, event := range events {
		if event.Kind != EventLaunch || event.Protocol != protocol {
			continue
		}
		launch, ok := event.Data.(Launch)
		if !ok {
			return Launch{}, fmt.Errorf("%s launch has unexpected payload %T", protocol, event.Data)
		}
		return launch, nil
	}
	return Launch{}, fmt.Errorf("%s launch event not found", protocol)
}

func validateLiveLaunch(launch Launch) error {
	if launch.Token == (common.Address{}) {
		return fmt.Errorf("%s launch has zero token", launch.Protocol)
	}
	switch launch.Protocol {
	case ProtocolPonsV2:
		if launch.Curve == (common.Address{}) || launch.Quote == (common.Address{}) || launch.Creator == (common.Address{}) {
			return fmt.Errorf("incomplete Pons V2 launch: %#v", launch)
		}
	case ProtocolLong:
		if launch.Creator == (common.Address{}) || launch.Pool == nil || launch.PoolID == (common.Hash{}) || !poolContainsToken(*launch.Pool, launch.Token) {
			return fmt.Errorf("incomplete Long launch: %#v", launch)
		}
	case ProtocolO1:
		if launch.Creator == (common.Address{}) || launch.Pool == nil || launch.PoolID == (common.Hash{}) || !poolContainsToken(*launch.Pool, launch.Token) {
			return fmt.Errorf("incomplete o1 launch: %#v", launch)
		}
	case ProtocolPoolsTrade:
		// TokenCreated(address) does not include the eventual pool key.
	case ProtocolPAIR:
		if launch.Quote == (common.Address{}) || launch.PoolID == (common.Hash{}) {
			return fmt.Errorf("incomplete PAIR launch: %#v", launch)
		}
	default:
		return fmt.Errorf("unexpected live fixture protocol %s", launch.Protocol)
	}
	return nil
}

func poolContainsToken(pool PoolKey, token common.Address) bool {
	return pool.Currency0 == token || pool.Currency1 == token
}
