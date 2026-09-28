package payment

import "strings"

// BEpusdtTradeType maps a checkout network to the BEpusdt trade type.
// TRON, Ethereum, and BSC are the networks shown after the user opens USDT.
func BEpusdtTradeType(network string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case USDTNetworkTron:
		return "usdt.trc20", true
	case USDTNetworkEthereum:
		return "usdt.erc20", true
	case USDTNetworkBSC:
		return "usdt.bep20", true
	default:
		return "", false
	}
}
