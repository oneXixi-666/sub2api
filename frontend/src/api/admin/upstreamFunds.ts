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
  tier: UpstreamWalletTier
  enabled: boolean
  alert_days: number
  target_days: number
  account_ids: number[]
}

export async function list(search?: string): Promise<UpstreamFundsOverview> {
  const { data } = await apiClient.get<UpstreamFundsOverview>('/admin/upstream-funds/wallets', {
    params: search ? { search } : undefined
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

export async function recordBalance(id: number, balance: number): Promise<UpstreamWallet> {
  const { data } = await apiClient.post<UpstreamWallet>(
    `/admin/upstream-funds/wallets/${id}/refresh-balance`,
    { balance }
  )
  return data
}

export async function listAccounts(): Promise<UpstreamFundsAccount[]> {
  const { data } = await apiClient.get<UpstreamFundsAccount[]>('/admin/upstream-funds/accounts')
  return data
}

const upstreamFundsAPI = { list, getById, create, update, recordBalance, listAccounts }
export default upstreamFundsAPI
