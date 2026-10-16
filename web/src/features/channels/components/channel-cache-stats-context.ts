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
import { createContext } from 'react'

import type { ChannelCacheStatsMap } from '../types'

export type ChannelCacheStatsState = {
  stats?: ChannelCacheStatsMap
  isLoading: boolean
}

/**
 * Cache counters of the channels on the current page. The table fetches them
 * once per page, and cells read them here so the memoized column definitions
 * stay stable while the counters load.
 */
export const ChannelCacheStatsContext = createContext<ChannelCacheStatsState>({
  isLoading: false,
})
