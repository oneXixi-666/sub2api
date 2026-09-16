import type { AccountPlatform, GroupPlatform } from '@/types'

export interface PlatformOption<T extends string = string> {
  value: T
  label: string
}

/**
 * Concrete upstream platforms supported by accounts and request routing.
 * Keep platform selectors derived from this catalog so newly added providers
 * do not silently disappear from list filters.
 */
export const CONCRETE_PLATFORM_OPTIONS = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'grok', label: 'Grok' },
  { value: 'kimi', label: 'Kimi' },
  { value: 'zhipu', label: 'Zhipu GLM' },
  { value: 'deepseek', label: 'DeepSeek' },
  { value: 'minimax', label: 'MiniMax' },
  { value: 'opencode_go', label: 'OpenCode' }
] as const satisfies readonly PlatformOption<AccountPlatform>[]

/** Platforms that can own a group. */
export const GROUP_PLATFORM_OPTIONS = [
  ...CONCRETE_PLATFORM_OPTIONS,
  { value: 'composite', label: 'Composite' }
] as const satisfies readonly PlatformOption<GroupPlatform>[]

export const GROUP_PLATFORM_VALUES: readonly GroupPlatform[] = GROUP_PLATFORM_OPTIONS.map((option) => option.value)

/**
 * Order platforms that actually appear in a collection by the shared catalog,
 * then append any unknown values so newly added providers (MiniMax, OpenCode,
 * …) show up in filters without another hardcoded list.
 */
export function orderPresentPlatforms(present: Iterable<string>): string[] {
  const seen = new Set<string>()
  for (const value of present) {
    const platform = value.trim()
    if (platform) seen.add(platform)
  }
  const catalog = new Set<string>(GROUP_PLATFORM_VALUES)
  const known = GROUP_PLATFORM_VALUES.filter((platform) => seen.has(platform))
  const unknown = [...seen].filter((platform) => !catalog.has(platform)).sort()
  return [...known, ...unknown]
}
