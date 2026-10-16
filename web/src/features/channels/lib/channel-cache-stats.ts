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
import type { StatusVariant } from '@/components/status-badge'

import { CACHE_HIT_RATE_THRESHOLDS } from '../constants'
import type { ChannelCacheStats, ChannelCacheStatsMap } from '../types'

/**
 * Sum the cache counters of the given channels (one channel, or every channel
 * of a tag row). Returns undefined when none of them has traffic.
 */
export function sumChannelCacheStats(
  stats: ChannelCacheStatsMap | undefined,
  ids: number[]
): ChannelCacheStats | undefined {
  if (!stats) return undefined
  let total: ChannelCacheStats | undefined
  for (const id of ids) {
    const item = stats[String(id)]
    if (!item) continue
    total = {
      request_count: (total?.request_count ?? 0) + item.request_count,
      hit_count: (total?.hit_count ?? 0) + item.hit_count,
      input_tokens: (total?.input_tokens ?? 0) + item.input_tokens,
      cache_read_tokens:
        (total?.cache_read_tokens ?? 0) + item.cache_read_tokens,
      cache_write_tokens:
        (total?.cache_write_tokens ?? 0) + item.cache_write_tokens,
    }
  }
  return total
}

/**
 * Token cache hit rate in percent: cache reads over total input tokens.
 * Returns undefined when there is no input to measure.
 */
export function getCacheHitRate(
  stats: ChannelCacheStats | undefined
): number | undefined {
  if (!stats || stats.input_tokens <= 0) return undefined
  return Math.min((stats.cache_read_tokens / stats.input_tokens) * 100, 100)
}

/** Higher hit rates get stronger colors; a low rate is not an error. */
export function getCacheHitRateVariant(rate: number): StatusVariant {
  if (rate >= CACHE_HIT_RATE_THRESHOLDS.HIGH) return 'success'
  if (rate >= CACHE_HIT_RATE_THRESHOLDS.MEDIUM) return 'info'
  return 'neutral'
}
