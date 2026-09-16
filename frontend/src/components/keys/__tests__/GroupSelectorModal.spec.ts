import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import GroupSelectorModal from '../GroupSelectorModal.vue'
import type { Group } from '@/types'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

function group(id: number, platform: string, name = platform): Group {
  return {
    id,
    name,
    platform,
    rate_multiplier: 1,
    subscription_type: 'standard',
  } as Group
}

function mountPicker(groups: Group[]) {
  return mount(GroupSelectorModal, {
    props: {
      show: true,
      groups,
      modelValue: null,
    },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show', 'title'],
          template: '<div v-if="show" role="dialog"><slot /></div>',
        },
        PlatformIcon: true,
        Icon: true,
      },
    },
  })
}

describe('GroupSelectorModal platform chips', () => {
  it('shows catalog platforms that are present, including MiniMax and OpenCode', () => {
    const wrapper = mountPicker([
      group(1, 'openai'),
      group(2, 'minimax'),
      group(3, 'opencode_go', 'OpenCode Go'),
      group(4, 'kimi'),
    ])
    const chips = wrapper.findAll('[role="tab"]').map((chip) => chip.text())
    expect(chips[0]).toContain('keys.groupPicker.allPlatforms')
    expect(chips).toEqual(expect.arrayContaining([
      expect.stringContaining('OpenAI'),
      expect.stringContaining('Kimi'),
      expect.stringContaining('MiniMax'),
      expect.stringContaining('OpenCode'),
    ]))
    expect(chips.findIndex((text) => text.includes('OpenAI')))
      .toBeLessThan(chips.findIndex((text) => text.includes('MiniMax')))
  })

  it('keeps unknown platform types as chips instead of dropping them', () => {
    const wrapper = mountPicker([
      group(1, 'openai'),
      group(2, 'future_vendor', 'Future'),
    ])
    const chips = wrapper.findAll('[role="tab"]').map((chip) => chip.text())
    expect(chips.some((text) => text.includes('future_vendor'))).toBe(true)
    expect(wrapper.text()).toContain('Future')
  })
})
