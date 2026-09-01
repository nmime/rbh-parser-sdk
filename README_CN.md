<div align="center">
    <h1>Pons Parser SDK for Go</h1>
    <h3><em>用于 pons v2 事件与自定义错误的 Go receipt/log 解析 SDK</em></h3>
</div>

<p align="center">
    <strong>将 pons v2 的 launch、curve、hook、vault、locker、graduation、ERC-20 事件和 revert data 解码为类型化或通用 Go 值。</strong>
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

## SDK

| SDK | Module |
|-----|--------|
| Trade | [`github.com/0xfnzero/pons-trade-sdk`](https://github.com/0xfnzero/pons-trade-sdk) |
| Parser | [`github.com/0xfnzero/pons-parser-sdk`](https://github.com/0xfnzero/pons-parser-sdk) |

## 这个 SDK 适合什么场景

`pons-parser-sdk` 适合索引器、监控 worker、交易机器人、分析系统和后端 API，从交易 receipt 中解析 pons v2 日志。解析行为以官网文档为第一优先级；官网没有完整列出的事件和错误细节，再由公开 V2 Solidity 源码补齐。

| 范围 | 覆盖内容 |
|------|----------|
| 输入 | `types.Log`、`types.Receipt`、自定义 ABI registry、revert data |
| 事件 | Factory、bonding curve、meme hook、buyback vault、launch locker、graduation executor、管理权变更、ERC-20 `Transfer` / `Approval` |
| 类型化事件 | `TokenLaunched`、`CurveBuy`、`CurveSell` |
| 通用事件 | 其他内置官方事件以 `GenericEvent` 返回 |
| 运行环境 | Go 1.25+；生产构建请使用仍受支持且已安装安全补丁的 Go 版本 |

## 功能

1. 内置来自官方 Solidity 源码的 pons v2 event topic。
2. 对 launch 和 bonding-curve trade 事件提供类型化解析。
3. 对其他 pons v2 运维事件提供通用解析。
4. 提供 `EventRegistry`，后续 ABI 扩展不需要改调用方代码。
5. 支持 pons v2 和常见依赖合约的 custom error 解码。
6. 提供 launch 和 curve trade 订阅 topic helper。

## 安装

### 源码克隆

```bash
git clone https://github.com/0xfnzero/pons-parser-sdk
cd pons-parser-sdk
go mod tidy
```

在另一个本地模块中引用：

```go
require github.com/0xfnzero/pons-parser-sdk v0.1.0

replace github.com/0xfnzero/pons-parser-sdk => ../pons-parser-sdk
```

### Go Modules

```bash
go get github.com/0xfnzero/pons-parser-sdk@v0.1.0
```

## 使用示例

### 解析 Receipt

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

### 解析单条 Log

```go
parsed, ok, err := ponsparser.ParseLogValidated(log, validator)
if err != nil {
    panic(err)
}
if ok {
    fmt.Printf("%s %#v\n", parsed.Kind, parsed.Event)
    if parsed.Log.Removed {
        fmt.Println("链重组：撤销该事件")
    }
}
```

普通的多合约 receipt 应使用 `ParseReceiptEventsFiltered` 或 `ParseLogsFiltered`：它们解析可信事件，并跳过其他合约发出的已知签名，例如 Factory 发出 `TokenLaunched` 前 token 构造函数产生的 `Transfer`。当未知来源出现相同签名必须作为安全告警并中止时，使用严格的 `Validated` 接口。两种模式遇到可信但格式错误的日志都会返回错误。

### 验证事件来源

Topic 签名不能证明事件发送方可信，因此上面的示例默认使用带来源过滤的接口。索引器和自动交易系统应按事件角色绑定预期合约，不要只使用一个平面地址集合：

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
    // 拒绝未知合约伪造的同签名事件。
}
```

### 解码 Revert Data

```go
decoded, ok, err := ponsparser.ParseError(revertData)
if err != nil {
    panic(err)
}
if ok {
    fmt.Println(decoded.Name, decoded.Values)
}
```

### 构建过滤 Topic

```go
query := ethereum.FilterQuery{
    Addresses: []common.Address{factory, curve},
    Topics:    ponsparser.TopicsForCurveTrades(),
}
```

## 项目结构

```text
.
├── ponsparser/                # Parser SDK package
├── examples/parse_receipt/    # 基础 receipt 解析示例
├── .github/workflows/ci.yml   # GitHub Actions 验证
├── go.mod
└── go.sum
```

## 开发

```bash
go test ./...
```

## 重要说明

1. 公开集成行为以 [docs.ponsfamily.com/v2](https://docs.ponsfamily.com/v2) 为准。官网未完整列出的事件和错误细节，由 [ponsdotdev/ponsfamily](https://github.com/ponsdotdev/ponsfamily) 的 `contractsV2/src/v2` 补齐。
2. 原始事件名重载时会以 `GenericEvent` 返回；展示时用 `RawName`，需要 ABI 内部唯一名时用 `Name`。
3. Solidity indexed 动态类型如果未来加入 ABI，只能恢复为 topic hash。
4. 需要 calldata builder 或合约读取时，请配合 `pons-trade-sdk` 使用。
5. `ParseLog` 和 `ParseReceiptEvents` 是不验证发送方的底层解码接口；生产 receipt 解析应默认使用 `Filtered`，需要把不可信同 topic 事件视为错误时使用严格的 `Validated` 接口。
6. `ParsedEvent.Log` 保留区块、交易、日志索引和 `Removed` 元数据，索引器可在链重组后撤销事件。
7. `AddressAllowlist` 适合信任角色相同的来源。不同合约只允许发送特定事件时，请使用感知 `EventKind` 的 `SourceValidator`。
8. 已识别事件必须具有准确的 topic 数量，并使用规范的 indexed address、bool、整数和定长 bytes 编码；可信来源的畸形日志会返回错误。
9. `ParseError` 会区分 Solidity `Panic(uint256)` 与 `Error(string)`。导出的 ABI 变量仅为兼容性快照，无法修改 parser 的内部 registry。
10. 已识别的 log 和 revert payload 上限为 1 MiB，且必须是 canonical ABI 编码；尾随数据、畸形偏移以及存在歧义的 4-byte custom-error selector 都会返回错误。
11. 为保持热路径效率，`ParsedEvent.Log` 是浅拷贝，`Data` 和 `Topics` 仍引用调用方的底层切片。修改原始 log 或交给可能并发修改的代码前，请先复制这两个切片。

## 许可证

MIT

## 联系

- Website: [fnzero.dev](https://fnzero.dev/)
- Telegram: [fnzero_group](https://t.me/fnzero_group)
- Discord: [FnZero Discord](https://discord.gg/vuazbGkqQE)
