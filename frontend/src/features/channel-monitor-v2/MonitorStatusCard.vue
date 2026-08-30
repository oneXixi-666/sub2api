<template>
  <article
    class="monitor-status-card group flex min-h-[268px] w-full flex-col rounded-2xl border border-gray-200/80 bg-transparent p-4 text-left transition duration-200 hover:-translate-y-0.5 hover:border-gray-300 hover:bg-white/35 hover:shadow-md focus-within:ring-2 focus-within:ring-primary-500/60 dark:border-dark-700/80 dark:hover:border-dark-600 dark:hover:bg-dark-800/30"
  >
    <button
      type="button"
      class="w-full text-left focus-visible:outline-none"
      :aria-label="cardLabel"
      @click="emit('select', row)"
    >
      <div class="flex items-start gap-3">
        <span
          class="grid h-9 w-9 shrink-0 place-items-center rounded-xl border border-gray-200/70 bg-transparent dark:border-dark-700/70"
          :class="providerTintClass"
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

      <div class="mt-5 grid gap-2" :class="showThroughput ? 'grid-cols-2 xl:grid-cols-4' : 'grid-cols-3'">
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
        </div>
      </div>
    </button>

    <MonitorStatusPulse
      class="mt-auto"
      :buckets="row.buckets || []"
      :coverage="coverage"
      :health-mode="healthMode"
      :show-throughput="showThroughput"
    />
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorCoverage, MonitorMatrixRow } from '@/api/channelMonitorV2'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import {
  formatLatencyPrivacy,
  formatMonitorGroupRate,
  formatMonitorMatrixModelLabel,
  formatMonitorMs,
  formatMonitorPercent,
  formatMonitorSuccessRateFromError,
  formatMonitorTokensPerSecond,
  tokensPerSecondFromTpm,
} from './monitorFormat'
import MonitorStatusPulse from './MonitorStatusPulse.vue'

type HealthMode = 'overall' | 'success' | 'ttft' | 'cache'
type StatusKind = 'healthy' | 'warning' | 'critical' | 'unknown'

const props = defineProps<{
  row: MonitorMatrixRow
  coverage: MonitorCoverage | null
  healthMode: HealthMode
  showThroughput: boolean
}>()

const emit = defineEmits<{
  (event: 'select', row: MonitorMatrixRow): void
}>()

const { t, locale } = useI18n()
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

const statusKind = computed<StatusKind>(() => {
  const state = props.healthMode === 'success'
    ? props.row.health.error_rate
    : props.healthMode === 'ttft'
      ? props.row.health.ttft
      : props.healthMode === 'cache'
        ? props.row.health.cache
        : props.row.health.overall
  return state === 'healthy' ? 'healthy' : state === 'warning' ? 'warning' : state === 'critical' ? 'critical' : 'unknown'
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
  const noCount = props.row.metrics.request_count <= 0
  const noThroughput = (props.row.metrics.rpm || 0) <= 0 && (props.row.metrics.tpm || 0) <= 0
  if (noCount && noThroughput && props.showThroughput) return '-'
  return formatMonitorSuccessRateFromError(props.row.metrics.error_rate)
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
  title?: string
  tone?: string
}

const metrics = computed<DisplayMetric[]>(() => {
  const values: DisplayMetric[] = [
    {
      label: t('channelMonitorV2.metrics.successRate'),
      value: successValue.value,
      tone: healthTone(props.row.health.error_rate),
    },
    {
      label: t('channelMonitorV2.metrics.ttft'),
      value: formatMonitorMs(props.row.metrics.ttft.p50_ms),
      title: formatLatencyPrivacy(props.row.metrics.ttft.p50_ms, props.row.metrics.ttft.p90_ms, props.row.metrics.ttft.avg_ms, props.row.metrics.ttft.p95_ms),
      tone: healthTone(props.row.health.ttft),
    },
  ]
  if (props.showThroughput) {
    values.push({
      label: t('channelMonitorV2.metrics.tps'),
      value: formatMonitorTokensPerSecond(props.row.metrics.tpm),
      title: Intl.NumberFormat(locale.value || undefined, { maximumFractionDigits: 3 }).format(tokensPerSecondFromTpm(props.row.metrics.tpm)),
    })
  }
  values.push({
    label: t('channelMonitorV2.metrics.cacheRate'),
    value: formatMonitorPercent(props.row.metrics.cache_rate),
    tone: healthTone(props.row.health.cache),
  })
  return values
})
</script>

<style scoped>
.monitor-status-metric {
  min-width: 0;
  border: 1px solid rgb(229 231 235 / 0.75);
  border-radius: 0.75rem;
  background: transparent;
  padding: 0.65rem 0.7rem;
}

:global(.dark) .monitor-status-metric {
  border-color: rgb(55 65 81 / 0.65);
  background: transparent;
}
</style>
