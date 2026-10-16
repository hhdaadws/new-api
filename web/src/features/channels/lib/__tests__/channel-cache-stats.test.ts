/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, it } from 'vitest'

import type { ChannelCacheStatsMap } from '../../types'
import {
  getCacheHitRate,
  getCacheHitRateVariant,
  sumChannelCacheStats,
} from '../channel-cache-stats'

const stats: ChannelCacheStatsMap = {
  '1': {
    request_count: 10,
    hit_count: 8,
    input_tokens: 1000,
    cache_read_tokens: 600,
    cache_write_tokens: 50,
  },
  '2': {
    request_count: 2,
    hit_count: 0,
    input_tokens: 600,
    cache_read_tokens: 0,
    cache_write_tokens: 100,
  },
}

describe('sumChannelCacheStats', () => {
  it('returns the counters of a single channel unchanged', () => {
    expect(sumChannelCacheStats(stats, [1])).toEqual(stats['1'])
  })

  it('adds up the channels of a tag row and skips channels without traffic', () => {
    expect(sumChannelCacheStats(stats, [1, 2, 3])).toEqual({
      request_count: 12,
      hit_count: 8,
      input_tokens: 1600,
      cache_read_tokens: 600,
      cache_write_tokens: 150,
    })
  })

  it('returns undefined when no requested channel has traffic', () => {
    expect(sumChannelCacheStats(stats, [3, 4])).toBeUndefined()
  })

  it('returns undefined before the counters load', () => {
    expect(sumChannelCacheStats(undefined, [1])).toBeUndefined()
  })
})

describe('getCacheHitRate', () => {
  it('divides cache reads by total input tokens in percent', () => {
    expect(getCacheHitRate(stats['1'])).toBe(60)
  })

  it('returns undefined without input tokens to measure', () => {
    expect(getCacheHitRate({ ...stats['2'], input_tokens: 0 })).toBeUndefined()
    expect(getCacheHitRate(undefined)).toBeUndefined()
  })

  it('caps inconsistent upstream counts at 100 percent', () => {
    expect(getCacheHitRate({ ...stats['1'], cache_read_tokens: 1200 })).toBe(
      100
    )
  })
})

describe('getCacheHitRateVariant', () => {
  it.each([
    [80, 'success'],
    [50, 'success'],
    [49.9, 'info'],
    [20, 'info'],
    [19.9, 'neutral'],
    [0, 'neutral'],
  ] as const)('maps %s%% to %s', (rate, variant) => {
    expect(getCacheHitRateVariant(rate)).toBe(variant)
  })
})
