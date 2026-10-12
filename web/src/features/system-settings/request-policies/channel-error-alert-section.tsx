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
import { useMutation } from '@tanstack/react-query'
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import type { UpdateOptionRequest } from '../types'
import { safeNumberFieldProps } from '../utils/numeric-field'
import { sendChannelErrorAlertTest } from './api'
import {
  createChannelErrorAlertSchema,
  MAX_ALERT_COOLDOWN_MINUTES,
  parsePushPlusTokens,
  type ChannelErrorAlertFormInput,
  type ChannelErrorAlertFormValues,
} from './channel-error-alert'
import type { ChannelErrorAlertSettings } from './defaults'

const TOKENS_KEY = 'channel_error_alert.pushplus_tokens'

type ChannelErrorAlertSectionProps = {
  defaultValues: ChannelErrorAlertSettings
}

export function ChannelErrorAlertSection(props: ChannelErrorAlertSectionProps) {
  const { t } = useTranslation()
  const isRoot = useAuthStore(
    (state) => state.auth.user?.role === ROLE.SUPER_ADMIN
  )
  const updateOption = useUpdateOption()
  const schema = useMemo(() => createChannelErrorAlertSchema(t), [t])
  const baselineRef = useRef(props.defaultValues)

  const formDefaults = useMemo<ChannelErrorAlertFormInput>(
    () => ({
      enabled: props.defaultValues['channel_error_alert.enabled'],
      threshold: props.defaultValues['channel_error_alert.threshold'],
      cooldownMinutes:
        props.defaultValues['channel_error_alert.cooldown_minutes'],
      tokens: '',
    }),
    [props.defaultValues]
  )

  const form = useForm<
    ChannelErrorAlertFormInput,
    unknown,
    ChannelErrorAlertFormValues
  >({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)
  useEffect(() => {
    baselineRef.current = props.defaultValues
  }, [props.defaultValues])

  const testAlert = useMutation({
    mutationFn: sendChannelErrorAlertTest,
    onSuccess: (results) => {
      const failed = results.filter((result) => !result.success)
      if (failed.length === 0) {
        toast.success(
          t('Test notification sent to {{count}} recipients', {
            count: results.length,
          })
        )
        return
      }
      toast.error(
        t('Test notification failed for {{failed}} of {{total}} recipients', {
          failed: failed.length,
          total: results.length,
        }),
        {
          description: (
            <ul className='space-y-1'>
              {failed.map((result) => (
                <li key={`${result.token}:${result.error}`}>
                  {result.token}: {result.error}
                </li>
              ))}
            </ul>
          ),
        }
      )
    },
    onError: (error) => handleServerError(error),
    meta: { errorToast: false },
  })

  const onSubmit = async (values: ChannelErrorAlertFormValues) => {
    const next: ChannelErrorAlertSettings = {
      'channel_error_alert.enabled': values.enabled,
      'channel_error_alert.threshold': values.threshold,
      'channel_error_alert.cooldown_minutes': values.cooldownMinutes,
    }
    const updates: UpdateOptionRequest[] = (
      Object.keys(next) as Array<keyof ChannelErrorAlertSettings>
    )
      .filter((key) => next[key] !== baselineRef.current[key])
      .map((key) => ({ key, value: next[key] }))
    const tokens = parsePushPlusTokens(values.tokens)
    if (tokens.length > 0) {
      updates.push({ key: TOKENS_KEY, value: JSON.stringify(tokens) })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    try {
      for (const update of updates) {
        await updateOption.mutateAsync(update)
      }
    } catch {
      // useUpdateOption already reported the failure; keep the edits.
      return
    }
    baselineRef.current = next
    if (tokens.length > 0) {
      form.setValue('tokens', '')
    }
  }

  return (
    <SettingsSection title={t('Channel error alerts')}>
      <div className='text-muted-foreground space-y-1 text-sm'>
        <p>{t('Source: global settings. Changes take effect after saving.')}</p>
        <p>
          {t(
            'Alerts are sent through PushPlus. Each token receives every alert, so add one token per person to notify several people.'
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
                  <FormLabel>{t('Alert on frequent channel errors')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Send a PushPlus message when one channel reports more errors within a minute than the threshold.'
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

          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='threshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Errors per minute threshold')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'An alert is sent when a channel has more errors than this within the same minute.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='cooldownMinutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Alert cooldown (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={MAX_ALERT_COOLDOWN_MINUTES}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'After an alert, the same channel is not alerted again for this many minutes. 0 alerts at most once per minute.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='tokens'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('PushPlus tokens')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={4}
                      autoComplete='off'
                      disabled={!isRoot}
                      placeholder={t('one token per line')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {isRoot
                      ? t(
                          'Saved tokens are hidden. Leave blank to keep them; entering tokens replaces the whole saved list.'
                        )
                      : t('Only the root user can change PushPlus tokens.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>

          {isRoot && (
            <div className='flex flex-wrap items-center gap-3'>
              <Button
                type='button'
                variant='outline'
                disabled={testAlert.isPending}
                onClick={() => testAlert.mutate()}
              >
                {t('Send test notification')}
              </Button>
              <p className='text-muted-foreground text-sm'>
                {t('Sends a test message to every saved token.')}
              </p>
            </div>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
