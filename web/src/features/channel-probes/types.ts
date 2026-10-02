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
export type ChannelProbeEndpointType =
  | 'openai'
  | 'anthropic'
  | 'openai-response'

export type ChannelProbe = {
  id: number
  name: string
  channel_id: number
  channel_name: string
  channel_type: number
  endpoint_type: ChannelProbeEndpointType
  headers: string
  body: string
  interval_seconds: number
  enabled: boolean
  next_run_at: number
  last_run_at: number
  last_success: boolean
  last_error: string
  last_anomaly: boolean
  created_at: number
  updated_at: number
}

export type ChannelProbePayload = Pick<
  ChannelProbe,
  | 'name'
  | 'channel_id'
  | 'endpoint_type'
  | 'headers'
  | 'body'
  | 'interval_seconds'
  | 'enabled'
>

export type ChannelProbeResult = {
  id: number
  probe_id: number
  channel_id: number
  success: boolean
  status_code: number
  latency_ms: number
  response: string
  error: string
  input_tokens: number
  output_tokens: number
  content: string
  anomalies: ChannelProbeAnomaly[]
  created_at: number
}

export type ChannelProbeAnomaly = 'input_tokens' | 'content'

export type ChannelProbeMajority = {
  established: boolean
  votes: number
  samples: number
}

export type ChannelProbeBaseline = {
  input_tokens: number
  input_tokens_majority: ChannelProbeMajority
  content: string
  content_majority: ChannelProbeMajority
}

export type ChannelProbeTarget = {
  id: number
  name: string
  type: number
  status: number
}

export type ApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}

export type ChannelProbeResultsPage = {
  page: number
  page_size: number
  total: number
  items: ChannelProbeResult[] | null
  baseline: ChannelProbeBaseline
}
