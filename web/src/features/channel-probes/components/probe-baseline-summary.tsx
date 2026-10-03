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

import { BASELINE_MIN_SAMPLES } from '../lib/probe-form'
import type { ChannelProbeBaseline, ChannelProbeMajority } from '../types'

type BaselineVoteProps = {
  label: string
  majority: ChannelProbeMajority
  children: React.ReactNode
}

function BaselineVote(props: BaselineVoteProps) {
  const { t } = useTranslation()
  return (
    <div className='space-y-1'>
      <div className='flex flex-wrap items-baseline justify-between gap-2'>
        <span className='text-sm font-medium'>{props.label}</span>
        <span className='text-muted-foreground text-xs'>
          {props.majority.established
            ? t('{{votes}} of {{samples}} runs agree', {
                votes: props.majority.votes,
                samples: props.majority.samples,
              })
            : t('Not established: needs a majority of at least {{min}} runs', {
                min: BASELINE_MIN_SAMPLES,
              })}
        </span>
      </div>
      {props.majority.established && props.children}
    </div>
  )
}

// Shows what the model's runs are compared with: the values most runs of the
// current request agree on, or the fixed expectation of a signature probe.
export function ProbeBaselineSummary(props: {
  model: string
  baseline: ChannelProbeBaseline
}) {
  const { t } = useTranslation()
  const title = t('Baseline: {{model}}', { model: props.model })
  if (props.baseline.expectation === 'signature_rejected') {
    return (
      <section
        aria-label={title}
        className='bg-muted/30 space-y-1 rounded-md border p-3'
      >
        <h3 className='text-sm font-semibold'>{title}</h3>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Expected: the upstream rejects the replayed thinking block with 400 "Invalid signature". Accepting it, returning no signed thinking block, or any other rejection is flagged.'
          )}
        </p>
      </section>
    )
  }
  return (
    <section
      aria-label={title}
      className='bg-muted/30 space-y-3 rounded-md border p-3'
    >
      <div>
        <h3 className='text-sm font-semibold'>{title}</h3>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Taken from what most successful runs of the current request return. Runs that differ are flagged.'
          )}
        </p>
      </div>
      <BaselineVote
        label={t('Input tokens')}
        majority={props.baseline.input_tokens_majority}
      >
        <p className='font-mono text-sm'>{props.baseline.input_tokens}</p>
      </BaselineVote>
      <BaselineVote
        label={t('Output')}
        majority={props.baseline.content_majority}
      >
        <pre className='bg-background max-h-32 overflow-y-auto rounded border p-2 text-sm wrap-break-word whitespace-pre-wrap'>
          {props.baseline.content || t('(empty output)')}
        </pre>
      </BaselineVote>
    </section>
  )
}
