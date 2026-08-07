import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn()
}))

vi.mock('../client', () => ({
  apiClient: { get, post, put }
}))

import { create, list, listAccounts, recordBalance, update } from '../admin/upstreamFunds'

describe('admin upstream funds API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads the wallet overview with an optional search term', async () => {
    const overview = { summary: { wallet_count: 0 }, wallets: [] }
    get.mockResolvedValueOnce({ data: overview })

    await expect(list('provider_a')).resolves.toBe(overview)
    expect(get).toHaveBeenCalledWith('/admin/upstream-funds/wallets', {
      params: { search: 'provider_a' }
    })
  })

  it('creates and updates wallets with the same input contract', async () => {
    const input = {
      name: 'Primary wallet',
      provider: 'provider_a',
      currency: 'USD',
      recharge_mode: 'manual' as const,
      tier: 'primary' as const,
      enabled: true,
      alert_days: 2,
      target_days: 7,
      account_ids: [3, 9]
    }
    const wallet = { id: 12, ...input }
    post.mockResolvedValueOnce({ data: wallet })
    put.mockResolvedValueOnce({ data: wallet })

    await expect(create(input)).resolves.toBe(wallet)
    await expect(update(12, input)).resolves.toBe(wallet)
    expect(post).toHaveBeenCalledWith('/admin/upstream-funds/wallets', input)
    expect(put).toHaveBeenCalledWith('/admin/upstream-funds/wallets/12', input)
  })

  it('records a manual balance snapshot and lists assignable accounts', async () => {
    const wallet = { id: 12, balance: 88.5 }
    const accounts = [{ id: 3, name: 'Account A', platform: 'openai' }]
    post.mockResolvedValueOnce({ data: wallet })
    get.mockResolvedValueOnce({ data: accounts })

    await expect(recordBalance(12, 88.5)).resolves.toBe(wallet)
    await expect(listAccounts()).resolves.toBe(accounts)
    expect(post).toHaveBeenCalledWith(
      '/admin/upstream-funds/wallets/12/refresh-balance',
      { balance: 88.5 }
    )
    expect(get).toHaveBeenCalledWith('/admin/upstream-funds/accounts')
  })
})
