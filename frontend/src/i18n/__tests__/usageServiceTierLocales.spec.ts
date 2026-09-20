import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'
import fr from '../locales/fr'
import ru from '../locales/ru'

describe('usage service tier locale keys', () => {
  it('contains zh labels for service tier tooltip', () => {
    expect(zh.usage.serviceTier).toBe('服务档位')
    expect(zh.usage.serviceTierPriority).toBe('Fast')
    expect(zh.usage.serviceTierUltrafast).toBe('Ultrafast')
    expect(zh.usage.serviceTierFlex).toBe('Flex')
    expect(zh.usage.serviceTierStandard).toBe('Standard')
  })

  it('contains en labels for service tier tooltip', () => {
    expect(en.usage.serviceTier).toBe('Service tier')
    expect(en.usage.serviceTierPriority).toBe('Fast')
    expect(en.usage.serviceTierUltrafast).toBe('Ultrafast')
    expect(en.usage.serviceTierFlex).toBe('Flex')
    expect(en.usage.serviceTierStandard).toBe('Standard')
  })

  it('contains fr labels for service tier tooltip', () => {
    expect(fr.usage.serviceTier).toBe('Niveau de service')
    expect(fr.usage.serviceTierPriority).toBe('Fast')
    expect(fr.usage.serviceTierUltrafast).toBe('Ultrafast')
    expect(fr.usage.serviceTierFlex).toBe('Flex')
    expect(fr.usage.serviceTierStandard).toBe('Standard')
  })

  it('contains ru labels for service tier tooltip', () => {
    expect(ru.usage.serviceTier).toBe('Уровень сервиса')
    expect(ru.usage.serviceTierPriority).toBe('Fast')
    expect(ru.usage.serviceTierUltrafast).toBe('Ultrafast')
    expect(ru.usage.serviceTierFlex).toBe('Flex')
    expect(ru.usage.serviceTierStandard).toBe('Standard')
  })
})
