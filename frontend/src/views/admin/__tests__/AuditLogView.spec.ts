import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import AuditLogView from '../AuditLogView.vue'

const { list } = vi.hoisted(() => ({ list: vi.fn() }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    audit: { list, get: vi.fn(), clear: vi.fn() }
  }
}))

vi.mock('@/api', () => ({
  totpAPI: { getStatus: vi.fn() }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } })
  }
})

const wrappers: VueWrapper[] = []

function renderView() {
  const wrapper = mount(AuditLogView, {
    global: {
      stubs: {
        AppLayout: { template: '<div data-testid="app-layout"><slot /></div>' },
        TablePageLayout: {
          template: '<div data-testid="audit-table"><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: true,
        Pagination: true,
        Select: true,
        BaseDialog: true,
        ConfirmDialog: true,
        Icon: true,
        CodexTicketLogsPanel: defineComponent({
          name: 'CodexTicketLogsPanel',
          template: '<div data-testid="codex-ticket-logs-panel" />'
        })
      }
    }
  })
  wrappers.push(wrapper)
  return wrapper
}

describe('AuditLogView ticket logs tab', () => {
  beforeEach(() => {
    list.mockReset().mockResolvedValue({ items: [], total: 0 })
  })

  afterEach(() => {
    wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  })

  it('keeps ticket harvest logs as a second admin tab and only mounts them when opened', async () => {
    const wrapper = renderView()
    await flushPromises()
    const tabs = wrapper.findAll('[data-testid="audit-section-tab"]')
    expect(tabs).toHaveLength(2)
    expect(tabs[0].attributes('data-tab')).toBe('audit')
    expect(tabs[1].attributes('data-tab')).toBe('tickets')
    expect(tabs[0].text()).toContain('admin.audit.tabs.operations')
    expect(tabs[1].text()).toContain('admin.audit.tabs.tickets')
    expect(wrapper.find('[data-testid="codex-ticket-logs-panel"]').exists()).toBe(false)

    await tabs[1].trigger('click')
    expect(wrapper.find('[data-testid="codex-ticket-logs-panel"]').exists()).toBe(true)

    await tabs[0].trigger('click')
    expect(wrapper.find('[data-testid="codex-ticket-logs-panel"]').exists()).toBe(true)
    expect(tabs[0].classes().join(' ')).toContain('border-primary-500')
    expect(tabs[1].classes().join(' ')).toContain('border-transparent')
  })
})
