import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'
import fr from '@/i18n/locales/fr'
import ru from '@/i18n/locales/ru'

function flattenKeys(obj: Record<string, any>, prefix = ''): string[] {
  const keys: string[] = []
  for (const [k, v] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
      keys.push(...flattenKeys(v, fullKey))
    } else {
      keys.push(fullKey)
    }
  }
  return keys
}

describe('ops locale key completeness', () => {
  const requiredKeys = [
    'admin.ops.result',
    'admin.ops.timeRange.custom',
    'admin.ops.customTimeRange.startTime',
    'admin.ops.customTimeRange.endTime',
    'admin.ops.errorDetail.upstreamStatus',
    'admin.ops.errorDetail.rootCause',
    'admin.ops.errorDetail.diagnosticPayloads',
    'admin.ops.errorDetail.payloads.client',
    'admin.ops.errorDetail.payloads.upstream_message',
    'admin.ops.errorDetail.payloads.upstream_detail',
    'admin.ops.errorDetail.payloads.upstream_events',
  ]

  const locales = { en, zh, fr, ru }

  for (const [code, messages] of Object.entries(locales)) {
    for (const key of requiredKeys) {
      it(`${code} locale has ${key}`, () => {
        expect(flattenKeys(messages)).toContain(key)
      })
    }
  }
})

describe('groups locale key completeness', () => {
  it('en locale has admin.groups.failedToSave', () => {
    const enKeys = flattenKeys(en)
    expect(enKeys).toContain('admin.groups.failedToSave')
  })

  const webSearchPricingKeys = [
    'admin.groups.webSearchPricing.title',
    'admin.groups.webSearchPricing.pricePerCall',
    'admin.groups.webSearchPricing.pricePerCallHint',
    'admin.groups.webSearchPricing.finalPricePreview',
  ]

  for (const key of webSearchPricingKeys) {
    it(`en, zh, fr, and ru locales all have ${key}`, () => {
      expect(flattenKeys(en)).toContain(key)
      expect(flattenKeys(zh)).toContain(key)
      expect(flattenKeys(fr)).toContain(key)
      expect(flattenKeys(ru)).toContain(key)
    })
  }
})
