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
import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import { RequestPolicies } from '../../request-policies'

const READ_ONLY_NOTICE =
  'You have read-only access to system settings. Changes cannot be saved with this account.'

let queryClient: QueryClient

function settingsAdmin(write: boolean): AuthUser {
  return {
    id: 2,
    username: 'operator',
    role: ROLE.ADMIN,
    permissions: {
      admin_permissions: { system_setting: { read: true, write } },
    },
  }
}

async function renderInterceptionSettings(user: AuthUser) {
  useAuthStore.getState().auth.setUser(user)
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
  })
  const policies = createRoute({
    getParentRoute: () => authenticated,
    path: '/system-settings/request-policies/$section',
    component: RequestPolicies,
  })
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren([policies])]),
    history: createMemoryHistory({
      initialEntries: ['/system-settings/request-policies/error-interception'],
    }),
  })
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await act(() => router.load())
  return screen.findByRole('textbox', { name: 'Error keywords' })
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [
        {
          key: 'upstream_error_interception.keywords',
          value: '["quota exceeded"]',
        },
      ],
    },
  })
})

afterEach(() => {
  cleanup()
  queryClient.clear()
  useAuthStore.getState().auth.reset()
})

describe('system settings read-only mode', () => {
  it('an admin without system_setting write sees the notice and disabled fields and save button', async () => {
    const keywords = await renderInterceptionSettings(settingsAdmin(false))
    expect(screen.getByText(READ_ONLY_NOTICE)).toBeVisible()
    expect(keywords).toBeDisabled()
    expect(keywords).toHaveValue('quota exceeded')
    expect(
      screen.getByRole('spinbutton', { name: 'Response status code' })
    ).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()
  })

  it('an admin with system_setting write gets editable fields without the notice', async () => {
    const keywords = await renderInterceptionSettings(settingsAdmin(true))
    expect(screen.queryByText(READ_ONLY_NOTICE)).not.toBeInTheDocument()
    expect(keywords).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
  })
})
