import { afterEach, describe, expect, it } from 'vitest'
import {
  BILLING_CURRENCY,
  BILLING_CURRENCY_SYMBOL,
  applyBillingDisplay,
  billingDisplay,
  currencySymbolFor,
  formatBillingAmount,
  formatBillingAmountSigned,
  getBillingCurrency,
  getBillingCurrencySymbol,
} from '../currency'

describe('billing currency config', () => {
  afterEach(() => {
    applyBillingDisplay(null)
  })

  it('uses CNY / ¥ as the site billing unit', () => {
    expect(BILLING_CURRENCY).toBe('CNY')
    expect(BILLING_CURRENCY_SYMBOL).toBe('¥')
    expect(currencySymbolFor(BILLING_CURRENCY)).toBe(BILLING_CURRENCY_SYMBOL)
    expect(getBillingCurrency()).toBe('CNY')
    expect(getBillingCurrencySymbol()).toBe('¥')
  })

  it('formats amounts from the shared symbol', () => {
    expect(formatBillingAmount(12.5)).toBe(`${BILLING_CURRENCY_SYMBOL}12.50`)
    expect(formatBillingAmount(null)).toBe(`${BILLING_CURRENCY_SYMBOL}0.00`)
    expect(formatBillingAmount(1.23456, 4)).toBe(`${BILLING_CURRENCY_SYMBOL}1.2346`)
    expect(formatBillingAmountSigned(8)).toBe(`+${BILLING_CURRENCY_SYMBOL}8.00`)
    expect(formatBillingAmountSigned(-3.2)).toBe(`${BILLING_CURRENCY_SYMBOL}-3.20`)
  })

  it('applies public display settings at runtime', () => {
    applyBillingDisplay({ display_currency: 'usd', display_currency_symbol: ' US$ ' })
    expect(billingDisplay.currency).toBe('USD')
    expect(billingDisplay.symbol).toBe('US$')
    expect(formatBillingAmount(12.5)).toBe('US$12.50')
  })

  it('derives a symbol when the admin leaves it empty', () => {
    applyBillingDisplay({ display_currency: 'EUR', display_currency_symbol: '' })
    expect(getBillingCurrency()).toBe('EUR')
    expect(getBillingCurrencySymbol()).toBe('€')
  })
})
