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
import { useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormLabel,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { searchChannels } from '@/features/channels/api'
import { getGroups } from '@/features/users/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import type { ErrorKeywordRetrySettings } from './defaults'
import {
  parseErrorKeywordRetryRules,
  type ErrorKeywordRetryRule,
} from './error-keyword-retry'
import { ErrorKeywordRetryRuleDialog } from './error-keyword-retry-rule-dialog'
import { ErrorKeywordRetryRulesTable } from './error-keyword-retry-rules-table'

// The channel search endpoint pages in memory, so one page lists every
// channel a rule can name.
const CHANNEL_PAGE_SIZE = 10000

type ErrorKeywordRetrySectionProps = {
  defaultValues: ErrorKeywordRetrySettings
}

type ErrorKeywordRetryFormValues = {
  enabled: boolean
  rules: ErrorKeywordRetryRule[]
}

type EditingRule = { index: number; rule: ErrorKeywordRetryRule | null }

export function ErrorKeywordRetrySection(props: ErrorKeywordRetrySectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef(props.defaultValues)
  const [editing, setEditing] = useState<EditingRule | null>(null)

  const formDefaults = useMemo<ErrorKeywordRetryFormValues>(
    () => ({
      enabled: props.defaultValues['error_keyword_retry.enabled'],
      rules: parseErrorKeywordRetryRules(
        props.defaultValues['error_keyword_retry.rules']
      ),
    }),
    [props.defaultValues]
  )
  const form = useForm<ErrorKeywordRetryFormValues>({
    defaultValues: formDefaults,
  })
  useResetForm(form, formDefaults)
  useEffect(() => {
    baselineRef.current = props.defaultValues
  }, [props.defaultValues])

  const groupsQuery = useQuery({
    queryKey: ['groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
  })
  const channelsQuery = useQuery({
    queryKey: ['error-keyword-retry', 'channels'],
    queryFn: async () =>
      requireServerSuccess(
        await searchChannels({ keyword: '', page_size: CHANNEL_PAGE_SIZE })
      ).data?.items ?? [],
    // The section reports the failure inline next to the rules.
    meta: { errorToast: false },
  })
  const channels = useMemo(
    () =>
      [...(channelsQuery.data ?? [])].sort((left, right) => left.id - right.id),
    [channelsQuery.data]
  )
  const channelsById = useMemo(
    () => new Map(channels.map((channel) => [channel.id, channel])),
    [channels]
  )

  const rules = useWatch({ control: form.control, name: 'rules' })
  // Groups of saved rules stay selectable after the group is removed.
  const groups = useMemo(
    () =>
      [
        ...new Set([
          ...(groupsQuery.data?.data ?? []),
          ...rules.map((rule) => rule.group),
        ]),
      ].sort((left, right) => left.localeCompare(right)),
    [groupsQuery.data, rules]
  )

  const setRules = (next: ErrorKeywordRetryRule[]) => {
    form.setValue('rules', next, { shouldDirty: true })
  }

  const onSubmit = async (values: ErrorKeywordRetryFormValues) => {
    // Rules are saved before the switch so a rejected rule list never turns
    // the feature on.
    const next: ErrorKeywordRetrySettings = {
      'error_keyword_retry.rules': JSON.stringify(values.rules),
      'error_keyword_retry.enabled': values.enabled,
    }
    const baseline: ErrorKeywordRetrySettings = {
      'error_keyword_retry.rules': JSON.stringify(
        parseErrorKeywordRetryRules(
          baselineRef.current['error_keyword_retry.rules']
        )
      ),
      'error_keyword_retry.enabled':
        baselineRef.current['error_keyword_retry.enabled'],
    }
    const updates = (
      Object.keys(next) as Array<keyof ErrorKeywordRetrySettings>
    ).filter((key) => next[key] !== baseline[key])

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    try {
      for (const key of updates) {
        await updateOption.mutateAsync({ key, value: next[key] })
      }
    } catch {
      // useUpdateOption already reported the failure; keep the edits.
      return
    }
    baselineRef.current = next
  }

  return (
    <SettingsSection title={t('Error keyword retry')}>
      <div className='text-muted-foreground space-y-1 text-sm'>
        <p>{t('Source: global settings. Changes take effect after saving.')}</p>
        <p>
          {t(
            'Rules are checked in order. The retry channel gets one extra attempt even when the retry budget is used up or the status code is not retried; if it fails too, its error is returned.'
          )}
        </p>
      </div>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={form.formState.isSubmitting}
          />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Retry matching errors on a chosen channel')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Errors marked not retryable, pinned channels and strict sessions are never redirected.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <section
            aria-label={t('Retry rules')}
            className='flex min-w-0 flex-col gap-3'
          >
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <div className='flex items-center gap-2'>
                <h4 className='text-sm font-medium'>{t('Retry rules')}</h4>
                <Badge variant='secondary' className='tabular-nums'>
                  {rules.length}
                </Badge>
              </div>
              <Button
                type='button'
                variant='outline'
                onClick={() => setEditing({ index: rules.length, rule: null })}
              >
                <Plus aria-hidden='true' data-icon='inline-start' />
                {t('Add retry rule')}
              </Button>
            </div>
            {channelsQuery.isError ? (
              <p role='alert' className='text-destructive text-sm'>
                {t('Failed to load channels')}
              </p>
            ) : null}
            <ErrorKeywordRetryRulesTable
              rules={rules}
              channelsById={channelsById}
              onEdit={(index) => setEditing({ index, rule: rules[index] })}
              onDelete={(index) =>
                setRules(rules.filter((_, item) => item !== index))
              }
            />
          </section>
        </SettingsForm>
      </Form>

      <ErrorKeywordRetryRuleDialog
        open={editing !== null}
        onOpenChange={(open) => {
          if (!open) setEditing(null)
        }}
        rule={editing?.rule ?? null}
        groups={groups}
        channels={channels}
        onSave={(rule) => {
          if (!editing) return
          const next = [...rules]
          next[editing.index] = rule
          setRules(next)
        }}
      />
    </SettingsSection>
  )
}
