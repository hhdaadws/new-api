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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, beforeAll, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { ProbeFormDialog } from '../components/probe-form-dialog'
import { ZERO_INJECTION_TEMPLATE } from '../lib/probe-form'
import type { ChannelProbe } from '../types'

const probe: ChannelProbe = {
  id: 5,
  name: 'existing',
  channel_id: 3,
  channel_name: 'upstream',
  channel_type: 1,
  endpoint_type: 'anthropic',
  headers: '',
  body: '{"model":"m","messages":[]}',
  interval_seconds: 120,
  enabled: true,
  next_run_at: 0,
  last_run_at: 0,
  last_success: false,
  last_error: '',
  created_at: 0,
  updated_at: 0,
}

beforeAll(async () => {
  await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function renderDialog(onOpenChange = vi.fn()) {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [{ id: 3, name: 'upstream', type: 1, status: 1 }],
    },
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true, data: probe } })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <ProbeFormDialog open probe={probe} onOpenChange={onOpenChange} />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return { put, onOpenChange }
}

test('applying the zero injection template saves the template request', async () => {
  const user = userEvent.setup()
  const { put, onOpenChange } = renderDialog()

  await user.click(
    screen.getByRole('button', { name: 'Use zero injection template' })
  )
  await user.click(screen.getByRole('button', { name: 'Save' }))

  await waitFor(() => expect(put).toHaveBeenCalledTimes(1))
  expect(put).toHaveBeenCalledWith('/api/channel_probe/5', {
    name: 'existing',
    channel_id: 3,
    endpoint_type: ZERO_INJECTION_TEMPLATE.endpoint_type,
    interval_seconds: 120,
    enabled: true,
    headers: '',
    body: ZERO_INJECTION_TEMPLATE.body,
  })
  expect(onOpenChange).toHaveBeenCalledWith(false)
})

test('a body without a model is rejected before saving', async () => {
  const user = userEvent.setup()
  const { put } = renderDialog()
  const bodyEditor = screen.getByRole('textbox', { name: 'Request body' })

  await user.clear(bodyEditor)
  await user.click(screen.getByRole('button', { name: 'Save' }))

  expect(
    await screen.findByText(
      'Request body must be a JSON object containing a model'
    )
  ).toBeInTheDocument()
  expect(put).not.toHaveBeenCalled()
})
