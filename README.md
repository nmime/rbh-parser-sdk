# RBH Parser SDK for Go

Low-latency, multi-protocol event and calldata parsing for Robinhood Chain.

The SDK is transport agnostic. It parses values supplied by the application and
does not include official Feed, BlockRazor, WebSocket, RPC subscription, or
network client implementations.

## Supported protocols

| Protocol | Launch events | Curve trades | Uniswap v4 pools and swaps | Transaction intents |
|----------|---------------|--------------|----------------------------|---------------------|
| Pons V2 | Yes | Yes | Yes | Yes |
| Long | Yes | No | Yes | Yes |
| o1 | Yes | No | Yes | Yes |
| Pools.trade | Yes | No | Yes | Yes |
| PAIR | Yes | No | Yes | Yes |
| Bags V2 | Yes | Yes | Yes | Yes |

Use `SupportedProtocols()` for machine-readable capabilities and
`ParseProtocol()` for configuration values.

## Guarantees

- Routes launch events by `(emitter, topic0)`, not by topic alone.
- Validates ABI words, signed integers, address padding, dynamic offsets, and
  bounded payload sizes.
- Reconstructs and verifies Uniswap v4 `PoolKey` and pool IDs.
- Associates launch, initialization, liquidity, swap, and curve events through
  a concurrency-safe local registry.
- Applies receipt registry updates transactionally: a malformed receipt cannot
  partially mutate parser state.
- Supports snapshot and restore of known curve and pool registrations.
- Performs no hidden RPC or network calls.

## Installation

```bash
go get github.com/0xfnzero/rbh-parser-sdk@latest
```

## Parse a receipt

```go
package main

import (
    "fmt"

    sdk "github.com/0xfnzero/rbh-parser-sdk/rbhparser"
    gethtypes "github.com/ethereum/go-ethereum/core/types"
)

func parse(receipt *gethtypes.Receipt) error {
    parser := sdk.New()
    events, err := parser.ParseReceipt(receipt)
    if err != nil {
        return err
    }
    for _, event := range events {
        fmt.Printf("protocol=%s kind=%s data=%#v\n", event.Protocol, event.Kind, event.Data)
    }
    return nil
}
```

`ParseReceipt` is preferred when a transaction can launch and initialize a pool
before its first swap. The parser stages all registry changes and commits them
only after the complete receipt succeeds.

## Parse a log stream

Keep one parser per ordered stream so its registry accumulates launches and
pools:

```go
parser := rbhparser.New()

event, ok, err := parser.ParseLog(log)
if err != nil {
    // Reject or quarantine malformed protocol data.
}
if ok {
    // Process the normalized event.
}
```

For lifecycle association, supply complete receipts for launch and migration
transactions. A bare Uniswap v4 `Initialize` event may not contain enough
information to distinguish launchpad ownership.

## Parse transaction calldata

Applications may pass any decoded EVM transaction directly to the intent
parser. The SDK does not connect to or depend on its source:

```go
if parser.IsPotentialIntent(tx) {
    intents, err := parser.ParseTransactionIntents(tx, sender)
    if err != nil {
        return err
    }
    _ = intents
}
```

`IsPotentialIntentWithFilter` performs a cheap contract and selector check
before full ABI decoding. Transaction intents describe submitted calldata, not
confirmed execution or fill amounts.

## Filters

```go
filter := rbhparser.EventFilter{
    IncludeProtocols: rbhparser.Protocols(
        rbhparser.ProtocolPonsV2,
        rbhparser.ProtocolBagsV2,
    ),
    IncludeKinds: rbhparser.EventKinds(
        rbhparser.EventCurveBuy,
        rbhparser.EventSwap,
    ),
}

events, err := parser.ParseReceiptFiltered(receipt, filter)
```

Excluded output does not prevent internal registry updates. This keeps later
pool and curve attribution correct when launch notifications are filtered out.

## Restore known registrations

```go
registry := rbhparser.NewRegistry()
if err := registry.Restore(snapshot); err != nil {
    return err
}
parser := rbhparser.NewWithRegistry(rbhparser.DefaultAddressBook(), registry)
```

Persist only finalized state in production and apply speculative state to an
overlay registry owned by the application.

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
```

Live deployment verification is opt-in:

```bash
ROBINHOOD_RPC_URL=https://rpc.mainnet.chain.robinhood.com go test ./rbhparser -run TestRobinhoodDeployment
```

## License

MIT. See [LICENSE](LICENSE).
