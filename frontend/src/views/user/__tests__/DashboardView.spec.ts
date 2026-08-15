import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import DashboardView from '../DashboardView.vue'

const {
  refreshUser,
  getDashboardStats,
  getDashboardSnapshotV2,
  query,
  getMyPlatformQuotas,
} = vi.hoisted(() => ({
  refreshUser: vi.fn(),
  getDashboardStats: vi.fn(),
  getDashboardSnapshotV2: vi.fn(),
  query: vi.fn(),
  getMyPlatformQuotas: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { balance: 0 },
    isSimpleMode: false,
    refreshUser,
  }),
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats,
    getDashboardSnapshotV2,
    query,
  },
}))

vi.mock('@/api/user', () => ({ getMyPlatformQuotas }))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('user DashboardView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 7, 15, 12, 0))
    refreshUser.mockReset().mockResolvedValue(undefined)
    getDashboardStats.mockReset().mockResolvedValue({})
    getDashboardSnapshotV2.mockReset().mockResolvedValue({ trend: [], models: [] })
    query.mockReset().mockResolvedValue({ items: [] })
    getMyPlatformQuotas.mockReset().mockResolvedValue({ platform_quotas: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('uses today and hourly granularity for range queries', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          UserDashboardStats: true,
          UserDashboardCharts: true,
          UserDashboardRecentUsage: true,
          UserDashboardQuickActions: true,
        },
      },
    })
    await flushPromises()

    expect(getDashboardSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: '2026-08-15',
      end_date: '2026-08-15',
      granularity: 'hour',
    }), expect.anything())
    expect(query).toHaveBeenCalledWith(expect.objectContaining({
      start_date: '2026-08-15',
      end_date: '2026-08-15',
    }), expect.anything())
  })
})
