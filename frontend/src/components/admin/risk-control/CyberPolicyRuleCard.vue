<template>
  <div class="space-y-4">
    <div class="grid grid-cols-2 gap-2 rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
      <button
        v-for="option in modeOptions"
        :key="option.value"
        type="button"
        class="rounded-md px-3 py-2 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500"
        :class="policy.mode === option.value
          ? option.value === 'enforce'
            ? 'bg-amber-500 text-white shadow-sm'
            : 'bg-white text-sky-700 shadow-sm dark:bg-dark-700 dark:text-sky-300'
          : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-100'"
        @click="updateField('mode', option.value)"
      >
        {{ option.label }}
      </button>
    </div>

    <div v-if="policy.mode === 'collect_only'" class="rounded-lg border border-sky-100 bg-sky-50/70 px-4 py-3 text-xs leading-5 text-sky-700 dark:border-sky-900/50 dark:bg-sky-950/20 dark:text-sky-300">
      {{ t('admin.riskControl.cyberPolicyCollectDescription') }}
    </div>

    <div v-else class="space-y-4">
      <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div class="flex items-center justify-between gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.riskControl.cyberSessionBlock') }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.riskControl.cyberSessionBlockShortHint') }}</p>
          </div>
          <Toggle :model-value="policy.session_block_enabled" @update:model-value="updateField('session_block_enabled', $event)" />
        </div>
        <div class="flex items-center justify-between gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.riskControl.cyberViolationCount') }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.riskControl.cyberViolationCountHint') }}</p>
          </div>
          <Toggle :model-value="policy.violation_count_enabled" @update:model-value="setViolationCounting($event)" />
        </div>
        <div class="flex items-center justify-between gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.riskControl.cyberEmailNotice') }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.riskControl.cyberEmailNoticeHint') }}</p>
          </div>
          <Toggle :model-value="policy.email_on_hit" @update:model-value="updateField('email_on_hit', $event)" />
        </div>
      </div>

      <div v-if="policy.session_block_enabled" class="rounded-lg border border-amber-100 bg-amber-50/50 p-3 dark:border-amber-900/40 dark:bg-amber-950/10">
        <label class="input-label">{{ t('admin.riskControl.cyberSessionBlockTTL') }}</label>
        <input
          :value="policy.session_block_ttl_seconds"
          type="number"
          min="1"
          max="2592000"
          class="input"
          @input="updateNumber('session_block_ttl_seconds', $event, 3600)"
        />
        <p class="mt-2 text-xs text-amber-700 dark:text-amber-300">{{ t('admin.riskControl.cyberSessionBlockMasterHint') }}</p>
      </div>

      <div v-if="policy.violation_count_enabled" class="grid grid-cols-1 gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700 md:grid-cols-3">
        <div class="flex items-center justify-between gap-3 md:col-span-3">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.riskControl.cyberAutoBanAccount') }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.riskControl.cyberAutoBanAccountHint') }}</p>
          </div>
          <Toggle :model-value="policy.auto_ban_enabled" @update:model-value="updateField('auto_ban_enabled', $event)" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.riskControl.banThreshold') }}</label>
          <input :value="policy.ban_threshold" type="number" min="1" max="1000" class="input" @input="updateNumber('ban_threshold', $event, 10)" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.riskControl.violationWindowHours') }}</label>
          <input :value="policy.violation_window_hours" type="number" min="1" max="8760" class="input" @input="updateNumber('violation_window_hours', $event, 720)" />
        </div>
        <div class="flex items-end">
          <p class="rounded-md bg-gray-50 px-3 py-2 text-xs leading-5 text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            {{ t('admin.riskControl.cyberGroupCounterHint') }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import Toggle from '@/components/common/Toggle.vue'
import type { CyberPolicyMode, CyberPolicySettings } from '@/api/admin/riskControl'

const props = defineProps<{ policy: CyberPolicySettings }>()
const emit = defineEmits<{ 'update:policy': [value: CyberPolicySettings] }>()
const { t } = useI18n()

const modeOptions = computed<Array<{ value: CyberPolicyMode; label: string }>>(() => [
  { value: 'collect_only', label: t('admin.riskControl.cyberPolicyCollectMode') },
  { value: 'enforce', label: t('admin.riskControl.cyberPolicyEnforceMode') },
])

function updateField<K extends keyof CyberPolicySettings>(field: K, value: CyberPolicySettings[K]) {
  emit('update:policy', { ...props.policy, [field]: value })
}

function setViolationCounting(enabled: boolean) {
  emit('update:policy', {
    ...props.policy,
    violation_count_enabled: enabled,
    auto_ban_enabled: enabled ? props.policy.auto_ban_enabled : false,
  })
}

function updateNumber(field: 'session_block_ttl_seconds' | 'ban_threshold' | 'violation_window_hours', event: Event, fallback: number) {
  const value = Number((event.target as HTMLInputElement).value)
  updateField(field, Number.isFinite(value) && value > 0 ? Math.trunc(value) : fallback)
}
</script>
