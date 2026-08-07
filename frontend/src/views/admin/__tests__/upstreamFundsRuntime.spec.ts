import { describe, expect, it } from 'vitest'
import type { UpstreamRechargeOrder, UpstreamWallet } from '@/api/admin'
import {
  isUpstreamRechargePollingTerminal,
  nextUpstreamRechargePollDelay,
  selectUpstreamBalanceSyncTargets,
  updateUpstreamSyncCatalog,
  UPSTREAM_BALANCE_SYNC_INTERVAL_MS
} from '../upstreamFundsRuntime'

function wallet(id: number, enabled = true, adapterConfigured = true): UpstreamWallet {
  return { id, enabled, adapter_configured: adapterConfigured } as UpstreamWallet
}

describe('upstream funds runtime controls', () => {
  it('preserves the unfiltered sync catalog while the visible list is searched', () => {
    const complete = [wallet(1), wallet(2)]
    const searched = [wallet(2)]

    expect(updateUpstreamSyncCatalog(complete, searched, 'provider-b')).toBe(complete)
    expect(updateUpstreamSyncCatalog(complete, searched, '')).toBe(searched)
    expect(selectUpstreamBalanceSyncTargets(complete).map(item => item.id)).toEqual([1, 2])
  })

  it('syncs only enabled wallets with a configured adapter', () => {
		expect(UPSTREAM_BALANCE_SYNC_INTERVAL_MS).toBe(10_000)
    const targets = selectUpstreamBalanceSyncTargets([
      wallet(1),
      wallet(2, false, true),
      wallet(3, true, false)
    ])
    expect(targets.map(item => item.id)).toEqual([1])
  })

  it('uses capped exponential backoff after transient poll failures', () => {
    expect(nextUpstreamRechargePollDelay(1)).toBe(2_500)
    expect(nextUpstreamRechargePollDelay(2)).toBe(5_000)
    expect(nextUpstreamRechargePollDelay(3)).toBe(10_000)
    expect(nextUpstreamRechargePollDelay(4)).toBe(20_000)
    expect(nextUpstreamRechargePollDelay(20)).toBe(20_000)
  })

  it('stops polling for every terminal or manual-review state', () => {
    const terminal: UpstreamRechargeOrder['status'][] = ['completed', 'manual_review', 'failed', 'expired', 'cancelled']
    expect(terminal.every(isUpstreamRechargePollingTerminal)).toBe(true)
    expect(isUpstreamRechargePollingTerminal('pending_payment')).toBe(false)
    expect(isUpstreamRechargePollingTerminal('verifying')).toBe(false)
  })
})
