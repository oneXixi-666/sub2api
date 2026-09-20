import { describe, expect, it } from 'vitest'
import { reactive } from 'vue'

import type { CustomMenuItem } from '@/types'
import {
  compactCustomMenuLabels,
  ensureCustomMenuItemLabels,
  resolveCustomMenuLabel,
  serializeCustomMenuItem,
} from '../customMenuLabel'

const item = (overrides: Partial<CustomMenuItem> = {}): CustomMenuItem => ({
  id: 'docs',
  label: 'Help Center',
  icon_svg: '',
  url: 'https://example.com/help',
  visibility: 'user',
  sort_order: 0,
  ...overrides,
})

describe('resolveCustomMenuLabel', () => {
  it('uses the localized name for the current language', () => {
    expect(resolveCustomMenuLabel(item({
      labels: { fr: 'Centre d’aide', zh: '帮助中心' },
    }), 'fr')).toBe('Centre d’aide')
  })

  it('falls back to the default name when a language is empty', () => {
    expect(resolveCustomMenuLabel(item({
      labels: { fr: 'Centre d’aide' },
    }), 'ru')).toBe('Help Center')
  })

  it('treats zh-CN as zh', () => {
    expect(resolveCustomMenuLabel(item({
      labels: { zh: '帮助中心' },
    }), 'zh-CN')).toBe('帮助中心')
  })
})

describe('serializeCustomMenuItem', () => {
  it('omits empty localized names and keeps the default label', () => {
    const serialized = serializeCustomMenuItem(ensureCustomMenuItemLabels(item({
      labels: { en: '  ', fr: 'Centre d’aide' },
    })))
    expect(serialized.label).toBe('Help Center')
    expect(serialized.labels).toEqual({ fr: 'Centre d’aide' })
  })

  it('fills the default label from the first translated name', () => {
    const serialized = serializeCustomMenuItem(item({
      label: '  ',
      labels: { ru: 'Справка' },
    }))
    expect(serialized.label).toBe('Справка')
    expect(serialized.labels).toEqual({ ru: 'Справка' })
  })

  it('does not persist empty locale slots', () => {
    const serialized = serializeCustomMenuItem(ensureCustomMenuItemLabels(item({
      hide_open_button: false,
    })))
    expect(serialized).toEqual({
      id: 'docs',
      label: 'Help Center',
      icon_svg: '',
      url: 'https://example.com/help',
      visibility: 'user',
      sort_order: 0,
      hide_open_button: false,
    })
    expect(serialized.labels).toBeUndefined()
  })

  it('strips empty labels from reactive menu items', () => {
    const serialized = serializeCustomMenuItem(reactive(ensureCustomMenuItemLabels(item({
      hide_open_button: true,
    }))))
    expect(serialized.labels).toBeUndefined()
    expect(serialized.hide_open_button).toBe(true)
  })
})

describe('compactCustomMenuLabels', () => {
  it('returns undefined when every language is blank', () => {
    expect(compactCustomMenuLabels({ en: ' ', zh: '' })).toBeUndefined()
  })
})
