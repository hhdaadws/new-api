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
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { useDataTable } from '@/components/data-table'
import type { AdminPermissionMatrix } from '@/lib/admin-permissions'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import type { Channel } from '../../types'
import { ChannelsPrimaryButtons } from '../channels-primary-buttons'
import { ChannelsProvider } from '../channels-provider'
import { DataTableBulkActions } from '../data-table-bulk-actions'
import { DataTableRowActions } from '../data-table-row-actions'
import { DataTableTagRowActions } from '../data-table-tag-row-actions'

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
const columns = [{ accessorKey: 'name' }]

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

function renderInChannels(children: ReactNode) {
  render(
    <QueryClientProvider client={client}>
      <ChannelsProvider>{children}</ChannelsProvider>
    </QueryClientProvider>
  )
}

function SelectableChannels() {
  const { table } = useDataTable({
    data: [channel],
    columns,
    enableRowSelection: true,
    getRowId: (row) => String(row.id),
  })
  return (
    <>
      {table.getRowModel().rows.map((row) => (
        <label key={row.id}>
          <input
            type='checkbox'
            checked={row.getIsSelected()}
            onChange={row.getToggleSelectedHandler()}
          />
          {row.original.name}
        </label>
      ))}
      <DataTableBulkActions table={table} />
    </>
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

describe('channel row actions', () => {
  test('a read-only admin keeps Edit but loses the enable/disable button', () => {
    signInAdmin({ read: true })
    renderInChannels(
      <DataTableRowActions row={{ original: channel } as Row<Channel>} />
    )

    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Disable' })
    ).not.toBeInTheDocument()
  })

  test('a read-only admin does not get balance or model fetching menu items', async () => {
    signInAdmin({ read: true })
    renderInChannels(
      <DataTableRowActions row={{ original: channel } as Row<Channel>} />
    )

    await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))

    expect(
      await screen.findByRole('menuitem', { name: /Copy Channel/ })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('menuitem', { name: /Query Balance/ })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('menuitem', { name: /Fetch Models/ })
    ).not.toBeInTheDocument()
  })

  test('an admin with channel:operate gets the enable/disable button', () => {
    signInAdmin({ read: true, operate: true })
    renderInChannels(
      <DataTableRowActions row={{ original: channel } as Row<Channel>} />
    )

    expect(screen.getByRole('button', { name: 'Disable' })).toBeInTheDocument()
  })
})

describe('channel tag row actions', () => {
  const tagRow = { original: { ...channel, tag: 'team-a' } } as Row<
    Channel & { tag?: string }
  >

  test('an admin without channel:write or channel:operate gets no tag actions', () => {
    signInAdmin({ read: true })
    renderInChannels(<DataTableTagRowActions row={tagRow} />)

    expect(
      screen.queryByRole('button', { name: 'Open menu' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Edit Tag' })
    ).not.toBeInTheDocument()
  })

  test('an admin with only channel:operate can enable the tag but not edit it', async () => {
    signInAdmin({ read: true, operate: true })
    renderInChannels(<DataTableTagRowActions row={tagRow} />)

    expect(
      screen.queryByRole('button', { name: 'Edit Tag' })
    ).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))
    expect(
      await screen.findByRole('menuitem', { name: /Enable All/ })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('menuitem', { name: /Batch Edit/ })
    ).not.toBeInTheDocument()
  })
})

describe('channel bulk actions', () => {
  test('a read-only admin cannot enable, disable, or tag selected channels', async () => {
    signInAdmin({ read: true })
    renderInChannels(<SelectableChannels />)

    await userEvent.click(screen.getByRole('checkbox', { name: 'prod' }))

    expect(
      await screen.findByRole('button', { name: 'Delete selected channels' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Enable selected channels' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Set tag for selected channels' })
    ).not.toBeInTheDocument()
  })

  test('an admin with channel:operate and channel:write gets the status and tag buttons', async () => {
    signInAdmin({ read: true, operate: true, write: true })
    renderInChannels(<SelectableChannels />)

    await userEvent.click(screen.getByRole('checkbox', { name: 'prod' }))

    expect(
      await screen.findByRole('button', { name: 'Enable selected channels' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Set tag for selected channels' })
    ).toBeInTheDocument()
  })
})

describe('channel toolbar menu', () => {
  test('a read-only admin does not get balance, upstream, or repair actions', async () => {
    signInAdmin({ read: true })
    renderInChannels(<ChannelsPrimaryButtons />)

    await userEvent.click(screen.getByRole('button', { name: 'More actions' }))

    expect(
      await screen.findByRole('menuitem', { name: /Delete All Disabled/ })
    ).toBeInTheDocument()
    for (const name of [
      /Update All Balances/,
      /Detect All Upstream Updates/,
      /Apply All Upstream Updates/,
      /Repair Channel Consistency/,
    ]) {
      expect(screen.queryByRole('menuitem', { name })).not.toBeInTheDocument()
    }
  })

  test('an admin with channel:operate and channel:write gets those actions', async () => {
    signInAdmin({ read: true, operate: true, write: true })
    renderInChannels(<ChannelsPrimaryButtons />)

    await userEvent.click(screen.getByRole('button', { name: 'More actions' }))

    expect(
      await screen.findByRole('menuitem', { name: /Update All Balances/ })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('menuitem', { name: /Apply All Upstream Updates/ })
    ).toBeInTheDocument()
  })
})
