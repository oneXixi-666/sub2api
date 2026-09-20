import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

import LocaleSwitcher from '../LocaleSwitcher.vue'

const locale = ref('en')
const setLocale = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale
    })
  }
})

vi.mock('@/i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/i18n')>()
  return {
    ...actual,
    setLocale
  }
})

describe('LocaleSwitcher', () => {
  afterEach(async () => {
    const { applyDisplayLocales } = await import('@/i18n')
    applyDisplayLocales(null)
  })

  it('lists English, Chinese, French, and Russian', async () => {
    const wrapper = mount(LocaleSwitcher, {
      global: {
        stubs: { Icon: true }
      }
    })

    await wrapper.get('.locale-switcher-trigger').trigger('click')
    const labels = wrapper.findAll('.locale-switcher-option').map((node) => node.text())

    expect(labels).toHaveLength(4)
    expect(labels.some((text) => text.includes('EN') && text.includes('English'))).toBe(true)
    expect(labels.some((text) => text.includes('ZH') && text.includes('中文'))).toBe(true)
    expect(labels.some((text) => text.includes('FR') && text.includes('Français'))).toBe(true)
    expect(labels.some((text) => text.includes('RU') && text.includes('Русский'))).toBe(true)
  })

  it('switches to French from the menu', async () => {
    setLocale.mockClear()
    const wrapper = mount(LocaleSwitcher, {
      global: {
        stubs: { Icon: true }
      }
    })

    await wrapper.get('.locale-switcher-trigger').trigger('click')
    const french = wrapper.findAll('.locale-switcher-option').find((node) => node.text().includes('Français'))
    expect(french).toBeDefined()
    await french!.trigger('click')
    expect(setLocale).toHaveBeenCalledWith('fr')
  })

  it('hides language packs disabled in public settings', async () => {
    const { applyDisplayLocales } = await import('@/i18n')
    applyDisplayLocales({ display_locales: ['en', 'zh'], default_locale: 'en' })

    const wrapper = mount(LocaleSwitcher, {
      global: {
        stubs: { Icon: true }
      }
    })

    await wrapper.get('.locale-switcher-trigger').trigger('click')
    const labels = wrapper.findAll('.locale-switcher-option').map((node) => node.text())

    expect(labels).toHaveLength(2)
    expect(labels.some((text) => text.includes('Français'))).toBe(false)
    expect(labels.some((text) => text.includes('English'))).toBe(true)
    expect(labels.some((text) => text.includes('中文'))).toBe(true)
  })
})
