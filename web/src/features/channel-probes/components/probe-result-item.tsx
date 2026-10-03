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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { StatusBadge } from '@/components/status-badge'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { ChannelProbeAnomaly, ChannelProbeResult } from '../types'

// i18n keys of each anomaly, passed through t() when rendered.
const ANOMALY_LABELS: Record<ChannelProbeAnomaly, string> = {
  input_tokens: 'Input tokens differ from baseline',
  content: 'Output differs from baseline',
  signature_accepted: 'Upstream accepted a tampered signature',
  signature_missing: 'No signed thinking block returned',
  signature_unexpected: 'Unexpected rejection of a tampered signature',
}

export function ProbeResultItem(props: { result: ChannelProbeResult }) {
  const { t } = useTranslation()
  const result = props.result
  const anomalous = result.anomalies.length > 0
  return (
    <section
      aria-label={formatTimestampToDate(result.created_at)}
      className={cn(
        'space-y-2 rounded-md border p-3',
        anomalous && 'border-warning/40 bg-warning/10'
      )}
    >
      <div className='flex flex-wrap items-center gap-2 text-sm'>
        <StatusBadge
          label={result.success ? t('Success') : t('Failed')}
          variant={result.success ? 'success' : 'danger'}
          copyable={false}
        />
        {result.model && (
          <StatusBadge
            label={result.model}
            variant='neutral'
            copyable={false}
          />
        )}
        {result.anomalies.map((anomaly) => (
          <StatusBadge
            key={anomaly}
            label={t(ANOMALY_LABELS[anomaly])}
            variant='warning'
            copyable={false}
          />
        ))}
        <span className='text-muted-foreground'>
          {formatTimestampToDate(result.created_at)}
        </span>
        {result.status_code > 0 && (
          <span className='text-muted-foreground'>
            {t('HTTP {{code}}', { code: result.status_code })}
          </span>
        )}
        <span className='text-muted-foreground'>
          {t('{{ms}} ms', { ms: result.latency_ms })}
        </span>
        {result.input_tokens > 0 && (
          <span className='text-muted-foreground'>
            {t('Input {{count}} tokens', { count: result.input_tokens })}
          </span>
        )}
        {result.output_tokens > 0 && (
          <span className='text-muted-foreground'>
            {t('Output {{count}} tokens', { count: result.output_tokens })}
          </span>
        )}
      </div>
      {result.error && (
        <p className='text-destructive text-sm break-all'>{result.error}</p>
      )}
      {result.success && (
        <div className='relative'>
          <CopyButton
            value={result.content}
            size='icon'
            variant='ghost'
            className='absolute top-1 right-1 size-7'
            tooltip={t('Copy output')}
            aria-label={t('Copy output')}
          />
          <pre
            aria-label={t('Output')}
            className='bg-muted/40 max-h-48 overflow-y-auto rounded border p-2 pr-9 text-sm leading-relaxed wrap-break-word whitespace-pre-wrap'
          >
            {result.content || t('(empty output)')}
          </pre>
        </div>
      )}
      {result.response && (
        <Collapsible>
          <CollapsibleTrigger className='text-muted-foreground cursor-pointer text-xs font-medium'>
            {t('Raw response')}
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div className='relative mt-2'>
              <CopyButton
                value={result.response}
                size='icon'
                variant='ghost'
                className='absolute top-1 right-1 size-7'
                tooltip={t('Copy response')}
                aria-label={t('Copy response')}
              />
              <pre className='bg-muted/40 max-h-64 overflow-y-auto rounded border p-2 pr-9 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap'>
                {result.response}
              </pre>
            </div>
          </CollapsibleContent>
        </Collapsible>
      )}
    </section>
  )
}
