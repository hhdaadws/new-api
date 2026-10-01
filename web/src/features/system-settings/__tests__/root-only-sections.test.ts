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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import i18next from 'i18next'
import { afterEach, describe, expect, it } from 'vitest'

import { SYSTEM_SETTINGS_VIEW } from '@/components/layout/config/system-settings.config'
import { ROLE } from '@/lib/roles'
import { Route as AuthRoute } from '@/routes/_authenticated/system-settings/auth/$section'
import { Route as BillingRoute } from '@/routes/_authenticated/system-settings/billing/$section'
import { Route as OperationsRoute } from '@/routes/_authenticated/system-settings/operations/$section'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

type SectionBeforeLoad = (context: { params: { section: string } }) => void

const superAdmin: AuthUser = {
  id: 1,
  username: 'root',
  role: ROLE.SUPER_ADMIN,
}
const settingsAdmin: AuthUser = {
  id: 2,
  username: 'operator',
  role: ROLE.ADMIN,
  permissions: {
    admin_permissions: { system_setting: { read: true, write: true } },
  },
}

const ROOT_ONLY_NAV_TITLES = [
  'Basic Authentication',
  'OAuth Integrations',
  'Passkey Authentication',
  'Bot Protection',
  'Custom OAuth',
  'Payment Gateway',
  'Model Deployment',
  'SSRF Protection',
  'SMTP Email',
  'Worker Proxy',
  'Log Maintenance',
  'Performance',
]

function visibleNav(user: AuthUser) {
  useAuthStore.getState().auth.setUser(user)
  const [group] = SYSTEM_SETTINGS_VIEW.getNavGroups(i18next.t)
  return {
    categories: group.items.map((item) => item.title),
    sections: group.items.flatMap(
      (item) => item.items?.map((section) => section.title) ?? []
    ),
  }
}

async function openSettingsPath(user: AuthUser, path: string) {
  useAuthStore.getState().auth.setUser(user)
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
  })
  const routes = [
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/site',
    }),
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/auth/$section',
      beforeLoad: AuthRoute.options.beforeLoad as SectionBeforeLoad,
    }),
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/billing/$section',
      beforeLoad: BillingRoute.options.beforeLoad as SectionBeforeLoad,
    }),
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/operations/$section',
      beforeLoad: OperationsRoute.options.beforeLoad as SectionBeforeLoad,
    }),
  ]
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren(routes)]),
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  await router.load()
  return router.state.location.pathname
}

afterEach(() => {
  useAuthStore.getState().auth.reset()
})

describe('root-only system settings sections', () => {
  it('a non-root settings admin does not see root-only sections in the sidebar', () => {
    const nav = visibleNav(settingsAdmin)
    for (const title of ROOT_ONLY_NAV_TITLES) {
      expect(nav.sections).not.toContain(title)
    }
    expect(nav.sections).toEqual(
      expect.arrayContaining([
        'System Information',
        'Quota Settings',
        'Rate Limiting',
        'Monitoring & Alerts',
        'Upstream error interception',
      ])
    )
  })

  it('a non-root settings admin does not see a category whose sections are all root-only', () => {
    expect(visibleNav(settingsAdmin).categories).not.toContain('Authentication')
  })

  it('a super admin sees every root-only section and category', () => {
    const nav = visibleNav(superAdmin)
    expect(nav.sections).toEqual(expect.arrayContaining(ROOT_ONLY_NAV_TITLES))
    expect(nav.categories).toContain('Authentication')
  })

  it.each([
    [
      '/system-settings/operations/email',
      '/system-settings/operations/behavior',
    ],
    ['/system-settings/billing/payment', '/system-settings/billing/quota'],
    ['/system-settings/auth/oauth', '/system-settings/site'],
  ])(
    'a non-root settings admin opening %s is redirected to %s',
    async (path, expected) => {
      expect(await openSettingsPath(settingsAdmin, path)).toBe(expected)
    }
  )

  it.each([
    '/system-settings/operations/email',
    '/system-settings/billing/payment',
    '/system-settings/auth/oauth',
  ])('a super admin can open the root-only section %s', async (path) => {
    expect(await openSettingsPath(superAdmin, path)).toBe(path)
  })
})
