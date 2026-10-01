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
import type { Row } from '@tanstack/react-table'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import type { AdminPermissionMatrix } from '@/lib/admin-permissions'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import type { Channel } from '../../types'
import { ChannelsProvider } from '../channels-provider'
import { DataTableRowActions } from '../data-table-row-actions'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const originalAuth = useAuthStore.getState().auth
let client: QueryClient

const channel: Channel = {
  id: 3,
  type: 1,
  key: '',
  status: 1,
  name: 'prod',
  created_time: 0,
  test_time: 0,
  response_time: 0,
  other: '',
  balance: 0,
  balance_updated_time: 0,
  models: 'gpt-4o',
  group: 'default',
  used_quota: 0,
  other_info: '',
  remark: '',
  max_input_tokens: 0,
  channel_info: {
    is_multi_key: false,
    multi_key_size: 0,
    multi_key_polling_index: 0,
    multi_key_mode: 'random',
  },
  settings: '{}',
}

function signInAdmin(channelPermissions: Record<string, boolean>) {
  const adminPermissions: AdminPermissionMatrix = {
    channel: channelPermissions,
  }
  useAuthStore.setState({
    auth: {
      ...originalAuth,
      user: {
        id: 2,
        username: 'admin',
        role: ROLE.ADMIN,
        permissions: { admin_permissions: adminPermissions },
      },
    },
  })
}

function renderRowActions() {
  render(
    <QueryClientProvider client={client}>
      <ChannelsProvider>
        <DataTableRowActions row={{ original: channel } as Row<Channel>} />
      </ChannelsProvider>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.setState({ auth: originalAuth })
})

describe('channel row test actions', () => {
  test('hides the quick test button without channel:test while keeping operate actions', () => {
    signInAdmin({ read: true, operate: true })
    renderRowActions()

    expect(
      screen.queryByRole('button', { name: 'Test Connection' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Disable' })).toBeInTheDocument()
  })

  test('hides the Test Connection menu item without channel:test', async () => {
    signInAdmin({ read: true, operate: true })
    renderRowActions()

    await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))

    expect(
      await screen.findByRole('menuitem', { name: /Query Balance/ })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('menuitem', { name: /Test Connection/ })
    ).not.toBeInTheDocument()
  })

  test('shows the quick test button with channel:test', () => {
    signInAdmin({ read: true, test: true })
    renderRowActions()

    expect(
      screen.getByRole('button', { name: 'Test Connection' })
    ).toBeInTheDocument()
  })

  test('shows the Test Connection menu item with channel:test', async () => {
    signInAdmin({ read: true, test: true })
    renderRowActions()

    await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))

    expect(
      await screen.findByRole('menuitem', { name: /Test Connection/ })
    ).toBeInTheDocument()
  })
})
