/**
 * Site-wide billing display currency.
 *
 * Defaults match the historical hardcoded CNY / ¥ so old installs look the
 * same until an admin changes 通用设置. Runtime values come from public
 * settings via `applyBillingDisplay`.
 */
import { reactive } from 'vue'

export const DEFAULT_BILLING_CURRENCY = 'CNY'
export const DEFAULT_BILLING_CURRENCY_SYMBOL = '¥'

/** Fallback / historical default. Prefer `getBillingCurrency()` at runtime. */
export const BILLING_CURRENCY = DEFAULT_BILLING_CURRENCY

export const CURRENCY_SYMBOLS: Record<string, string> = {
  USD: '$',
  CNY: '¥',
  RMB: '¥',
  EUR: '€',
  GBP: '£',
  JPY: '¥',
  HKD: 'HK$',
  TWD: 'NT$',
  KRW: '₩',
  AUD: 'A$',
  CAD: 'C$',
  SGD: 'S$',
  NZD: 'NZ$',
  MOP: 'MOP$',
  MYR: 'RM',
  THB: '฿',
  PHP: '₱',
  INR: '₹',
}

const DISPLAY_CURRENCY_RE = /^[A-Z]{3}$/
const MAX_CURRENCY_SYMBOL_LENGTH = 16

export function normalizeDisplayCurrency(currency?: string | null): string {
  const normalized = String(currency || '').trim().toUpperCase()
  return DISPLAY_CURRENCY_RE.test(normalized) ? normalized : DEFAULT_BILLING_CURRENCY
}

export function defaultSymbolForCurrency(currency?: string | null): string {
  const normalized = normalizeDisplayCurrency(currency)
  return CURRENCY_SYMBOLS[normalized] || normalized
}

export function normalizeDisplayCurrencySymbol(
  symbol?: string | null,
  currency?: string | null
): string {
  const trimmed = String(symbol || '').trim()
  if (!trimmed) {
    return defaultSymbolForCurrency(currency)
  }
  return [...trimmed].slice(0, MAX_CURRENCY_SYMBOL_LENGTH).join('')
}

export function currencySymbolFor(currency?: string | null, fallback: string = BILLING_CURRENCY): string {
  const normalized = String(currency || fallback).trim().toUpperCase()
  if (!DISPLAY_CURRENCY_RE.test(normalized)) {
    return CURRENCY_SYMBOLS[fallback] || fallback
  }
  return CURRENCY_SYMBOLS[normalized] || normalized
}

export const BILLING_CURRENCY_SYMBOL = currencySymbolFor(BILLING_CURRENCY)

export const billingDisplay = reactive({
  currency: DEFAULT_BILLING_CURRENCY,
  symbol: DEFAULT_BILLING_CURRENCY_SYMBOL,
})

export function getBillingCurrency(): string {
  return billingDisplay.currency
}

export function getBillingCurrencySymbol(): string {
  return billingDisplay.symbol
}

export function applyBillingDisplay(settings?: {
  display_currency?: string | null
  display_currency_symbol?: string | null
} | null): void {
  const currency = normalizeDisplayCurrency(settings?.display_currency)
  billingDisplay.currency = currency
  billingDisplay.symbol = normalizeDisplayCurrencySymbol(settings?.display_currency_symbol, currency)
}

export function formatBillingAmount(
  amount: number | null | undefined,
  fractionDigits = 2
): string {
  const n = typeof amount === 'number' && Number.isFinite(amount) ? amount : 0
  return `${getBillingCurrencySymbol()}${n.toFixed(fractionDigits)}`
}

export function formatBillingAmountSigned(amount: number, fractionDigits = 2): string {
  const sign = amount >= 0 ? '+' : ''
  return `${sign}${formatBillingAmount(amount, fractionDigits)}`
}
