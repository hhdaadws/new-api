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
import { useWatch, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import {
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

import type { ProbeFormValues } from '../lib/probe-form'

type ProbeRequestFieldsProps = {
  form: UseFormReturn<ProbeFormValues>
  // Models configured on the selected channel.
  channelModels: string[]
}

// The probe type, request format and probed models of a probe.
export function ProbeRequestFields(props: ProbeRequestFieldsProps) {
  const { t } = useTranslation()
  const probeType = useWatch({
    control: props.form.control,
    name: 'probe_type',
  })
  const isSignature = probeType === 'signature'

  const typeOptions = [
    { value: 'custom', label: t('Custom request') },
    { value: 'signature', label: t('Signature tamper check') },
  ]
  const endpointOptions = [
    { value: 'openai', label: t('OpenAI Chat Completions') },
    { value: 'anthropic', label: t('Anthropic Messages') },
    { value: 'openai-response', label: t('OpenAI Responses') },
  ]

  return (
    <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
      <FormField
        control={props.form.control}
        name='probe_type'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Probe type')}</FormLabel>
            <Select
              items={typeOptions}
              value={field.value}
              onValueChange={(value) => {
                field.onChange(value)
                if (value === 'signature') {
                  props.form.setValue('endpoint_type', 'anthropic', {
                    shouldValidate: true,
                  })
                }
              }}
            >
              <FormControl>
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
              </FormControl>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {typeOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <FormDescription>
              {isSignature
                ? t(
                    'Replays the first turn with a tampered thinking signature. A genuine upstream rejects it with "invalid signature".'
                  )
                : t('Runs that differ from the majority of runs are flagged.')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={props.form.control}
        name='endpoint_type'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Request format')}</FormLabel>
            <Select
              items={endpointOptions}
              value={field.value}
              onValueChange={(value) => field.onChange(value)}
              disabled={isSignature}
            >
              <FormControl>
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
              </FormControl>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {endpointOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <FormDescription>
              {t('The channel converts this format to its upstream protocol.')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={props.form.control}
        name='models'
        render={({ field }) => (
          <FormItem className='sm:col-span-2'>
            <FormLabel>{t('Models')}</FormLabel>
            <FormControl>
              <MultiSelect
                options={props.channelModels.map((model) => ({
                  label: model,
                  value: model,
                }))}
                selected={field.value}
                onChange={field.onChange}
                allowCreate
                placeholder={t('Select the models to probe')}
                emptyText={t('This channel has no configured models')}
              />
            </FormControl>
            <FormDescription>
              {t(
                'Each run probes every selected model and keeps a separate baseline per model. Leave empty to use the model in the request body.'
              )}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )
}
