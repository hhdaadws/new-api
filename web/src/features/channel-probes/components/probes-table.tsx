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
import { History, Play } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  BadgeCell,
  StaticDataTable,
  StaticRowActions,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { formatTimestampToDate } from '@/lib/format'

import {
  useDeleteChannelProbe,
  useRunChannelProbe,
} from '../hooks/use-channel-probes'
import type { ChannelProbe } from '../types'
import { ProbeResultsDialog } from './probe-results-dialog'

type ProbesTableProps = {
  probes: ChannelProbe[]
  onEdit: (probe: ChannelProbe) => void
}

export function ProbesTable(props: ProbesTableProps) {
  const { t } = useTranslation()
  const deleteProbe = useDeleteChannelProbe()
  const runProbe = useRunChannelProbe()
  const [deleteTarget, setDeleteTarget] = useState<ChannelProbe | null>(null)
  const [resultsTarget, setResultsTarget] = useState<ChannelProbe | null>(null)

  const endpointLabels: Record<string, string> = {
    openai: t('OpenAI Chat Completions'),
    anthropic: t('Anthropic Messages'),
    'openai-response': t('OpenAI Responses'),
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    await deleteProbe.mutateAsync(deleteTarget.id)
    setDeleteTarget(null)
  }

  return (
    <>
      <StaticDataTable
        data={props.probes}
        getRowKey={(probe) => probe.id}
        emptyClassName='text-sm'
        emptyContent={t(
          'No probes yet. Add one to start monitoring a channel.'
        )}
        columns={[
          {
            id: 'name',
            header: t('Name'),
            cellClassName: 'font-medium',
            cell: (probe) => probe.name,
          },
          {
            id: 'channel',
            header: t('Channel'),
            cell: (probe) =>
              `#${probe.channel_id} ${probe.channel_name || t('Deleted channel')}`,
          },
          {
            id: 'endpoint',
            header: t('Request format'),
            cell: (probe) =>
              endpointLabels[probe.endpoint_type] ?? probe.endpoint_type,
          },
          {
            id: 'interval',
            header: t('Interval'),
            cell: (probe) =>
              t('{{seconds}}s', { seconds: probe.interval_seconds }),
          },
          {
            id: 'status',
            header: t('Status'),
            cell: (probe) => (
              <BadgeCell>
                <StatusBadge
                  label={probe.enabled ? t('Enabled') : t('Disabled')}
                  variant={probe.enabled ? 'success' : 'neutral'}
                  copyable={false}
                />
              </BadgeCell>
            ),
          },
          {
            id: 'last-run',
            header: t('Last run'),
            cell: (probe) =>
              probe.last_run_at > 0 ? (
                <BadgeCell>
                  <StatusBadge
                    label={probe.last_success ? t('Success') : t('Failed')}
                    variant={probe.last_success ? 'success' : 'danger'}
                    copyable={false}
                    title={probe.last_error || undefined}
                  />
                  <span className='text-muted-foreground text-xs'>
                    {formatTimestampToDate(probe.last_run_at)}
                  </span>
                </BadgeCell>
              ) : (
                <span className='text-muted-foreground'>{t('Never')}</span>
              ),
          },
          {
            id: 'next-run',
            header: t('Next run'),
            cellClassName: 'text-muted-foreground',
            cell: (probe) =>
              probe.enabled ? formatTimestampToDate(probe.next_run_at) : '-',
          },
          {
            id: 'actions',
            header: t('Actions'),
            className: 'text-right',
            cellClassName: 'text-right',
            cell: (probe) => (
              <div className='flex justify-end gap-1'>
                <Button
                  variant='ghost'
                  size='icon-sm'
                  aria-label={t('Run now')}
                  title={t('Run now')}
                  disabled={runProbe.isPending}
                  onClick={() => runProbe.mutate(probe.id)}
                >
                  <Play />
                </Button>
                <Button
                  variant='ghost'
                  size='icon-sm'
                  aria-label={t('View results')}
                  title={t('View results')}
                  onClick={() => setResultsTarget(probe)}
                >
                  <History />
                </Button>
                <StaticRowActions
                  editLabel={t('Edit')}
                  deleteLabel={t('Delete')}
                  menuLabel={t('Open menu')}
                  onEdit={() => props.onEdit(probe)}
                  onDelete={() => setDeleteTarget(probe)}
                />
              </div>
            ),
          },
        ]}
      />

      {resultsTarget && (
        <ProbeResultsDialog
          key={resultsTarget.id}
          probe={resultsTarget}
          onOpenChange={(open) => !open && setResultsTarget(null)}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('Delete Probe')}
        desc={t(
          'Delete probe "{{name}}" and all of its recorded results? This cannot be undone.',
          { name: deleteTarget?.name || '' }
        )}
        confirmText={t('Delete')}
        destructive
        handleConfirm={handleDelete}
        isLoading={deleteProbe.isPending}
      />
    </>
  )
}
