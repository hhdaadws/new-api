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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { DataTablePagination, useDataTable } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { StatusBadge } from '@/components/status-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'

import { useChannelProbeResults } from '../hooks/use-channel-probes'
import { PROBE_RESULT_KEEP } from '../lib/probe-form'
import type { ChannelProbe, ChannelProbeResult } from '../types'

type ProbeResultsDialogProps = {
  probe: ChannelProbe
  onOpenChange: (open: boolean) => void
}

const EMPTY_RESULTS: ChannelProbeResult[] = []

// Mount one dialog per probe so pagination starts on the newest page.
export function ProbeResultsDialog(props: ProbeResultsDialogProps) {
  const { t } = useTranslation()
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 10 })
  const resultsQuery = useChannelProbeResults(
    props.probe.id,
    pagination.pageIndex + 1,
    pagination.pageSize
  )
  const results = resultsQuery.data?.items ?? EMPTY_RESULTS
  const { table } = useDataTable({
    data: results,
    columns: [],
    totalCount: resultsQuery.data?.total ?? 0,
    manualPagination: true,
    columnFilters: [],
    pagination,
    onPaginationChange: setPagination,
    columnVisibilityStorageKey: false,
    columnSizingStorageKey: false,
  })

  return (
    <Dialog
      open
      onOpenChange={props.onOpenChange}
      title={t('Probe results: {{name}}', { name: props.probe.name })}
      description={t('Newest first. The latest {{count}} results are kept.', {
        count: PROBE_RESULT_KEEP,
      })}
      contentClassName='max-h-[min(85dvh,var(--dialog-available-height))] overflow-y-auto sm:max-w-3xl'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      <div aria-busy={resultsQuery.isFetching} className='space-y-3'>
        {resultsQuery.isLoading && <Skeleton className='h-24 w-full' />}
        {resultsQuery.isError && (
          <ErrorState
            title={t('We could not load probe results.')}
            description={resultsQuery.error.message}
            onRetry={() => void resultsQuery.refetch()}
          />
        )}
        {!resultsQuery.isLoading &&
          !resultsQuery.isError &&
          results.length === 0 && (
            <EmptyState
              title={t('This probe has not run yet.')}
              className='min-h-32'
              bordered
            />
          )}
        {results.map((result) => (
          <ProbeResultItem key={result.id} result={result} />
        ))}
        {results.length > 0 && <DataTablePagination table={table} compact />}
      </div>
    </Dialog>
  )
}

function ProbeResultItem(props: { result: ChannelProbeResult }) {
  const { t } = useTranslation()
  const result = props.result
  return (
    <section
      aria-label={formatTimestampToDate(result.created_at)}
      className='space-y-2 rounded-md border p-3'
    >
      <div className='flex flex-wrap items-center gap-2 text-sm'>
        <StatusBadge
          label={result.success ? t('Success') : t('Failed')}
          variant={result.success ? 'success' : 'danger'}
          copyable={false}
        />
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
      </div>
      {result.error && (
        <p className='text-destructive text-sm break-all'>{result.error}</p>
      )}
      {result.response && (
        <div className='relative'>
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
      )}
    </section>
  )
}
