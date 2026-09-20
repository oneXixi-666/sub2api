import { describe, expect, it, vi } from 'vitest'

const { get, del } = vi.hoisted(() => ({ get: vi.fn(), del: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, delete: del } }))

import { accountsAPI, clearCodexTicketLogs, getCodexTicketLogs, listCodexTicketLogs } from '@/api/admin/accounts'

describe('Codex ticket logs API', () => {
  it('passes the selected model and cancellation signal to the account logs endpoint', async () => {
    const controller = new AbortController()
    const response = { model: 'gpt-5.6-sol', entries: [], status: null, limit: 100 }
    get.mockResolvedValueOnce({ data: response })
    await expect(getCodexTicketLogs(91, 'gpt-5.6-sol', { signal: controller.signal })).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith('/admin/accounts/91/codex-ticket-logs', {
      params: { model: 'gpt-5.6-sol' }, signal: controller.signal
    })
    expect(accountsAPI.getCodexTicketLogs).toBe(getCodexTicketLogs)
  })

  it('reads the process-local ticket log feed', async () => {
    const response = { items: [{ id: 1, account_id: 41, model: 'gpt-6-astra' }], limit: 500 }
    get.mockResolvedValueOnce({ data: response })
    await expect(listCodexTicketLogs()).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith('/admin/codex-ticket-logs', { signal: undefined })
    expect(accountsAPI.listCodexTicketLogs).toBe(listCodexTicketLogs)
  })

  it('clears the process-local ticket log feed', async () => {
    del.mockResolvedValueOnce({ data: { cleared: true } })
    await expect(clearCodexTicketLogs()).resolves.toBeUndefined()
    expect(del).toHaveBeenCalledWith('/admin/codex-ticket-logs')
    expect(accountsAPI.clearCodexTicketLogs).toBe(clearCodexTicketLogs)
  })
})
