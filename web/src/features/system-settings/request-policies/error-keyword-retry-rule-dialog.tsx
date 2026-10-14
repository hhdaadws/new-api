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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useMemo } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import type { Channel } from '@/features/channels/types'

import { parseKeywordLines } from './error-interception'
import {
  createErrorKeywordRetryRuleSchema,
  type ErrorKeywordRetryRule,
  type ErrorKeywordRetryRuleFormValues,
} from './error-keyword-retry'

const RULE_FORM_ID = 'error-keyword-retry-rule-form'

const EMPTY_RULE_VALUES: ErrorKeywordRetryRuleFormValues = {
  group: '',
  channelId: '',
  keywords: '',
  retryChannelId: '',
}

function channelInGroup(channel: Channel, group: string): boolean {
  return channel.group.split(',').some((name) => name.trim() === group)
}

type ErrorKeywordRetryRuleDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The rule being edited, or null to add a rule. */
  rule: ErrorKeywordRetryRule | null
  groups: string[]
  channels: Channel[]
  onSave: (rule: ErrorKeywordRetryRule) => void
}

export function ErrorKeywordRetryRuleDialog(
  props: ErrorKeywordRetryRuleDialogProps
) {
  const { t } = useTranslation()
  const schema = useMemo(() => createErrorKeywordRetryRuleSchema(t), [t])
  const form = useForm<ErrorKeywordRetryRuleFormValues>({
    resolver: zodResolver(schema),
    defaultValues: EMPTY_RULE_VALUES,
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(
      props.rule
        ? {
            group: props.rule.group,
            channelId: String(props.rule.channel_id),
            keywords: props.rule.keywords.join('\n'),
            retryChannelId: String(props.rule.retry_channel_id),
          }
        : EMPTY_RULE_VALUES
    )
  }, [props.open, props.rule, form])

  const group = useWatch({ control: form.control, name: 'group' })
  const channelId = useWatch({ control: form.control, name: 'channelId' })
  // Only a channel of the group can fail in that group; the retry channel
  // may be any other channel that serves the requested model.
  const channelOptions = useMemo(
    () =>
      props.channels
        .filter((channel) => channelInGroup(channel, group))
        .map((channel) => ({
          value: String(channel.id),
          label: `#${channel.id} ${channel.name}`,
        })),
    [props.channels, group]
  )
  const retryChannelOptions = useMemo(
    () =>
      props.channels
        .filter((channel) => String(channel.id) !== channelId)
        .map((channel) => ({
          value: String(channel.id),
          label: `#${channel.id} ${channel.name}`,
        })),
    [props.channels, channelId]
  )

  const onSubmit = (values: ErrorKeywordRetryRuleFormValues) => {
    props.onSave({
      group: values.group,
      channel_id: Number(values.channelId),
      keywords: parseKeywordLines(values.keywords),
      retry_channel_id: Number(values.retryChannelId),
    })
    props.onOpenChange(false)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.rule ? t('Edit retry rule') : t('Add retry rule')}
      description={t(
        'When the channel fails in the group with an error containing a keyword, the next attempt runs on the retry channel.'
      )}
      contentClassName='sm:max-w-xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={RULE_FORM_ID}>
            {t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={RULE_FORM_ID}
          className='space-y-4'
          onSubmit={form.handleSubmit(onSubmit)}
        >
          <FormField
            control={form.control}
            name='group'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Group')}</FormLabel>
                <Select
                  items={props.groups.map((name) => ({
                    value: name,
                    label: name,
                  }))}
                  value={field.value || null}
                  onValueChange={(value) => {
                    const next = value ?? ''
                    field.onChange(next)
                    const channel = props.channels.find(
                      (item) => String(item.id) === form.getValues('channelId')
                    )
                    if (channel && !channelInGroup(channel, next)) {
                      form.setValue('channelId', '')
                    }
                  }}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue placeholder={t('Select a group')} />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {props.groups.map((name) => (
                        <SelectItem key={name} value={name}>
                          {name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='channelId'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Channel')}</FormLabel>
                <FormControl>
                  <Combobox
                    options={channelOptions}
                    value={field.value}
                    onValueChange={(value) => field.onChange(value ?? '')}
                    onBlur={field.onBlur}
                    name={field.name}
                    disabled={!group}
                    searchPlaceholder={
                      group ? t('Search channels') : t('Select a group first')
                    }
                    emptyText={t('No channels found')}
                  />
                </FormControl>
                <FormDescription>
                  {t('Channels that belong to the group.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='keywords'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Error keywords')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={4}
                    placeholder={t('one keyword per line')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'One keyword per line, case insensitive. Matched against the error text shown in the error logs and the error code.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='retryChannelId'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Retry channel')}</FormLabel>
                <FormControl>
                  <Combobox
                    options={retryChannelOptions}
                    value={field.value}
                    onValueChange={(value) => field.onChange(value ?? '')}
                    onBlur={field.onBlur}
                    name={field.name}
                    searchPlaceholder={t('Search channels')}
                    emptyText={t('No channels found')}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Any enabled channel that serves the requested model, including channels outside the group.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}
