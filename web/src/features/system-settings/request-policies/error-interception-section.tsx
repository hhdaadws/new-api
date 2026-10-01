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
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

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

import {
  SettingsControlChildren,
  SettingsControlGroup,
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'
import type { ErrorInterceptionSettings } from './defaults'
import {
  createErrorInterceptionSchema,
  interceptionKeywordsFromText,
  interceptionKeywordsToText,
  MAX_INTERCEPTION_STATUS_CODE,
  MIN_INTERCEPTION_STATUS_CODE,
  type ErrorInterceptionFormInput,
  type ErrorInterceptionFormValues,
} from './error-interception'

type ErrorInterceptionSectionProps = {
  defaultValues: ErrorInterceptionSettings
}

const normalizeSettings = (
  settings: ErrorInterceptionSettings
): ErrorInterceptionSettings => ({
  ...settings,
  'upstream_error_interception.keywords': interceptionKeywordsFromText(
    interceptionKeywordsToText(settings['upstream_error_interception.keywords'])
  ),
  'upstream_error_interception.message':
    settings['upstream_error_interception.message'].trim(),
})

export function ErrorInterceptionSection(props: ErrorInterceptionSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const schema = useMemo(() => createErrorInterceptionSchema(t), [t])
  const baselineRef = useRef(normalizeSettings(props.defaultValues))

  const formDefaults = useMemo<ErrorInterceptionFormInput>(
    () => ({
      enabled: props.defaultValues['upstream_error_interception.enabled'],
      keywords: interceptionKeywordsToText(
        props.defaultValues['upstream_error_interception.keywords']
      ),
      statusCode:
        props.defaultValues['upstream_error_interception.status_code'],
      message: props.defaultValues['upstream_error_interception.message'],
      retryOnMatch:
        props.defaultValues['upstream_error_interception.retry_on_match'],
    }),
    [props.defaultValues]
  )

  const form = useForm<
    ErrorInterceptionFormInput,
    unknown,
    ErrorInterceptionFormValues
  >({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)
  useEffect(() => {
    baselineRef.current = normalizeSettings(props.defaultValues)
  }, [props.defaultValues])

  const onSubmit = async (values: ErrorInterceptionFormValues) => {
    const next: ErrorInterceptionSettings = {
      'upstream_error_interception.enabled': values.enabled,
      'upstream_error_interception.keywords': interceptionKeywordsFromText(
        values.keywords
      ),
      'upstream_error_interception.status_code': values.statusCode,
      'upstream_error_interception.message': values.message,
      'upstream_error_interception.retry_on_match': values.retryOnMatch,
    }
    const updates = (
      Object.keys(next) as Array<keyof ErrorInterceptionSettings>
    ).filter((key) => next[key] !== baselineRef.current[key])

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
    <SettingsSection title={t('Upstream error interception')}>
      <div className='text-muted-foreground space-y-1 text-sm'>
        <p>{t('Source: global settings. Changes take effect after saving.')}</p>
        <p>
          {t(
            'Only error responses from upstream channels are checked; successful responses and streamed output are never inspected. Administrators can still see the original upstream error in the logs.'
          )}
        </p>
      </div>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={form.formState.isSubmitting}
          />

          <SettingsControlGroup>
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('Intercept matching upstream errors')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'When an upstream error response contains any keyword (case insensitive), clients receive the status code and message below instead.'
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

            <SettingsControlChildren>
              <FormField
                control={form.control}
                name='retryOnMatch'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Keep retrying other channels after a match')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'When off, a matched error is returned immediately without trying other channels.'
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
            </SettingsControlChildren>
          </SettingsControlGroup>

          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='statusCode'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Response status code')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={MIN_INTERCEPTION_STATUS_CODE}
                      max={MAX_INTERCEPTION_STATUS_CODE}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('HTTP status code returned to clients (400-599).')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='message'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Response error message')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Error message returned to clients on a match.')}
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
                      rows={6}
                      placeholder={t('one keyword per line')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'One keyword per line. Blank lines and duplicates are removed when saving.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
