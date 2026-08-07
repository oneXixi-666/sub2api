import { apiClient } from '../client'

export type UpstreamRechargeMode = 'direct' | 'product' | 'manual'
export type UpstreamWalletTier = 'primary' | 'hot_backup' | 'cold_backup'

export interface UpstreamFundsAccount {
  id: number
  name: string
  platform: string
  type?: string
  wallet_id?: number
  wallet_name?: string
}

export interface UpstreamFundsGroup {
  id: number
  name: string
}

export interface UpstreamWallet {
  id: number
  name: string
  provider: string
  currency: string
  cost_currency: string
  recharge_mode: UpstreamRechargeMode
	card_site_url: string
  tier: UpstreamWalletTier
  enabled: boolean
  balance: number | null
  balance_updated_at: string | null
  balance_error: string
  alert_days: number
  target_days: number
  account_ids: number[]
  accounts: UpstreamFundsAccount[]
  configured_groups: UpstreamFundsGroup[]
  actual_groups: UpstreamFundsGroup[]
  cost_1h: number
  cost_today: number
  cost_24h: number
  cost_7d: number
  daily_cost_7d: number
  runway_days: number | null
  recommended_top_up: number | null
  needs_attention: boolean
  adapter_configured: boolean
	redeem_configured: boolean
	recharge_configured: boolean
  created_at: string
  updated_at: string
}

export interface UpstreamFundsSummary {
  wallet_count: number
  enabled_count: number
  attention_count: number
  cost_today: number
  cost_24h: number
  balance_by_currency: Record<string, number>
}

export interface UpstreamFundsOverview {
  summary: UpstreamFundsSummary
  wallets: UpstreamWallet[]
}

export interface UpstreamWalletInput {
  name: string
  provider: string
  currency: string
  recharge_mode: UpstreamRechargeMode
	card_site_url: string
  tier: UpstreamWalletTier
  enabled: boolean
  alert_days: number
  target_days: number
  account_ids: number[]
}

export interface UpstreamRedeemResult {
	status: 'verified' | 'manual_review'
	wallet: UpstreamWallet
}

export interface UpstreamPaymentChannel {
	id: string
	name: string
	currency: string
	single_min: number
	single_max: number
	fee_rate: number
	daily_remaining: number
}

export interface UpstreamRechargeOrder {
	id: number
	order_no: string
	wallet_id: number
	provider_order_id: string
	payment_channel_id: string
	face_value: number
	pay_amount: number
	currency: string
	status: 'creating' | 'pending_payment' | 'paid' | 'verifying' | 'completed' | 'manual_review' | 'failed' | 'expired' | 'cancelled'
	payment_qr: string
	payment_url: string
	payment_expires_at: string | null
	balance_before: number | null
	balance_after: number | null
	error_code: string
	error_message: string
	created_at: string
	updated_at: string
	completed_at: string | null
}

export async function list(search?: string): Promise<UpstreamFundsOverview> {
  const { data } = await apiClient.get<UpstreamFundsOverview>('/admin/upstream-funds/wallets', {
    params: search ? { search } : undefined
  })
  return data
}

export async function listAll(): Promise<UpstreamFundsOverview> {
  const { data } = await apiClient.get<UpstreamFundsOverview>('/admin/upstream-funds/wallets', {
    params: { search: '' }
  })
  return data
}

export async function getById(id: number): Promise<UpstreamWallet> {
  const { data } = await apiClient.get<UpstreamWallet>(`/admin/upstream-funds/wallets/${id}`)
  return data
}

export async function create(input: UpstreamWalletInput): Promise<UpstreamWallet> {
  const { data } = await apiClient.post<UpstreamWallet>('/admin/upstream-funds/wallets', input)
  return data
}

export async function update(id: number, input: UpstreamWalletInput): Promise<UpstreamWallet> {
  const { data } = await apiClient.put<UpstreamWallet>(`/admin/upstream-funds/wallets/${id}`, input)
  return data
}

export async function refreshBalance(id: number): Promise<UpstreamWallet> {
	const { data } = await apiClient.post<UpstreamWallet>(
		`/admin/upstream-funds/wallets/${id}/refresh-balance`
	)
	return data
}

export async function recordBalance(id: number, balance: number): Promise<UpstreamWallet> {
  const { data } = await apiClient.post<UpstreamWallet>(
		`/admin/upstream-funds/wallets/${id}/manual-balance`,
    { balance }
  )
  return data
}

export async function redeemCode(id: number, code: string): Promise<UpstreamRedeemResult> {
	const { data } = await apiClient.post<UpstreamRedeemResult>(
		`/admin/upstream-funds/wallets/${id}/redeem-code`,
		{ code }
	)
	return data
}

export async function listPaymentChannels(id: number): Promise<UpstreamPaymentChannel[]> {
	const { data } = await apiClient.get<UpstreamPaymentChannel[]>(`/admin/upstream-funds/wallets/${id}/payment-channels`)
	return data
}

export async function createRechargeOrder(id: number, input: { amount: number; payment_channel_id: string; idempotency_key: string }): Promise<UpstreamRechargeOrder> {
	const { data } = await apiClient.post<UpstreamRechargeOrder>(`/admin/upstream-funds/wallets/${id}/recharge-orders`, input)
	return data
}

export async function getRechargeOrder(id: number): Promise<UpstreamRechargeOrder> {
	const { data } = await apiClient.get<UpstreamRechargeOrder>(`/admin/upstream-funds/recharge-orders/${id}`)
	return data
}

export async function pollRechargeOrder(id: number): Promise<UpstreamRechargeOrder> {
	const { data } = await apiClient.post<UpstreamRechargeOrder>(`/admin/upstream-funds/recharge-orders/${id}/poll`)
	return data
}

export async function manualCompleteRechargeOrder(id: number, input: { balance_after: number; reason: string }): Promise<UpstreamRechargeOrder> {
	const { data } = await apiClient.post<UpstreamRechargeOrder>(`/admin/upstream-funds/recharge-orders/${id}/manual-complete`, input)
	return data
}

export async function listAccounts(): Promise<UpstreamFundsAccount[]> {
  const { data } = await apiClient.get<UpstreamFundsAccount[]>('/admin/upstream-funds/accounts')
  return data
}

const upstreamFundsAPI = { list, listAll, getById, create, update, refreshBalance, recordBalance, redeemCode, listPaymentChannels, createRechargeOrder, getRechargeOrder, pollRechargeOrder, manualCompleteRechargeOrder, listAccounts }
export default upstreamFundsAPI
