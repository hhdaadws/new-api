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
import { FileText } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { JsonCodeEditor } from '@/components/json-code-editor'
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
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  useChannelProbeTargets,
  useSaveChannelProbe,
} from '../hooks/use-channel-probes'
import {
  formValuesToPayload,
  getProbeFormSchema,
  PROBE_FORM_DEFAULT_VALUES,
  PROBE_INTERVAL_MAX_SECONDS,
  PROBE_INTERVAL_MIN_SECONDS,
  probeToFormValues,
  SIGNATURE_TAMPER_TEMPLATE,
  ZERO_INJECTION_TEMPLATE,
  type ProbeFormValues,
  type ProbeTemplate,
} from '../lib/probe-form'
import type { ChannelProbe } from '../types'
import { ProbeRequestFields } from './probe-request-fields'

type ProbeFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  probe: ChannelProbe | null
}

const PROBE_FORM_ID = 'channel-probe-form'

export function ProbeFormDialog(props: ProbeFormDialogProps) {
  const { t } = useTranslation()
  const isEditing = props.probe !== null
  const saveProbe = useSaveChannelProbe()
  const targets = useChannelProbeTargets(props.open)
  const schema = useMemo(() => getProbeFormSchema(t), [t])

  const form = useForm<ProbeFormValues>({
    resolver: zodResolver(schema),
    defaultValues: PROBE_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(
      props.probe ? probeToFormValues(props.probe) : PROBE_FORM_DEFAULT_VALUES
    )
  }, [props.open, props.probe, form])

  const channelOptions = useMemo(
    () =>
      (targets.data ?? []).map((target) => ({
        value: String(target.id),
        label: `#${target.id} ${target.name}`,
      })),
    [targets.data]
  )

  const channelId = useWatch({ control: form.control, name: 'channel_id' })
  const probeType = useWatch({ control: form.control, name: 'probe_type' })
  const channelModels = useMemo(
    () =>
      targets.data?.find((target) => String(target.id) === channelId)?.models ??
      [],
    [targets.data, channelId]
  )

  const applyTemplate = (template: ProbeTemplate) => {
    if (form.getValues('name').trim() === '') {
      form.setValue('name', t(template.name), { shouldValidate: true })
    }
    form.setValue('probe_type', template.probe_type)
    form.setValue('endpoint_type', template.endpoint_type)
    form.setValue('headers', template.headers, { shouldValidate: true })
    form.setValue('body', template.body, { shouldValidate: true })
  }

  const onSubmit = async (values: ProbeFormValues) => {
    await saveProbe.mutateAsync({
      id: props.probe?.id,
      data: formValuesToPayload(values),
    })
    props.onOpenChange(false)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={isEditing ? t('Edit Probe') : t('Add Probe')}
      description={t(
        'Send this request to the channel on a schedule and record every response.'
      )}
      contentClassName='max-h-[min(85dvh,var(--dialog-available-height))] overflow-y-auto sm:max-w-2xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={saveProbe.isPending}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form={PROBE_FORM_ID}
            disabled={saveProbe.isPending}
          >
            {saveProbe.isPending ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={PROBE_FORM_ID}
          className='space-y-4'
          onSubmit={(event) => {
            void form
              .handleSubmit(onSubmit)(event)
              .catch(() => {})
          }}
        >
          <div className='flex flex-wrap justify-end gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => applyTemplate(ZERO_INJECTION_TEMPLATE)}
            >
              <FileText aria-hidden='true' />
              {t('Use zero injection template')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => applyTemplate(SIGNATURE_TAMPER_TEMPLATE)}
            >
              <FileText aria-hidden='true' />
              {t('Use signature tamper template')}
            </Button>
          </div>

          <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Probe name')} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='channel_id'
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
                      placeholder={t('Select a channel')}
                      searchPlaceholder={t('Search channels')}
                      emptyText={t('No channels found')}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='interval_seconds'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Interval (seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type='number'
                      min={PROBE_INTERVAL_MIN_SECONDS}
                      max={PROBE_INTERVAL_MAX_SECONDS}
                      value={Number.isNaN(field.value) ? '' : field.value}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Minimum {{min}} seconds', {
                      min: PROBE_INTERVAL_MIN_SECONDS,
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <ProbeRequestFields form={form} channelModels={channelModels} />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between'>
                <div className='space-y-0.5'>
                  <FormLabel>{t('Enabled')}</FormLabel>
                  <FormDescription>
                    {t('Run this probe automatically on its interval')}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='headers'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Request headers')}</FormLabel>
                <FormControl>
                  <JsonCodeEditor
                    value={field.value}
                    onChange={field.onChange}
                    name={field.name}
                    onBlur={field.onBlur}
                    textareaRef={field.ref}
                    placeholder='{"X-Custom-Header": "value"}'
                    heightClassName='h-24 min-h-24 max-h-24'
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Optional JSON object. Overrides the channel headers with the same name; {api_key} is replaced by the channel key.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='body'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Request body')}</FormLabel>
                <FormControl>
                  <JsonCodeEditor
                    value={field.value}
                    onChange={field.onChange}
                    name={field.name}
                    onBlur={field.onBlur}
                    textareaRef={field.ref}
                    placeholder='{"model": "gpt-4o-mini", "messages": [...]}'
                    heightClassName='h-64 min-h-64 max-h-64'
                  />
                </FormControl>
                <FormDescription>
                  {probeType === 'signature'
                    ? t(
                        'First turn of the check, sent without streaming. Keep thinking enabled so the reply carries a signed thinking block.'
                      )
                    : t(
                        'JSON request in the selected format. Each selected model replaces the model field; set "stream": true to probe streaming.'
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
