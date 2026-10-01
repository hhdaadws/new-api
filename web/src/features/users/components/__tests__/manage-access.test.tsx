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
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import { Users } from '../..'

const clients: QueryClient[] = []

function userAdmin(write: boolean): AuthUser {
  return {
    id: 1,
    username: 'operator',
    role: ROLE.ADMIN,
    permissions: {
      admin_permissions: { user_management: { read: true, write } },
    },
  }
}

async function renderUsersPage(viewer: AuthUser) {
  useAuthStore.getState().auth.setUser(viewer)
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        items: [
          {
            id: 2,
            username: 'listed-user',
            display_name: '',
            role: ROLE.USER,
            status: 1,
            quota: 0,
            used_quota: 0,
            request_count: 0,
            group: 'default',
          },
        ],
        total: 1,
      },
    },
  })
  const root = createRootRoute()
  const auth = createRoute({ getParentRoute: () => root, id: '_authenticated' })
  const users = createRoute({
    getParentRoute: () => auth,
    path: 'users/',
    component: Users,
  })
  const router = createRouter({
    routeTree: root.addChildren([auth.addChildren([users])]),
    history: createMemoryHistory({ initialEntries: ['/users/'] }),
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  await router.load()
  render(
    <TooltipProvider>
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TooltipProvider>
  )
  await screen.findByText('listed-user')
}

afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  useAuthStore.getState().auth.reset()
})

describe('user management write access', () => {
  it('an admin with only user_management read sees no create, edit or row action controls', async () => {
    await renderUsersPage(userAdmin(false))
    expect(
      screen.queryByRole('button', { name: 'Add User' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Edit' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Open menu' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('columnheader', { name: 'Actions' })
    ).not.toBeInTheDocument()
  })

  it('an admin with user_management write sees create, edit and row action controls', async () => {
    await renderUsersPage(userAdmin(true))
    expect(screen.getByRole('button', { name: 'Add User' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Open menu' })
    ).toBeInTheDocument()
  })

  it('a super admin without explicit grants sees create and row action controls', async () => {
    await renderUsersPage({ id: 1, username: 'root', role: ROLE.SUPER_ADMIN })
    expect(screen.getByRole('button', { name: 'Add User' })).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Open menu' })
    ).toBeInTheDocument()
  })
})
