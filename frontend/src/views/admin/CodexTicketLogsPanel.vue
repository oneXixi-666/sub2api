<template>
  <TablePageLayout>
    <template #filters>
      <div class="card p-4 sm:p-6">
        <div class="flex flex-wrap items-end justify-between gap-4">
          <p class="max-w-3xl text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.audit.ticketLogs.hint', { count: limit || '—' }) }}
          </p>
          <div class="flex flex-wrap items-center justify-end gap-3">
            <button type="button" class="btn btn-secondary" :disabled="loading" data-testid="codex-ticket-logs-refresh" @click="load">
              {{ t('common.refresh') }}
            </button>
            <button type="button" class="btn btn-danger" :disabled="loading || !items.length" data-testid="codex-ticket-logs-clear" @click="clearConfirmVisible = true">
              <Icon name="trash" size="sm" class="mr-1.5" />
              {{ t('admin.audit.ticketLogs.clear') }}
            </button>
          </div>
        </div>
      </div>
    </template>

    <template #table>
      <DataTable :columns="columns" :data="items" :loading="loading" row-key="id">
        <template #cell-time="{ value }">
          <span class="whitespace-nowrap text-gray-600 dark:text-gray-300">{{ formatTime(value) }}</span>
        </template>
        <template #cell-account_id="{ row }">
          <span class="font-mono text-gray-800 dark:text-gray-200">#{{ row.account_id }}</span>
        </template>
        <template #cell-model="{ value }">
          <span class="break-all font-mono text-xs text-gray-700 dark:text-gray-300">{{ value }}</span>
        </template>
        <template #cell-event="{ value }">
          <span class="whitespace-nowrap" :class="eventClass(value)">{{ eventLabel(value) }}</span>
        </template>
        <template #cell-reason="{ value }">
          <span class="text-gray-700 dark:text-gray-300">{{ reasonLabel(value) }}</span>
        </template>
        <template #cell-http_status="{ value }">
          <span class="tabular-nums text-gray-600 dark:text-gray-300">{{ value || '—' }}</span>
        </template>
        <template #cell-egress_ip="{ row }">
          <span class="whitespace-nowrap font-mono text-gray-600 dark:text-gray-300">{{ row.egress_ip || '—' }}</span>
        </template>
        <template #cell-length="{ row }">
          <span class="whitespace-nowrap tabular-nums text-gray-600 dark:text-gray-300">{{ row.ticket_length ?? '—' }} / {{ row.target_length }}</span>
        </template>
        <template #cell-duration_ms="{ value }">
          <span class="whitespace-nowrap tabular-nums text-gray-500 dark:text-gray-400">{{ value === undefined || value === null ? '—' : `${value} ms` }}</span>
        </template>
        <template #empty>
          <div class="flex flex-col items-center py-8">
            <Icon name="document" size="xl" class="mb-4 h-12 w-12 text-gray-300 dark:text-dark-600" />
            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('admin.audit.ticketLogs.empty') }}</p>
          </div>
        </template>
      </DataTable>
    </template>
  </TablePageLayout>

  <ConfirmDialog
    :show="clearConfirmVisible"
    :title="t('admin.audit.ticketLogs.clearConfirmTitle')"
    :message="t('admin.audit.ticketLogs.clearConfirmMessage')"
    :confirm-text="t('admin.audit.ticketLogs.clear')"
    :cancel-text="t('common.cancel')"
    danger
    @confirm="clearLogs"
    @cancel="clearConfirmVisible = false"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import type { CodexTicketLogFeedItem } from '@/types'
import { useAppStore } from '@/stores'

const { t, locale } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const items = ref<CodexTicketLogFeedItem[]>([])
const limit = ref(0)
const clearConfirmVisible = ref(false)

const knownEvents = new Set(['started', 'success', 'miss', 'error', 'skipped', 'invalidated'])
const knownReasons = new Set([
  'request_started', 'harvested', 'http_error', 'quota_exhausted', 'missing_state', 'length_mismatch',
  'invalid_state', 'token_error', 'token_invalid', 'timeout', 'canceled', 'network_error', 'request_error',
  'degraded_length', 'model_mismatch'
])

const columns = computed<Column[]>(() => [
  { key: 'time', label: t('admin.accounts.openai.codexTicketLogs.time') },
  { key: 'account_id', label: t('admin.audit.ticketLogs.account') },
  { key: 'model', label: t('admin.audit.ticketLogs.model') },
  { key: 'event', label: t('admin.accounts.openai.codexTicketLogs.result') },
  { key: 'reason', label: t('admin.accounts.openai.codexTicketLogs.reason') },
  { key: 'http_status', label: 'HTTP' },
  { key: 'egress_ip', label: t('admin.accounts.openai.codexTicketLogs.egressIp') },
  { key: 'length', label: t('admin.accounts.openai.codexTicketLogs.length') },
  { key: 'duration_ms', label: t('admin.accounts.openai.codexTicketLogs.duration') },
])

function eventLabel(event: string) {
  return t(`admin.accounts.openai.codexTicketLogs.events.${knownEvents.has(event) ? event : 'unknown'}`)
}

function reasonLabel(reason: string) {
  return t(`admin.accounts.openai.codexTicketLogs.reasons.${knownReasons.has(reason) ? reason : 'unknown'}`)
}

function eventClass(event: string) {
  if (event === 'success') return 'text-emerald-600 dark:text-emerald-400'
  if (event === 'error' || event === 'invalidated') return 'text-red-600 dark:text-red-400'
  if (event === 'miss') return 'text-amber-600 dark:text-amber-400'
  if (event === 'started') return 'text-blue-600 dark:text-blue-400'
  return 'text-gray-500 dark:text-gray-400'
}

function formatTime(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  return date.toLocaleString(locale.value, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
}

async function load() {
  loading.value = true
  try {
    const feed = await adminAPI.accounts.listCodexTicketLogs()
    items.value = feed.items || []
    limit.value = feed.limit || 0
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.ticketLogs.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function clearLogs() {
  clearConfirmVisible.value = false
  loading.value = true
  try {
    await adminAPI.accounts.clearCodexTicketLogs()
    items.value = []
    appStore.showSuccess(t('admin.audit.ticketLogs.clearSuccess'))
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.ticketLogs.loadFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
