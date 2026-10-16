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
import { useContext, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatPercent } from '@/lib/format'

import { CHANNEL_CACHE_STATS_HOURS } from '../constants'
import {
  getCacheHitRate,
  getCacheHitRateVariant,
  isTagAggregateRow,
  sumChannelCacheStats,
} from '../lib'
import type { Channel } from '../types'
import { ChannelCacheStatsContext } from './channel-cache-stats-context'

/**
 * Token cache hit rate of a channel, or of every channel in a tag row, over
 * the last CHANNEL_CACHE_STATS_HOURS hours. The tooltip breaks it down.
 */
export function ChannelCacheHitCell(props: { channel: Channel }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const cacheStats = useContext(ChannelCacheStatsContext)

  const stats = useMemo(() => {
    const ids = isTagAggregateRow(props.channel)
      ? props.channel.children.map((child) => child.id)
      : [props.channel.id]
    return sumChannelCacheStats(cacheStats.stats, ids)
  }, [cacheStats.stats, props.channel])
  const rate = getCacheHitRate(stats)

  if (cacheStats.isLoading) {
    return <Skeleton className='h-5 w-14' />
  }
  if (!stats || rate === undefined) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  const requestHitRate =
    stats.request_count > 0 ? (stats.hit_count / stats.request_count) * 100 : 0
  const details = [
    { label: t('Token hit rate'), value: formatPercent(rate) },
    {
      label: t('Request hit rate'),
      value: `${formatPercent(requestHitRate)} (${formatNumber(stats.hit_count, locale)} / ${formatNumber(stats.request_count, locale)})`,
    },
    {
      label: t('Input Tokens'),
      value: formatNumber(stats.input_tokens, locale),
    },
    {
      label: t('Cache Read'),
      value: formatNumber(stats.cache_read_tokens, locale),
    },
    {
      label: t('Cache Write'),
      value: formatNumber(stats.cache_write_tokens, locale),
    },
  ]

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger
          render={
            <StatusBadge
              label={formatPercent(rate)}
              variant={getCacheHitRateVariant(rate)}
              size='sm'
              copyable={false}
              tabIndex={0}
              className='-ml-1.5 cursor-default tabular-nums'
            />
          }
        />
        <TooltipContent side='top' className='flex-col items-stretch gap-1'>
          <p className='font-medium'>
            {t('Last {{hours}} hours', { hours: CHANNEL_CACHE_STATS_HOURS })}
          </p>
          <dl className='grid grid-cols-[auto_auto] gap-x-4 gap-y-0.5 text-xs'>
            {details.map((item) => (
              <div key={item.label} className='contents'>
                <dt className='opacity-80'>{item.label}</dt>
                <dd className='text-right font-mono tabular-nums'>
                  {item.value}
                </dd>
              </div>
            ))}
          </dl>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
