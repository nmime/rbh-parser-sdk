<div align="center">
    <h1>Pons Parser SDK for Go</h1>
    <h3><em>Go receipt and log parser for pons v2 events and custom errors</em></h3>
</div>

<p align="center">
    <strong>Decode pons v2 launch, curve, hook, vault, locker, graduation, ERC-20 events, and revert data into typed or generic Go values.</strong>
</p>

<p align="center">
    <a href="https://pkg.go.dev/github.com/0xfnzero/pons-parser-sdk">
        <img src="https://pkg.go.dev/badge/github.com/0xfnzero/pons-parser-sdk.svg" alt="Go Reference">
    </a>
    <a href="https://github.com/0xfnzero/pons-parser-sdk/actions/workflows/ci.yml">
        <img src="https://github.com/0xfnzero/pons-parser-sdk/actions/workflows/ci.yml/badge.svg" alt="CI">
    </a>
    <a href="https://github.com/0xfnzero/pons-parser-sdk/releases/latest">
        <img src="https://img.shields.io/github/v/release/0xfnzero/pons-parser-sdk" alt="Release">
    </a>
    <a href="LICENSE">
        <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License">
    </a>
</p>

<p align="center">
    <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
    <img src="https://img.shields.io/badge/EVM-3C3C3D?style=for-the-badge&logo=ethereum&logoColor=white" alt="EVM">
    <img src="https://img.shields.io/badge/Pons-v2-4B8BBE?style=for-the-badge" alt="Pons v2">
</p>

<p align="center">
    <a href="./README_CN.md">中文</a> |
    <a href="./README.md">English</a> |
    <a href="https://fnzero.dev/">Website</a> |
    <a href="https://t.me/fnzero_group">Telegram</a> |
    <a href="https://discord.gg/vuazbGkqQE">Discord</a>
</p>

---

## SDKs

| SDK | Module |
|-----|--------|
| Trade | [`github.com/0xfnzero/pons-trade-sdk`](https://github.com/0xfnzero/pons-trade-sdk) |
| Parser | [`github.com/0xfnzero/pons-parser-sdk`](https://github.com/0xfnzero/pons-parser-sdk) |

## What This SDK Is For

`pons-parser-sdk` is for indexers, monitoring workers, trading bots, analytics systems, and backend APIs that need pons v2 logs decoded from receipts. The parser follows official docs first, then fills incomplete event/error coverage from the public V2 Solidity source.

| Area | Coverage |
|------|----------|
| Inputs | `types.Log`, `types.Receipt`, custom ABI registries, revert data |
| Events | Factory, bonding curve, meme hook, buyback vault, launch locker, graduation executor, ownership changes, ERC-20 `Transfer` / `Approval` |
| Typed events | `TokenLaunched`, `CurveBuy`, `CurveSell` |
| Generic events | All other built-in official events as `GenericEvent` |
| Runtime | Go 1.25+; use a supported, fully patched Go release for production builds |

## Features

1. Built-in pons v2 event topics from the official Solidity source.
2. Typed parsers for launch and bonding-curve trade events.
3. Generic parser for all supported pons v2 operational events.
4. `EventRegistry` for future ABI extensions without changing call sites.
5. Custom error decoding for pons v2 and common dependency errors.
6. Topic filter helpers for launch and curve trade subscriptions.

## Installation

### Direct Clone

```bash
git clone https://github.com/0xfnzero/pons-parser-sdk
cd pons-parser-sdk
go mod tidy
```

Use from another local module:

```go
require github.com/0xfnzero/pons-parser-sdk v0.1.0

replace github.com/0xfnzero/pons-parser-sdk => ../pons-parser-sdk
```

### Go Modules

```bash
go get github.com/0xfnzero/pons-parser-sdk@v0.1.0
```

## Usage Examples

### Parse A Receipt

```go
validator := func(kind ponsparser.EventKind, emitter common.Address) bool {
    switch kind {
    case ponsparser.EventTokenLaunched:
        return emitter == factory
    case ponsparser.EventCurveBuy, ponsparser.EventCurveSell:
        return emitter == curve
    default:
        return false
    }
}
events, err := ponsparser.ParseReceiptEventsFiltered(receipt, validator)
if err != nil {
    panic(err)
}

for _, ev := range events {
    switch data := ev.Event.(type) {
    case ponsparser.TokenLaunched:
        fmt.Println("launched", data.Token.Hex(), data.Curve.Hex())
    case ponsparser.CurveBuy:
        fmt.Println("buy", data.Buyer.Hex(), data.TokensOut)
    case ponsparser.GenericEvent:
        fmt.Println(data.RawName, data.Values)
    }
}
```

### Parse One Log

```go
parsed, ok, err := ponsparser.ParseLogValidated(log, validator)
if err != nil {
    panic(err)
}
if ok {
    fmt.Printf("%s %#v\n", parsed.Kind, parsed.Event)
    if parsed.Log.Removed {
        fmt.Println("remove this event because of a chain reorganization")
    }
}
```

### Validate Event Sources

Topic signatures do not authenticate an emitter. The examples above therefore use source-filtered parsing. For indexers and automated trading systems, bind every event role to its expected contract instead of using one flat address set:

```go
validator := func(kind ponsparser.EventKind, emitter common.Address) bool {
    switch kind {
    case ponsparser.EventTokenLaunched:
        return emitter == factory
    case ponsparser.EventCurveBuy, ponsparser.EventCurveSell:
        return emitter == curve
    case ponsparser.EventKind("Transfer"), ponsparser.EventKind("Approval"):
        return emitter == token
    default:
        return false
    }
}
parsed, ok, err := ponsparser.ParseLogValidated(log, validator)
if errors.Is(err, ponsparser.ErrUntrustedSource) {
    // Reject a matching event signature emitted by an unknown contract.
}
```

Use `ParseReceiptEventsFiltered` or `ParseLogsFiltered` for normal multi-contract receipts. They decode trusted events and skip recognized signatures from other contracts, such as the token `Transfer` emitted before the factory's `TokenLaunched`. Use the strict `Validated` variants when an unexpected matching signature must abort processing as a security signal. Trusted malformed logs always return an error in both modes.

### Decode Revert Data

```go
decoded, ok, err := ponsparser.ParseError(revertData)
if err != nil {
    panic(err)
}
if ok {
    fmt.Println(decoded.Name, decoded.Values)
}
```

### Build Filter Topics

```go
query := ethereum.FilterQuery{
    Addresses: []common.Address{factory, curve},
    Topics:    ponsparser.TopicsForCurveTrades(),
}
```

## Project Structure

```text
.
├── ponsparser/                # Parser SDK package
├── examples/parse_receipt/    # Basic receipt parser example
├── .github/workflows/ci.yml   # GitHub Actions verification
├── go.mod
└── go.sum
```

## Development

```bash
go test ./...
```

## Important Notes

1. Public integration behavior follows [docs.ponsfamily.com/v2](https://docs.ponsfamily.com/v2). Event and error details that are incomplete in the docs are filled from `contractsV2/src/v2` in [ponsdotdev/ponsfamily](https://github.com/ponsdotdev/ponsfamily).
2. Events with overloaded raw names are returned as `GenericEvent`; use `RawName` for display and `Name` for ABI-internal uniqueness.
3. Indexed dynamic Solidity values, if added in future ABIs, can only be recovered as topic hashes.
4. Pair this SDK with `pons-trade-sdk` when you need calldata builders or contract reads.
5. `ParseLog` and `ParseReceiptEvents` are low-level, unauthenticated decoders. Production receipt consumers should default to `Filtered`; use strict `Validated` parsing when an untrusted matching topic must be treated as an error.
6. `ParsedEvent.Log` preserves block, transaction, index, and `Removed` metadata so indexers can reverse events after a chain reorganization.
7. `AddressAllowlist` is a convenience for sources with the same trust role. Use a kind-aware `SourceValidator` when different contracts are allowed to emit different events.
8. Recognized events require an exact topic count and canonical indexed address, boolean, integer, and fixed-bytes encodings; malformed trusted logs return an error.
9. `ParseError` distinguishes Solidity `Panic(uint256)` from `Error(string)`. Exported ABI variables are compatibility snapshots and cannot mutate the parser's internal registry.
10. Recognized log and revert payloads are limited to 1 MiB and must use canonical ABI encoding; trailing data, malformed offsets, and ambiguous 4-byte custom-error selectors return errors.
11. `ParsedEvent.Log` is a shallow copy for hot-path efficiency: its `Data` and `Topics` slices retain caller-owned backing storage. Copy those slices before mutating the source log or sharing it with code that may mutate it concurrently.

## License

MIT

## Contact

- Website: [fnzero.dev](https://fnzero.dev/)
- Telegram: [fnzero_group](https://t.me/fnzero_group)
- Discord: [FnZero Discord](https://discord.gg/vuazbGkqQE)
