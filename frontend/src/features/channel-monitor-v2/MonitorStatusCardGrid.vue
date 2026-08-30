<template>
  <div>
    <div
      v-if="loading && rows.length === 0"
      class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
      aria-hidden="true"
    >
      <div
        v-for="index in 8"
        :key="index"
        class="min-h-[268px] animate-pulse rounded-2xl border border-gray-200/70 bg-transparent p-4 dark:border-dark-700/70"
      >
        <div class="flex items-start gap-3">
          <div class="h-9 w-9 rounded-xl bg-gray-200 dark:bg-dark-700" />
          <div class="flex-1 space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700" />
            <div class="h-3 w-1/2 rounded bg-gray-100 dark:bg-dark-700/70" />
          </div>
          <div class="h-6 w-16 rounded-full bg-gray-200 dark:bg-dark-700" />
        </div>
        <div class="mt-5 grid grid-cols-3 gap-2">
          <div v-for="metric in 3" :key="metric" class="h-20 rounded-xl bg-gray-100 dark:bg-dark-900/40" />
        </div>
        <div class="mt-5 h-8 rounded bg-gray-100 dark:bg-dark-900/40" />
      </div>
    </div>

    <EmptyState
      v-else-if="rows.length === 0"
      :title="t('channelMonitorV2.empty.title')"
      :description="t('channelMonitorV2.empty.description')"
    />

    <div v-else class="space-y-6">
      <section
        v-for="group in platformGroups"
        :key="group.platform"
        class="space-y-3"
        :aria-labelledby="`monitor-platform-${group.platform}`"
      >
        <div class="flex items-center gap-2 px-1">
          <span
            class="grid h-7 w-7 place-items-center rounded-lg border border-gray-200/70 bg-transparent dark:border-dark-700/70"
            aria-hidden="true"
          >
            <ProviderIcon :provider="group.platform" :size="16" />
          </span>
          <h3
            :id="`monitor-platform-${group.platform}`"
            class="text-sm font-bold text-gray-900 dark:text-white"
          >
            {{ providerLabel(group.platform) }}
          </h3>
          <span class="text-xs text-gray-400 dark:text-dark-400">
            {{ t('channelMonitorV2.cards.platformCount', { count: group.rows.length }) }}
          </span>
        </div>
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          <MonitorStatusCard
            v-for="row in group.rows"
            :key="rowKey(row)"
            :row="row"
            :coverage="coverage"
            :health-mode="healthMode"
            :show-throughput="showThroughput"
            @select="emit('select', $event)"
          />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorCoverage, MonitorMatrixRow } from '@/api/channelMonitorV2'
import EmptyState from '@/components/common/EmptyState.vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import MonitorStatusCard from './MonitorStatusCard.vue'

const props = defineProps<{
  rows: MonitorMatrixRow[]
  coverage: MonitorCoverage | null
  healthMode: 'overall' | 'success' | 'ttft' | 'cache'
  showThroughput: boolean
  loading: boolean
}>()

const emit = defineEmits<{
  (event: 'select', row: MonitorMatrixRow): void
}>()

const { t } = useI18n()
const { providerLabel } = useChannelMonitorFormat()

const PLATFORM_ORDER = ['openai', 'anthropic', 'grok', 'gemini', 'antigravity', 'kiro', 'kimi', 'deepseek', 'zhipu']

const platformGroups = computed(() => {
  const groups = new Map<string, MonitorMatrixRow[]>()
  for (const row of props.rows) {
    const platform = row.platform.trim().toLowerCase() || 'unknown'
    const existing = groups.get(platform)
    if (existing) {
      existing.push(row)
    } else {
      groups.set(platform, [row])
    }
  }
  return Array.from(groups, ([platform, rows]) => ({ platform, rows })).sort((left, right) => {
    const leftIndex = PLATFORM_ORDER.indexOf(left.platform)
    const rightIndex = PLATFORM_ORDER.indexOf(right.platform)
    if (leftIndex >= 0 || rightIndex >= 0) {
      return (leftIndex < 0 ? PLATFORM_ORDER.length : leftIndex) - (rightIndex < 0 ? PLATFORM_ORDER.length : rightIndex)
    }
    return left.platform.localeCompare(right.platform)
  })
})

function rowKey(row: MonitorMatrixRow) {
  return [row.platform, row.group_id || 0, row.model || ''].join(':')
}
</script>
