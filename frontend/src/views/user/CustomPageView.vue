<template>
  <AppLayout>
    <div class="custom-page-layout">
      <div class="custom-page-frame flex-1 min-h-0 overflow-hidden">
        <div v-if="loading" class="custom-page-state">
          <LoadingSpinner size="lg" />
        </div>

        <div v-else-if="!menuItem" class="custom-page-state p-10">
          <EmptyState
            :title="t('customPage.notFoundTitle')"
            :description="t('customPage.notFoundDesc')"
          >
            <template #icon>
              <Icon name="link" size="lg" class="text-gray-400" />
            </template>
          </EmptyState>
        </div>

        <!-- Markdown mode with TOC -->
        <div v-else-if="isMarkdownMode" class="flex h-full overflow-hidden">
          <!-- TOC Sidebar -->
          <aside
            v-show="tocVisible"
            class="toc-sidebar"
          >
            <div class="toc-header">
              <span class="toc-title">{{ t('customPage.tableOfContents') }}</span>
              <button
                class="toc-close-btn"
                :aria-label="t('customPage.closeContents')"
                :title="t('customPage.closeContents')"
                @click="tocVisible = false"
              >
                <Icon name="chevronLeft" size="sm" :stroke-width="2" />
              </button>
            </div>
            <nav class="toc-nav">
              <a
                v-for="item in tocItems"
                :key="item.id"
                :href="'#' + item.id"
                class="toc-item"
                :class="[
                  `toc-level-${item.level}`,
                  { 'toc-active': activeHeadingId === item.id }
                ]"
                @click.prevent="scrollToHeading(item.id)"
              >
                {{ item.text }}
              </a>
            </nav>
          </aside>

          <!-- TOC Toggle Button (when collapsed) -->
          <button
            v-show="!tocVisible && tocItems.length > 0"
            class="toc-toggle-btn"
            :aria-label="t('customPage.openContents')"
            :title="t('customPage.openContents')"
            @click="tocVisible = true"
          >
            <Icon name="menu" size="sm" :stroke-width="2" />
            <span class="ml-1 text-xs">{{ t('customPage.tableOfContents') }}</span>
          </button>

          <!-- Content -->
          <div
            ref="markdownContainer"
            class="markdown-page-content flex-1 h-full overflow-auto p-6 md:p-10"
            v-html="renderedHtml"
            @scroll="onContentScroll"
          ></div>
        </div>

        <!-- URL not configured -->
        <div v-else-if="!isValidUrl" class="custom-page-state p-10">
          <EmptyState
            :title="t('customPage.notConfiguredTitle')"
            :description="t('customPage.notConfiguredDesc')"
          >
            <template #icon>
              <Icon name="link" size="lg" class="text-gray-400" />
            </template>
          </EmptyState>
        </div>

        <!-- Iframe embed mode -->
        <div v-else class="custom-embed-shell">
          <a
            :href="embeddedUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary btn-sm custom-open-fab"
          >
            <Icon name="externalLink" size="sm" class="mr-1.5" :stroke-width="2" />
            {{ t('customPage.openInNewTab') }}
          </a>
          <iframe
            :src="embeddedUrl"
            :title="menuItem.label"
            class="custom-embed-frame"
            allowfullscreen
          ></iframe>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { buildApiUrl } from '@/api/client'
import { buildEmbeddedUrl, detectTheme } from '@/utils/embedded-url'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

interface TocItem {
  id: string
  text: string
  level: number
}

const { t, locale } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()

const loading = ref(false)
const pageTheme = ref<'light' | 'dark'>('light')
const renderedHtml = ref('')
const markdownContainer = ref<HTMLElement | null>(null)
const tocItems = ref<TocItem[]>([])
const tocVisible = ref(typeof window !== 'undefined' ? window.innerWidth > 768 : true)
const activeHeadingId = ref('')
let themeObserver: MutationObserver | null = null

const menuItemId = computed(() => route.params.id as string)

const menuItem = computed(() => {
  const id = menuItemId.value
  const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
  const found = publicItems.find((item) => item.id === id) ?? null
  if (found) return found
  if (authStore.isAdmin) {
    return adminSettingsStore.customMenuItems.find((item) => item.id === id) ?? null
  }
  return null
})

const markdownSlug = computed(() => {
  const item = menuItem.value
  if (!item) return ''
  if (item.page_slug) return item.page_slug
  if (item.url?.startsWith('md:')) return item.url.slice(3)
  return ''
})

const isMarkdownMode = computed(() => !!markdownSlug.value)

const embeddedUrl = computed(() => {
  if (!menuItem.value || isMarkdownMode.value) return ''
  return buildEmbeddedUrl(
    menuItem.value.url,
    authStore.user?.id,
    authStore.token,
    pageTheme.value,
    locale.value,
  )
})

const isValidUrl = computed(() => {
  if (isMarkdownMode.value) return false
  const url = embeddedUrl.value
  return url.startsWith('http://') || url.startsWith('https://')
})

function generateHeadingId(text: string, index: number): string {
  const base = text
    .toLowerCase()
    .replace(/[^\w一-鿿]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return base ? `${base}-${index}` : `heading-${index}`
}

function isRelativeMarkdownAsset(src: string): boolean {
  const trimmed = src.trim()
  if (!trimmed || /^[a-z][a-z0-9+.-]*:/i.test(trimmed) || trimmed.startsWith('//') || trimmed.startsWith('/')) {
    return false
  }
  const [pathPart] = trimmed.split(/([?#].*)/, 2)
  return pathPart
    .split('/')
    .filter((part) => part && part !== '.')
    .every((part) => part !== '..' && !part.includes('\\'))
}

function buildPageImageUrl(slug: string, src: string): string {
  const trimmed = src.trim()
  const [pathPart, suffix = ''] = trimmed.split(/([?#].*)/, 2)
  const encodedPath = pathPart
    .split('/')
    .filter((part) => part && part !== '.')
    .map((part) => encodeURIComponent(part))
    .join('/')
  return buildApiUrl(`/pages/${encodeURIComponent(slug)}/images/${encodedPath}${suffix}`)
}

async function fetchAndRenderMarkdown(slug: string) {
  loading.value = true
  tocItems.value = []
  activeHeadingId.value = ''
  try {
    const resp = await fetch(buildApiUrl(`/pages/${encodeURIComponent(slug)}`), {
      headers: authStore.token ? { Authorization: `Bearer ${authStore.token}` } : {},
    })
    if (!resp.ok) {
      renderedHtml.value = `<p class="text-red-500">${t('common.pageNotFound')}</p>`
      return
    }
    let raw = await resp.text()

    raw = raw.replace(
      /!\[([^\]]*)\]\(([^)]+)\)/g,
      (match, alt, src) => isRelativeMarkdownAsset(src) ? `![${alt}](${buildPageImageUrl(slug, src)})` : match
    )

    const html = marked.parse(raw) as string
    const sanitized = DOMPurify.sanitize(html, {
      ADD_TAGS: ['iframe'],
      ADD_ATTR: ['allowfullscreen', 'frameborder', 'src'],
    })

    // Inject IDs into headings and build TOC
    const toc: TocItem[] = []
    let headingIndex = 0
    const withIds = sanitized.replace(
      /<(h[1-4])[^>]*>(.*?)<\/h[1-4]>/gi,
      (_, tag: string, content: string) => {
        const level = parseInt(tag[1])
        const text = content.replace(/<[^>]+>/g, '').trim()
        const id = generateHeadingId(text, headingIndex++)
        toc.push({ id, text, level })
        return `<${tag} id="${id}">${content}</${tag}>`
      }
    )

    renderedHtml.value = withIds
    tocItems.value = toc
  } catch {
    renderedHtml.value = `<p class="custom-page-error">${t('customPage.loadFailed')}</p>`
  } finally {
    loading.value = false
    await nextTick()
    await nextTick()
    injectCopyButtons()
  }
}

function scrollToHeading(id: string) {
  const container = markdownContainer.value
  if (!container) return
  const el = container.querySelector(`#${CSS.escape(id)}`)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    activeHeadingId.value = id
    if (window.innerWidth <= 640) {
      tocVisible.value = false
    }
  }
}

let scrollRafId = 0
function onContentScroll() {
  if (scrollRafId) return
  scrollRafId = requestAnimationFrame(() => {
    scrollRafId = 0
    const container = markdownContainer.value
    if (!container || tocItems.value.length === 0) return

    const containerRect = container.getBoundingClientRect()
    let current = ''

    for (const item of tocItems.value) {
      const el = container.querySelector(`#${CSS.escape(item.id)}`) as HTMLElement | null
      if (el) {
        const elRect = el.getBoundingClientRect()
        if (elRect.top - containerRect.top <= 100) {
          current = item.id
        }
      }
    }
    activeHeadingId.value = current
  })
}

function injectCopyButtons() {
  const container = markdownContainer.value
  if (!container) return

  container.querySelectorAll('pre').forEach((pre) => {
    if (pre.querySelector('.copy-btn')) return
    const btn = document.createElement('button')
    btn.className = 'copy-btn'
    btn.textContent = t('customPage.copyCode')
    btn.addEventListener('click', async () => {
      const code = pre.querySelector('code')?.textContent ?? pre.textContent ?? ''
      try {
        await navigator.clipboard.writeText(code)
        btn.textContent = t('customPage.copiedCode')
        setTimeout(() => { btn.textContent = t('customPage.copyCode') }, 2000)
      } catch {
        btn.textContent = t('customPage.copyCodeFailed')
        setTimeout(() => { btn.textContent = t('customPage.copyCode') }, 2000)
      }
    })
    pre.style.position = 'relative'
    pre.appendChild(btn)
  })
}

function syncCopyButtonLabels() {
  markdownContainer.value?.querySelectorAll<HTMLButtonElement>('.copy-btn').forEach((button) => {
    button.textContent = t('customPage.copyCode')
  })
}

watch(markdownSlug, (slug) => {
  if (slug) {
    fetchAndRenderMarkdown(slug)
  } else {
    renderedHtml.value = ''
    tocItems.value = []
  }
}, { immediate: true })

watch(locale, async () => {
  await nextTick()
  syncCopyButtonLabels()
})

onMounted(async () => {
  pageTheme.value = detectTheme()

  if (typeof document !== 'undefined') {
    themeObserver = new MutationObserver(() => {
      pageTheme.value = detectTheme()
    })
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    })
  }

  if (appStore.publicSettingsLoaded) return
  loading.value = true
  try {
    await appStore.fetchPublicSettings()
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  if (themeObserver) {
    themeObserver.disconnect()
    themeObserver = null
  }
})
</script>

<style scoped>
.custom-page-layout {
  position: relative;
  display: flex;
  min-height: 28rem;
  height: calc(100dvh - var(--promo-header-height) - 4rem);
  flex-direction: column;
}

.custom-page-frame {
  position: relative;
  border: 2px solid var(--promo-border);
  border-radius: var(--promo-radius-sm);
  background: var(--promo-surface);
  box-shadow: var(--promo-shadow-sm);
  color: var(--promo-text);
}

.custom-page-state {
  display: flex;
  height: 100%;
  min-height: 22rem;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.toc-sidebar {
  display: flex;
  width: min(240px, 30%);
  min-width: 160px;
  max-width: 280px;
  height: 100%;
  flex-direction: column;
  overflow: hidden;
  border-right: 2px solid var(--promo-border);
  background: var(--promo-surface-muted);
}

@media (max-width: 640px) {
  .custom-page-layout {
    height: calc(100dvh - var(--promo-header-height) - 1.5rem);
  }

  .toc-sidebar {
    position: absolute;
    left: 0;
    top: 0;
    z-index: 20;
    width: min(82vw, 280px);
    max-width: none;
    height: 100%;
    border-right-width: 3px;
    box-shadow: 4px 0 0 var(--promo-red);
  }
}

.toc-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1rem;
  border-bottom: 2px solid var(--promo-border);
  background: var(--promo-yellow);
}

.toc-title {
  color: var(--promo-black);
  font-family: var(--promo-font-data);
  font-size: 0.875rem;
  font-weight: 900;
}

.toc-close-btn {
  display: inline-flex;
  width: 36px;
  height: 36px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--promo-black);
  border-radius: var(--promo-radius-xs);
  background: var(--promo-white);
  color: var(--promo-black);
}

.toc-nav {
  flex: 1;
  overflow-y: auto;
  padding: 0.5rem;
}

.toc-item {
  display: block;
  overflow: hidden;
  padding-block: 0.5rem;
  border: 1px solid transparent;
  border-radius: var(--promo-radius-xs);
  color: var(--promo-text-muted);
  font-size: 0.875rem;
  text-overflow: ellipsis;
  transition: background-color 160ms ease, border-color 160ms ease, color 160ms ease;
  white-space: nowrap;
}

.toc-item:hover {
  border-color: var(--promo-border-soft);
  background: var(--promo-surface-raised);
  color: var(--promo-text);
}

.toc-item.toc-active {
  border-color: var(--promo-border);
  background: var(--promo-red);
  color: #ffffff;
  font-weight: 800;
}

.toc-level-1 { padding-left: 8px; }
.toc-level-2 { padding-left: 20px; }
.toc-level-3 { padding-left: 32px; }
.toc-level-4 { padding-left: 44px; }

.toc-toggle-btn {
  position: absolute;
  left: 0.75rem;
  top: 0.75rem;
  z-index: 10;
  display: flex;
  min-height: 40px;
  align-items: center;
  padding: 0.4rem 0.65rem;
  border: 2px solid var(--promo-border);
  border-radius: var(--promo-radius-sm);
  background: var(--promo-yellow);
  box-shadow: var(--promo-shadow-xs);
  color: var(--promo-black);
  cursor: pointer;
  font-size: 0.875rem;
}

.custom-embed-shell {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: var(--promo-surface-muted);
}

.custom-open-fab {
  position: absolute;
  right: 0.75rem;
  top: 0.75rem;
  z-index: 10;
}

.custom-embed-frame {
  display: block;
  margin: 0;
  width: 100%;
  height: 100%;
  border: 0;
  background: var(--promo-surface-raised);
}
</style>

<style>
.markdown-page-content {
  color: var(--promo-text);
  line-height: 1.72;
}
.markdown-page-content h1,
.markdown-page-content h2,
.markdown-page-content h3,
.markdown-page-content h4 {
  color: var(--promo-text);
  font-family: var(--promo-font-display);
  letter-spacing: 0;
  scroll-margin-top: 1rem;
}
.markdown-page-content h1 { margin: 2rem 0 1rem; padding-bottom: 0.6rem; border-bottom: 3px solid var(--promo-border); font-size: 1.875rem; font-weight: 900; }
.markdown-page-content h2 { margin: 1.6rem 0 0.75rem; font-size: 1.5rem; font-weight: 900; }
.markdown-page-content h3 { margin: 1.25rem 0 0.5rem; font-size: 1.25rem; font-weight: 800; }
.markdown-page-content h4 { margin: 1rem 0 0.5rem; font-size: 1.125rem; font-weight: 800; }
.markdown-page-content p { margin-bottom: 1rem; }
.markdown-page-content ul { margin-bottom: 1rem; padding-left: 1.5rem; list-style: disc; }
.markdown-page-content ol { margin-bottom: 1rem; padding-left: 1.5rem; list-style: decimal; }
.markdown-page-content li { margin-bottom: 0.25rem; }
.markdown-page-content a { color: var(--promo-red); font-weight: 700; text-decoration: underline; }
.markdown-page-content a:hover { color: var(--promo-red-strong); }
.markdown-page-content blockquote { margin-block: 1rem; padding: 0.8rem 1rem; border-left: 6px solid var(--promo-cyan-strong); background: var(--promo-surface-muted); color: var(--promo-text-muted); }
.markdown-page-content img { max-width: 100%; height: auto; margin-block: 1rem; border: 2px solid var(--promo-border); border-radius: var(--promo-radius-sm); }
.markdown-page-content table { display: block; width: 100%; margin-block: 1rem; overflow-x: auto; border-collapse: collapse; }
.markdown-page-content th { padding: 0.6rem 0.75rem; border: 1px solid var(--promo-border); background: var(--promo-yellow); color: var(--promo-black); font-weight: 800; text-align: left; }
.markdown-page-content td { padding: 0.6rem 0.75rem; border: 1px solid var(--promo-border-soft); }
.markdown-page-content code { padding: 0.12rem 0.35rem; border-radius: var(--promo-radius-xs); background: var(--promo-surface-muted); font-family: var(--promo-font-data); font-size: 0.875rem; }
.markdown-page-content pre { position: relative; margin-block: 1rem; overflow-x: auto; padding: 1rem; border: 2px solid var(--promo-border); border-radius: var(--promo-radius-sm); background: #111111; box-shadow: var(--promo-shadow-xs); color: #f5f0e6; }
.markdown-page-content pre code { padding: 0; background: transparent; color: inherit; }
.markdown-page-content hr { margin-block: 1.5rem; border: 0; border-top: 2px solid var(--promo-border); }
.markdown-page-content .custom-page-error { padding: 0.8rem 1rem; border: 2px solid var(--promo-danger); background: color-mix(in srgb, var(--promo-danger) 10%, var(--promo-surface)); color: var(--promo-text); font-weight: 800; }

.copy-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  padding: 4px 10px;
  font-size: 12px;
  border: 1px solid #f5f0e6;
  border-radius: var(--promo-radius-xs);
  background: #ffd628;
  color: #111111;
  cursor: pointer;
  opacity: 0.82;
  transition: opacity 0.2s, background 0.2s;
  font-family: inherit;
}
.copy-btn:hover { background: #ffffff; opacity: 1; }
pre:hover .copy-btn { opacity: 1; }
</style>
