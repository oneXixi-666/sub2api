<template>
  <div v-if="variant === 'promo-login'" class="promo-auth-shell promo-light">
    <div class="promo-auth-ticker" aria-hidden="true">
      <span>{{ t('auth.promoTickerAccess') }}</span>
      <span>{{ t('auth.promoTickerKeys') }}</span>
      <span>{{ t('auth.promoTickerUsage') }}</span>
      <span>{{ t('auth.promoTickerTheme') }}</span>
    </div>

    <header class="promo-auth-header">
      <div class="promo-auth-brand">
        <span class="promo-auth-logo">
          <img :src="siteLogo || '/logo.svg'" alt="" class="h-full w-full object-contain" />
        </span>
        <span class="min-w-0">
          <strong class="promo-auth-brand-name">{{ siteName }}</strong>
          <small class="promo-auth-brand-subtitle">{{ siteSubtitle }}</small>
        </span>
      </div>
      <LocaleSwitcher />
    </header>

    <main class="promo-auth-main">
      <section class="promo-auth-story" aria-labelledby="promo-auth-heading">
        <p class="promo-auth-eyebrow">{{ t('auth.promoEyebrow') }}</p>
        <h1 id="promo-auth-heading" class="promo-auth-headline">
          <span>{{ t('auth.promoHeadlineAccess') }}</span>
          <mark>{{ t('auth.promoHeadlineControl') }}</mark>
        </h1>
        <p class="promo-auth-description">{{ t('auth.promoDescription') }}</p>

        <div class="promo-auth-receipt" :aria-label="t('auth.promoReceiptTitle')">
          <div class="promo-auth-receipt-header">
            <span>{{ t('auth.promoReceiptTitle') }}</span>
            <span>SUB2API</span>
          </div>
          <div v-for="row in receiptRows" :key="row.label" class="promo-auth-receipt-row">
            <span>{{ row.label }}</span>
            <strong>{{ row.value }}</strong>
          </div>
          <div class="promo-auth-receipt-footer">
            <span>{{ t('auth.promoReceiptFooter') }}</span>
            <span class="promo-auth-status-dot"></span>
          </div>
        </div>
      </section>

      <section class="promo-auth-form-column">
        <div class="promo-auth-access-label">{{ t('auth.secureEntry') }}</div>
        <div class="promo-auth-form-frame">
          <slot />
        </div>
        <div class="promo-auth-form-footer">
          <slot name="footer" />
        </div>
      </section>
    </main>

    <footer class="promo-auth-copyright">
      &copy; {{ currentYear }} {{ siteName }}. {{ t('auth.rightsReserved') }}
    </footer>
  </div>

  <div v-else class="relative flex min-h-screen items-center justify-center overflow-hidden p-4">
    <div
      class="absolute inset-0 bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950"
    ></div>

    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"></div>
      <div class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/15 blur-3xl"></div>
      <div class="absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-300/10 blur-3xl"></div>
      <div class="absolute inset-0 bg-[linear-gradient(rgba(226,29,37,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(226,29,37,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"></div>
    </div>

    <div class="relative z-10 w-full max-w-md">
      <div class="mb-8 text-center">
        <template v-if="settingsLoaded">
          <div class="mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-md border-2 border-black bg-white shadow-card">
            <img :src="siteLogo || '/logo.svg'" alt="" class="h-full w-full object-contain" />
          </div>
          <h1 class="mb-2 text-3xl font-bold text-primary-600">{{ siteName }}</h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ siteSubtitle }}</p>
        </template>
      </div>

      <div class="card-glass p-8">
        <slot />
      </div>

      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. {{ t('auth.rightsReserved') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { sanitizeUrl } from '@/utils/url'

const props = withDefaults(defineProps<{
  variant?: 'default' | 'promo-login'
}>(), {
  variant: 'default'
})

const { t } = useI18n()
const appStore = useAppStore()

const variant = computed(() => props.variant)
const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || t('auth.platformSubtitle'))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = computed(() => new Date().getFullYear())
const receiptRows = computed(() => [
  { label: t('auth.promoReceiptKeys'), value: t('auth.promoReceiptManage') },
  { label: t('auth.promoReceiptRequests'), value: t('auth.promoReceiptVisible') },
  { label: t('auth.promoReceiptUsage'), value: t('auth.promoReceiptTrack') },
  { label: t('auth.promoReceiptBilling'), value: t('auth.promoReceiptClear') }
])

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
