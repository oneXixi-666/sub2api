export const intlLocales = {
  en: 'en-US',
  zh: 'zh-CN',
  fr: 'fr-FR',
  ru: 'ru-RU'
} as const

export type IntlLocaleCode = keyof typeof intlLocales

export function toIntlLocale(locale: string): string {
  if (locale in intlLocales) {
    return intlLocales[locale as IntlLocaleCode]
  }
  return locale || intlLocales.en
}
