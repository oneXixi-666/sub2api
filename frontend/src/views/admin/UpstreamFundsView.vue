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
          <button type="button" class="btn btn-secondary btn-icon" :disabled="loading || syncCycleActive" :title="t('common.refresh')" @click="syncAllBalances(true)">
            <Icon name="refresh" size="md" :class="loading || syncCycleActive ? 'animate-spin' : ''" />
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

      <nav class="balance-tabs" role="tablist" :aria-label="t('admin.upstreamFunds.tabs.label')">
        <button
          v-for="tab in balanceTabs"
          :key="tab.value"
          type="button"
          role="tab"
          class="balance-tab"
          :class="`balance-tab-${tab.value}`"
          :aria-selected="activeBalanceTab === tab.value"
          :tabindex="activeBalanceTab === tab.value ? 0 : -1"
          @click="activeBalanceTab = tab.value"
        >
          <span>{{ tab.label }}</span>
          <strong>{{ tab.count }}</strong>
        </button>
      </nav>

      <section v-if="loading && wallets.length === 0" class="wallet-grid" aria-live="polite">
        <div v-for="i in 2" :key="i" class="wallet-card wallet-skeleton">
          <div class="h-5 w-2/5 animate-pulse rounded bg-gray-200 dark:bg-dark-600"></div>
          <div class="mt-8 h-10 w-1/2 animate-pulse rounded bg-gray-200 dark:bg-dark-600"></div>
          <div class="mt-8 h-2 animate-pulse rounded bg-gray-200 dark:bg-dark-600"></div>
        </div>
      </section>

      <section v-else-if="filteredWallets.length" class="wallet-grid">
        <article v-for="wallet in filteredWallets" :key="wallet.id" class="wallet-card" :class="`wallet-card-${walletBalanceStatus(wallet)}`">
          <header class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="wallet-name truncate">{{ wallet.name }}</h2>
                <span class="badge badge-gray">{{ tierLabel(wallet.tier) }}</span>
                <span class="badge" :class="balanceStatusBadgeClass(wallet)">{{ balanceStatusLabel(wallet) }}</span>
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
              <p v-if="wallet.balance_error" class="balance-error mt-2">{{ t('admin.upstreamFunds.wallet.syncFailed') }}</p>
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
				<a v-if="wallet.recharge_mode === 'product' && wallet.card_site_url" class="btn btn-ghost btn-sm" :href="wallet.card_site_url" target="_blank" rel="noopener noreferrer" :title="t('admin.upstreamFunds.openCardSite')"><Icon name="externalLink" size="sm" /><span class="ml-1 hidden sm:inline">{{ t('admin.upstreamFunds.openCardSite') }}</span></a>
				<button v-if="wallet.recharge_mode === 'product'" type="button" class="btn btn-ghost btn-sm" :title="t('admin.upstreamFunds.redeemCode')" @click="openRedeemDialog(wallet)"><Icon name="key" size="sm" /><span class="ml-1 hidden sm:inline">{{ t('admin.upstreamFunds.redeemCode') }}</span></button>
              <button type="button" class="btn btn-ghost btn-sm btn-icon" :disabled="isWalletRefreshing(wallet.id) || !wallet.adapter_configured" :title="t('admin.upstreamFunds.refreshBalance')" @click="refreshOneWallet(wallet)"><Icon name="refresh" size="sm" :class="isWalletRefreshing(wallet.id) ? 'animate-spin' : ''" /></button>
              <button type="button" class="btn btn-ghost btn-sm" :title="t('admin.upstreamFunds.recordBalance')" @click="openBalanceDialog(wallet)"><Icon name="dollar" size="sm" /><span class="ml-1 hidden sm:inline">{{ t('admin.upstreamFunds.recordBalance') }}</span></button>
              <button type="button" class="btn btn-ghost btn-sm" :title="t('common.edit')" @click="openEditDialog(wallet)"><Icon name="edit" size="sm" /><span class="ml-1 hidden sm:inline">{{ t('common.edit') }}</span></button>
				<button v-if="wallet.recharge_mode === 'direct'" type="button" class="btn btn-secondary btn-sm" :title="t('admin.upstreamFunds.recharge')" @click="openRechargeDialog(wallet)"><Icon name="creditCard" size="sm" /><span class="ml-1 hidden sm:inline">{{ t('admin.upstreamFunds.recharge') }}</span></button>
            </div>
          </footer>
        </article>
      </section>

		<div v-else-if="wallets.length" class="wallet-empty">
			<div class="empty-mark"><Icon name="filter" size="xl" /></div>
			<h2>{{ t('admin.upstreamFunds.empty.filterTitle') }}</h2>
			<p>{{ t('admin.upstreamFunds.empty.filterDescription') }}</p>
		</div>

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
			<div v-if="walletForm.recharge_mode === 'product'" class="md:col-span-2"><label class="input-label">{{ t('admin.upstreamFunds.form.cardSiteURL') }}</label><input v-model.trim="walletForm.card_site_url" class="input" type="url" maxlength="2048" :placeholder="t('admin.upstreamFunds.form.cardSiteURLPlaceholder')" /></div>
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

		<BaseDialog :show="showRedeemDialog" :title="t('admin.upstreamFunds.redeemCode')" @close="closeRedeemDialog">
			<form id="upstream-redeem-form" class="space-y-4" @submit.prevent="submitRedeemCode">
				<div><p class="text-sm font-bold text-[var(--promo-text)]">{{ redeemWallet?.name }}</p><p class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ redeemWallet?.provider }}</p></div>
				<div><label class="input-label">{{ t('admin.upstreamFunds.form.redeemCode') }}</label><input v-model.trim="redeemCodeValue" class="input font-mono" type="text" maxlength="512" autocomplete="off" autocapitalize="off" spellcheck="false" required autofocus /></div>
				<p v-if="redeemWallet && !redeemWallet.redeem_configured" class="redeem-warning">{{ t('admin.upstreamFunds.form.redeemUnavailable') }}</p>
			</form>
			<template #footer><div class="flex flex-wrap justify-end gap-3"><a v-if="redeemWallet?.card_site_url" class="btn btn-secondary" :href="redeemWallet.card_site_url" target="_blank" rel="noopener noreferrer"><Icon name="externalLink" size="sm" class="mr-2" />{{ t('admin.upstreamFunds.openCardSite') }}</a><button type="button" class="btn btn-secondary" @click="closeRedeemDialog">{{ t('common.cancel') }}</button><button type="submit" form="upstream-redeem-form" class="btn btn-primary" :disabled="redeeming || !redeemWallet?.redeem_configured || !redeemCodeValue">{{ redeeming ? t('admin.upstreamFunds.redeeming') : t('admin.upstreamFunds.redeemCode') }}</button></div></template>
		</BaseDialog>

		<BaseDialog :show="showRechargeDialog" :title="t('admin.upstreamFunds.recharge')" @close="closeRechargeDialog">
			<div v-if="rechargeOrder" class="space-y-4">
				<div class="flex items-center justify-between gap-3"><div><p class="text-sm font-bold">{{ rechargeWallet?.name }}</p><p class="mt-1 font-mono text-xs text-gray-500">{{ rechargeOrder.order_no }}</p></div><span class="badge" :class="rechargeStatusClass(rechargeOrder.status)">{{ rechargeStatusLabel(rechargeOrder.status) }}</span></div>
					<div v-if="rechargeOrder.payment_qr || rechargeOrder.payment_url" class="recharge-qr"><canvas ref="rechargeQRCanvas"></canvas></div>
				<a v-if="rechargeOrder.payment_url" class="btn btn-secondary w-full" :href="rechargeOrder.payment_url" target="_blank" rel="noopener noreferrer"><Icon name="externalLink" size="sm" class="mr-2" />{{ t('admin.upstreamFunds.rechargeForm.openPayment') }}</a>
				<div class="grid grid-cols-2 gap-3 text-sm"><div><p class="metric-kicker">{{ t('admin.upstreamFunds.rechargeForm.faceValue') }}</p><strong class="data-number">{{ formatCurrency(rechargeOrder.face_value, rechargeWallet?.currency || 'USD') }}</strong></div><div><p class="metric-kicker">{{ t('admin.upstreamFunds.rechargeForm.payAmount') }}</p><strong class="data-number">{{ formatCurrency(rechargeOrder.pay_amount, rechargeOrder.currency) }}</strong></div></div>
				<div v-if="rechargeOrder.status === 'manual_review'" class="space-y-3"><p class="redeem-warning">{{ t('admin.upstreamFunds.messages.rechargeManualReview') }}</p><div><label class="input-label">{{ t('admin.upstreamFunds.rechargeForm.balanceAfter') }}</label><input v-model.number="manualBalanceAfter" class="input data-number" type="number" min="0" step="0.000001" /></div><div><label class="input-label">{{ t('admin.upstreamFunds.rechargeForm.manualReason') }}</label><input v-model.trim="manualCompleteReason" class="input" maxlength="500" /></div></div>
			</div>
			<form v-else id="upstream-recharge-form" class="space-y-4" @submit.prevent="createDirectRechargeOrder">
				<div><p class="text-sm font-bold">{{ rechargeWallet?.name }}</p><p class="mt-1 text-xs text-gray-500">{{ rechargeWallet?.provider }}</p></div>
				<p v-if="rechargeWallet && !rechargeWallet.recharge_configured" class="redeem-warning">{{ t('admin.upstreamFunds.rechargeForm.unavailable') }}</p>
				<template v-else>
					<div><label class="input-label">{{ t('admin.upstreamFunds.rechargeForm.amount') }}</label><input v-model.number="rechargeAmount" class="input data-number" type="number" min="0.01" step="0.01" required /></div>
					<div><label class="input-label">{{ t('admin.upstreamFunds.rechargeForm.channel') }}</label><Select v-model="selectedPaymentChannelID" :options="paymentChannelOptions" :disabled="loadingPaymentChannels" /></div>
				</template>
			</form>
			<template #footer><div class="flex justify-end gap-3"><button type="button" class="btn btn-secondary" @click="closeRechargeDialog">{{ rechargeOrder ? t('common.close') : t('common.cancel') }}</button><button v-if="rechargeOrder?.status === 'manual_review'" type="button" class="btn btn-primary" :disabled="completingRechargeOrder || manualBalanceAfter < 0 || !manualCompleteReason" @click="manualCompleteRecharge">{{ t('admin.upstreamFunds.rechargeForm.manualComplete') }}</button><button v-if="!rechargeOrder" type="submit" form="upstream-recharge-form" class="btn btn-primary" :disabled="creatingRechargeOrder || !rechargeWallet?.recharge_configured || !selectedPaymentChannelID || rechargeAmount <= 0">{{ creatingRechargeOrder ? t('common.processing') : t('admin.upstreamFunds.rechargeForm.createOrder') }}</button></div></template>
		</BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type UpstreamFundsAccount, type UpstreamFundsSummary, type UpstreamPaymentChannel, type UpstreamRechargeOrder, type UpstreamWallet, type UpstreamWalletInput } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatCurrency, formatRelativeTime } from '@/utils/format'
import {
	isUpstreamRechargePollingTerminal,
	nextUpstreamRechargePollDelay,
	selectUpstreamBalanceSyncTargets,
	updateUpstreamSyncCatalog,
	UPSTREAM_BALANCE_SYNC_INTERVAL_MS,
	UPSTREAM_RECHARGE_POLL_BASE_MS
} from './upstreamFundsRuntime'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import QRCode from 'qrcode'

const { t } = useI18n()
const appStore = useAppStore()
type BalanceTab = 'all' | 'healthy' | 'normal' | 'alert'
type BalanceStatus = Exclude<BalanceTab, 'all'> | 'unknown'
const wallets = ref<UpstreamWallet[]>([])
const syncWallets = ref<UpstreamWallet[]>([])
const accountOptions = ref<UpstreamFundsAccount[]>([])
const searchQuery = ref('')
const accountSearch = ref('')
const loading = ref(false)
const saving = ref(false)
const savingBalance = ref(false)
const redeeming = ref(false)
const syncCycleActive = ref(false)
const refreshingWalletIDs = ref<Set<number>>(new Set())
const showWalletDialog = ref(false)
const showBalanceDialog = ref(false)
const showRedeemDialog = ref(false)
const showRechargeDialog = ref(false)
const editingWallet = ref<UpstreamWallet | null>(null)
const balanceWallet = ref<UpstreamWallet | null>(null)
const redeemWallet = ref<UpstreamWallet | null>(null)
const rechargeWallet = ref<UpstreamWallet | null>(null)
const balanceValue = ref<number | null>(null)
const redeemCodeValue = ref('')
const rechargeAmount = ref(100)
const paymentChannels = ref<UpstreamPaymentChannel[]>([])
const selectedPaymentChannelID = ref('')
const loadingPaymentChannels = ref(false)
const creatingRechargeOrder = ref(false)
const completingRechargeOrder = ref(false)
const rechargeOrder = ref<UpstreamRechargeOrder | null>(null)
const manualBalanceAfter = ref(0)
const manualCompleteReason = ref('')
const rechargeQRCanvas = ref<HTMLCanvasElement | null>(null)
const activeBalanceTab = ref<BalanceTab>('all')
const summary = reactive<UpstreamFundsSummary>({ wallet_count: 0, enabled_count: 0, attention_count: 0, cost_today: 0, cost_24h: 0, balance_by_currency: {} })
const walletForm = reactive<UpstreamWalletInput>({ name: '', provider: '', currency: 'USD', recharge_mode: 'manual', card_site_url: '', tier: 'primary', enabled: true, alert_days: 2, target_days: 7, account_ids: [] })

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
const balanceTabs = computed(() => {
	const counts: Record<BalanceTab, number> = { all: wallets.value.length, healthy: 0, normal: 0, alert: 0 }
	for (const wallet of wallets.value) {
		const status = walletBalanceStatus(wallet)
		if (status !== 'unknown') counts[status]++
	}
	return (['all', 'healthy', 'normal', 'alert'] as const).map(value => ({
		value,
		label: t(`admin.upstreamFunds.tabs.${value}`),
		count: counts[value]
	}))
})
const filteredWallets = computed(() => {
	if (activeBalanceTab.value === 'all') return wallets.value
	return wallets.value.filter(wallet => walletBalanceStatus(wallet) === activeBalanceTab.value)
})
const paymentChannelOptions = computed(() => paymentChannels.value.map(channel => ({
	value: channel.id,
	label: `${channel.name} · ${channel.currency}`
})))

let searchTimer: number | null = null
let syncTimer: number | null = null
let rechargePollTimer: number | null = null
let rechargePollFailures = 0
let rechargePollGeneration = 0
let componentActive = false
const inflightWalletRefreshes = new Map<number, Promise<UpstreamWallet>>()

async function loadOverview(options: { silent?: boolean } = {}) {
	const silent = options.silent === true
	const search = searchQuery.value.trim()
	if (!silent) loading.value = true
  try {
    const overview = await adminAPI.upstreamFunds.list(search || undefined)
    wallets.value = overview.wallets || []
		syncWallets.value = updateUpstreamSyncCatalog(syncWallets.value, overview.wallets || [], search)
    Object.assign(summary, overview.summary)
  } catch (error: any) {
		if (!silent) appStore.showError(error?.message || t('admin.upstreamFunds.messages.loadFailed'))
  } finally {
		if (!silent) loading.value = false
  }
}

async function loadSyncWalletCatalog(): Promise<UpstreamWallet[]> {
	try {
		const overview = await adminAPI.upstreamFunds.listAll()
		const catalog = overview.wallets || []
		syncWallets.value = catalog
		return catalog
	} catch {
		// Keep the last complete catalog so one transient list failure does not stop balance sync.
		return syncWallets.value
	}
}

function setWalletRefreshing(id: number, refreshing: boolean) {
	const next = new Set(refreshingWalletIDs.value)
	if (refreshing) next.add(id)
	else next.delete(id)
	refreshingWalletIDs.value = next
}

function isWalletRefreshing(id: number) {
	return refreshingWalletIDs.value.has(id)
}

function replaceWalletInCollections(updated: UpstreamWallet) {
	for (const collection of [wallets, syncWallets]) {
		const index = collection.value.findIndex(item => item.id === updated.id)
		if (index >= 0) collection.value[index] = updated
	}
}

function refreshWalletRequest(wallet: UpstreamWallet): Promise<UpstreamWallet> {
	const existing = inflightWalletRefreshes.get(wallet.id)
	if (existing) return existing
	setWalletRefreshing(wallet.id, true)
	const request = adminAPI.upstreamFunds.refreshBalance(wallet.id).finally(() => {
		inflightWalletRefreshes.delete(wallet.id)
		setWalletRefreshing(wallet.id, false)
	})
	inflightWalletRefreshes.set(wallet.id, request)
	return request
}

async function refreshOneWallet(wallet: UpstreamWallet) {
	if (!wallet.adapter_configured) {
		appStore.showWarning(t('admin.upstreamFunds.messages.syncUnavailable'))
		return
	}
	try {
			const updated = await refreshWalletRequest(wallet)
		replaceWalletInCollections(updated)
		appStore.showSuccess(t('admin.upstreamFunds.messages.balanceSynced'))
	} catch (error: any) {
		appStore.showError(error?.message || t('admin.upstreamFunds.messages.syncFailed'))
	} finally {
		await loadOverview({ silent: true })
	}
}

async function syncAllBalances(showFeedback = false) {
	if (syncCycleActive.value) return
	syncCycleActive.value = true
	try {
		const catalog = await loadSyncWalletCatalog()
		const targets = selectUpstreamBalanceSyncTargets(catalog)
		if (targets.length === 0) {
			await loadOverview({ silent: !showFeedback })
			if (showFeedback) appStore.showWarning(t('admin.upstreamFunds.messages.syncUnavailable'))
			return
		}
			const results = await Promise.allSettled(targets.map(wallet => refreshWalletRequest(wallet)))
		for (const result of results) {
			if (result.status === 'fulfilled') replaceWalletInCollections(result.value)
		}
		await loadOverview({ silent: true })
		if (showFeedback) {
			const failed = results.filter(result => result.status === 'rejected').length
			if (failed > 0) appStore.showWarning(t('admin.upstreamFunds.messages.syncPartial', { failed, total: targets.length }))
			else appStore.showSuccess(t('admin.upstreamFunds.messages.allBalancesSynced'))
		}
	} finally {
		syncCycleActive.value = false
	}
}

function scheduleBalanceSync() {
	if (!componentActive) return
	if (syncTimer !== null) window.clearTimeout(syncTimer)
	syncTimer = window.setTimeout(async () => {
		await syncAllBalances(false)
		if (componentActive) scheduleBalanceSync()
	}, UPSTREAM_BALANCE_SYNC_INTERVAL_MS)
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
  Object.assign(walletForm, { name: '', provider: '', currency: 'USD', recharge_mode: 'manual', card_site_url: '', tier: 'primary', enabled: true, alert_days: 2, target_days: 7, account_ids: [] })
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
  Object.assign(walletForm, { name: wallet.name, provider: wallet.provider, currency: wallet.currency, recharge_mode: wallet.recharge_mode, card_site_url: wallet.card_site_url || '', tier: wallet.tier, enabled: wallet.enabled, alert_days: wallet.alert_days, target_days: wallet.target_days, account_ids: [...wallet.account_ids] })
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

function openRedeemDialog(wallet: UpstreamWallet) {
	redeemWallet.value = wallet
	redeemCodeValue.value = ''
	showRedeemDialog.value = true
}

function closeRedeemDialog() {
	showRedeemDialog.value = false
	redeemWallet.value = null
	redeemCodeValue.value = ''
}

async function submitRedeemCode() {
	if (!redeemWallet.value || !redeemWallet.value.redeem_configured || !redeemCodeValue.value) return
	redeeming.value = true
	try {
		const result = await adminAPI.upstreamFunds.redeemCode(redeemWallet.value.id, redeemCodeValue.value)
		closeRedeemDialog()
		if (result.status === 'verified') appStore.showSuccess(t('admin.upstreamFunds.messages.redeemVerified'))
		else appStore.showWarning(t('admin.upstreamFunds.messages.redeemManualReview'))
		await loadOverview({ silent: true })
	} catch (error: any) {
		appStore.showError(error?.message || t('admin.upstreamFunds.messages.redeemFailed'))
	} finally {
		redeeming.value = false
	}
}

async function openRechargeDialog(wallet: UpstreamWallet) {
	stopRechargePolling()
	rechargeWallet.value = wallet
	rechargeOrder.value = null
	manualBalanceAfter.value = wallet.balance ?? 0
	manualCompleteReason.value = ''
	paymentChannels.value = []
	selectedPaymentChannelID.value = ''
	rechargeAmount.value = wallet.recommended_top_up && wallet.recommended_top_up > 0
		? Math.ceil(wallet.recommended_top_up * 100) / 100
		: 100
	showRechargeDialog.value = true
	if (!wallet.recharge_configured) return
	loadingPaymentChannels.value = true
	try {
		paymentChannels.value = await adminAPI.upstreamFunds.listPaymentChannels(wallet.id)
		const first = paymentChannels.value[0]
		if (first) {
			selectedPaymentChannelID.value = first.id
			if (first.single_min > 0 && rechargeAmount.value < first.single_min) rechargeAmount.value = first.single_min
		}
	} catch (error: any) {
		appStore.showError(error?.message || t('admin.upstreamFunds.messages.channelsFailed'))
	} finally {
		loadingPaymentChannels.value = false
	}
}

function closeRechargeDialog() {
	stopRechargePolling()
	showRechargeDialog.value = false
	rechargeWallet.value = null
	rechargeOrder.value = null
	paymentChannels.value = []
	selectedPaymentChannelID.value = ''
}

function stopRechargePolling() {
	if (rechargePollTimer !== null) window.clearTimeout(rechargePollTimer)
	rechargePollTimer = null
	rechargePollFailures = 0
	rechargePollGeneration++
}

function scheduleRechargePoll(delay = UPSTREAM_RECHARGE_POLL_BASE_MS) {
	if (!showRechargeDialog.value || !rechargeOrder.value || isUpstreamRechargePollingTerminal(rechargeOrder.value.status)) return
	if (rechargePollTimer !== null) window.clearTimeout(rechargePollTimer)
	const generation = rechargePollGeneration
	rechargePollTimer = window.setTimeout(() => {
		if (generation !== rechargePollGeneration) return
		rechargePollTimer = null
		void pollRechargeOrder()
	}, delay)
}

async function createDirectRechargeOrder() {
	if (!rechargeWallet.value || !selectedPaymentChannelID.value || rechargeAmount.value <= 0) return
	creatingRechargeOrder.value = true
	try {
		rechargeOrder.value = await adminAPI.upstreamFunds.createRechargeOrder(rechargeWallet.value.id, {
			amount: rechargeAmount.value,
			payment_channel_id: selectedPaymentChannelID.value,
			idempotency_key: createIdempotencyKey()
		})
		await renderRechargeQR()
		handleRechargeOrderState()
	} catch (error: any) {
		appStore.showError(error?.message || t('admin.upstreamFunds.messages.rechargeCreateFailed'))
	} finally {
		creatingRechargeOrder.value = false
	}
}

function createIdempotencyKey() {
	if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
	return `upstream-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

async function renderRechargeQR() {
	await nextTick()
	const paymentPayload = rechargeOrder.value?.payment_qr || rechargeOrder.value?.payment_url
	if (!rechargeQRCanvas.value || !paymentPayload) return
	await QRCode.toCanvas(rechargeQRCanvas.value, paymentPayload, { width: 224, margin: 1, errorCorrectionLevel: 'M' })
}

function handleRechargeOrderState() {
	if (!rechargeOrder.value) return
	const status = rechargeOrder.value.status
	if (isUpstreamRechargePollingTerminal(status)) stopRechargePolling()
	if (status === 'completed') {
		appStore.showSuccess(t('admin.upstreamFunds.messages.rechargeCompleted'))
		void loadOverview({ silent: true })
		return
	}
	if (status === 'manual_review') {
		appStore.showWarning(t('admin.upstreamFunds.messages.rechargeManualReview'))
		return
	}
	if (status === 'failed' || status === 'expired' || status === 'cancelled') return
	scheduleRechargePoll()
}

async function pollRechargeOrder() {
	if (!showRechargeDialog.value || !rechargeOrder.value) return
	const orderID = rechargeOrder.value.id
	try {
		const updated = await adminAPI.upstreamFunds.pollRechargeOrder(orderID)
		if (!showRechargeDialog.value || rechargeOrder.value?.id !== orderID) return
		rechargeOrder.value = updated
		rechargePollFailures = 0
		await renderRechargeQR()
		handleRechargeOrderState()
	} catch (error: any) {
		if (!showRechargeDialog.value || rechargeOrder.value?.id !== orderID) return
		rechargePollFailures++
		appStore.showError(error?.message || t('admin.upstreamFunds.messages.rechargePollFailed'))
		scheduleRechargePoll(nextUpstreamRechargePollDelay(rechargePollFailures))
	}
}

async function manualCompleteRecharge() {
	if (!rechargeOrder.value || rechargeOrder.value.status !== 'manual_review' || !Number.isFinite(manualBalanceAfter.value) || manualBalanceAfter.value < 0 || !manualCompleteReason.value) return
	completingRechargeOrder.value = true
	try {
		rechargeOrder.value = await adminAPI.upstreamFunds.manualCompleteRechargeOrder(rechargeOrder.value.id, {
			balance_after: manualBalanceAfter.value,
			reason: manualCompleteReason.value
		})
		appStore.showSuccess(t('admin.upstreamFunds.messages.rechargeCompleted'))
		stopRechargePolling()
		await loadOverview({ silent: true })
	} catch (error: any) {
		appStore.showError(error?.message || t('admin.upstreamFunds.messages.manualCompleteFailed'))
	} finally {
		completingRechargeOrder.value = false
	}
}

function rechargeStatusLabel(status: UpstreamRechargeOrder['status']) { return t(`admin.upstreamFunds.rechargeStatus.${status}`) }
function rechargeStatusClass(status: UpstreamRechargeOrder['status']) {
	if (status === 'completed') return 'badge-success'
	if (status === 'failed' || status === 'expired' || status === 'cancelled') return 'badge-danger'
	if (status === 'manual_review') return 'badge-warning'
	return 'badge-primary'
}

function tierLabel(value: UpstreamWallet['tier']) { return t(`admin.upstreamFunds.tier.${value}`) }
function modeLabel(value: UpstreamWallet['recharge_mode']) { return t(`admin.upstreamFunds.mode.${value}`) }
function formatWalletBalance(wallet: UpstreamWallet) { return wallet.balance === null ? '—' : formatCurrency(wallet.balance, wallet.currency) }
function walletBalanceStatus(wallet: UpstreamWallet): BalanceStatus {
	if (wallet.balance === null) return 'unknown'
	if (wallet.balance > 100) return 'healthy'
	if (wallet.balance >= 50) return 'normal'
	return 'alert'
}
function balanceStatusLabel(wallet: UpstreamWallet) { return t(`admin.upstreamFunds.tabs.${walletBalanceStatus(wallet)}`) }
function balanceStatusBadgeClass(wallet: UpstreamWallet) {
	const status = walletBalanceStatus(wallet)
	return status === 'healthy' ? 'badge-success' : status === 'normal' ? 'badge-warning' : status === 'alert' ? 'badge-danger' : 'badge-gray'
}
function runwayClass(wallet: UpstreamWallet) { return wallet.runway_days !== null && wallet.runway_days < wallet.alert_days ? 'runway-low' : wallet.runway_days !== null ? 'runway-healthy' : 'runway-unknown' }
function runwayPercent(wallet: UpstreamWallet) { return wallet.runway_days === null || wallet.target_days <= 0 ? 0 : Math.min(100, Math.max(0, wallet.runway_days / wallet.target_days * 100)) }
function markerPercent(days: number, target: number) { return target <= 0 ? 0 : Math.min(100, Math.max(0, days / target * 100)) }
function runwayTitle(wallet: UpstreamWallet) { return wallet.runway_days === null ? runwayUnavailableReason(wallet) : `${t('admin.upstreamFunds.wallet.alertLine', { days: wallet.alert_days })} / ${t('admin.upstreamFunds.wallet.targetLine', { days: wallet.target_days })}` }
function runwayUnavailableReason(wallet: UpstreamWallet) { return wallet.currency !== wallet.cost_currency ? t('admin.upstreamFunds.wallet.runwayCurrencyMismatch') : t('admin.upstreamFunds.wallet.runwayNoCost') }

onMounted(() => {
	componentActive = true
	void (async () => {
		await Promise.all([loadOverview(), loadAccounts()])
		if (!componentActive) return
		await syncAllBalances(false)
		scheduleBalanceSync()
	})()
})

onBeforeUnmount(() => {
	componentActive = false
	if (searchTimer !== null) window.clearTimeout(searchTimer)
	if (syncTimer !== null) window.clearTimeout(syncTimer)
	stopRechargePolling()
})
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
.summary-label, .metric-kicker { margin: 0; color: var(--promo-text-muted); font-size: 0.72rem; font-weight: 800; line-height: 1.4; letter-spacing: 0; text-transform: uppercase; }
.summary-value { margin: 0.18rem 0 0; font-family: var(--promo-font-display); font-size: 1.45rem; line-height: 1.1; }
.summary-note { margin: 0.3rem 0 0; color: var(--promo-text-muted); font-size: 0.75rem; line-height: 1.4; }
.data-number { font-family: var(--promo-font-body); font-variant-numeric: tabular-nums; font-weight: 800; letter-spacing: 0; }
.balance-tabs { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); overflow: hidden; border: var(--promo-border-width) solid var(--promo-border); border-radius: var(--promo-radius-sm); background: var(--promo-surface-muted); }
@media (min-width: 640px) { .balance-tabs { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
.balance-tab { display: flex; min-height: 3rem; align-items: center; justify-content: space-between; gap: 0.75rem; border-right: 1px solid var(--promo-border-soft); border-bottom: 1px solid var(--promo-border-soft); padding: 0.65rem 0.85rem; color: var(--promo-text-muted); font-size: 0.78rem; font-weight: 900; }
.balance-tab:nth-child(2n) { border-right: 0; }
.balance-tab:nth-child(n+3) { border-bottom: 0; }
@media (min-width: 640px) { .balance-tab { border-bottom: 0; } .balance-tab:nth-child(2n) { border-right: 1px solid var(--promo-border-soft); } .balance-tab:last-child { border-right: 0; } }
.balance-tab strong { min-width: 1.7rem; font-family: var(--promo-font-body); font-size: 0.9rem; font-variant-numeric: tabular-nums; font-weight: 800; text-align: right; }
.balance-tab[aria-selected="true"] { background: var(--promo-black); color: var(--promo-white); }
.balance-tab-healthy[aria-selected="true"] { background: var(--promo-green); }
.balance-tab-normal[aria-selected="true"] { background: var(--promo-yellow); color: var(--promo-black); }
.balance-tab-alert[aria-selected="true"] { background: var(--promo-red); }
.wallet-grid { display: grid; grid-template-columns: repeat(1, minmax(0, 1fr)); gap: 1.25rem; }
@media (min-width: 1280px) { .wallet-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
.wallet-card { min-width: 0; border: var(--promo-border-width) solid var(--promo-border); border-radius: var(--promo-radius-sm); background: var(--promo-card-surface); padding: 1.25rem; box-shadow: var(--promo-shadow-md); }
.wallet-card-healthy { border-color: var(--promo-green); }
.wallet-card-normal { border-color: var(--promo-yellow); }
.wallet-card-alert { border-color: var(--promo-red); }
.wallet-skeleton { min-height: 330px; }
.wallet-name { margin: 0; font-family: var(--promo-font-display); font-size: 1.05rem; line-height: 1.25; }
.balance-value { margin-top: 0.2rem; font-size: 1.8rem; font-weight: 900; line-height: 1; }
.balance-error { color: var(--promo-red-strong); font-size: 0.72rem; font-weight: 800; }
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
.cost-grid span { display: block; color: var(--promo-text-muted); font-size: 0.72rem; line-height: 1.4; }
.cost-grid strong { display: block; margin-top: 0.25rem; overflow: hidden; font-family: var(--promo-font-body); font-size: 0.88rem; font-variant-numeric: tabular-nums; font-weight: 800; text-overflow: ellipsis; white-space: nowrap; }
.tag-list { display: flex; flex-wrap: wrap; gap: 0.35rem; }
.tag { max-width: 100%; overflow: hidden; border: 1px solid var(--promo-border-soft); border-radius: 999px; background: var(--promo-surface-raised); padding: 0.18rem 0.48rem; color: var(--promo-text); font-size: 0.68rem; text-overflow: ellipsis; white-space: nowrap; }
.tag-cyan { border-color: var(--promo-cyan-strong); background: color-mix(in srgb, var(--promo-cyan) 24%, transparent); }
.tag-muted { color: var(--promo-text-muted); }
.wallet-empty { display: flex; min-height: 310px; flex-direction: column; align-items: center; justify-content: center; border: 2px dashed var(--promo-border); border-radius: var(--promo-radius-sm); background: var(--promo-surface-muted); padding: 2rem; text-align: center; }
.empty-mark { display: flex; height: 3.7rem; width: 3.7rem; align-items: center; justify-content: center; border: 2px solid var(--promo-black); border-radius: 50%; background: var(--promo-yellow); color: var(--promo-black); }
.wallet-empty h2 { margin: 1rem 0 0; font-family: var(--promo-font-display); font-size: 1.25rem; }
.wallet-empty p { max-width: 28rem; margin: 0.45rem 0 0; color: var(--promo-text-muted); font-size: 0.85rem; }
.redeem-warning { border-left: 3px solid var(--promo-yellow); background: var(--promo-surface-muted); padding: 0.65rem 0.75rem; color: var(--promo-text-muted); font-size: 0.78rem; line-height: 1.45; }
.recharge-qr { display: flex; min-height: 248px; align-items: center; justify-content: center; border: 2px solid var(--promo-border); background: #fff; padding: 0.75rem; }
.recharge-qr canvas { display: block; height: 224px; width: 224px; max-width: 100%; }
.account-picker { max-height: 14rem; overflow-y: auto; border: 2px solid var(--promo-border-soft); border-radius: var(--promo-radius-sm); background: var(--promo-surface); }
.account-option { display: flex; align-items: center; gap: 0.65rem; border-bottom: 1px solid var(--promo-border-soft); padding: 0.65rem 0.75rem; cursor: pointer; }
.account-option:last-child { border-bottom: 0; }
.account-option:hover { background: var(--promo-surface-hover); }
.account-option-disabled { cursor: not-allowed; opacity: 0.55; }
.account-option input { height: 1rem; width: 1rem; flex-shrink: 0; }
</style>
