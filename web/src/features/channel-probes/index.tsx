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
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import { ProbeFormDialog } from './components/probe-form-dialog'
import { ProbesTable } from './components/probes-table'
import { useChannelProbes } from './hooks/use-channel-probes'
import type { ChannelProbe } from './types'

export function ChannelProbes() {
  const { t } = useTranslation()
  const probesQuery = useChannelProbes()
  const [formOpen, setFormOpen] = useState(false)
  const [editingProbe, setEditingProbe] = useState<ChannelProbe | null>(null)

  const openForm = (probe: ChannelProbe | null) => {
    setEditingProbe(probe)
    setFormOpen(true)
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Channel Probes')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button size='sm' onClick={() => openForm(null)}>
            <Plus aria-hidden='true' />
            {t('Add Probe')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-3'>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Send custom requests to channels on a schedule and keep the responses, for example to check whether an upstream injects a system prompt.'
              )}
            </p>
            {probesQuery.isLoading && <Skeleton className='h-32 w-full' />}
            {probesQuery.isError && (
              <ErrorState
                title={t('We could not load probes.')}
                description={probesQuery.error.message}
                onRetry={() => void probesQuery.refetch()}
              />
            )}
            {probesQuery.isSuccess && (
              <ProbesTable probes={probesQuery.data} onEdit={openForm} />
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ProbeFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        probe={editingProbe}
      />
    </>
  )
}
