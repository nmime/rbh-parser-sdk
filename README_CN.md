<div align="center">
  <h1>RBH Parser SDK for Go</h1>
  <h3><em>面向 Robinhood Chain 事件、Receipt 与 calldata 的低延迟解析 SDK</em></h3>
</div>

<p align="center">
  <strong>类型化解析 Pons V2、Long、o1、Pools.trade、PAIR、Bags V2 与 Uniswap v4。</strong>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/0xfnzero/rbh-parser-sdk"><img src="https://pkg.go.dev/badge/github.com/0xfnzero/rbh-parser-sdk.svg" alt="Go Reference"></a>
  <a href="https://github.com/0xfnzero/rbh-parser-sdk/releases/latest"><img src="https://img.shields.io/github/v/release/0xfnzero/rbh-parser-sdk" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
</p>

<p align="center">
  <a href="./README_CN.md">中文</a> |
  <a href="./README.md">English</a> |
  <a href="https://fnzero.dev/">Website</a> |
  <a href="https://t.me/fnzero_group">Telegram</a> |
  <a href="https://discord.gg/vuazbGkqQE">Discord</a>
</p>

SDK 与数据传输层解耦：只解析由应用传入的数据，不包含官方 Feed、
BlockRazor、WebSocket、RPC 订阅或其他网络客户端实现。

## SDK

| SDK | Go Module |
|-----|-----------|
| Parser | [`github.com/0xfnzero/rbh-parser-sdk`](https://github.com/0xfnzero/rbh-parser-sdk) |
| Trade | [`github.com/0xfnzero/rbh-trade-sdk`](https://github.com/0xfnzero/rbh-trade-sdk) |

## 支持的协议

| 协议 | Launch 事件 | 内盘交易 | Uniswap v4 池与交易 | 交易意图 |
|------|-------------|----------|------------------------|----------|
| Pons V2 | 支持 | 支持 | 支持 | 支持 |
| Long | 支持 | 不适用 | 支持 | 支持 |
| o1 | 支持 | 不适用 | 支持 | 支持 |
| Pools.trade | 支持 | 不适用 | 支持 | 支持 |
| PAIR | 支持 | 不适用 | 支持 | 支持 |
| Bags V2 | 支持 | 支持 | 支持 | 支持 |

可通过 `SupportedProtocols()` 获取机器可读的能力列表，通过
`ParseProtocol()` 解析配置中的协议名称。

## 正确性保证

- 使用 `(emitter, topic0)` 路由 Launch 事件，不会只依赖 topic。
- 严格校验 ABI word、带符号整数、地址填充、动态偏移及数据长度上限。
- 重建并校验 Uniswap v4 `PoolKey` 与 pool ID。
- 使用并发安全的本地注册表关联 Launch、初始化、流动性、Swap 和内盘事件。
- 在解析同一 Receipt 的首笔 Curve/v4 成交前完成新 Curve 和 Pool 注册。
- Bags 买入的 `CurveTrade.AmountIn` 表示扣除退款后的实际成交输入；提交金额、
  退款、费用拆分与交易后虚拟储备保存在 `CurveTrade.Bags`。
- Receipt 解析采用事务式注册表更新，解析失败不会留下部分状态。
- 支持已知 curve 和 pool 注册信息的快照恢复。
- 即使未观察到更早的 Bags `TokenCreated`，精确匹配官方 Bags Hook、WETH、dynamic fee 和 tick spacing 的 `Initialize` 也会注册为 Bags V2；官方 Hook 只允许其已注册 bonding curve 初始化该池。
- 不会隐式调用 RPC 或任何网络接口。

## 安装

```bash
go get github.com/0xfnzero/rbh-parser-sdk@v0.3.0
```

## 解析 Receipt

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

当同一笔交易内包含 Launch、池初始化和首次 Swap 时，应优先使用
`ParseReceipt`。完整 Receipt 解析成功后才会提交注册表变更。

## 解析日志流

每个有序日志流应长期复用同一个 parser，使注册表持续积累 curve 和 pool：

```go
parser := rbhparser.New()

event, ok, err := parser.ParseLog(log)
if err != nil {
    // 拒绝或隔离格式错误的协议数据。
}
if ok {
    // 处理归一化事件。
}
```

Launch 和迁移交易应尽量传入完整 Receipt。单独的 Uniswap v4
`Initialize` 事件可能不足以判断其所属 Launchpad。

## 解析交易 calldata

应用可以把任意来源的 EVM 交易传给意图解析器，SDK 不连接也不依赖数据来源：

```go
if parser.IsPotentialIntent(tx) {
    intents, err := parser.ParseTransactionIntents(tx, sender)
    if err != nil {
        return err
    }
    _ = intents
}
```

`IsPotentialIntentWithFilter` 会先进行低成本的合约和 selector 检查，再做完整
ABI 解码。交易意图只代表已提交的 calldata，不代表最终执行结果或成交数量。

## 过滤

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

过滤输出不会阻止内部注册表更新，因此即使隐藏 Launch 通知，后续 pool 和
curve 归属仍然正确。

## 恢复注册表

```go
registry := rbhparser.NewRegistry()
if err := registry.Restore(snapshot); err != nil {
    return err
}
parser := rbhparser.NewWithRegistry(rbhparser.DefaultAddressBook(), registry)
```

生产环境只应持久化 finalized 状态；推测状态应由应用放在独立 overlay 注册表。

## 开发

```bash
go test ./...
go test -race ./...
go vet ./...
```

在线部署与 Receipt 校验默认跳过，需要显式提供 RPC。测试会检查六个平台合约
的 runtime code，并解析 Pons V2、Long、o1、Pools.trade、PAIR 的固定主网
Launch Receipt。Bags V2 主网工厂尚未发出首次 Launch 事件，因此其 Launch
解码目前由离线 ABI 测试覆盖。

```bash
ROBINHOOD_RPC_URL=https://rpc.mainnet.chain.robinhood.com go test ./rbhparser -run TestRobinhood -count=1 -v
```

## License

MIT，详见 [LICENSE](LICENSE)。
