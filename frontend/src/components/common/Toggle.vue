<template>
  <button
    type="button"
    @click="toggle"
    class="switch flex-shrink-0"
    :class="{ 'switch-active': modelValue }"
    role="switch"
    :aria-checked="modelValue"
  >
    <span
      class="switch-thumb pointer-events-none inline-block"
      :class="[modelValue ? 'translate-x-5' : 'translate-x-0']"
    />
  </button>
</template>

<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function toggle() {
  emit('update:modelValue', !props.modelValue)
}
</script>
