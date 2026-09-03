<template>
  <section class="space-y-5" data-test="cyber-policy-editor">
    <div class="overflow-hidden rounded-xl border border-amber-200 bg-white dark:border-amber-900/60 dark:bg-dark-900">
      <div class="flex flex-col gap-2 border-b border-amber-100 bg-amber-50/80 px-4 py-3 dark:border-amber-900/40 dark:bg-amber-950/20 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div class="flex items-center gap-2">
            <span class="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-amber-500 text-white">
              <Icon name="shield" size="sm" />
            </span>
            <h3 class="text-sm font-semibold text-amber-950 dark:text-amber-100">{{ t('admin.riskControl.cyberDefaultPolicy') }}</h3>
          </div>
          <p class="mt-1 text-xs leading-5 text-amber-700 dark:text-amber-300">{{ t('admin.riskControl.cyberDefaultPolicyHint') }}</p>
        </div>
        <span class="inline-flex w-fit rounded-full bg-white px-2.5 py-1 text-xs font-medium text-amber-700 shadow-sm dark:bg-dark-800 dark:text-amber-300">
          {{ t('admin.riskControl.cyberPolicyInheritedBy', { count: inheritedGroupCount }) }}
        </span>
      </div>
      <div class="p-4">
        <CyberPolicyRuleCard :policy="defaultPolicy" @update:policy="emit('update:defaultPolicy', $event)" />
      </div>
    </div>

    <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.riskControl.cyberGroupPolicies') }}</h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.riskControl.cyberGroupPoliciesHint') }}</p>
      </div>
      <div class="flex w-full gap-2 sm:w-auto">
        <div class="min-w-0 flex-1 sm:w-72">
          <Select
            v-model="pendingGroupID"
            :options="availableGroupOptions"
            :placeholder="t('admin.riskControl.cyberSelectGroup')"
            :empty-text="t('admin.riskControl.cyberNoGroupsToAdd')"
            searchable
          />
        </div>
        <button type="button" class="btn btn-secondary shrink-0" :disabled="!pendingGroupID" @click="addGroupPolicy">
          <Icon name="plus" size="sm" />
          {{ t('admin.riskControl.cyberAddGroupPolicy') }}
        </button>
      </div>
    </div>

    <div v-if="groupPolicies.length === 0" class="rounded-xl border border-dashed border-gray-200 px-5 py-8 text-center dark:border-dark-600">
      <p class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.riskControl.cyberNoGroupPolicies') }}</p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.riskControl.cyberNoGroupPoliciesHint') }}</p>
    </div>

    <div v-else class="space-y-3">
      <article v-for="item in groupPolicies" :key="item.group_id" class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900" :data-test="`cyber-group-policy-${item.group_id}`">
        <div class="flex items-center justify-between gap-3 border-b border-gray-100 bg-gray-50/80 px-4 py-3 dark:border-dark-700 dark:bg-dark-800/70">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h4 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ groupName(item.group_id) }}</h4>
              <span class="rounded-md bg-gray-200/70 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-600 dark:text-gray-300">{{ groupPlatform(item.group_id) }}</span>
              <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="item.policy.mode === 'enforce' ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300' : 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'">
                {{ item.policy.mode === 'enforce' ? t('admin.riskControl.cyberPolicyEnforceMode') : t('admin.riskControl.cyberPolicyCollectMode') }}
              </span>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.riskControl.cyberGroupOverrideHint') }}</p>
          </div>
          <button type="button" class="btn btn-ghost shrink-0 text-red-600 dark:text-red-300" :aria-label="t('admin.riskControl.cyberRemoveGroupPolicy')" @click="removeGroupPolicy(item.group_id)">
            <Icon name="trash" size="sm" />
          </button>
        </div>
        <div class="p-4">
          <CyberPolicyRuleCard :policy="item.policy" @update:policy="updateGroupPolicy(item.group_id, $event)" />
        </div>
      </article>
    </div>

    <div class="rounded-lg border border-sky-100 bg-sky-50/70 px-4 py-3 text-xs leading-5 text-sky-700 dark:border-sky-900/50 dark:bg-sky-950/20 dark:text-sky-300">
      {{ t('admin.riskControl.cyberCollectionInvariant') }}
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import CyberPolicyRuleCard from './CyberPolicyRuleCard.vue'
import type { CyberPolicyGroupPolicy, CyberPolicySettings } from '@/api/admin/riskControl'
import type { AdminGroup, SelectOption } from '@/types'

const props = defineProps<{
  defaultPolicy: CyberPolicySettings
  groupPolicies: CyberPolicyGroupPolicy[]
  groups: AdminGroup[]
}>()
const emit = defineEmits<{
  'update:defaultPolicy': [value: CyberPolicySettings]
  'update:groupPolicies': [value: CyberPolicyGroupPolicy[]]
}>()
const { t } = useI18n()
const pendingGroupID = ref<number | null>(null)

const configuredGroupIDs = computed(() => new Set(props.groupPolicies.map((item) => item.group_id)))
const availableGroupOptions = computed<SelectOption[]>(() => props.groups
  .filter((group) => !configuredGroupIDs.value.has(group.id))
  .map((group) => ({ value: group.id, label: `${group.name} · ${group.platform}` })))
const inheritedGroupCount = computed(() => Math.max(0, props.groups.length - props.groupPolicies.length))

function clonePolicy(policy: CyberPolicySettings): CyberPolicySettings {
  return { ...policy }
}

function addGroupPolicy() {
  const groupID = Number(pendingGroupID.value)
  if (!Number.isFinite(groupID) || groupID <= 0 || configuredGroupIDs.value.has(groupID)) return
  emit('update:groupPolicies', [
    ...props.groupPolicies,
    { group_id: groupID, policy: clonePolicy(props.defaultPolicy) },
  ].sort((a, b) => a.group_id - b.group_id))
  pendingGroupID.value = null
}

function removeGroupPolicy(groupID: number) {
  emit('update:groupPolicies', props.groupPolicies.filter((item) => item.group_id !== groupID))
}

function updateGroupPolicy(groupID: number, policy: CyberPolicySettings) {
  emit('update:groupPolicies', props.groupPolicies.map((item) => (
    item.group_id === groupID ? { ...item, policy: clonePolicy(policy) } : item
  )))
}

function findGroup(groupID: number): AdminGroup | undefined {
  return props.groups.find((group) => group.id === groupID)
}

function groupName(groupID: number): string {
  return findGroup(groupID)?.name || t('admin.riskControl.cyberUnknownGroup', { id: groupID })
}

function groupPlatform(groupID: number): string {
  return String(findGroup(groupID)?.platform || '-')
}
</script>
