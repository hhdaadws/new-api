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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import type { TagRow } from '../../lib'
import {
  channelSchema,
  type Channel,
  type ChannelCacheStatsMap,
} from '../../types'
import { ChannelCacheHitCell } from '../channel-cache-hit-cell'
import {
  ChannelCacheStatsContext,
  type ChannelCacheStatsState,
} from '../channel-cache-stats-context'

function channel(id: number): Channel {
  return channelSchema.parse({
    id,
    type: 1,
    key: '',
    name: `channel-${id}`,
    status: 1,
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
  })
}

const stats: ChannelCacheStatsMap = {
  '1': {
    request_count: 10,
    hit_count: 8,
    input_tokens: 2000,
    cache_read_tokens: 1250,
    cache_write_tokens: 40,
  },
  '2': {
    request_count: 6,
    hit_count: 0,
    input_tokens: 3000,
    cache_read_tokens: 0,
    cache_write_tokens: 0,
  },
}

function renderCell(row: Channel, state: ChannelCacheStatsState) {
  return render(
    <ChannelCacheStatsContext.Provider value={state}>
      <ChannelCacheHitCell channel={row} />
    </ChannelCacheStatsContext.Provider>
  )
}

describe('ChannelCacheHitCell', () => {
  it('shows the token hit rate of a channel with traffic', () => {
    renderCell(channel(1), { stats, isLoading: false })

    expect(screen.getByText('62.5%')).toBeInTheDocument()
  })

  it('shows the combined rate of every channel in a tag row', () => {
    const tagRow: TagRow = {
      ...channel(0),
      tag: 'shared',
      children: [channel(1), channel(2)],
    }

    renderCell(tagRow, { stats, isLoading: false })

    expect(screen.getByText('25%')).toBeInTheDocument()
  })

  it('shows a dash for a channel without traffic in the window', () => {
    renderCell(channel(3), { stats, isLoading: false })

    expect(screen.getByText('-')).toBeInTheDocument()
  })

  it('shows a placeholder instead of a dash while the counters load', () => {
    const { container } = renderCell(channel(1), { isLoading: true })

    expect(container.querySelector('[data-slot="skeleton"]')).not.toBeNull()
    expect(screen.queryByText('-')).not.toBeInTheDocument()
  })

  it('breaks the rate down when the badge receives keyboard focus', async () => {
    const user = userEvent.setup()
    renderCell(channel(1), { stats, isLoading: false })

    await user.tab()

    expect(document.activeElement).toHaveTextContent('62.5%')
    expect(await screen.findByText('Last 24 hours')).toBeInTheDocument()
    expect(screen.getByText('Token hit rate')).toBeInTheDocument()
    expect(screen.getByText('Request hit rate')).toBeInTheDocument()
    expect(screen.getByText('80% (8 / 10)')).toBeInTheDocument()
    expect(screen.getByText('1,250')).toBeInTheDocument()
  })
})
