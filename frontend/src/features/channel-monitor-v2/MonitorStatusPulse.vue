<template>
  <div class="border-t border-gray-200/70 pt-4 dark:border-dark-700/70">
    <div class="mb-2 flex items-center justify-between gap-3 text-[10px] font-semibold text-gray-400">
      <span>{{ t('channelMonitorV2.cards.history', { count: slots.length }) }}</span>
      <span class="shrink-0">{{ bucketLabel }}</span>
    </div>
    <div
      class="grid h-5 items-stretch"
      :style="pulseStyle"
      role="group"
      :aria-label="t('channelMonitorV2.cards.historyAria', { count: slots.length })"
    >
      <span
        v-for="slot in slots"
        :key="slot.start"
        class="pulse-cell relative min-w-0 rounded-sm border-0 p-0 outline-offset-1"
        :class="[
          slot.bucket ? cellClass(slot.bucket) : 'health-unknown',
          slot.bucket ? 'has-data' : 'is-empty',
        ]"
        tabindex="0"
        role="img"
        :title="slot.bucket ? bucketTooltip(slot.bucket) : emptyTooltip(slot.start)"
        :aria-label="slot.bucket ? bucketTooltip(slot.bucket) : emptyTooltip(slot.start)"
        @mouseenter="showTooltip($event, slot)"
        @mousemove="moveTooltip"
        @mouseleave="hideTooltip"
        @focus="showTooltip($event, slot)"
        @blur="hideTooltip"
      />
    </div>
    <div class="mt-1 flex justify-between text-[9px] text-gray-400">
      <span>{{ t('channelMonitorV2.cards.past') }}</span>
      <span>{{ t('channelMonitorV2.cards.now') }}</span>
    </div>

    <Teleport to="body">
      <div
        v-if="floatingTooltip.visible"
        class="monitor-card-floating-tooltip"
        :style="{ left: `${floatingTooltip.x}px`, top: `${floatingTooltip.y}px` }"
        role="tooltip"
      >
        <span
          v-for="(line, index) in floatingTooltip.lines"
          :key="`${index}:${line}`"
          class="monitor-card-floating-tooltip-line"
          :class="index === 0 ? 'monitor-card-floating-tooltip-title' : ''"
        >
          {{ line }}
        </span>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  LatencyMetric,
  MonitorCoverage,
  MonitorHealth,
  MonitorMatrixBucket,
  MonitorMetric,
} from '@/api/channelMonitorV2'
import {
  formatLatencyPrivacy,
  formatMonitorPercent,
  formatMonitorSuccessRateFromError,
  formatMonitorThroughput,
  formatMonitorTokensPerSecond,
  healthModeScore,
  healthScoreClass,
} from './monitorFormat'

type HealthMode = 'overall' | 'success' | 'ttft' | 'cache'
type PulseSlot = { start: string; bucket?: MonitorMatrixBucket }

const props = defineProps<{
  buckets: MonitorMatrixBucket[]
  coverage: MonitorCoverage | null
  healthMode: HealthMode
  showThroughput: boolean
}>()

const { t, locale } = useI18n()
const floatingTooltip = reactive({
  visible: false,
  x: 0,
  y: 0,
  lines: [] as string[],
})

const bucketStarts = computed(() => {
  if (!props.coverage) {
    return props.buckets
      .map((bucket) => new Date(bucket.bucket_start).toISOString())
      .sort()
  }
  const step = Math.max(60, props.coverage.bucket_seconds) * 1000
  const requestedStart = new Date(props.coverage.requested_start).getTime()
  const requestedEndRaw = props.coverage.requested_end
    ? new Date(props.coverage.requested_end).getTime()
    : NaN
  const dataThrough = new Date(props.coverage.data_through).getTime()
  const end = Number.isFinite(requestedEndRaw) && requestedEndRaw > requestedStart
    ? requestedEndRaw
    : dataThrough
  if (![requestedStart, end].every(Number.isFinite) || requestedStart >= end) return []

  const starts: string[] = []
  for (let cursor = Math.floor(requestedStart / step) * step; cursor < end; cursor += step) {
    starts.push(new Date(cursor).toISOString())
  }
  return starts
})

const slots = computed<PulseSlot[]>(() => {
  const bucketByStart = new Map(
    props.buckets.map((bucket) => [new Date(bucket.bucket_start).toISOString(), bucket]),
  )
  return bucketStarts.value.map((start) => ({ start, bucket: bucketByStart.get(start) }))
})

const pulseStyle = computed(() => ({
  gridTemplateColumns: `repeat(${Math.max(1, slots.value.length)}, minmax(0, 1fr))`,
  gap: slots.value.length > 24 ? '2px' : '4px',
}))

const bucketLabel = computed(() => {
  const seconds = props.coverage?.bucket_seconds || 0
  if (!seconds) return '-'
  const minutes = seconds / 60
  if (minutes < 60) return t('channelMonitorV2.bucket.minutes', { count: minutes })
  const hours = minutes / 60
  if (hours < 24) return t('channelMonitorV2.bucket.hours', { count: hours })
  return t('channelMonitorV2.bucket.days', { count: hours / 24 })
})

function cellClass(bucket: MonitorMatrixBucket) {
  return healthScoreClass(bucket.health, props.healthMode, bucket.metrics.request_count)
}

function successRate(metrics: MonitorMetric) {
  const noCount = metrics.request_count <= 0
  const noThroughput = (metrics.rpm || 0) <= 0 && (metrics.tpm || 0) <= 0
  if (noCount && noThroughput && props.showThroughput) return '-'
  return formatMonitorSuccessRateFromError(metrics.error_rate)
}

function formatScore(health: MonitorHealth) {
  const score = healthModeScore(health, props.healthMode)
  return score == null ? '\u2014' : `${Math.round(score)}`
}

function latencyPrivacy(metric: LatencyMetric) {
  return formatLatencyPrivacy(metric.p50_ms, metric.p90_ms, metric.avg_ms, metric.p95_ms)
}

function bucketTooltipLines(bucket: MonitorMatrixBucket) {
  const metrics = bucket.metrics
  const lines = [
    formatBucketRange(bucket.bucket_start),
    t('channelMonitorV2.matrix.scoreLine', { score: formatScore(bucket.health) }),
    t('channelMonitorV2.metrics.successRateValue', { value: successRate(metrics) }),
    t('channelMonitorV2.metrics.ttftValue', { value: latencyPrivacy(metrics.ttft) }),
  ]
  if (props.showThroughput) {
    lines.push(t('channelMonitorV2.metrics.tpsValue', { value: formatMonitorTokensPerSecond(metrics.tpm) }))
  }
  lines.push(
    t('channelMonitorV2.metrics.cacheRateValue', { value: formatMonitorPercent(metrics.cache_rate) }),
    t('channelMonitorV2.metrics.errorRateValue', { value: formatMonitorPercent(metrics.error_rate) }),
  )
  if (props.showThroughput) {
    lines.push(t('channelMonitorV2.metrics.rpmValue', { value: formatMonitorThroughput(metrics.rpm) }))
  }
  lines.push(t('channelMonitorV2.metrics.durationValue', { value: latencyPrivacy(metrics.duration) }))
  return lines
}

function bucketTooltip(bucket: MonitorMatrixBucket) {
  return bucketTooltipLines(bucket).join('\n')
}

function emptyTooltip(start: string) {
  return t('channelMonitorV2.matrix.noTrafficAt', { time: formatBucketRange(start) })
}

function showTooltip(event: MouseEvent | FocusEvent, slot: PulseSlot) {
  floatingTooltip.lines = slot.bucket
    ? bucketTooltipLines(slot.bucket)
    : [formatBucketRange(slot.start), t('channelMonitorV2.matrix.noTraffic')]
  floatingTooltip.visible = true
  positionTooltip(event)
}

function moveTooltip(event: MouseEvent) {
  if (floatingTooltip.visible) positionTooltip(event)
}

function hideTooltip() {
  floatingTooltip.visible = false
}

function positionTooltip(event: MouseEvent | FocusEvent) {
  if ('clientX' in event) {
    floatingTooltip.x = Math.min(window.innerWidth - 12, Math.max(12, event.clientX))
    floatingTooltip.y = Math.min(window.innerHeight - 12, Math.max(12, event.clientY)) - 12
    return
  }
  const target = event.target as HTMLElement | null
  const rect = target?.getBoundingClientRect()
  if (!rect) return
  floatingTooltip.x = rect.left + rect.width / 2
  floatingTooltip.y = rect.top - 10
}

function formatAxisTime(value: string) {
  return new Intl.DateTimeFormat(locale.value || undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function formatBucketRange(value: string) {
  const start = new Date(value)
  const end = new Date(start.getTime() + (props.coverage?.bucket_seconds || 0) * 1000)
  const endLabel = new Intl.DateTimeFormat(locale.value || undefined, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(end)
  return `${formatAxisTime(start.toISOString())} - ${endLabel}`
}
</script>

<style scoped>
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

.pulse-cell.has-data {
  cursor: help;
}

.pulse-cell.is-empty {
  cursor: default;
  opacity: 0.55;
}

.pulse-cell.has-data:hover,
.pulse-cell.has-data:focus-visible {
  z-index: 5;
  outline: 2px solid rgb(var(--color-primary-500, 99 102 241) / 0.55);
  outline-offset: 1px;
}

.monitor-card-floating-tooltip {
  pointer-events: none;
  position: fixed;
  z-index: 9999;
  min-width: 11.5rem;
  max-width: min(18rem, calc(100vw - 1.5rem));
  transform: translate(-50%, -100%);
  border: 1px solid rgb(229 231 235);
  border-radius: 0.75rem;
  background: rgb(255 255 255);
  padding: 0.5rem 0.625rem;
  box-shadow: 0 18px 40px -12px rgb(0 0 0 / 0.28);
  white-space: nowrap;
}

:global(.dark) .monitor-card-floating-tooltip {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
  color: rgb(229 231 235);
}

.monitor-card-floating-tooltip-line {
  display: block;
  color: rgb(75 85 99);
  font-size: 11px;
  line-height: 1.45;
}

:global(.dark) .monitor-card-floating-tooltip-line {
  color: rgb(209 213 219);
}

.monitor-card-floating-tooltip-title {
  margin-bottom: 0.2rem;
  color: rgb(17 24 39);
  font-weight: 600;
}

:global(.dark) .monitor-card-floating-tooltip-title {
  color: rgb(243 244 246);
}
</style>
