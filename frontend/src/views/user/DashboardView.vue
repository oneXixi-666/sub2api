<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="promo-dashboard-hero">
        <div>
          <p class="promo-dashboard-eyebrow">{{ t('dashboard.workspaceEyebrow') }}</p>
          <h2>{{ t('dashboard.workspaceTitle') }}</h2>
          <p>{{ t('dashboard.workspaceDescription') }}</p>
        </div>
        <div class="promo-dashboard-hero-actions">
          <span class="promo-dashboard-range">{{ startDate }} / {{ endDate }}</span>
          <button type="button" class="btn btn-primary" :disabled="refreshing" @click="refreshAll">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': refreshing }" />
            {{ t('common.refresh') }}
          </button>
        </div>
      </section>

      <div v-if="loadingOverview && !stats" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else>
        <div v-if="statsError" class="promo-section-error" role="alert">
          <div>
            <strong>{{ t('dashboard.overviewLoadFailed') }}</strong>
            <p>{{ statsError }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="loadOverview">
            {{ t('common.retry') }}
          </button>
        </div>

        <template v-if="stats">
          <UserDashboardStats
            :stats="stats"
            :balance="user?.balance || 0"
            :is-simple="authStore.isSimpleMode"
            :platform-quotas="platformQuotas"
          />
          <p v-if="quotaError" class="promo-inline-warning" role="status">{{ quotaError }}</p>
        </template>

        <div v-if="chartsError" class="promo-section-error" role="alert">
          <div>
            <strong>{{ t('dashboard.chartsLoadFailed') }}</strong>
            <p>{{ chartsError }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="loadRangeData">
            {{ t('common.retry') }}
          </button>
        </div>

        <UserDashboardCharts
          v-model:startDate="startDate"
          v-model:endDate="endDate"
          v-model:granularity="granularity"
          :loading="loadingCharts"
          :trend="trendData"
          :models="modelStats"
          @dateRangeChange="loadRangeData"
          @granularityChange="loadRangeData"
          @refresh="refreshAll"
        />

        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="min-w-0 lg:col-span-2">
            <div v-if="usageError" class="promo-section-error mb-4" role="alert">
              <div>
                <strong>{{ t('dashboard.recentLoadFailed') }}</strong>
                <p>{{ usageError }}</p>
              </div>
              <button type="button" class="btn btn-secondary btn-sm" @click="loadRangeData">
                {{ t('common.retry') }}
              </button>
            </div>
            <UserDashboardRecentUsage
              :data="recentUsage"
              :loading="loadingUsage"
              :range-label="recentRangeLabel"
            />
          </div>
          <div class="min-w-0 lg:col-span-1">
            <UserDashboardQuickActions />
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { getMyPlatformQuotas } from '@/api/user'
import { getTodayDateRange } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const user = computed(() => authStore.user)

const stats = ref<UserStatsType | null>(null)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)

const loadingOverview = ref(false)
const loadingCharts = ref(false)
const loadingUsage = ref(false)
const statsError = ref('')
const chartsError = ref('')
const usageError = ref('')
const quotaError = ref('')

const defaultRange = getTodayDateRange()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const granularity = ref<'day' | 'hour'>('hour')
const recentRangeLabel = computed(() => `${startDate.value} - ${endDate.value}`)
const refreshing = computed(() => loadingOverview.value || loadingCharts.value || loadingUsage.value)

let overviewSequence = 0
let rangeSequence = 0
let rangeController: AbortController | null = null

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) return error.message
  return fallback
}

function isCanceled(error: unknown): boolean {
  const candidate = error as { code?: string; name?: string }
  return candidate?.code === 'ERR_CANCELED' || candidate?.name === 'CanceledError' || candidate?.name === 'AbortError'
}

async function loadOverview(): Promise<void> {
  const sequence = ++overviewSequence
  loadingOverview.value = true
  statsError.value = ''
  quotaError.value = ''

  const [identityResult, statsResult, quotaResult] = await Promise.allSettled([
    authStore.refreshUser(),
    usageAPI.getDashboardStats(),
    getMyPlatformQuotas()
  ])

  if (sequence !== overviewSequence) return

  if (identityResult.status === 'rejected') {
    console.warn('Failed to refresh dashboard identity:', identityResult.reason)
  }

  if (statsResult.status === 'fulfilled') {
    stats.value = statsResult.value
  } else {
    statsError.value = errorMessage(statsResult.reason, t('dashboard.overviewLoadFailed'))
  }

  if (quotaResult.status === 'fulfilled') {
    platformQuotas.value = quotaResult.value.platform_quotas ?? []
  } else {
    platformQuotas.value = []
    quotaError.value = errorMessage(quotaResult.reason, t('dashboard.quotaLoadFailed'))
  }

  loadingOverview.value = false
}

async function loadRangeData(): Promise<void> {
  rangeController?.abort()
  const controller = new AbortController()
  rangeController = controller
  const sequence = ++rangeSequence

  loadingCharts.value = true
  loadingUsage.value = true
  chartsError.value = ''
  usageError.value = ''

  const [snapshotResult, usageResult] = await Promise.allSettled([
    usageAPI.getDashboardSnapshotV2(
      {
        include_trend: true,
        include_model_stats: true,
        include_group_stats: false,
        start_date: startDate.value,
        end_date: endDate.value,
        granularity: granularity.value
      },
      { signal: controller.signal }
    ),
    usageAPI.query(
      {
        page: 1,
        page_size: 5,
        start_date: startDate.value,
        end_date: endDate.value
      },
      { signal: controller.signal }
    )
  ])

  if (sequence !== rangeSequence || controller.signal.aborted) return

  if (snapshotResult.status === 'fulfilled') {
    trendData.value = snapshotResult.value.trend ?? []
    modelStats.value = snapshotResult.value.models ?? []
  } else if (!isCanceled(snapshotResult.reason)) {
    chartsError.value = errorMessage(snapshotResult.reason, t('dashboard.chartsLoadFailed'))
  }

  if (usageResult.status === 'fulfilled') {
    recentUsage.value = usageResult.value.items ?? []
  } else if (!isCanceled(usageResult.reason)) {
    usageError.value = errorMessage(usageResult.reason, t('dashboard.recentLoadFailed'))
  }

  loadingCharts.value = false
  loadingUsage.value = false
}

async function refreshAll(): Promise<void> {
  await Promise.all([loadOverview(), loadRangeData()])
}

onMounted(() => {
  void refreshAll()
})

onBeforeUnmount(() => {
  overviewSequence += 1
  rangeSequence += 1
  rangeController?.abort()
})
</script>
