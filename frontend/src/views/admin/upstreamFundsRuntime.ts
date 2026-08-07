import type { UpstreamRechargeOrder, UpstreamWallet } from '@/api/admin'

export const UPSTREAM_BALANCE_SYNC_INTERVAL_MS = 10_000
export const UPSTREAM_RECHARGE_POLL_BASE_MS = 2_500
export const UPSTREAM_RECHARGE_POLL_MAX_MS = 20_000

export type UpstreamBalanceStatus = 'healthy' | 'normal' | 'alert' | 'unknown'

export function classifyUpstreamBalance(balance: number | null | undefined): UpstreamBalanceStatus {
  if (balance === null || balance === undefined || !Number.isFinite(balance)) return 'unknown'
  if (balance >= 100) return 'healthy'
  if (balance >= 50) return 'normal'
  return 'alert'
}

export function updateUpstreamSyncCatalog(
  current: UpstreamWallet[],
  loaded: UpstreamWallet[],
  search: string,
  groupID?: number | null
): UpstreamWallet[] {
  return search.trim() === '' && !groupID ? loaded : current
}

export function selectUpstreamBalanceSyncTargets(wallets: UpstreamWallet[]): UpstreamWallet[] {
  return wallets.filter(wallet => wallet.enabled && wallet.adapter_configured)
}

export function nextUpstreamRechargePollDelay(consecutiveFailures: number): number {
  const exponent = Math.max(0, Math.min(16, Math.floor(consecutiveFailures) - 1))
  return Math.min(UPSTREAM_RECHARGE_POLL_MAX_MS, UPSTREAM_RECHARGE_POLL_BASE_MS * (2 ** exponent))
}

export function isUpstreamRechargePollingTerminal(status: UpstreamRechargeOrder['status']): boolean {
  return status === 'completed' || status === 'manual_review' || status === 'failed' || status === 'expired' || status === 'cancelled'
}
