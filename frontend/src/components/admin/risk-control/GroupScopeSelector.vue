<template>
  <section class="space-y-4 rounded-xl border p-4" :class="sectionClass" :data-test="dataTest">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
      <div>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ title }}</h3>
        <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ hint }}</p>
      </div>
      <div class="inline-flex shrink-0 rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
        <button
          type="button"
          :aria-pressed="allGroups"
          class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
          :class="allGroups ? activeButtonClass : inactiveButtonClass"
          @click="emit('update:allGroups', true)"
        >
          {{ t('admin.riskControl.allGroups') }}
        </button>
        <button
          type="button"
          :aria-pressed="!allGroups"
          class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
          :class="!allGroups ? activeButtonClass : inactiveButtonClass"
          @click="emit('update:allGroups', false)"
        >
          {{ t('admin.riskControl.selectedGroups') }}
        </button>
      </div>
    </div>

    <div v-if="!allGroups" class="space-y-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="relative flex-1">
          <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
          <input v-model.trim="search" type="search" class="input pl-9" :placeholder="t('admin.riskControl.searchGroups')" />
        </div>
        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
          {{ t('admin.riskControl.selectedGroupCount', { count: selectedIds.length }) }}
        </span>
      </div>
      <div class="grid max-h-[420px] grid-cols-1 gap-3 overflow-y-auto pr-1 md:grid-cols-2 xl:grid-cols-3">
        <button
          v-for="group in filteredGroups"
          :key="group.id"
          type="button"
          :aria-pressed="isSelected(group.id)"
          class="flex min-h-20 items-center justify-between rounded-lg border p-4 text-left transition-colors"
          :class="isSelected(group.id) ? selectedCardClass : idleCardClass"
          @click="toggle(group.id)"
        >
          <span class="min-w-0">
            <span class="block truncate text-sm font-semibold text-gray-900 dark:text-white">{{ group.name }}</span>
            <span class="mt-1 inline-flex rounded-md bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">{{ group.platform }}</span>
          </span>
          <span
            class="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full border"
            :class="isSelected(group.id) ? selectedCheckClass : 'border-gray-300 text-transparent dark:border-dark-500'"
          >
            <Icon name="check" size="xs" :stroke-width="2" />
          </span>
        </button>
        <p v-if="filteredGroups.length === 0" class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.riskControl.noGroups') }}</p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'
import type { AdminGroup } from '@/types'

const props = withDefaults(defineProps<{
  allGroups: boolean
  selectedIds: number[]
  groups: AdminGroup[]
  title: string
  hint: string
  tone?: 'primary' | 'amber'
  dataTest?: string
}>(), {
  tone: 'primary',
  dataTest: '',
})

const emit = defineEmits<{
  'update:allGroups': [value: boolean]
  'update:selectedIds': [value: number[]]
}>()

const { t } = useI18n()
const search = ref('')

const filteredGroups = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  if (!keyword) return props.groups
  return props.groups.filter((group) => (
    group.name.toLowerCase().includes(keyword)
      || String(group.platform).toLowerCase().includes(keyword)
  ))
})

const sectionClass = computed(() => (
  props.tone === 'amber'
    ? 'border-amber-200 bg-amber-50/50 dark:border-amber-900/60 dark:bg-amber-950/10'
    : 'border-gray-100 dark:border-dark-700'
))
const activeButtonClass = computed(() => (
  props.tone === 'amber'
    ? 'bg-white text-amber-900 shadow-sm dark:bg-dark-800 dark:text-amber-100'
    : 'bg-white text-gray-900 shadow-sm dark:bg-dark-800 dark:text-white'
))
const inactiveButtonClass = 'text-gray-500 dark:text-gray-400'
const selectedCardClass = computed(() => (
  props.tone === 'amber'
    ? 'border-amber-300 bg-white dark:border-amber-700 dark:bg-amber-900/20'
    : 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20'
))
const selectedCheckClass = computed(() => (
  props.tone === 'amber'
    ? 'border-amber-500 bg-amber-500 text-white'
    : 'border-primary-500 bg-primary-500 text-white'
))
const idleCardClass = 'border-gray-100 bg-white hover:bg-gray-50 dark:border-dark-700 dark:bg-transparent dark:hover:bg-dark-700/60'

function isSelected(groupID: number): boolean {
  return props.selectedIds.includes(groupID)
}

function toggle(groupID: number) {
  const next = isSelected(groupID)
    ? props.selectedIds.filter((id) => id !== groupID)
    : [...props.selectedIds, groupID]
  emit('update:selectedIds', next)
}
</script>
