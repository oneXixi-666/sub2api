const i18nT = (key: string, params?: Record<string, unknown>) => {
  const map: Record<string, string> = {
    'channelMonitorV2.otherModels': '其他模型',
    'channelMonitorV2.cards.rate': '用户倍率 {value}',
    'channelMonitorV2.cards.status.healthy': '正常',
    'channelMonitorV2.cards.status.warning': '降级',
    'channelMonitorV2.cards.status.critical': '失败',
    'channelMonitorV2.cards.status.unknown': '样本不足',
    'channelMonitorV2.cards.history': '最近 {count} 个区间',
    'channelMonitorV2.cards.historyAria': '最近 {count} 个监控区间',
    'channelMonitorV2.cards.past': '过去',
    'channelMonitorV2.cards.now': '现在',
    'channelMonitorV2.bucket.minutes': '{count} 分钟粒度',
    'channelMonitorV2.bucket.hours': '{count} 小时粒度',
    'channelMonitorV2.bucket.days': '{count} 天粒度',
    'channelMonitorV2.metrics.successRate': '成功率',
    'channelMonitorV2.metrics.ttft': '首 Token',
    'channelMonitorV2.metrics.tps': '每秒 Token',
    'channelMonitorV2.metrics.cacheRate': '缓存率',
    'channelMonitorV2.matrix.scoreLine': '评分 {score}',
    'channelMonitorV2.metrics.successRateValue': '成功率 {value}',
    'channelMonitorV2.metrics.errorRateValue': '错误率 {value}',
    'channelMonitorV2.metrics.rpmValue': 'RPM {value}',
    'channelMonitorV2.metrics.tpsValue': '每秒 Token {value}',
    'channelMonitorV2.metrics.ttftValue': '首 Token {value}',
    'channelMonitorV2.metrics.durationValue': '时长 {value}',
    'channelMonitorV2.metrics.cacheRateValue': '缓存率 {value}',
    'channelMonitorV2.matrix.noTrafficAt': '{time} 无流量',
    'channelMonitorV2.matrix.noTraffic': '无流量',
  }
  const template = map[key] || key
  return template.replace(/\{(\w+)\}/g, (_, name) => String(params?.[name] ?? ''))
}

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: i18nT,
      te: () => true,
      locale: { value: 'zh-CN' },
    }),
  }
})

import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type {
  MonitorCoverage,
  MonitorHealth,
  MonitorMatrixRow,
  MonitorMetric,
} from '@/api/channelMonitorV2'
import MonitorStatusCard from '../MonitorStatusCard.vue'

const health: MonitorHealth = {
  overall: 'warning',
  error_rate: 'warning',
  ttft: 'healthy',
  cache: 'warning',
  score: 52,
  error_rate_score: 40,
  ttft_score: 100,
  cache_score: 50,
  minimum_sample: 20,
}

function metrics(requestCount: number): MonitorMetric {
  return {
    success_requests: requestCount ? requestCount - 1 : 0,
    error_requests: requestCount ? 1 : 0,
    request_count: requestCount,
    token_count: 100,
    rpm: 1.5,
    tpm: 10.2,
    error_rate: requestCount ? 1 / requestCount : 0,
    cache_rate: 0.5,
    cache_rate_numerator: 50,
    cache_rate_denominator: 100,
    ttft: { sample_count: requestCount, p50_ms: 100, p95_ms: 300, avg_ms: 150 },
    duration: { sample_count: requestCount, p50_ms: 500, p95_ms: 900, avg_ms: 600 },
  }
}

const coverage: MonitorCoverage = {
  requested_start: '2026-08-01T00:00:00Z',
  requested_end: '2026-08-01T00:03:00Z',
  coverage_start: '2026-08-01T00:00:00Z',
  data_through: '2026-08-01T00:03:00Z',
  computed_at: '2026-08-01T00:03:00Z',
  aggregation_lag_seconds: 0,
  coverage_complete: true,
  bucket_seconds: 60,
}

function row(): MonitorMatrixRow {
  return {
    platform: 'openai',
    group_id: 7,
    group_name: '【福利】',
    rate_multiplier: 0.5,
    model: 'gpt-5',
    metrics: metrics(10),
    health,
    buckets: [
      { bucket_start: '2026-08-01T00:00:00Z', metrics: metrics(10), health },
      { bucket_start: '2026-08-01T00:01:00Z', metrics: metrics(0), health },
    ],
  }
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('MonitorStatusCard V2 contract', () => {
  it('keeps the V2 summary metrics, selected range, and hover details', async () => {
    const wrapper = mount(MonitorStatusCard, {
      attachTo: document.body,
      props: {
        row: row(),
        coverage,
        healthMode: 'overall',
        showThroughput: true,
      },
    })

    const summary = wrapper.findAll('.monitor-status-metric')
    expect(summary).toHaveLength(4)
    expect(summary.map((item) => item.text())).toEqual([
      expect.stringContaining('成功率'),
      expect.stringContaining('首 Token'),
      expect.stringContaining('每秒 Token'),
      expect.stringContaining('缓存率'),
    ])
    expect(wrapper.text()).toContain('90.0%')
    expect(wrapper.text()).toContain('100ms')
    expect(wrapper.text()).toContain('0.2')
    expect(wrapper.text()).toContain('50.0%')

    const cells = wrapper.findAll('.pulse-cell')
    expect(cells).toHaveLength(3)
    expect(cells[0].classes().some((name) => name.startsWith('health-score'))).toBe(true)
    expect(cells[2].classes()).toContain('health-unknown')

    await cells[0].trigger('mouseenter', { clientX: 200, clientY: 200 })
    const tooltip = document.body.querySelector('.monitor-card-floating-tooltip')
    expect(tooltip?.textContent).toContain('评分 52')
    expect(tooltip?.textContent).toContain('成功率 90.0%')
    expect(tooltip?.textContent).toContain('首 Token')
    expect(tooltip?.textContent).toContain('每秒 Token')
    expect(tooltip?.textContent).toContain('缓存率 50.0%')
    expect(tooltip?.textContent).toContain('错误率 10.0%')
    expect(tooltip?.textContent).toContain('RPM 1.5')
    expect(tooltip?.textContent).toContain('时长')
    expect(tooltip?.textContent).not.toContain('请求数')

    expect(wrapper.find('button').exists()).toBe(false)
    expect(wrapper.emitted('select')).toBeUndefined()
    wrapper.unmount()
  })

  it('honors the V2 throughput privacy switch in metrics and tooltip', async () => {
    const wrapper = mount(MonitorStatusCard, {
      attachTo: document.body,
      props: {
        row: row(),
        coverage,
        healthMode: 'overall',
        showThroughput: false,
      },
    })

    expect(wrapper.findAll('.monitor-status-metric')).toHaveLength(3)
    expect(wrapper.text()).not.toContain('每秒 Token')
    const firstCell = wrapper.findAll('.pulse-cell')[0]
    await firstCell.trigger('mouseenter', { clientX: 200, clientY: 200 })
    const tooltipText = document.body.querySelector('.monitor-card-floating-tooltip')?.textContent || ''
    expect(tooltipText).not.toContain('每秒 Token')
    expect(tooltipText).not.toContain('RPM')
    wrapper.unmount()
  })
})
