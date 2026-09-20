import type { Account } from '@/types'

export function isCodexTicketRateLimited(
  account: Pick<Account, 'rate_limit_reset_at' | 'extra'>,
  model: string,
  now: number
): boolean {
  return [
    account.rate_limit_reset_at,
    account.extra?.model_rate_limits?.[model]?.rate_limit_reset_at
  ].some((resetAt) => resetAt != null && Date.parse(resetAt) > now)
}
