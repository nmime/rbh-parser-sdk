package rbhparser

import (
	"fmt"
	"strings"
)

type Protocol uint8

const (
	ProtocolUnknown Protocol = iota
	ProtocolPonsV2
	ProtocolLong
	ProtocolO1
	ProtocolPoolsTrade
	ProtocolPAIR
	ProtocolBagsV2
)

func (p Protocol) Valid() bool { return p >= ProtocolPonsV2 && p <= ProtocolBagsV2 }

func (p Protocol) String() string {
	switch p {
	case ProtocolPonsV2:
		return "pons-v2"
	case ProtocolLong:
		return "long"
	case ProtocolO1:
		return "o1"
	case ProtocolPoolsTrade:
		return "pools.trade"
	case ProtocolPAIR:
		return "pair"
	case ProtocolBagsV2:
		return "bags-v2"
	default:
		return "unknown"
	}
}

// ProtocolCapabilities describes which protocol-specific facts the parser can
// establish without an RPC lookup. It is deliberately separate from trading
// support: discovering a launch does not imply that its output address or hook
// pricing can be predicted before execution.
type ProtocolCapabilities struct {
	ConfirmedLaunch      bool
	PendingLaunchIntent  bool
	CurveTrades          bool
	PendingCurveIntents  bool
	RegistersV4Pools     bool
	PendingV4SwapIntents bool
}

type ProtocolInfo struct {
	Protocol     Protocol
	Capabilities ProtocolCapabilities
}

var supportedProtocols = [...]ProtocolInfo{
	{Protocol: ProtocolPonsV2, Capabilities: ProtocolCapabilities{ConfirmedLaunch: true, PendingLaunchIntent: true, CurveTrades: true, PendingCurveIntents: true, RegistersV4Pools: true, PendingV4SwapIntents: true}},
	{Protocol: ProtocolLong, Capabilities: ProtocolCapabilities{ConfirmedLaunch: true, PendingLaunchIntent: true, RegistersV4Pools: true, PendingV4SwapIntents: true}},
	{Protocol: ProtocolO1, Capabilities: ProtocolCapabilities{ConfirmedLaunch: true, PendingLaunchIntent: true, RegistersV4Pools: true, PendingV4SwapIntents: true}},
	{Protocol: ProtocolPoolsTrade, Capabilities: ProtocolCapabilities{ConfirmedLaunch: true, PendingLaunchIntent: true, RegistersV4Pools: true, PendingV4SwapIntents: true}},
	{Protocol: ProtocolPAIR, Capabilities: ProtocolCapabilities{ConfirmedLaunch: true, PendingLaunchIntent: true, RegistersV4Pools: true, PendingV4SwapIntents: true}},
	{Protocol: ProtocolBagsV2, Capabilities: ProtocolCapabilities{ConfirmedLaunch: true, PendingLaunchIntent: true, CurveTrades: true, PendingCurveIntents: true, RegistersV4Pools: true, PendingV4SwapIntents: true}},
}

func SupportedProtocols() []ProtocolInfo {
	out := make([]ProtocolInfo, len(supportedProtocols))
	copy(out, supportedProtocols[:])
	return out
}

func ParseProtocol(value string) (Protocol, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, info := range supportedProtocols {
		if info.Protocol.String() == value {
			return info.Protocol, nil
		}
	}
	return ProtocolUnknown, fmt.Errorf("unsupported protocol %q", value)
}
