import { apiClient } from '../client'

export type UpstreamRechargeMode = 'direct' | 'product' | 'manual'
export type UpstreamPanelSessionStatus = 'not_configured' | 'unchecked' | 'healthy' | 'degraded' | 'expired'
export const UPSTREAM_ALIPAY_CHANNEL_ID = 'alipay'

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

export interface UpstreamPanelSessionState {
	configured: boolean
	encryption_key_configured: boolean
	credentials_saved: boolean
	saved_identity?: string
	saved_account_id?: number
	saved_account_name?: string
	status: UpstreamPanelSessionStatus
	identity?: string
	account_id?: number
	account_name?: string
	expires_at?: string
	last_checked_at?: string
	next_check_at?: string
	last_error?: string
}

export interface UpstreamPanelLoginResult {
	requires_2fa: boolean
	challenge?: string
	session: UpstreamPanelSessionState
}

export interface UpstreamPanelImportInput {
	account_id: number
	access_token: string
	refresh_token?: string
	identity?: string
	expires_in?: number
}

export interface UpstreamWallet {
  id: number
  name: string
  provider: string
	sync_domain?: string
  currency: string
	consumption_currency: string
  recharge_mode: UpstreamRechargeMode
	card_site_url: string
  enabled: boolean
  balance: number | null
  balance_updated_at: string | null
  balance_error: string
  account_ids: number[]
  accounts: UpstreamFundsAccount[]
  configured_groups: UpstreamFundsGroup[]
  actual_groups: UpstreamFundsGroup[]
  consumption_1h: number
  consumption_today: number
  consumption_24h: number
  needs_attention: boolean
  adapter_configured: boolean
	redeem_configured: boolean
	recharge_configured: boolean
	panel_session: UpstreamPanelSessionState
  created_at: string
  updated_at: string
}

export interface UpstreamFundsSummary {
  wallet_count: number
  enabled_count: number
  attention_count: number
	consumption_today: number
	consumption_24h: number
  balance_by_currency: Record<string, number>
}

export interface UpstreamFundsOverview {
  summary: UpstreamFundsSummary
  wallets: UpstreamWallet[]
	groups?: UpstreamFundsGroup[]
}

export interface UpstreamWalletInput {
  name: string
  provider: string
  currency: string
  recharge_mode: UpstreamRechargeMode
	card_site_url: string
  enabled: boolean
  account_ids: number[]
}

export interface UpstreamWalletSyncResult {
	domains: number
	created_wallets: number
	classified_wallets: number
	linked_accounts: number
	skipped_accounts: number
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

export async function list(search?: string, groupID?: number | null): Promise<UpstreamFundsOverview> {
	const params: Record<string, string | number> = {}
	if (search?.trim()) params.search = search.trim()
	if (groupID && groupID > 0) params.group_id = groupID
  const { data } = await apiClient.get<UpstreamFundsOverview>('/admin/upstream-funds/wallets', {
	    params: Object.keys(params).length ? params : undefined
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

export async function remove(id: number): Promise<void> {
	await apiClient.delete(`/admin/upstream-funds/wallets/${id}`)
}

export async function syncWallets(): Promise<UpstreamWalletSyncResult> {
	const { data } = await apiClient.post<UpstreamWalletSyncResult>('/admin/upstream-funds/wallets/sync')
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

export async function getPanelSession(id: number): Promise<UpstreamPanelSessionState> {
	const { data } = await apiClient.get<UpstreamPanelSessionState>(`/admin/upstream-funds/wallets/${id}/panel-session`)
	return data
}

export async function loginPanelSession(id: number, input: { account_id?: number; email?: string; password?: string }): Promise<UpstreamPanelLoginResult> {
	const { data } = await apiClient.post<UpstreamPanelLoginResult>(`/admin/upstream-funds/wallets/${id}/panel-session/login`, input)
	return data
}

export async function importPanelSession(id: number, input: UpstreamPanelImportInput): Promise<UpstreamPanelSessionState> {
	const { data } = await apiClient.post<UpstreamPanelSessionState>(`/admin/upstream-funds/wallets/${id}/panel-session/import`, input)
	return data
}

export async function completePanelSessionTwoFactor(id: number, input: { challenge: string; code: string }): Promise<UpstreamPanelLoginResult> {
	const { data } = await apiClient.post<UpstreamPanelLoginResult>(`/admin/upstream-funds/wallets/${id}/panel-session/login/2fa`, input)
	return data
}

export async function checkPanelSession(id: number): Promise<UpstreamPanelSessionState> {
	const { data } = await apiClient.post<UpstreamPanelSessionState>(`/admin/upstream-funds/wallets/${id}/panel-session/check`)
	return data
}

export async function deletePanelSession(id: number): Promise<UpstreamPanelSessionState> {
	const { data } = await apiClient.delete<UpstreamPanelSessionState>(`/admin/upstream-funds/wallets/${id}/panel-session`)
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

const upstreamFundsAPI = {
	list, listAll, getById, create, update, remove, syncWallets, refreshBalance, recordBalance, redeemCode,
	getPanelSession, loginPanelSession, importPanelSession, completePanelSessionTwoFactor, checkPanelSession, deletePanelSession,
	listPaymentChannels, createRechargeOrder, getRechargeOrder, pollRechargeOrder, manualCompleteRechargeOrder, listAccounts
}
export default upstreamFundsAPI
