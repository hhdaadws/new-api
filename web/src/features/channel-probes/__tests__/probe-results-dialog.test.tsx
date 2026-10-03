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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, beforeAll, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { ProbeResultsDialog } from '../components/probe-results-dialog'
import type {
  ChannelProbe,
  ChannelProbeBaseline,
  ChannelProbeResult,
} from '../types'

const probe = {
  id: 5,
  name: 'zero',
  channel_id: 3,
  endpoint_type: 'openai',
  effective_models: ['m'],
} as ChannelProbe

function result(
  id: number,
  overrides: Partial<ChannelProbeResult>
): ChannelProbeResult {
  return {
    id,
    probe_id: 5,
    channel_id: 3,
    model: 'm',
    success: true,
    status_code: 200,
    latency_ms: 120,
    response: '{}',
    error: '',
    input_tokens: 20,
    output_tokens: 1,
    content: 'NONE',
    anomalies: [],
    created_at: 1_790_000_000 + id,
    ...overrides,
  }
}

beforeAll(async () => {
  await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function renderResults(
  items: ChannelProbeResult[],
  baselines: Record<string, ChannelProbeBaseline>,
  probeOverrides: Partial<ChannelProbe> = {}
) {
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: items.length, items, baselines },
    },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <ProbeResultsDialog
          probe={{ ...probe, ...probeOverrides }}
          onOpenChange={() => undefined}
        />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return get
}

test('a run that differs from the majority baseline is flagged', async () => {
  renderResults(
    [
      result(2, {
        input_tokens: 35,
        content: 'You are a helpful assistant.',
        anomalies: ['input_tokens', 'content'],
      }),
      result(1, {}),
    ],
    {
      m: {
        input_tokens: 20,
        input_tokens_majority: { established: true, votes: 3, samples: 4 },
        content: 'NONE',
        content_majority: { established: true, votes: 3, samples: 4 },
      },
    }
  )

  const baseline = await screen.findByRole('region', { name: 'Baseline: m' })
  expect(within(baseline).getByText('20')).toBeInTheDocument()
  expect(within(baseline).getAllByText('3 of 4 runs agree')).toHaveLength(2)
  const runs = screen.getAllByRole('region', { name: /^\d{4}-\d{2}-\d{2}/ })
  expect(
    within(runs[0]).getByText('Input tokens differ from baseline')
  ).toBeInTheDocument()
  expect(
    within(runs[0]).getByText('Output differs from baseline')
  ).toBeInTheDocument()
  expect(
    within(runs[1]).queryByText('Output differs from baseline')
  ).not.toBeInTheDocument()
})

test('without a majority the baseline is reported as not established', async () => {
  renderResults([result(1, { content: 'a' }), result(2, { content: 'b' })], {
    m: {
      input_tokens: 20,
      input_tokens_majority: { established: true, votes: 2, samples: 2 },
      content: '',
      content_majority: { established: false, votes: 1, samples: 2 },
    },
  })

  const baseline = await screen.findByRole('region', { name: 'Baseline: m' })
  expect(
    within(baseline).getByText(
      'Not established: needs a majority of at least 3 runs'
    )
  ).toBeInTheDocument()
  expect(screen.queryByText('Output differs from baseline')).toBeNull()
})

const signatureBaseline: ChannelProbeBaseline = {
  expectation: 'signature_rejected',
  input_tokens: 0,
  input_tokens_majority: { established: false, votes: 0, samples: 0 },
  content: '',
  content_majority: { established: false, votes: 0, samples: 0 },
}

test('a signature probe shows its fixed expectation and flags an accepted tampered signature', async () => {
  renderResults(
    [
      result(2, { content: '3.6 hours', anomalies: ['signature_accepted'] }),
      result(1, {
        status_code: 400,
        content:
          'messages.1.content.0: Invalid `signature` in `thinking` block',
      }),
    ],
    { m: signatureBaseline },
    { probe_type: 'signature' }
  )

  const baseline = await screen.findByRole('region', { name: 'Baseline: m' })
  expect(baseline).toHaveTextContent('Invalid signature')
  const runs = screen.getAllByRole('region', { name: /^\d{4}-\d{2}-\d{2}/ })
  expect(
    within(runs[0]).getByText('Upstream accepted a tampered signature')
  ).toBeInTheDocument()
  expect(
    within(runs[1]).queryByText('Upstream accepted a tampered signature')
  ).not.toBeInTheDocument()
})

test('with several models each model has its own baseline and can be filtered', async () => {
  const user = userEvent.setup()
  const get = renderResults(
    [result(2, { model: 'b' }), result(1, { model: 'a' })],
    { a: signatureBaseline, b: signatureBaseline },
    { probe_type: 'signature', effective_models: ['a', 'b'] }
  )

  expect(
    await screen.findByRole('region', { name: 'Baseline: a' })
  ).toBeInTheDocument()
  expect(
    screen.getByRole('region', { name: 'Baseline: b' })
  ).toBeInTheDocument()

  await user.click(screen.getByRole('combobox', { name: 'Model' }))
  await user.click(await screen.findByRole('option', { name: 'b' }))

  await waitFor(() =>
    expect(get).toHaveBeenLastCalledWith('/api/channel/probe/5/results', {
      params: { model: 'b', p: 1, page_size: 10 },
    })
  )
  await waitFor(() =>
    expect(
      screen.queryByRole('region', { name: 'Baseline: a' })
    ).not.toBeInTheDocument()
  )
})
