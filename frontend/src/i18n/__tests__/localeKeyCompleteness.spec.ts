import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'
import fr from '../locales/fr'
import ru from '../locales/ru'

type LocaleValue = Record<string, unknown>

function flattenLeafKeys(value: unknown, prefix = ''): string[] {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return prefix ? [prefix] : []
  }

  return Object.entries(value as LocaleValue).flatMap(([key, child]) => {
    const path = prefix ? `${prefix}.${key}` : key
    return flattenLeafKeys(child, path)
  })
}

function collectStaticSourceKeys(source: string): string[] {
  const keys = new Set<string>()

  // Covers useI18n().t(), the global $t(), and direct i18n.t() calls in both
  // TypeScript and Vue script/template source. Dynamic suffixes are checked by
  // the runtime type of the value and cannot be proven from source text alone.
  const translationCalls = /(?:\bi18n\.t|\$t|\bt)\s*\(\s*(['"])([^'"\r\n]+)\1/g
  for (const match of source.matchAll(translationCalls)) {
    const key = match[2]
    if (!key.endsWith('.')) {
      keys.add(key)
    }
  }

  // Router metadata and i18n-t are key references without a t() call.
  const keyReferences = /(?:keypath|titleKey|descriptionKey)\s*[:=]\s*(['"])([^'"\r\n]+)\1/g
  for (const match of source.matchAll(keyReferences)) {
    keys.add(match[2])
  }

  const i18nTKeypaths = /<i18n-t\b[^>]*\bkeypath\s*=\s*(['"])([^'"\r\n]+)\1/gi
  for (const match of source.matchAll(i18nTKeypaths)) {
    keys.add(match[2])
  }

  return [...keys]
}

function sourceKeys(): string[] {
  const sourceFiles = import.meta.glob('../../**/*.{ts,vue}', {
    query: '?raw',
    import: 'default',
    eager: true
  }) as Record<string, string>

  return Object.entries(sourceFiles)
    .filter(([path]) => !path.includes('/__tests__/') && !/\.(spec|test)\.ts$/.test(path))
    .flatMap(([, source]) => collectStaticSourceKeys(source))
}

function missingKeys(usedKeys: string[], availableKeys: Set<string>): string[] {
  return usedKeys.filter((key) => !availableKeys.has(key)).sort()
}

describe('locale key completeness', () => {
  const locales = { en, zh, fr, ru } as const
  const enKeys = new Set(flattenLeafKeys(en))
  const usedKeys = [...new Set(sourceKeys())].sort()

  it('keeps all locale schemas identical to English', () => {
    for (const [code, messages] of Object.entries(locales)) {
      if (code === 'en') continue
      const keys = new Set(flattenLeafKeys(messages))
      expect([...enKeys].filter((key) => !keys.has(key)).sort(), `${code} missing vs en`).toEqual([])
      expect([...keys].filter((key) => !enKeys.has(key)).sort(), `${code} extra vs en`).toEqual([])
    }
  })

  it('contains a non-empty message for every locale leaf', () => {
    for (const [locale, messages] of Object.entries(locales)) {
      const emptyKeys = flattenLeafKeys(messages).filter((key) => {
        let current: unknown = messages
        for (const segment of key.split('.')) {
          current = (current as LocaleValue)[segment]
        }
        return typeof current !== 'string' || current.trim() === ''
      })
      expect(emptyKeys, `${locale} has empty or non-string messages`).toEqual([])
    }
  })

  it('contains every statically referenced production key', () => {
    for (const [code, messages] of Object.entries(locales)) {
      expect(
        missingKeys(usedKeys, new Set(flattenLeafKeys(messages))),
        `${code} locale is missing referenced keys`
      ).toEqual([])
    }
  })

  it('translates French and Russian user-facing copy', () => {
    expect(fr.common.save).toBe('Enregistrer')
    expect(ru.common.save).toBe('Сохранить')
    expect(fr.nav.dashboard).not.toBe(en.nav.dashboard)
    expect(ru.nav.dashboard).not.toBe(en.nav.dashboard)
    expect(fr.home.heroSubtitle).not.toBe(en.home.heroSubtitle)
    expect(ru.home.heroSubtitle).not.toBe(en.home.heroSubtitle)
    expect(fr.keyUsage.windowDay).toBe('J')
    expect(ru.keyUsage.windowDay).toBe('Д')
  })
})
