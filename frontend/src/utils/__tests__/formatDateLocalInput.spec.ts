import { describe, expect, it, vi } from 'vitest'

import { formatDateLocalInput, getTodayDateRange } from '../format'

describe('formatDateLocalInput', () => {
  it('formats the calendar date in local time', () => {
    const localDate = new Date('2026-07-12T16:30:00Z')
    vi.spyOn(localDate, 'getFullYear').mockReturnValue(2026)
    vi.spyOn(localDate, 'getMonth').mockReturnValue(6)
    vi.spyOn(localDate, 'getDate').mockReturnValue(13)

    expect(formatDateLocalInput(localDate)).toBe('2026-07-13')
  })

  it('returns an empty string for an invalid date', () => {
    expect(formatDateLocalInput(new Date('invalid'))).toBe('')
  })

  it('builds a same-day range in local time', () => {
    expect(getTodayDateRange(new Date(2026, 7, 15, 12, 30))).toEqual({
      start: '2026-08-15',
      end: '2026-08-15',
    })
  })
})
