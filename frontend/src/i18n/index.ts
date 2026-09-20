import { computed, reactive } from 'vue'
import { createI18n } from 'vue-i18n'

export type LocaleCode = 'en' | 'zh' | 'fr' | 'ru'
export { toIntlLocale } from './intlLocale'

type LocaleMessages = Record<string, any>

const LOCALE_KEY = 'sub2api_locale'
const DEFAULT_LOCALE: LocaleCode = 'en'
export const BUILTIN_LOCALE_CODES: LocaleCode[] = ['en', 'zh', 'fr', 'ru']

const localeLoaders: Record<LocaleCode, () => Promise<{ default: LocaleMessages }>> = {
  en: () => import('./locales/en'),
  zh: () => import('./locales/zh'),
  fr: () => import('./locales/fr'),
  ru: () => import('./locales/ru')
}

export function isLocaleCode(value: string): value is LocaleCode {
  return value === 'en' || value === 'zh' || value === 'fr' || value === 'ru'
}

export function normalizeDisplayLocales(raw?: readonly string[] | null): LocaleCode[] {
  const enabled = new Set<LocaleCode>()
  for (const item of raw || []) {
    const code = String(item || '').trim().toLowerCase()
    if (isLocaleCode(code)) {
      enabled.add(code)
    }
  }
  const ordered = BUILTIN_LOCALE_CODES.filter((code) => enabled.has(code))
  return ordered.length > 0 ? ordered : [...BUILTIN_LOCALE_CODES]
}

export function normalizeDefaultLocale(
  raw?: string | null,
  locales: readonly string[] = BUILTIN_LOCALE_CODES
): LocaleCode {
  const enabled = normalizeDisplayLocales(locales)
  const code = String(raw || '').trim().toLowerCase()
  if (isLocaleCode(code) && enabled.includes(code)) {
    return code
  }
  return enabled[0]
}

const displayLocaleState = reactive({
  enabled: [...BUILTIN_LOCALE_CODES] as LocaleCode[],
  defaultLocale: DEFAULT_LOCALE as LocaleCode,
})

export function applyDisplayLocales(settings?: {
  display_locales?: string[] | null
  default_locale?: string | null
} | null): void {
  const enabled = normalizeDisplayLocales(settings?.display_locales)
  displayLocaleState.enabled = enabled
  displayLocaleState.defaultLocale = normalizeDefaultLocale(settings?.default_locale, enabled)
}

export function getEnabledLocaleCodes(): LocaleCode[] {
  return [...displayLocaleState.enabled]
}

export function getConfiguredDefaultLocale(): LocaleCode {
  return displayLocaleState.defaultLocale
}

function localeFromBrowser(): LocaleCode | null {
  const browserLang = typeof navigator === 'undefined' ? '' : navigator.language.toLowerCase()
  if (browserLang.startsWith('zh')) return 'zh'
  if (browserLang.startsWith('fr')) return 'fr'
  if (browserLang.startsWith('ru')) return 'ru'
  if (browserLang.startsWith('en')) return 'en'
  return null
}

function getDefaultLocale(): LocaleCode {
  const enabled = displayLocaleState.enabled
  const saved = typeof localStorage === 'undefined' ? null : localStorage.getItem(LOCALE_KEY)
  if (saved && isLocaleCode(saved) && enabled.includes(saved)) {
    return saved
  }

  const browser = localeFromBrowser()
  if (browser && enabled.includes(browser)) {
    return browser
  }

  return displayLocaleState.defaultLocale
}

export function resolveLocale(preferred?: string | null): LocaleCode {
  const enabled = displayLocaleState.enabled
  const code = String(preferred || '').trim().toLowerCase()
  if (isLocaleCode(code) && enabled.includes(code)) {
    return code
  }
  return getDefaultLocale()
}

export const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: {},
  // 禁用 HTML 消息警告 - 引导步骤使用富文本内容（driver.js 支持 HTML）
  // 这些内容是内部定义的，不存在 XSS 风险
  warnHtmlMessage: false
})

const loadedLocales = new Set<LocaleCode>()

export async function loadLocaleMessages(locale: LocaleCode): Promise<void> {
  if (loadedLocales.has(locale)) {
    return
  }

  const loader = localeLoaders[locale]
  const module = await loader()
  i18n.global.setLocaleMessage(locale, module.default)
  loadedLocales.add(locale)
}

export async function initI18n(): Promise<void> {
  if (typeof window !== 'undefined') {
    applyDisplayLocales(window.__APP_CONFIG__)
  }
  i18n.global.fallbackLocale.value = displayLocaleState.defaultLocale
  const current = getDefaultLocale()
  i18n.global.locale.value = current
  await loadLocaleMessages(current)
  document.documentElement.setAttribute('lang', current)
}

export async function setLocale(locale: string): Promise<void> {
  if (!isLocaleCode(locale) || !displayLocaleState.enabled.includes(locale)) {
    return
  }

  await loadLocaleMessages(locale)
  i18n.global.locale.value = locale
  localStorage.setItem(LOCALE_KEY, locale)
  document.documentElement.setAttribute('lang', locale)

  // 同步更新浏览器页签标题，使其跟随语言切换
  const { resolveRouteDocumentTitle } = await import('@/router/title')
  const { default: router } = await import('@/router')
  const { useAppStore } = await import('@/stores/app')
  const { useAuthStore } = await import('@/stores/auth')
  const { useAdminSettingsStore } = await import('@/stores/adminSettings')
  const route = router.currentRoute.value
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]
  const { resolveSiteBillingMode } = await import('@/utils/siteBillingMode')
  document.title = resolveRouteDocumentTitle(route, appStore.siteName, customMenuItems, {
    billingMode: resolveSiteBillingMode(appStore.cachedPublicSettings),
  })
}

export function getLocale(): LocaleCode {
  const current = i18n.global.locale.value
  return isLocaleCode(current) && displayLocaleState.enabled.includes(current)
    ? current
    : getDefaultLocale()
}

export async function syncLocaleWithDisplaySettings(): Promise<void> {
  i18n.global.fallbackLocale.value = displayLocaleState.defaultLocale
  const next = resolveLocale(i18n.global.locale.value)
  if (next !== i18n.global.locale.value) {
    await setLocale(next)
  }
}

export const availableLocales = [
  { code: 'en', name: 'English', flag: '🇺🇸' },
  { code: 'zh', name: '中文', flag: '🇨🇳' },
  { code: 'fr', name: 'Français', flag: '🇫🇷' },
  { code: 'ru', name: 'Русский', flag: '🇷🇺' }
] as const

export const enabledLocales = computed(() =>
  availableLocales.filter((locale) => displayLocaleState.enabled.includes(locale.code))
)

export default i18n
