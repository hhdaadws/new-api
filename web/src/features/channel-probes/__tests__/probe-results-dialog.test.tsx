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
import { cleanup, render, screen, within } from '@testing-library/react'
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
} as ChannelProbe

function result(
  id: number,
  overrides: Partial<ChannelProbeResult>
): ChannelProbeResult {
  return {
    id,
    probe_id: 5,
    channel_id: 3,
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
  baseline: ChannelProbeBaseline
) {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { page: 1, page_size: 10, total: items.length, items, baseline },
    },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <ProbeResultsDialog probe={probe} onOpenChange={() => undefined} />
      </QueryClientProvider>
    </I18nextProvider>
  )
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
      input_tokens: 20,
      input_tokens_majority: { established: true, votes: 3, samples: 4 },
      content: 'NONE',
      content_majority: { established: true, votes: 3, samples: 4 },
    }
  )

  const baseline = await screen.findByRole('region', { name: 'Baseline' })
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
    input_tokens: 20,
    input_tokens_majority: { established: true, votes: 2, samples: 2 },
    content: '',
    content_majority: { established: false, votes: 1, samples: 2 },
  })

  const baseline = await screen.findByRole('region', { name: 'Baseline' })
  expect(
    within(baseline).getByText(
      'Not established: needs a majority of at least 3 runs'
    )
  ).toBeInTheDocument()
  expect(screen.queryByText('Output differs from baseline')).toBeNull()
})
