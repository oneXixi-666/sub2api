<template>
  <button
    type="button"
    class="monitor-status-card group flex min-h-[268px] w-full flex-col rounded-2xl border border-gray-200/80 bg-white p-4 text-left shadow-sm transition duration-200 hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/60 dark:border-dark-700/80 dark:bg-dark-800 dark:hover:border-dark-600"
    :aria-label="cardLabel"
    @click="emit('select', row)"
  >
    <div class="flex items-start gap-3">
      <span
        class="grid h-9 w-9 shrink-0 place-items-center rounded-xl ring-1 ring-black/5 dark:ring-white/10"
        :class="[providerGradient(row.platform), providerTintClass]"
      >
        <ProviderIcon :provider="row.platform" :size="20" />
      </span>
      <div class="min-w-0 flex-1">
        <div class="truncate text-[15px] font-semibold text-gray-900 dark:text-gray-100">
          {{ titleLabel }}
        </div>
        <div class="mt-1 flex min-w-0 flex-wrap items-center gap-1.5">
          <span
            class="inline-flex shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium"
            :class="providerBadgeClass(row.platform)"
          >
            {{ providerLabel(row.platform) }}
          </span>
          <span
            v-if="modelLabel"
            class="min-w-0 truncate font-mono text-[11px] text-gray-500 dark:text-gray-400"
            :title="modelLabel"
          >
            {{ modelLabel }}
          </span>
          <span
            v-if="groupRateLabel"
            class="inline-flex shrink-0 items-center rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
          >
            {{ groupRateLabel }}
          </span>
        </div>
      </div>
      <span
        class="shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold"
        :class="statusBadgeClass"
      >
        {{ statusLabel }}
      </span>
    </div>

    <div class="mt-5 grid grid-cols-3 gap-2">
      <div v-for="metric in metrics" :key="metric.label" class="monitor-status-metric">
        <span class="block text-[10px] font-semibold uppercase tracking-wider text-gray-400">
          {{ metric.label }}
        </span>
        <strong
          class="mt-1 block truncate font-mono text-[18px] font-bold tabular-nums"
          :class="metric.tone || 'text-gray-900 dark:text-gray-100'"
          :title="metric.title"
        >
          {{ metric.value }}
        </strong>
        <span v-if="metric.detail" class="mt-0.5 block truncate text-[10px] text-gray-400">
          {{ metric.detail }}
        </span>
      </div>
    </div>

    <div class="mt-auto border-t border-gray-100 pt-4 dark:border-dark-700/70">
      <div class="mb-2 flex items-center justify-between gap-3 text-[10px] font-semibold uppercase tracking-wider text-gray-400">
        <span>{{ t('channelMonitorV2.cards.history', { count: displayBuckets.length }) }}</span>
        <span class="shrink-0">{{ bucketLabel }}</span>
      </div>
      <div
        class="flex h-7 items-end gap-1"
        role="img"
        :aria-label="t('channelMonitorV2.cards.historyAria', { count: displayBuckets.length })"
      >
        <span
          v-for="(bar, index) in displayBuckets"
          :key="bar.key || index"
          class="monitor-status-bar min-w-0 flex-1 rounded-sm transition-opacity duration-200 group-hover:opacity-90"
          :class="bar.className"
          :style="{ height: `${bar.height}%` }"
          :title="bar.title"
        />
      </div>
      <div class="mt-1 flex justify-between text-[9px] uppercase tracking-wider text-gray-400">
        <span>{{ t('channelMonitorV2.cards.past') }}</span>
        <span>{{ t('channelMonitorV2.cards.now') }}</span>
      </div>
    </div>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorMatrixRow, MonitorMatrixBucket } from '@/api/channelMonitorV2'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import { providerGradient, useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import {
  formatLatencyPrivacy,
  formatMonitorGroupRate,
  formatMonitorMatrixModelLabel,
  formatMonitorMs,
  formatMonitorPercent,
  formatMonitorSuccessRateFromError,
  healthModeScore,
  healthScoreClass,
} from './monitorFormat'

type HealthMode = 'overall' | 'success' | 'ttft' | 'cache'
type StatusKind = 'healthy' | 'warning' | 'critical' | 'unknown'

const props = defineProps<{
  row: MonitorMatrixRow
  coverage: { bucket_seconds: number } | null
  healthMode: HealthMode
}>()

const emit = defineEmits<{
  (event: 'select', row: MonitorMatrixRow): void
}>()

const { t } = useI18n()
const { providerLabel, providerBadgeClass } = useChannelMonitorFormat()

const PROVIDER_TINT: Record<string, string> = {
  openai: 'text-emerald-600 dark:text-emerald-300',
  anthropic: 'text-orange-600 dark:text-orange-300',
  gemini: 'text-sky-600 dark:text-sky-300',
  grok: 'text-zinc-700 dark:text-zinc-200',
  antigravity: 'text-purple-600 dark:text-purple-300',
  kimi: 'text-pink-600 dark:text-pink-300',
  zhipu: 'text-indigo-600 dark:text-indigo-300',
  deepseek: 'text-teal-600 dark:text-teal-300',
}

const providerTintClass = computed(() => PROVIDER_TINT[props.row.platform] ?? 'text-gray-500 dark:text-gray-300')
const titleLabel = computed(() => {
  const model = formatMonitorMatrixModelLabel(props.row.model, t('channelMonitorV2.otherModels'))
  return props.row.group_name || model || providerLabel(props.row.platform)
})
const modelLabel = computed(() => {
  const model = formatMonitorMatrixModelLabel(props.row.model, t('channelMonitorV2.otherModels'))
  return model && titleLabel.value !== model ? model : ''
})
const groupRateLabel = computed(() => {
  const rate = formatMonitorGroupRate(props.row.rate_multiplier)
  if (rate) return t('channelMonitorV2.cards.rate', { value: rate })
  if (!props.row.group_name && !props.row.group_id) return ''
  const group = props.row.group_name || `#${props.row.group_id}`
  return titleLabel.value === group ? '' : group
})

const score = computed(() => healthModeScore(props.row.health, props.healthMode))
const statusKind = computed<StatusKind>(() => {
  if (score.value != null) {
    if (score.value >= 80) return 'healthy'
    if (score.value >= 50) return 'warning'
    return 'critical'
  }
  if (props.row.metrics.request_count <= 0) return 'unknown'
  const coarse = props.healthMode === 'success'
    ? props.row.health.error_rate
    : props.healthMode === 'ttft'
      ? props.row.health.ttft
      : props.healthMode === 'cache'
        ? props.row.health.cache
        : props.row.health.overall
  return coarse === 'healthy' ? 'healthy' : coarse === 'warning' ? 'warning' : coarse === 'critical' ? 'critical' : 'unknown'
})
const statusLabel = computed(() => t(`channelMonitorV2.cards.status.${statusKind.value}`))
const statusBadgeClass = computed(() => {
  if (statusKind.value === 'healthy') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  if (statusKind.value === 'warning') return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
  if (statusKind.value === 'critical') return 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
})
const cardLabel = computed(() => `${titleLabel.value} - ${statusLabel.value}`)

const successValue = computed(() => {
  if (props.row.metrics.request_count <= 0) return '-'
  return formatMonitorSuccessRateFromError(props.row.metrics.error_rate)
})
const successTone = computed(() => {
  if (statusKind.value === 'critical') return 'text-red-600 dark:text-red-400'
  if (statusKind.value === 'warning') return 'text-amber-600 dark:text-amber-400'
  if (statusKind.value === 'healthy') return 'text-emerald-600 dark:text-emerald-400'
  return 'text-gray-500 dark:text-gray-400'
})

function healthTone(state?: string) {
  if (state === 'critical') return 'text-red-600 dark:text-red-400'
  if (state === 'warning') return 'text-amber-600 dark:text-amber-400'
  if (state === 'healthy') return 'text-emerald-600 dark:text-emerald-400'
  return 'text-gray-500 dark:text-gray-400'
}
interface DisplayMetric {
  label: string
  value: string
  detail?: string
  title?: string
  tone?: string
}

const metrics = computed<DisplayMetric[]>(() => {
  const values: DisplayMetric[] = [
    {
      label: t('channelMonitorV2.metrics.cacheRate'),
      value: formatMonitorPercent(props.row.metrics.cache_rate),
      detail: t('channelMonitorV2.metrics.cacheDetail'),
      tone: healthTone(props.row.health.cache),
    },
    {
      label: t('channelMonitorV2.cards.availability'),
      value: successValue.value,
      detail: t('channelMonitorV2.metrics.errorRateValue', { value: formatMonitorPercent(props.row.metrics.error_rate) }),
      tone: successTone.value,
    },
    {
      label: t('channelMonitorV2.cards.firstToken'),
      value: formatMonitorMs(props.row.metrics.ttft.p50_ms),
      detail: formatLatencyPrivacy(props.row.metrics.ttft.p50_ms, props.row.metrics.ttft.p90_ms, props.row.metrics.ttft.avg_ms, props.row.metrics.ttft.p95_ms),
      title: formatLatencyPrivacy(props.row.metrics.ttft.p50_ms, props.row.metrics.ttft.p90_ms, props.row.metrics.ttft.avg_ms, props.row.metrics.ttft.p95_ms),
    },
  ]
  return values
})

interface DisplayBar {
  key: string
  className: string
  height: number
  title: string
}

const bucketLabel = computed(() => {
  const seconds = props.coverage?.bucket_seconds || 0
  const minutes = seconds / 60
  if (minutes < 60) return t('channelMonitorV2.bucket.minutes', { count: minutes })
  const hours = minutes / 60
  if (hours < 24) return t('channelMonitorV2.bucket.hours', { count: hours })
  return t('channelMonitorV2.bucket.days', { count: hours / 24 })
})

function barHeight(bucket: MonitorMatrixBucket): number {
  return bucket.metrics.request_count > 0 ? 72 : 18
}

function bucketTitle(bucket: MonitorMatrixBucket): string {
  const scoreText = healthModeScore(bucket.health, props.healthMode)
  const scoreLabel = scoreText == null ? '-' : String(Math.round(scoreText))
  return `${bucket.bucket_start} - ${t('channelMonitorV2.matrix.scoreLine', { score: scoreLabel })} - ${t('channelMonitorV2.metrics.successRateValue', { value: formatMonitorSuccessRateFromError(bucket.metrics.error_rate) })}`
}

const displayBuckets = computed<DisplayBar[]>(() => {
  const source = (props.row.buckets || []).slice(-18)
  const placeholders = Array.from({ length: Math.max(0, 18 - source.length) }, (_, index) => ({
    key: `empty-${index}`,
    className: 'health-unknown',
    height: 18,
    title: t('channelMonitorV2.matrix.noTraffic'),
  }))
  return [
    ...placeholders,
    ...source.map((bucket) => ({
      key: bucket.bucket_start,
      className: healthScoreClass(bucket.health, props.healthMode, bucket.metrics.request_count),
      height: barHeight(bucket),
      title: bucketTitle(bucket),
    })),
  ]
})
</script>

<style scoped>
.monitor-status-metric {
  min-width: 0;
  border: 1px solid rgb(229 231 235 / 0.75);
  border-radius: 0.75rem;
  background: rgb(249 250 251 / 0.82);
  padding: 0.65rem 0.7rem;
}

:global(.dark) .monitor-status-metric {
  border-color: rgb(55 65 81 / 0.65);
  background: rgb(17 24 39 / 0.38);
}

.health-score10 { background: #16a34a; }
.health-score9  { background: #22c55e; }
.health-score8  { background: #4ade80; }
.health-score7  { background: #a3e635; }
.health-score6  { background: #facc15; }
.health-score5  { background: #fbbf24; }
.health-score4  { background: #f59e0b; }
.health-score3  { background: #f97316; }
.health-score2  { background: #fb7185; }
.health-score1  { background: #f87171; }
.health-score0  { background: #ef4444; }
.health-healthy  { background: #22c55e; }
.health-warning  { background: #f59e0b; }
.health-critical { background: #ef4444; }
.health-unknown  { background: #d1d5db; }

:global(.dark) .health-unknown { background: #4b5563; }
</style>
