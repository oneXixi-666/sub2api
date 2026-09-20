import type { CustomMenuItem, CustomMenuLocale } from '@/types'

export const CUSTOM_MENU_LOCALES: readonly CustomMenuLocale[] = ['en', 'zh', 'fr', 'ru']

export function emptyCustomMenuLabels(): Record<CustomMenuLocale, string> {
  return { en: '', zh: '', fr: '', ru: '' }
}

export function normalizeCustomMenuLocale(locale?: string): CustomMenuLocale | '' {
  const code = String(locale || '').trim().toLowerCase().split('-')[0]
  return CUSTOM_MENU_LOCALES.includes(code as CustomMenuLocale) ? (code as CustomMenuLocale) : ''
}

export function compactCustomMenuLabels(
  labels?: Partial<Record<CustomMenuLocale, string>> | null,
): Partial<Record<CustomMenuLocale, string>> | undefined {
  if (!labels) return undefined
  const compacted: Partial<Record<CustomMenuLocale, string>> = {}
  for (const code of CUSTOM_MENU_LOCALES) {
    const value = labels[code]?.trim()
    if (value) compacted[code] = value
  }
  return Object.keys(compacted).length > 0 ? compacted : undefined
}

export function resolveCustomMenuLabel(
  item: Pick<CustomMenuItem, 'label' | 'labels'> | null | undefined,
  locale?: string,
): string {
  if (!item) return ''
  const code = normalizeCustomMenuLocale(locale)
  const localized = code ? item.labels?.[code]?.trim() : ''
  if (localized) return localized
  return (item.label || '').trim()
}

export function ensureCustomMenuItemLabels(item: CustomMenuItem): CustomMenuItem {
  return {
    ...item,
    labels: {
      ...emptyCustomMenuLabels(),
      ...(item.labels || {}),
    },
  }
}

export function serializeCustomMenuItem(item: CustomMenuItem): CustomMenuItem {
  const labels = compactCustomMenuLabels(item.labels)
  const label = (item.label || '').trim()
    || labels?.en
    || labels?.zh
    || labels?.fr
    || labels?.ru
    || ''
  const next: CustomMenuItem = {
    id: item.id,
    label,
    icon_svg: item.icon_svg || '',
    url: item.url,
    visibility: item.visibility,
    sort_order: item.sort_order,
  }
  if (item.page_slug) {
    next.page_slug = item.page_slug
  }
  if (typeof item.hide_open_button === 'boolean') {
    next.hide_open_button = item.hide_open_button
  }
  if (labels) {
    next.labels = labels
  }
  return next
}
