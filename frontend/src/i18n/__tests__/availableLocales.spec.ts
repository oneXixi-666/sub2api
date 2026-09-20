import { afterEach, describe, expect, it } from 'vitest'

import {
  applyDisplayLocales,
  availableLocales,
  getEnabledLocaleCodes,
  normalizeDefaultLocale,
  normalizeDisplayLocales,
} from '@/i18n'
import { toIntlLocale } from '@/i18n/intlLocale'

describe('available locales', () => {
  afterEach(() => {
    applyDisplayLocales(null)
  })

  it('exposes English, Chinese, French, and Russian', () => {
    expect(availableLocales.map((locale) => locale.code)).toEqual(['en', 'zh', 'fr', 'ru'])
    expect(availableLocales.find((locale) => locale.code === 'fr')?.name).toBe('Français')
    expect(availableLocales.find((locale) => locale.code === 'ru')?.name).toBe('Русский')
  })

  it('normalizes enabled language packs to the built-in list', () => {
    expect(normalizeDisplayLocales(['ZH', 'de', 'en', 'zh'])).toEqual(['en', 'zh'])
    expect(normalizeDisplayLocales([])).toEqual(['en', 'zh', 'fr', 'ru'])
    expect(normalizeDefaultLocale('fr', ['en', 'zh'])).toBe('en')
    applyDisplayLocales({ display_locales: ['fr'], default_locale: 'zh' })
    expect(getEnabledLocaleCodes()).toEqual(['fr'])
  })

  it('maps UI locales to Intl BCP 47 tags', () => {
    expect(toIntlLocale('en')).toBe('en-US')
    expect(toIntlLocale('zh')).toBe('zh-CN')
    expect(toIntlLocale('fr')).toBe('fr-FR')
    expect(toIntlLocale('ru')).toBe('ru-RU')
    expect(toIntlLocale('en-GB')).toBe('en-GB')
    expect(toIntlLocale('de')).toBe('de')
  })
})
