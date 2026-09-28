<template>
  <div class="mt-4">
    <p class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
      {{ t('payment.usdtNetwork') }}
    </p>
    <div class="grid grid-cols-3 gap-3">
      <button
        v-for="network in USDT_NETWORKS"
        :key="network.id"
        type="button"
        :class="[
          'flex h-[60px] flex-col items-center justify-center rounded-lg border px-2 transition-all',
          modelValue === network.id
            ? 'border-[#26A17B] bg-emerald-50 text-gray-900 shadow-sm dark:bg-emerald-950 dark:text-gray-100'
            : 'border-gray-300 bg-white text-gray-700 hover:border-gray-400 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200',
        ]"
        @click="emit('update:modelValue', network.id)"
      >
        <span class="text-sm font-semibold">{{ t(network.labelKey) }}</span>
        <span class="mt-1 text-[10px] tracking-wide text-gray-500 dark:text-gray-400">{{ network.standard }}</span>
      </button>
    </div>
    <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.usdtExactAmountHint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { USDT_NETWORKS, type UsdtNetworkId } from './providerConfig'

defineProps<{
  modelValue: UsdtNetworkId | ''
}>()

const emit = defineEmits<{
  'update:modelValue': [value: UsdtNetworkId]
}>()

const { t } = useI18n()
</script>
