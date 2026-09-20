import { afterEach, describe, expect, it } from 'vitest'
import { applyBillingDisplay } from '@/constants/currency'
import { formatCurrency } from '../format'

describe('formatCurrency display settings', () => {
  afterEach(() => {
    applyBillingDisplay(null)
  })

  it('uses the configured billing symbol for site amounts', () => {
    applyBillingDisplay({ display_currency: 'USD', display_currency_symbol: 'US$' })
    expect(formatCurrency(12.5)).toBe('US$12.50')
    expect(formatCurrency(12.5, 'USD')).toBe('US$12.50')
  })

  it('keeps Intl symbols for a different explicit currency', () => {
    applyBillingDisplay({ display_currency: 'CNY', display_currency_symbol: '¥' })
    expect(formatCurrency(12.5, 'USD')).toMatch(/12\.50/)
    expect(formatCurrency(12.5, 'USD')).not.toContain('¥')
  })
})
