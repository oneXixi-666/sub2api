<template>
  <BaseDialog
    :show="show"
    :title="t('keys.groupPicker.title')"
    width="extra-wide"
    :close-on-click-outside="true"
    @close="emit('close')"
  >
    <div class="group-picker -mx-1 space-y-4 sm:-mx-2">
      <div class="space-y-3">
        <div class="relative">
          <Icon
            name="search"
            size="sm"
            class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-400"
          />
          <input
            ref="searchInput"
            v-model="searchQuery"
            type="search"
            :placeholder="t('keys.groupPicker.searchPlaceholder')"
            :aria-label="t('keys.groupPicker.searchPlaceholder')"
            class="h-11 w-full rounded-xl border border-gray-200 bg-gray-50 pl-10 pr-4 text-sm text-gray-900 outline-none transition placeholder:text-gray-400 focus:border-primary-400 focus:ring-2 focus:ring-primary-500/20 dark:border-dark-600 dark:bg-dark-900/60 dark:text-white dark:placeholder:text-dark-500 dark:focus:border-primary-500"
            @keydown.esc.stop="emit('close')"
          />
        </div>

        <div class="flex flex-wrap items-center gap-2" role="tablist" :aria-label="t('keys.groupPicker.platformFilter')">
          <button
            type="button"
            role="tab"
            :aria-selected="selectedPlatform === 'all'"
            class="group-picker-chip"
            :class="selectedPlatform === 'all' ? 'group-picker-chip-active' : 'group-picker-chip-muted'"
            @click="selectedPlatform = 'all'"
          >
            {{ t('keys.groupPicker.allPlatforms') }}
          </button>
          <button
            v-for="platform in availablePlatforms"
            :key="platform"
            type="button"
            role="tab"
            :aria-selected="selectedPlatform === platform"
            class="group-picker-chip"
            :class="selectedPlatform === platform ? 'group-picker-chip-active' : 'group-picker-chip-muted'"
            :style="{ '--chip-accent': platformAccentColor(platform) }"
            @click="selectedPlatform = platform"
          >
            <PlatformIcon :platform="platform" size="xs" />
            {{ platformLabel(platform) }}
          </button>
        </div>

        <div class="flex items-center justify-between border-t border-gray-200/80 pt-3 text-xs dark:border-dark-700">
          <span class="text-gray-500 dark:text-dark-400">
            {{ t('keys.groupPicker.resultCount', { count: filteredGroups.length }) }}
          </span>
        </div>
      </div>

      <div v-if="allowEmpty" class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <button
          type="button"
          class="group-picker-card text-left"
          :class="modelValue === null ? 'group-picker-card-selected' : ''"
          :aria-pressed="modelValue === null"
          @click="emit('select', null)"
        >
          <div class="flex items-start justify-between gap-3">
            <span class="inline-flex min-w-0 items-center gap-2 text-sm font-semibold text-gray-800 dark:text-gray-100">
              <span class="grid h-7 w-7 shrink-0 place-items-center rounded-lg bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300">
                <Icon name="ban" size="sm" />
              </span>
              <span class="truncate">{{ t('keys.groupPicker.noGroup') }}</span>
            </span>
            <Icon v-if="modelValue === null" name="checkCircle" size="sm" class="shrink-0 text-primary-500" :stroke-width="2" />
          </div>
          <p class="mt-3 text-xs leading-relaxed text-gray-500 dark:text-dark-400">
            {{ t('keys.groupPicker.noGroupDescription') }}
          </p>
        </button>
      </div>

      <div v-if="groupSections.length" class="space-y-5">
        <section v-for="section in groupSections" :key="section.platform" class="space-y-2.5">
          <div class="flex items-center gap-2 px-1">
            <PlatformIcon :platform="section.platform" size="sm" :class="platformIconClass(section.platform)" />
            <h4 class="text-sm font-semibold text-gray-800 dark:text-gray-100">{{ platformLabel(section.platform) }}</h4>
            <span class="text-xs text-gray-400 dark:text-dark-500">{{ section.groups.length }}</span>
          </div>

          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <button
              v-for="group in section.groups"
              :key="group.id"
              type="button"
              class="group-picker-card text-left"
              :class="[
                modelValue === group.id ? 'group-picker-card-selected' : ''
              ]"
              :aria-pressed="modelValue === group.id"
              @click="emit('select', group.id)"
            >
              <div class="flex items-start justify-between gap-2">
                <span class="inline-flex min-w-0 items-center gap-2 text-sm font-semibold text-gray-800 dark:text-gray-100">
                  <PlatformIcon :platform="group.platform" size="sm" :class="platformIconClass(group.platform)" />
                  <span class="truncate">{{ group.name }}</span>
                </span>
                <span :class="['group-picker-rate', platformRateClass(group.platform)]">
                  {{ formatRate(effectiveRate(group)) }}x
                </span>
              </div>

              <div class="mt-2 flex flex-wrap items-center gap-1.5">
                <span
                  v-if="group.subscription_type === 'subscription'"
                  class="group-picker-meta bg-violet-100 text-violet-700 dark:bg-violet-500/15 dark:text-violet-300"
                >
                  {{ t('groups.subscription') }}
                </span>
                <span
                  v-if="group.is_exclusive"
                  class="group-picker-meta bg-purple-100 text-purple-700 dark:bg-purple-500/15 dark:text-purple-300"
                >
                  {{ t('keys.groupPicker.exclusive') }}
                </span>
                <span v-if="hasCustomRate(group)" class="group-picker-meta bg-primary-100 text-primary-700 dark:bg-primary-500/15 dark:text-primary-300">
                  {{ t('keys.groupPicker.personalRate') }}
                </span>
                <Icon v-if="modelValue === group.id" name="checkCircle" size="xs" class="ml-auto text-primary-500 dark:text-primary-400" :stroke-width="2" />
              </div>

              <p v-if="group.description" class="mt-2 line-clamp-2 text-xs leading-relaxed text-gray-500 dark:text-dark-400">
                {{ group.description }}
              </p>
            </button>
          </div>
        </section>
      </div>

      <div v-else class="rounded-xl border border-dashed border-gray-300 px-5 py-12 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400">
        <Icon name="search" size="lg" class="mx-auto mb-3 text-gray-300 dark:text-dark-600" />
        {{ t('keys.groupPicker.noResults') }}
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { Group, GroupPlatform } from '@/types'
import {
  platformAccentColor,
  platformIconClass,
  platformLabel,
  platformTextClass
} from '@/utils/platformColors'

const props = withDefaults(defineProps<{
  show: boolean
  groups: Group[]
  userGroupRates?: Record<number, number>
  modelValue: number | null
  allowEmpty?: boolean
}>(), {
  userGroupRates: () => ({}),
  allowEmpty: false
})

const emit = defineEmits<{
  close: []
  select: [value: number | null]
}>()

const { t } = useI18n()
const searchQuery = ref('')
const selectedPlatform = ref<'all' | GroupPlatform>('all')
const searchInput = ref<HTMLInputElement | null>(null)

const availablePlatforms = computed<GroupPlatform[]>(() => {
  const preferred: GroupPlatform[] = ['openai', 'anthropic', 'gemini', 'grok', 'antigravity', 'kimi', 'zhipu', 'deepseek', 'composite']
  const present = new Set(props.groups.map((group) => group.platform))
  return preferred.filter((platform) => present.has(platform))
})

// Sort by the rate the current user will actually pay, including personal overrides.
const effectiveRate = (group: Group): number => props.userGroupRates[group.id] ?? group.rate_multiplier

const filteredGroups = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  return props.groups
    .filter((group) => selectedPlatform.value === 'all' || group.platform === selectedPlatform.value)
    .filter((group) => !query || group.name.toLowerCase().includes(query) || (group.description || '').toLowerCase().includes(query))
    .sort((left, right) => effectiveRate(left) - effectiveRate(right) || left.name.localeCompare(right.name))
})

const groupSections = computed(() => {
  const sections = new Map<GroupPlatform, Group[]>()
  for (const group of filteredGroups.value) {
    const current = sections.get(group.platform)
    if (current) current.push(group)
    else sections.set(group.platform, [group])
  }
  return Array.from(sections, ([platform, groups]) => ({ platform, groups }))
})

const formatRate = (rate: number): string => {
  if (!Number.isFinite(rate)) return '—'
  return rate.toFixed(4).replace(/\.0+$/, '').replace(/(\.\d*?)0+$/, '$1')
}

const hasCustomRate = (group: Group): boolean => {
  const userRate = props.userGroupRates[group.id]
  return userRate !== undefined && userRate !== group.rate_multiplier
}

const platformRateClass = (platform: GroupPlatform): string => `${platformTextClass(platform)} bg-current/10`

watch(
  () => props.show,
  async (isOpen) => {
    if (isOpen) {
      searchQuery.value = ''
      selectedPlatform.value = 'all'
      await nextTick()
      searchInput.value?.focus()
    }
  },
  { immediate: true }
)
</script>

<style scoped>
.group-picker-chip {
  @apply inline-flex min-h-9 items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition;
}

.group-picker-chip-active {
  @apply bg-primary-600 text-white shadow-sm shadow-primary-500/20 dark:bg-primary-500;
}

.group-picker-chip-muted {
  color: color-mix(in srgb, var(--chip-accent, #64748b) 82%, black);
  background-color: color-mix(in srgb, var(--chip-accent, #64748b) 8%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--chip-accent, #64748b) 24%, transparent);
}

.group-picker-chip-muted:hover {
  background-color: color-mix(in srgb, var(--chip-accent, #64748b) 15%, transparent);
}

.dark .group-picker-chip-muted {
  color: color-mix(in srgb, var(--chip-accent, #94a3b8) 76%, white);
  background-color: color-mix(in srgb, var(--chip-accent, #94a3b8) 12%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--chip-accent, #94a3b8) 30%, transparent);
}

.group-picker-card {
  @apply min-h-[112px] rounded-xl border border-gray-200 bg-white p-3.5 transition duration-200 hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/70 dark:border-dark-600 dark:bg-dark-800/75 dark:hover:border-dark-500 dark:hover:bg-dark-800;
}

.group-picker-card-selected {
  @apply border-primary-500 bg-primary-50/70 ring-1 ring-primary-500/50 dark:border-primary-400 dark:bg-primary-500/10 dark:ring-primary-400/50;
}

.group-picker-rate {
  @apply inline-flex shrink-0 items-center rounded-md px-2 py-1 text-[11px] font-semibold tabular-nums;
}

.group-picker-meta {
  @apply inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium;
}

@media (prefers-reduced-motion: reduce) {
  .group-picker-card,
  .group-picker-chip {
    transition-duration: 1ms;
  }
}
</style>
