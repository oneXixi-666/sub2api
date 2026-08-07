<template>
  <AppLayout>
    <div class="upstream-funds-page space-y-6">
      <div class="grid grid-cols-1 gap-3 sm:flex sm:flex-wrap sm:items-center">
        <div class="relative w-full min-w-0 sm:min-w-[240px] sm:flex-1 sm:max-w-sm">
          <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
          <input
            v-model="searchQuery"
            class="input pl-10"
            type="search"
            :placeholder="t('admin.upstreamFunds.searchPlaceholder')"
            @input="handleSearch"
          />
        </div>
        <div class="flex w-full flex-wrap justify-end gap-2 sm:flex-1">
          <button type="button" class="btn btn-secondary btn-icon" :disabled="loading" :title="t('common.refresh')" @click="loadOverview">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button type="button" class="btn btn-primary whitespace-nowrap" @click="openCreateDialog">
            <Icon name="plus" size="md" class="mr-2" />
            {{ t('admin.upstreamFunds.createWallet') }}
          </button>
        </div>
      </div>

      <section class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4" :aria-label="t('admin.upstreamFunds.title')">
        <article class="summary-tile">
          <div class="summary-icon summary-icon-red"><Icon name="creditCard" size="md" /></div>
          <div class="summary-content">
            <p class="summary-label">{{ t('admin.upstreamFunds.summary.wallets') }}</p>
            <p class="summary-value">{{ summary.wallet_count }}</p>
            <p class="summary-note">{{ t('admin.upstreamFunds.summary.enabled', { count: summary.enabled_count }) }}</p>
          </div>
        </article>
        <article class="summary-tile">
          <div class="summary-icon summary-icon-cyan"><Icon name="trendingUp" size="md" /></div>
          <div class="summary-content">
            <p class="summary-label">{{ t('admin.upstreamFunds.summary.todayCost') }}</p>
            <p class="summary-value data-number">{{ formatCurrency(summary.cost_today, 'USD') }}</p>
            <p class="summary-note">{{ t('admin.upstreamFunds.summary.costCurrency') }}</p>
          </div>
        </article>
        <article class="summary-tile">
          <div class="summary-icon summary-icon-yellow"><Icon name="dollar" size="md" /></div>
          <div class="summary-content">
            <p class="summary-label">{{ t('admin.upstreamFunds.summary.balance') }}</p>
            <p class="summary-value data-number">{{ formattedBalances }}</p>
            <p class="summary-note">{{ formatCurrency(summary.cost_24h, 'USD') }} / {{ t('admin.upstreamFunds.summary.cost24h') }}</p>
          </div>
        </article>
        <article class="summary-tile" :class="summary.attention_count > 0 ? 'summary-tile-alert' : ''">
          <div class="summary-icon" :class="summary.attention_count > 0 ? 'summary-icon-red' : 'summary-icon-green'"><Icon name="exclamationTriangle" size="md" /></div>
          <div class="summary-content">
            <p class="summary-label">{{ t('admin.upstreamFunds.summary.attention') }}</p>
            <p class="summary-value">{{ summary.attention_count }}</p>
            <p class="summary-note">{{ t('admin.upstreamFunds.summary.enabled', { count: summary.enabled_count }) }}</p>
          </div>
        </article>
      </section>

      <div class="adapter-strip">
        <div class="flex min-w-0 flex-1 items-start gap-3">
          <div class="adapter-mark"><Icon name="beaker" size="md" /></div>
          <div class="min-w-0">
            <p class="adapter-title">{{ t('admin.upstreamFunds.adapterPending') }}</p>
            <p class="adapter-copy">{{ t('admin.upstreamFunds.adapterPendingHint') }}</p>
          </div>
        </div>
        <span class="badge badge-warning shrink-0 self-start sm:self-auto">{{ t('admin.upstreamFunds.phaseOne') }}</span>
      </div>

      <section v-if="loading && wallets.length === 0" class="wallet-grid" aria-live="polite">
        <div v-for="i in 2" :key="i" class="wallet-card wallet-skeleton">
          <div class="h-5 w-2/5 animate-pulse rounded bg-gray-200 dark:bg-dark-600"></div>
          <div class="mt-8 h-10 w-1/2 animate-pulse rounded bg-gray-200 dark:bg-dark-600"></div>
          <div class="mt-8 h-2 animate-pulse rounded bg-gray-200 dark:bg-dark-600"></div>
        </div>
      </section>

      <section v-else-if="wallets.length" class="wallet-grid">
        <article v-for="wallet in wallets" :key="wallet.id" class="wallet-card" :class="{ 'wallet-card-alert': wallet.needs_attention }">
          <header class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="wallet-name truncate">{{ wallet.name }}</h2>
                <span class="badge badge-gray">{{ tierLabel(wallet.tier) }}</span>
                <span v-if="wallet.needs_attention" class="badge badge-danger">{{ t('admin.upstreamFunds.wallet.attention') }}</span>
              </div>
              <p class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-300">
                <span class="font-mono">{{ wallet.provider }}</span>
                <span class="text-gray-300 dark:text-dark-600">/</span>
                <span>{{ modeLabel(wallet.recharge_mode) }}</span>
                <span v-if="!wallet.enabled" class="badge badge-gray">{{ t('admin.upstreamFunds.wallet.disabled') }}</span>
              </p>
            </div>
            <Icon name="creditCard" size="lg" class="shrink-0 text-[var(--promo-red)]" />
          </header>

          <div class="mt-6 flex flex-wrap items-end justify-between gap-4">
            <div>
              <p class="metric-kicker">{{ t('admin.upstreamFunds.wallet.balance') }}</p>
              <p class="balance-value data-number">{{ formatWalletBalance(wallet) }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">
                {{ wallet.balance_updated_at ? t('admin.upstreamFunds.wallet.updated', { time: formatRelativeTime(wallet.balance_updated_at) }) : t('admin.upstreamFunds.wallet.neverUpdated') }}
              </p>
            </div>
            <div class="text-right">
              <p class="metric-kicker">{{ t('admin.upstreamFunds.wallet.runway') }}</p>
              <p class="runway-value data-number" :class="runwayClass(wallet)">
                {{ wallet.runway_days === null ? t('admin.upstreamFunds.wallet.runwayUnknown') : t('admin.upstreamFunds.wallet.runwayDays', { days: wallet.runway_days.toFixed(1) }) }}
              </p>
            </div>
          </div>

          <div class="runway-rail mt-4" :class="runwayClass(wallet)" :title="runwayTitle(wallet)">
            <div class="runway-fill" :style="{ width: `${runwayPercent(wallet)}%` }"></div>
            <span class="runway-marker runway-marker-alert" :style="{ left: `${markerPercent(wallet.alert_days, wallet.target_days)}%` }"></span>
            <span class="runway-marker runway-marker-target" :style="{ left: '100%' }"></span>
          </div>
          <div class="mt-1 flex justify-between text-[11px] text-gray-500 dark:text-dark-300">
            <span>{{ t('admin.upstreamFunds.wallet.alertLine', { days: wallet.alert_days }) }}</span>
            <span>{{ wallet.currency }} / {{ t('admin.upstreamFunds.wallet.targetLine', { days: wallet.target_days }) }}</span>
          </div>

          <div class="cost-grid mt-5">
            <div><span>{{ t('admin.upstreamFunds.wallet.cost1h') }}</span><strong>{{ formatCurrency(wallet.cost_1h, wallet.cost_currency) }}</strong></div>
            <div><span>{{ t('admin.upstreamFunds.wallet.costToday') }}</span><strong>{{ formatCurrency(wallet.cost_today, wallet.cost_currency) }}</strong></div>
            <div><span>{{ t('admin.upstreamFunds.wallet.cost24h') }}</span><strong>{{ formatCurrency(wallet.cost_24h, wallet.cost_currency) }}</strong></div>
            <div><span>{{ t('admin.upstreamFunds.wallet.cost7d') }}</span><strong>{{ formatCurrency(wallet.cost_7d, wallet.cost_currency) }}</strong></div>
          </div>

          <div class="mt-5 grid grid-cols-1 gap-4 border-t border-[var(--promo-border-soft)] pt-4 sm:grid-cols-2">
            <div>
              <p class="metric-kicker">{{ t('admin.upstreamFunds.wallet.accounts') }} <span class="font-mono">({{ wallet.accounts.length }})</span></p>
              <div v-if="wallet.accounts.length" class="tag-list mt-2">
                <span v-for="account in wallet.accounts.slice(0, 4)" :key="account.id" class="tag">{{ account.name }}</span>
                <span v-if="wallet.accounts.length > 4" class="tag tag-muted">+{{ wallet.accounts.length - 4 }}</span>
              </div>
              <p v-else class="mt-2 text-xs text-gray-500 dark:text-dark-300">{{ t('admin.upstreamFunds.wallet.noAccounts') }}</p>
            </div>
            <div>
              <p class="metric-kicker">{{ t('admin.upstreamFunds.wallet.actualGroups') }}</p>
              <div v-if="wallet.actual_groups.length" class="tag-list mt-2">
                <span v-for="group in wallet.actual_groups.slice(0, 3)" :key="group.id" class="tag tag-cyan">{{ group.name }}</span>
                <span v-if="wallet.actual_groups.length > 3" class="tag tag-muted">+{{ wallet.actual_groups.length - 3 }}</span>
              </div>
              <p v-else class="mt-2 text-xs text-gray-500 dark:text-dark-300">{{ t('admin.upstreamFunds.wallet.noGroups') }}</p>
            </div>
          </div>

          <footer class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-[var(--promo-border-soft)] pt-4">
            <div v-if="wallet.recommended_top_up !== null" class="text-xs font-bold text-[var(--promo-red-strong)] dark:text-[var(--promo-yellow)]">
              {{ wallet.recommended_top_up > 0 ? t('admin.upstreamFunds.wallet.suggestedTopUp', { amount: formatCurrency(wallet.recommended_top_up, wallet.currency) }) : t('admin.upstreamFunds.wallet.healthyReserve') }}
            </div>
            <div v-else class="text-xs text-gray-500 dark:text-dark-300">{{ runwayUnavailableReason(wallet) }}</div>
            <div class="flex items-center gap-1">
              <button type="button" class="btn btn-ghost btn-sm" :title="t('admin.upstreamFunds.recordBalance')" @click="openBalanceDialog(wallet)"><Icon name="dollar" size="sm" /><span class="ml-1 hidden sm:inline">{{ t('admin.upstreamFunds.recordBalance') }}</span></button>
              <button type="button" class="btn btn-ghost btn-sm" :title="t('common.edit')" @click="openEditDialog(wallet)"><Icon name="edit" size="sm" /><span class="ml-1 hidden sm:inline">{{ t('common.edit') }}</span></button>
              <button type="button" class="btn btn-secondary btn-sm" disabled :title="t('admin.upstreamFunds.adapterPendingHint')"><Icon name="creditCard" size="sm" /><span class="ml-1 hidden sm:inline">{{ t('admin.upstreamFunds.recharge') }}</span></button>
            </div>
          </footer>
        </article>
      </section>

      <div v-else class="wallet-empty">
        <div class="empty-mark"><Icon name="creditCard" size="xl" /></div>
        <h2>{{ t('admin.upstreamFunds.empty.title') }}</h2>
        <p>{{ t('admin.upstreamFunds.empty.description') }}</p>
        <button type="button" class="btn btn-primary mt-4" @click="openCreateDialog"><Icon name="plus" size="md" class="mr-2" />{{ t('admin.upstreamFunds.createWallet') }}</button>
      </div>
    </div>

    <BaseDialog :show="showWalletDialog" :title="editingWallet ? t('admin.upstreamFunds.editWallet') : t('admin.upstreamFunds.createWallet')" width="wide" @close="closeWalletDialog">
      <form id="upstream-wallet-form" class="space-y-5" @submit.prevent="saveWallet">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div><label class="input-label">{{ t('admin.upstreamFunds.form.name') }}</label><input v-model="walletForm.name" class="input" required :placeholder="t('admin.upstreamFunds.form.namePlaceholder')" /></div>
          <div><label class="input-label">{{ t('admin.upstreamFunds.form.provider') }}</label><input v-model="walletForm.provider" class="input font-mono" required :placeholder="t('admin.upstreamFunds.form.providerPlaceholder')" /></div>
          <div><label class="input-label">{{ t('admin.upstreamFunds.form.currency') }}</label><input v-model="walletForm.currency" class="input font-mono uppercase" maxlength="3" minlength="3" required /></div>
          <div><label class="input-label">{{ t('admin.upstreamFunds.form.rechargeMode') }}</label><Select v-model="walletForm.recharge_mode" :options="modeOptions" /></div>
          <div><label class="input-label">{{ t('admin.upstreamFunds.form.tier') }}</label><Select v-model="walletForm.tier" :options="tierOptions" /></div>
          <div class="grid grid-cols-2 gap-3">
            <div><label class="input-label">{{ t('admin.upstreamFunds.form.alertDays') }}</label><input v-model.number="walletForm.alert_days" class="input" type="number" min="0" max="365" required /></div>
            <div><label class="input-label">{{ t('admin.upstreamFunds.form.targetDays') }}</label><input v-model.number="walletForm.target_days" class="input" type="number" min="0" max="365" required /></div>
          </div>
        </div>
        <label class="flex cursor-pointer items-center gap-2 text-sm font-bold text-[var(--promo-text)]"><input v-model="walletForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />{{ t('admin.upstreamFunds.form.enabled') }}</label>
        <div class="border-t border-[var(--promo-border-soft)] pt-4">
          <div class="flex flex-wrap items-center justify-between gap-2"><label class="input-label mb-0">{{ t('admin.upstreamFunds.form.accounts') }}</label><span class="text-xs text-gray-500 dark:text-dark-300">{{ t('admin.upstreamFunds.form.selectedAccounts', { count: walletForm.account_ids.length }) }}</span></div>
          <div class="relative mt-2"><Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" /><input v-model="accountSearch" class="input pl-9" :placeholder="t('admin.upstreamFunds.form.accountSearch')" /></div>
          <div class="account-picker mt-3">
            <label v-for="account in filteredAccountOptions" :key="account.id" class="account-option" :class="{ 'account-option-disabled': isAccountUnavailable(account) }">
              <input type="checkbox" :checked="walletForm.account_ids.includes(account.id)" :disabled="isAccountUnavailable(account)" @change="toggleAccount(account.id)" />
              <span class="min-w-0 flex-1"><span class="block truncate font-medium">{{ account.name }}</span><span class="block truncate text-xs text-gray-500 dark:text-dark-300">{{ account.platform }} · #{{ account.id }}</span></span>
              <span v-if="isAccountUnavailable(account)" class="text-[11px] text-gray-500">{{ t('admin.upstreamFunds.form.accountOwned', { wallet: account.wallet_name }) }}</span>
            </label>
            <p v-if="filteredAccountOptions.length === 0" class="px-3 py-6 text-center text-sm text-gray-500">{{ t('common.noOptionsFound') }}</p>
          </div>
        </div>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button type="button" class="btn btn-secondary" @click="closeWalletDialog">{{ t('common.cancel') }}</button><button type="submit" form="upstream-wallet-form" class="btn btn-primary" :disabled="saving"><Icon v-if="saving" name="refresh" size="sm" class="mr-2 animate-spin" />{{ saving ? t('common.saving') : t('common.save') }}</button></div></template>
    </BaseDialog>

    <BaseDialog :show="showBalanceDialog" :title="t('admin.upstreamFunds.recordBalance')" @close="closeBalanceDialog">
      <form id="upstream-balance-form" class="space-y-4" @submit.prevent="saveBalance">
        <div><p class="text-sm font-bold text-[var(--promo-text)]">{{ balanceWallet?.name }}</p><p class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ balanceWallet?.provider }} · {{ balanceWallet?.currency }}</p></div>
        <div><label class="input-label">{{ t('admin.upstreamFunds.form.balance') }}</label><input v-model.number="balanceValue" class="input data-number" type="number" min="0" step="0.000001" required autofocus /><p class="input-hint">{{ t('admin.upstreamFunds.form.balanceHint') }}</p></div>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button type="button" class="btn btn-secondary" @click="closeBalanceDialog">{{ t('common.cancel') }}</button><button type="submit" form="upstream-balance-form" class="btn btn-primary" :disabled="savingBalance">{{ savingBalance ? t('common.saving') : t('common.save') }}</button></div></template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type UpstreamFundsAccount, type UpstreamFundsSummary, type UpstreamWallet, type UpstreamWalletInput } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatCurrency, formatRelativeTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const wallets = ref<UpstreamWallet[]>([])
const accountOptions = ref<UpstreamFundsAccount[]>([])
const searchQuery = ref('')
const accountSearch = ref('')
const loading = ref(false)
const saving = ref(false)
const savingBalance = ref(false)
const showWalletDialog = ref(false)
const showBalanceDialog = ref(false)
const editingWallet = ref<UpstreamWallet | null>(null)
const balanceWallet = ref<UpstreamWallet | null>(null)
const balanceValue = ref<number | null>(null)
const summary = reactive<UpstreamFundsSummary>({ wallet_count: 0, enabled_count: 0, attention_count: 0, cost_today: 0, cost_24h: 0, balance_by_currency: {} })
const walletForm = reactive<UpstreamWalletInput>({ name: '', provider: '', currency: 'USD', recharge_mode: 'manual', tier: 'primary', enabled: true, alert_days: 2, target_days: 7, account_ids: [] })

const modeOptions = computed(() => (['direct', 'product', 'manual'] as const).map(value => ({ value, label: t(`admin.upstreamFunds.mode.${value}`) })))
const tierOptions = computed(() => (['primary', 'hot_backup', 'cold_backup'] as const).map(value => ({ value, label: t(`admin.upstreamFunds.tier.${value}`) })))
const filteredAccountOptions = computed(() => {
  const needle = accountSearch.value.trim().toLowerCase()
  if (!needle) return accountOptions.value
  return accountOptions.value.filter(account => `${account.name} ${account.platform} ${account.id}`.toLowerCase().includes(needle))
})
const formattedBalances = computed(() => {
  const entries = Object.entries(summary.balance_by_currency)
  return entries.length ? entries.map(([currency, amount]) => formatCurrency(amount, currency)).join(' / ') : '—'
})

let searchTimer: number | null = null
async function loadOverview() {
  loading.value = true
  try {
    const overview = await adminAPI.upstreamFunds.list(searchQuery.value.trim() || undefined)
    wallets.value = overview.wallets || []
    Object.assign(summary, overview.summary)
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.upstreamFunds.messages.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadAccounts() {
  try {
    accountOptions.value = await adminAPI.upstreamFunds.listAccounts()
  } catch (error) {
    console.error('Failed to load upstream account options', error)
  }
}

function handleSearch() {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(loadOverview, 280)
}

function resetWalletForm() {
  Object.assign(walletForm, { name: '', provider: '', currency: 'USD', recharge_mode: 'manual', tier: 'primary', enabled: true, alert_days: 2, target_days: 7, account_ids: [] })
  accountSearch.value = ''
}

function openCreateDialog() {
  editingWallet.value = null
  resetWalletForm()
  showWalletDialog.value = true
  void loadAccounts()
}

function openEditDialog(wallet: UpstreamWallet) {
  editingWallet.value = wallet
  Object.assign(walletForm, { name: wallet.name, provider: wallet.provider, currency: wallet.currency, recharge_mode: wallet.recharge_mode, tier: wallet.tier, enabled: wallet.enabled, alert_days: wallet.alert_days, target_days: wallet.target_days, account_ids: [...wallet.account_ids] })
  accountSearch.value = ''
  showWalletDialog.value = true
  void loadAccounts()
}

function closeWalletDialog() { showWalletDialog.value = false; editingWallet.value = null }

function isAccountUnavailable(account: UpstreamFundsAccount) {
  return Boolean(account.wallet_id && account.wallet_id !== editingWallet.value?.id)
}

function toggleAccount(id: number) {
  const index = walletForm.account_ids.indexOf(id)
  if (index === -1) walletForm.account_ids.push(id)
  else walletForm.account_ids.splice(index, 1)
}

async function saveWallet() {
  if (walletForm.target_days < walletForm.alert_days) {
    appStore.showError(t('admin.upstreamFunds.messages.invalidReserve'))
    return
  }
  saving.value = true
  try {
    if (editingWallet.value) {
      await adminAPI.upstreamFunds.update(editingWallet.value.id, walletForm)
      appStore.showSuccess(t('admin.upstreamFunds.messages.updated'))
    } else {
      await adminAPI.upstreamFunds.create(walletForm)
      appStore.showSuccess(t('admin.upstreamFunds.messages.created'))
    }
    closeWalletDialog()
    await loadOverview()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.upstreamFunds.messages.saveFailed'))
  } finally {
    saving.value = false
  }
}

function openBalanceDialog(wallet: UpstreamWallet) {
  balanceWallet.value = wallet
  balanceValue.value = wallet.balance
  showBalanceDialog.value = true
}

function closeBalanceDialog() { showBalanceDialog.value = false; balanceWallet.value = null; balanceValue.value = null }

async function saveBalance() {
  if (!balanceWallet.value || balanceValue.value === null || !Number.isFinite(balanceValue.value) || balanceValue.value < 0) return
  savingBalance.value = true
  try {
    await adminAPI.upstreamFunds.recordBalance(balanceWallet.value.id, balanceValue.value)
    appStore.showSuccess(t('admin.upstreamFunds.messages.balanceRecorded'))
    closeBalanceDialog()
    await loadOverview()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.upstreamFunds.messages.balanceFailed'))
  } finally {
    savingBalance.value = false
  }
}

function tierLabel(value: UpstreamWallet['tier']) { return t(`admin.upstreamFunds.tier.${value}`) }
function modeLabel(value: UpstreamWallet['recharge_mode']) { return t(`admin.upstreamFunds.mode.${value}`) }
function formatWalletBalance(wallet: UpstreamWallet) { return wallet.balance === null ? '—' : formatCurrency(wallet.balance, wallet.currency) }
function runwayClass(wallet: UpstreamWallet) { return wallet.runway_days !== null && wallet.runway_days < wallet.alert_days ? 'runway-low' : wallet.runway_days !== null ? 'runway-healthy' : 'runway-unknown' }
function runwayPercent(wallet: UpstreamWallet) { return wallet.runway_days === null || wallet.target_days <= 0 ? 0 : Math.min(100, Math.max(0, wallet.runway_days / wallet.target_days * 100)) }
function markerPercent(days: number, target: number) { return target <= 0 ? 0 : Math.min(100, Math.max(0, days / target * 100)) }
function runwayTitle(wallet: UpstreamWallet) { return wallet.runway_days === null ? runwayUnavailableReason(wallet) : `${t('admin.upstreamFunds.wallet.alertLine', { days: wallet.alert_days })} / ${t('admin.upstreamFunds.wallet.targetLine', { days: wallet.target_days })}` }
function runwayUnavailableReason(wallet: UpstreamWallet) { return wallet.currency !== wallet.cost_currency ? t('admin.upstreamFunds.wallet.runwayCurrencyMismatch') : t('admin.upstreamFunds.wallet.runwayNoCost') }

onMounted(() => { void Promise.all([loadOverview(), loadAccounts()]) })
</script>

<style scoped>
.upstream-funds-page { color: var(--promo-text); }
.summary-tile { display: flex; align-items: center; gap: 0.85rem; min-height: 104px; padding: 1rem; border: var(--promo-border-width) solid var(--promo-border); border-radius: var(--promo-radius-sm); background: var(--promo-surface-raised); box-shadow: var(--promo-shadow-sm); }
.summary-content { min-width: 0; flex: 1; }
.summary-tile-alert { border-color: var(--promo-red); }
.summary-icon { display: flex; height: 2.55rem; width: 2.55rem; flex-shrink: 0; align-items: center; justify-content: center; border: 2px solid var(--promo-black); border-radius: 50%; }
.summary-icon-red { background: var(--promo-red); color: #fff; }
.summary-icon-cyan { background: var(--promo-cyan); color: var(--promo-black); }
.summary-icon-yellow { background: var(--promo-yellow); color: var(--promo-black); }
.summary-icon-green { background: var(--promo-green); color: #fff; }
.summary-label, .metric-kicker { margin: 0; color: var(--promo-text-muted); font-size: 0.68rem; font-weight: 900; letter-spacing: 0; text-transform: uppercase; }
.summary-value { margin: 0.18rem 0 0; font-family: var(--promo-font-display); font-size: 1.45rem; line-height: 1.1; }
.summary-note { margin: 0.3rem 0 0; color: var(--promo-text-muted); font-size: 0.72rem; }
.data-number { font-family: var(--promo-font-data); letter-spacing: 0; }
.adapter-strip { display: flex; align-items: center; justify-content: space-between; gap: 1rem; border: 2px dashed var(--promo-border); border-radius: var(--promo-radius-sm); background: var(--promo-surface-muted); padding: 0.9rem 1rem; }
@media (max-width: 639px) { .adapter-strip { align-items: stretch; flex-direction: column; } }
.adapter-mark { display: flex; height: 2rem; width: 2rem; align-items: center; justify-content: center; flex-shrink: 0; border-radius: var(--promo-radius-xs); background: var(--promo-black); color: var(--promo-yellow); }
.adapter-title { margin: 0; font-size: 0.85rem; font-weight: 900; }
.adapter-copy { margin: 0.15rem 0 0; color: var(--promo-text-muted); font-size: 0.75rem; line-height: 1.45; }
.wallet-grid { display: grid; grid-template-columns: repeat(1, minmax(0, 1fr)); gap: 1.25rem; }
@media (min-width: 1280px) { .wallet-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
.wallet-card { min-width: 0; border: var(--promo-border-width) solid var(--promo-border); border-radius: var(--promo-radius-sm); background: var(--promo-card-surface); padding: 1.25rem; box-shadow: var(--promo-shadow-md); }
.wallet-card-alert { border-color: var(--promo-red); }
.wallet-skeleton { min-height: 330px; }
.wallet-name { margin: 0; font-family: var(--promo-font-display); font-size: 1.05rem; line-height: 1.25; }
.balance-value { margin-top: 0.2rem; font-size: 1.8rem; font-weight: 900; line-height: 1; }
@media (min-width: 640px) { .balance-value { font-size: 2.45rem; } }
.runway-value { margin-top: 0.2rem; font-size: 1.15rem; font-weight: 900; }
.runway-low { color: var(--promo-red-strong); }
.runway-healthy { color: var(--promo-green); }
.runway-unknown { color: var(--promo-text-muted); }
.runway-rail { position: relative; height: 0.72rem; overflow: visible; border: 2px solid var(--promo-border); border-radius: 99px; background: var(--promo-surface-muted); }
.runway-fill { height: 100%; min-width: 0; border-radius: 99px; background: var(--promo-green); transition: width 240ms ease; }
.runway-low .runway-fill { background: var(--promo-red); }
.runway-unknown .runway-fill { background: var(--promo-border-soft); }
.runway-marker { position: absolute; top: -0.3rem; height: 1.25rem; width: 2px; background: var(--promo-black); }
.runway-marker-alert { background: var(--promo-red); }
.runway-marker-target { background: var(--promo-black); }
.cost-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.55rem; }
@media (min-width: 640px) { .cost-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
.cost-grid > div { min-width: 0; border-left: 3px solid var(--promo-yellow); padding-left: 0.6rem; }
.cost-grid span { display: block; color: var(--promo-text-muted); font-size: 0.68rem; }
.cost-grid strong { display: block; margin-top: 0.25rem; overflow: hidden; font-family: var(--promo-font-data); font-size: 0.88rem; text-overflow: ellipsis; white-space: nowrap; }
.tag-list { display: flex; flex-wrap: wrap; gap: 0.35rem; }
.tag { max-width: 100%; overflow: hidden; border: 1px solid var(--promo-border-soft); border-radius: 999px; background: var(--promo-surface-raised); padding: 0.18rem 0.48rem; color: var(--promo-text); font-size: 0.68rem; text-overflow: ellipsis; white-space: nowrap; }
.tag-cyan { border-color: var(--promo-cyan-strong); background: color-mix(in srgb, var(--promo-cyan) 24%, transparent); }
.tag-muted { color: var(--promo-text-muted); }
.wallet-empty { display: flex; min-height: 310px; flex-direction: column; align-items: center; justify-content: center; border: 2px dashed var(--promo-border); border-radius: var(--promo-radius-sm); background: var(--promo-surface-muted); padding: 2rem; text-align: center; }
.empty-mark { display: flex; height: 3.7rem; width: 3.7rem; align-items: center; justify-content: center; border: 2px solid var(--promo-black); border-radius: 50%; background: var(--promo-yellow); color: var(--promo-black); }
.wallet-empty h2 { margin: 1rem 0 0; font-family: var(--promo-font-display); font-size: 1.25rem; }
.wallet-empty p { max-width: 28rem; margin: 0.45rem 0 0; color: var(--promo-text-muted); font-size: 0.85rem; }
.account-picker { max-height: 14rem; overflow-y: auto; border: 2px solid var(--promo-border-soft); border-radius: var(--promo-radius-sm); background: var(--promo-surface); }
.account-option { display: flex; align-items: center; gap: 0.65rem; border-bottom: 1px solid var(--promo-border-soft); padding: 0.65rem 0.75rem; cursor: pointer; }
.account-option:last-child { border-bottom: 0; }
.account-option:hover { background: var(--promo-surface-hover); }
.account-option-disabled { cursor: not-allowed; opacity: 0.55; }
.account-option input { height: 1rem; width: 1rem; flex-shrink: 0; }
</style>
