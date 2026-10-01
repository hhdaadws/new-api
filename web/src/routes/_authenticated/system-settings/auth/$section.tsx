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
import { createFileRoute, redirect } from '@tanstack/react-router'

import { AuthSettings } from '@/features/system-settings/auth'
import {
  AUTH_DEFAULT_SECTION,
  AUTH_SECTION_IDS,
  getAuthSectionMeta,
} from '@/features/system-settings/auth/section-registry.tsx'
import { isSettingsSectionAvailable } from '@/features/system-settings/utils/section-registry'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute(
  '/_authenticated/system-settings/auth/$section'
)({
  beforeLoad: ({ params }) => {
    const user = useAuthStore.getState().auth.user
    const sectionAvailable = isSettingsSectionAvailable(
      AUTH_SECTION_IDS,
      getAuthSectionMeta,
      params.section,
      user
    )
    if (sectionAvailable) return

    // Every authentication section is root-only, so other admins leave the
    // category entirely instead of bouncing between unavailable sections.
    const defaultAvailable = isSettingsSectionAvailable(
      AUTH_SECTION_IDS,
      getAuthSectionMeta,
      AUTH_DEFAULT_SECTION,
      user
    )
    if (!defaultAvailable) {
      throw redirect({ to: '/system-settings/site' })
    }
    throw redirect({
      to: '/system-settings/auth/$section',
      params: { section: AUTH_DEFAULT_SECTION },
    })
  },
  component: AuthSettings,
})
