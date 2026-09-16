import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { MonitorMatrixRow } from '@/api/channelMonitorV2'
import MonitorStatusCardGrid from '../MonitorStatusCardGrid.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

function row(platform: string, groupName: string): MonitorMatrixRow {
  return {
    platform,
    group_id: 1,
    group_name: groupName,
    model: 'm',
    health: {
      overall: 'healthy',
      error_rate: 'healthy',
      ttft: 'healthy',
      cache: 'healthy',
      score: 100,
      error_rate_score: 100,
      ttft_score: 100,
      cache_score: 100,
      minimum_sample: 1,
    },
    metrics: {
      success_requests: 1,
      error_requests: 0,
      request_count: 1,
      token_count: 1,
      rpm: 1,
      tpm: 1,
      error_rate: 0,
      cache_rate: 0,
      cache_rate_numerator: 0,
      cache_rate_denominator: 0,
      ttft: { sample_count: 1, p50_ms: 1, p90_ms: 1, p95_ms: 1, avg_ms: 1 },
      duration: { sample_count: 1, p50_ms: 1, p90_ms: 1, p95_ms: 1, avg_ms: 1 },
    },
    buckets: [],
  } as MonitorMatrixRow
}

describe('MonitorStatusCardGrid platform grouping', () => {
  it('groups MiniMax, OpenCode, and unknown platforms instead of dropping them', () => {
    const wrapper = mount(MonitorStatusCardGrid, {
      props: {
        rows: [
          row('opencode_go', 'OpenCode group'),
          row('minimax', 'MiniMax group'),
          row('future_vendor', 'Future group'),
          row('openai', 'OpenAI group'),
        ],
        coverage: null,
        healthMode: 'overall',
        showThroughput: false,
        loading: false,
      },
      global: {
        stubs: {
          EmptyState: true,
          ProviderIcon: true,
          MonitorStatusCard: {
            props: ['row'],
            template: '<div class="card-stub">{{ row.group_name }}</div>',
          },
        },
      },
    })

    const headings = wrapper.findAll('h3').map((heading) => heading.text())
    expect(headings).toEqual([
      'monitorCommon.providers.openai',
      'monitorCommon.providers.minimax',
      'monitorCommon.providers.opencode_go',
      'future_vendor',
    ])
  })
})
