import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'
import fr from '../locales/fr'
import ru from '../locales/ru'

describe('usage ipGeo locale keys', () => {
  it('contains zh labels for IP geolocation UI', () => {
    expect(zh.usage.ipGeo.fetch).toBe('获取地区')
    expect(zh.usage.ipGeo.fetching).toBe('获取中...')
    expect(zh.usage.ipGeo.failed).toBe('获取失败')
    expect(zh.usage.ipGeo.private).toBe('内网地址')
    expect(zh.usage.ipGeo.batchFetch).toBe('批量获取地区')
    expect(zh.usage.ipGeo.pending).toBe('{count} 个 IP 待获取地区')
  })

  it('contains en labels for IP geolocation UI', () => {
    expect(en.usage.ipGeo.fetch).toBe('Fetch region')
    expect(en.usage.ipGeo.fetching).toBe('Fetching...')
    expect(en.usage.ipGeo.failed).toBe('Failed')
    expect(en.usage.ipGeo.private).toBe('Private address')
    expect(en.usage.ipGeo.batchFetch).toBe('Batch fetch regions')
    expect(en.usage.ipGeo.pending).toBe('{count} IPs pending')
  })

  it('contains fr labels for IP geolocation UI', () => {
    expect(fr.usage.ipGeo.fetch).toBe('Récupérer la région')
    expect(fr.usage.ipGeo.fetching).toBe('Récupération...')
    expect(fr.usage.ipGeo.failed).toBe('Échec')
    expect(fr.usage.ipGeo.private).toBe('Adresse privée')
    expect(fr.usage.ipGeo.batchFetch).toBe('Récupérer les régions en lot')
    expect(fr.usage.ipGeo.pending).toBe('{count} IP en attente')
  })

  it('contains ru labels for IP geolocation UI', () => {
    expect(ru.usage.ipGeo.fetch).toBe('Получить регион')
    expect(ru.usage.ipGeo.fetching).toBe('Получение...')
    expect(ru.usage.ipGeo.failed).toBe('Ошибка')
    expect(ru.usage.ipGeo.private).toBe('Частный адрес')
    expect(ru.usage.ipGeo.batchFetch).toBe('Пакетно получить регионы')
    expect(ru.usage.ipGeo.pending).toBe('Ожидают {count} IP')
  })
})
