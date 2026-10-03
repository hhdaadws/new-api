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
import { api } from '@/lib/api'

import type {
  ApiResponse,
  ChannelProbe,
  ChannelProbePayload,
  ChannelProbeResult,
  ChannelProbeResultsPage,
  ChannelProbeTarget,
} from './types'

export async function getChannelProbes(): Promise<ApiResponse<ChannelProbe[]>> {
  const res = await api.get('/api/channel_probe/')
  return res.data
}

export async function getChannelProbeTargets(): Promise<
  ApiResponse<ChannelProbeTarget[]>
> {
  const res = await api.get('/api/channel_probe/channels')
  return res.data
}

export async function createChannelProbe(
  data: ChannelProbePayload
): Promise<ApiResponse<ChannelProbe>> {
  const res = await api.post('/api/channel_probe/', data)
  return res.data
}

export async function updateChannelProbe(
  id: number,
  data: ChannelProbePayload
): Promise<ApiResponse<ChannelProbe>> {
  const res = await api.put(`/api/channel_probe/${id}`, data)
  return res.data
}

export async function deleteChannelProbe(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/channel_probe/${id}`)
  return res.data
}

export async function runChannelProbe(
  id: number
): Promise<ApiResponse<ChannelProbeResult[]>> {
  const res = await api.post(`/api/channel_probe/${id}/run`)
  return res.data
}

// An empty model returns the results of every model.
export async function getChannelProbeResults(
  id: number,
  model: string,
  page: number,
  pageSize: number
): Promise<ApiResponse<ChannelProbeResultsPage>> {
  const res = await api.get(`/api/channel_probe/${id}/results`, {
    params: { model: model || undefined, p: page, page_size: pageSize },
  })
  return res.data
}
