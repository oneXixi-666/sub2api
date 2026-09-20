import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import CodexTicketLogsPanel from '../CodexTicketLogsPanel.vue'
import type { CodexTicketLogFeed, CodexTicketLogFeedItem } from '@/types'
import zhAccounts from '@/i18n/locales/zh/admin/accounts'
import zhAudit from '@/i18n/locales/zh/admin/audit'
import zhCommon from '@/i18n/locales/zh/common'
import { testMessageCompiler } from '@/__tests__/i18n'

const { listCodexTicketLogs, clearCodexTicketLogs, showError, showSuccess } = vi.hoisted(() => ({
  listCodexTicketLogs: vi.fn(),
  clearCodexTicketLogs: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { accounts: { listCodexTicketLogs, clearCodexTicketLogs } }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

function stubDesktopMatchMedia() {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: true,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      dispatchEvent: vi.fn()
    }))
  })
}

const item = (overrides: Partial<CodexTicketLogFeedItem> = {}): CodexTicketLogFeedItem => ({
  id: 9,
  time: '2026-09-19T01:00:00Z',
  attempt: 2,
  event: 'success',
  reason: 'harvested',
  http_status: 200,
  ticket_length: 292,
  target_length: 292,
  duration_ms: 88,
  account_id: 41,
  model: 'gpt-6-astra',
  ...overrides
})

const feed = (overrides: Partial<CodexTicketLogFeed> = {}): CodexTicketLogFeed => ({
  items: [item()],
  limit: 500,
  ...overrides
})

const wrappers: VueWrapper[] = []

function renderPanel() {
  const wrapper = mount(CodexTicketLogsPanel, {
    global: {
      plugins: [createI18n({
        legacy: false,
        locale: 'zh',
        messageCompiler: testMessageCompiler,
        messages: {
          zh: { ...zhCommon, admin: { ...zhAccounts, ...zhAudit } }
        }
      })],
      stubs: { teleport: true }
    }
  })
  wrappers.push(wrapper)
  return wrapper
}

describe('CodexTicketLogsPanel', () => {
  beforeEach(() => {
    stubDesktopMatchMedia()
    listCodexTicketLogs.mockReset().mockResolvedValue(feed())
    clearCodexTicketLogs.mockReset().mockResolvedValue(undefined)
    showError.mockReset()
    showSuccess.mockReset()
  })

  afterEach(() => {
    wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  })

  it('reads the process-local feed and shows account/model without persisting anything', async () => {
    const wrapper = renderPanel()
    await flushPromises()
    expect(listCodexTicketLogs).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('只读当前进程内存里的打票记录，不入库')
    expect(wrapper.text()).toContain('最多保留最近 500 条')
    expect(wrapper.text()).toContain('#41')
    expect(wrapper.text()).toContain('gpt-6-astra')
    expect(wrapper.text()).toContain('获得门票')
    expect(wrapper.text()).toContain('已取得符合目标长度的门票')
    expect(wrapper.text()).toContain('292 / 292')
    expect(wrapper.text()).toContain('88 ms')
  })

  it('disables clear when the feed is empty', async () => {
    listCodexTicketLogs.mockResolvedValueOnce(feed({ items: [] }))
    const wrapper = renderPanel()
    await flushPromises()
    expect((wrapper.get('[data-testid="codex-ticket-logs-clear"]').element as HTMLButtonElement).disabled).toBe(true)
    expect(wrapper.text()).toContain('暂无打票日志')
  })

  it('clears memory after a plain confirm, without TOTP', async () => {
    const wrapper = renderPanel()
    await flushPromises()
    await wrapper.get('[data-testid="codex-ticket-logs-clear"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('这只会清掉当前进程内存里的打票记录，不用二次验证')
    expect(wrapper.text()).not.toContain('二次验证码')
    const confirm = wrapper.findAll('.modal-footer .btn-danger').find((button) => button.text().includes('清空'))
    expect(confirm).toBeTruthy()
    await confirm!.trigger('click')
    await flushPromises()
    expect(clearCodexTicketLogs).toHaveBeenCalledTimes(1)
    expect(showSuccess).toHaveBeenCalledWith('已清空打票日志')
    expect(wrapper.text()).toContain('暂无打票日志')
  })

  it('reloads from memory when refresh is clicked', async () => {
    const wrapper = renderPanel()
    await flushPromises()
    listCodexTicketLogs.mockResolvedValueOnce(feed({
      items: [item({ id: 10, event: 'miss', reason: 'missing_state', ticket_length: undefined })]
    }))
    await wrapper.get('[data-testid="codex-ticket-logs-refresh"]').trigger('click')
    await flushPromises()
    expect(listCodexTicketLogs).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('未命中')
    expect(wrapper.text()).toContain('上游响应未携带门票')
  })
})
