import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
	del: vi.fn()
}))

vi.mock('../client', () => ({
  apiClient: { get, post, put, delete: del }
}))

import {
	create,
	createRechargeOrder,
	checkPanelSession,
	completePanelSessionTwoFactor,
	remove,
	deletePanelSession,
	getPanelSession,
	importPanelSession,
	list,
	listAll,
	listAccounts,
  listPaymentChannels,
	loginPanelSession,
  manualCompleteRechargeOrder,
  pollRechargeOrder,
  recordBalance,
  redeemCode,
	refreshBalance,
	syncWallets,
  update
} from '../admin/upstreamFunds'

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

		get.mockResolvedValueOnce({ data: overview })
		await expect(list(undefined, 17)).resolves.toBe(overview)
		expect(get).toHaveBeenLastCalledWith('/admin/upstream-funds/wallets', {
			params: { group_id: 17 }
		})

		get.mockResolvedValueOnce({ data: overview })
		await expect(listAll()).resolves.toBe(overview)
		expect(get).toHaveBeenLastCalledWith('/admin/upstream-funds/wallets', {
			params: { search: '' }
		})
	})

  it('creates and updates wallets with the same input contract', async () => {
    const input = {
      name: 'Primary wallet',
      provider: 'provider_a',
      currency: 'USD',
      recharge_mode: 'manual' as const,
			card_site_url: '',
      enabled: true,
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

	it('deletes and synchronizes wallets through dedicated commands', async () => {
		const syncResult = { domains: 2, created_wallets: 1, classified_wallets: 1, linked_accounts: 3, skipped_accounts: 4 }
		del.mockResolvedValueOnce({ data: { deleted: true } })
		post.mockResolvedValueOnce({ data: syncResult })

		await expect(remove(12)).resolves.toBeUndefined()
		await expect(syncWallets()).resolves.toBe(syncResult)
		expect(del).toHaveBeenCalledWith('/admin/upstream-funds/wallets/12')
		expect(post).toHaveBeenCalledWith('/admin/upstream-funds/wallets/sync')
	})

  it('refreshes and records balances through separate commands', async () => {
    const wallet = { id: 12, balance: 88.5 }
    const accounts = [{ id: 3, name: 'Account A', platform: 'openai' }]
		post.mockResolvedValueOnce({ data: wallet })
    post.mockResolvedValueOnce({ data: wallet })
    get.mockResolvedValueOnce({ data: accounts })

		await expect(refreshBalance(12)).resolves.toBe(wallet)
    await expect(recordBalance(12, 88.5)).resolves.toBe(wallet)
    await expect(listAccounts()).resolves.toBe(accounts)
		expect(post).toHaveBeenNthCalledWith(1, '/admin/upstream-funds/wallets/12/refresh-balance')
    expect(post).toHaveBeenCalledWith(
			'/admin/upstream-funds/wallets/12/manual-balance',
      { balance: 88.5 }
    )
    expect(get).toHaveBeenCalledWith('/admin/upstream-funds/accounts')
  })

  it('uses command endpoints for redeem and direct recharge state changes', async () => {
    const channels = [{ id: 'alipay', currency: 'CNY' }]
    const order = { id: 71, status: 'pending_payment' }
    get.mockResolvedValueOnce({ data: channels })
    post.mockResolvedValueOnce({ data: { status: 'verified' } })
    post.mockResolvedValueOnce({ data: order })
    post.mockResolvedValueOnce({ data: { ...order, status: 'paid' } })
    post.mockResolvedValueOnce({ data: { ...order, status: 'completed' } })

    await expect(listPaymentChannels(9)).resolves.toBe(channels)
    await redeemCode(9, 'one-time-code')
    await createRechargeOrder(9, { amount: 100, payment_channel_id: 'alipay', idempotency_key: 'key-1' })
    await pollRechargeOrder(71)
    await manualCompleteRechargeOrder(71, { balance_after: 125, reason: 'verified' })

    expect(get).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/payment-channels')
    expect(post).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/redeem-code', { code: 'one-time-code' })
    expect(post).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/recharge-orders', {
      amount: 100,
      payment_channel_id: 'alipay',
      idempotency_key: 'key-1'
    })
    expect(post).toHaveBeenCalledWith('/admin/upstream-funds/recharge-orders/71/poll')
    expect(post).toHaveBeenCalledWith('/admin/upstream-funds/recharge-orders/71/manual-complete', {
      balance_after: 125,
      reason: 'verified'
    })
  })

	it('manages the encrypted upstream panel authorization through wallet-scoped endpoints', async () => {
		const session = { configured: true, status: 'healthy' }
		const loginResult = { requires_2fa: false, session }
		get.mockResolvedValueOnce({ data: session })
		post.mockResolvedValueOnce({ data: loginResult })
		post.mockResolvedValueOnce({ data: loginResult })
		post.mockResolvedValueOnce({ data: loginResult })
		post.mockResolvedValueOnce({ data: session })
		del.mockResolvedValueOnce({ data: { configured: false, status: 'not_configured' } })

		await expect(getPanelSession(9)).resolves.toBe(session)
		await expect(loginPanelSession(9, { account_id: 42, email: 'owner@example.com', password: 'transient' })).resolves.toBe(loginResult)
		await expect(loginPanelSession(9, {})).resolves.toBe(loginResult)
		await expect(completePanelSessionTwoFactor(9, { challenge: 'opaque', code: '123456' })).resolves.toBe(loginResult)
		await expect(checkPanelSession(9)).resolves.toBe(session)
		await deletePanelSession(9)

		expect(get).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/panel-session')
		expect(post).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/panel-session/login', { account_id: 42, email: 'owner@example.com', password: 'transient' })
		expect(post).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/panel-session/login', {})
		expect(post).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/panel-session/login/2fa', { challenge: 'opaque', code: '123456' })
		expect(post).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/panel-session/check')
		expect(del).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/panel-session')
	})

	it('imports a browser-authorized panel session through the wallet endpoint', async () => {
		const session = { configured: true, status: 'healthy' }
		post.mockResolvedValueOnce({ data: session })

		await expect(importPanelSession(9, {
			account_id: 42,
			access_token: 'opaque-token',
			refresh_token: 'opaque-refresh',
			identity: 'owner@example.com',
			expires_in: 3600
		})).resolves.toBe(session)
		expect(post).toHaveBeenCalledWith('/admin/upstream-funds/wallets/9/panel-session/import', {
			account_id: 42,
			access_token: 'opaque-token',
			refresh_token: 'opaque-refresh',
			identity: 'owner@example.com',
			expires_in: 3600
		})
	})
})
