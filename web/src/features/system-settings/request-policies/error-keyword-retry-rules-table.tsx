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

import {
  StaticDataTable,
  StaticRowActions,
  TruncatedCell,
} from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadgeList } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { CHANNEL_STATUS } from '@/features/channels/constants'
import type { Channel } from '@/features/channels/types'

import type { ErrorKeywordRetryRule } from './error-keyword-retry'

type ErrorKeywordRetryRulesTableProps = {
  rules: ErrorKeywordRetryRule[]
  channelsById: Map<number, Channel>
  onEdit: (index: number) => void
  onDelete: (index: number) => void
}

export function ErrorKeywordRetryRulesTable(
  props: ErrorKeywordRetryRulesTableProps
) {
  const { t } = useTranslation()

  // A rule keeps its channel ids when the channel list is unavailable, and a
  // disabled retry channel is shown because the rule is skipped while it is.
  const channelLabel = (id: number) => {
    const channel = props.channelsById.get(id)
    if (!channel) return `#${id}`
    if (channel.status !== CHANNEL_STATUS.ENABLED) {
      return `#${id} ${channel.name} (${t('Disabled')})`
    }
    return `#${id} ${channel.name}`
  }

  return (
    <StaticDataTable
      className='overflow-x-auto'
      tableClassName='min-w-[720px]'
      data={props.rules}
      getRowKey={(_, index) => index}
      emptyContent={t(
        'No retry rules yet. Add a rule to choose where a matching error is retried.'
      )}
      columns={[
        {
          id: 'group',
          header: t('Group'),
          className: 'w-36',
          cell: (rule) => <GroupBadge group={rule.group} />,
        },
        {
          id: 'channel',
          header: t('Channel'),
          cell: (rule) => (
            <TruncatedCell tabIndex={0}>
              {channelLabel(rule.channel_id)}
            </TruncatedCell>
          ),
        },
        {
          id: 'keywords',
          header: t('Error keywords'),
          cell: (rule) => (
            <StatusBadgeList
              items={rule.keywords}
              max={3}
              className='flex-wrap gap-1.5'
              renderItem={(keyword) => (
                <Badge variant='outline'>{keyword}</Badge>
              )}
            />
          ),
        },
        {
          id: 'retry-channel',
          header: t('Retry channel'),
          cell: (rule) => (
            <TruncatedCell tabIndex={0}>
              {channelLabel(rule.retry_channel_id)}
            </TruncatedCell>
          ),
        },
        {
          id: 'actions',
          header: t('Actions'),
          className: 'w-24 text-right',
          cellClassName: 'text-right',
          cell: (_, index) => (
            <StaticRowActions
              editLabel={t('Edit retry rule')}
              deleteLabel={t('Delete')}
              menuLabel={t('More actions')}
              onEdit={() => props.onEdit(index)}
              onDelete={() => props.onDelete(index)}
            />
          ),
        },
      ]}
    />
  )
}
