import { describe, expect, it } from 'vitest'
import {
  CONCRETE_PLATFORM_OPTIONS,
  GROUP_PLATFORM_OPTIONS,
  orderPresentPlatforms
} from '@/constants/platforms'

const concretePlatforms = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  'kimi',
  'zhipu',
  'deepseek',
  'minimax',
  'opencode_go'
]

describe('platform option catalogs', () => {
  it('exposes every concrete account platform', () => {
    expect(CONCRETE_PLATFORM_OPTIONS.map((option) => option.value)).toEqual(concretePlatforms)
  })

  it('adds composite for group-backed filters', () => {
    expect(GROUP_PLATFORM_OPTIONS.map((option) => option.value)).toEqual([
      ...concretePlatforms,
      'composite'
    ])
  })

  it('orders present platforms from the catalog and keeps unknown types', () => {
    expect(orderPresentPlatforms(['minimax', 'openai', 'opencode_go', 'future_vendor', 'kimi'])).toEqual([
      'openai',
      'kimi',
      'minimax',
      'opencode_go',
      'future_vendor'
    ])
  })
})
