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
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'

import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import {
  createChannelProbe,
  deleteChannelProbe,
  getChannelProbeResults,
  getChannelProbes,
  getChannelProbeTargets,
  runChannelProbe,
  updateChannelProbe,
} from '../api'
import type { ChannelProbePayload } from '../types'

export const channelProbeQueryKeys = {
  all: ['channel-probes'] as const,
  list: () => [...channelProbeQueryKeys.all, 'list'] as const,
  targets: () => [...channelProbeQueryKeys.all, 'targets'] as const,
  results: (id: number, model: string, page: number, pageSize: number) =>
    [
      ...channelProbeQueryKeys.all,
      'results',
      id,
      model,
      page,
      pageSize,
    ] as const,
}

export function useChannelProbes() {
  return useQuery({
    queryKey: channelProbeQueryKeys.list(),
    queryFn: async () => {
      const res = requireServerSuccess(await getChannelProbes())
      return res.data ?? []
    },
    refetchInterval: 30_000,
  })
}

export function useChannelProbeTargets(enabled: boolean) {
  return useQuery({
    queryKey: channelProbeQueryKeys.targets(),
    queryFn: async () => {
      const res = requireServerSuccess(await getChannelProbeTargets())
      return res.data ?? []
    },
    enabled,
  })
}

export function useChannelProbeResults(
  id: number,
  model: string,
  page: number,
  pageSize: number
) {
  return useQuery({
    queryKey: channelProbeQueryKeys.results(id, model, page, pageSize),
    queryFn: async () => {
      const res = requireServerSuccess(
        await getChannelProbeResults(id, model, page, pageSize)
      )
      return res.data
    },
    placeholderData: keepPreviousData,
  })
}

export function useSaveChannelProbe() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: { id?: number; data: ChannelProbePayload }) => {
      const res =
        input.id === undefined
          ? await createChannelProbe(input.data)
          : await updateChannelProbe(input.id, input.data)
      return requireServerSuccess(res)
    },
    onSuccess: () => {
      toast.success(i18next.t('Probe saved'))
      queryClient.invalidateQueries({ queryKey: channelProbeQueryKeys.all })
    },
    onError: (error: Error) => {
      handleServerError(error, i18next.t('Failed to save probe'))
    },
  })
}

export function useDeleteChannelProbe() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: number) =>
      requireServerSuccess(await deleteChannelProbe(id)),
    onSuccess: () => {
      toast.success(i18next.t('Probe deleted'))
      queryClient.invalidateQueries({ queryKey: channelProbeQueryKeys.all })
    },
    onError: (error: Error) => {
      handleServerError(error, i18next.t('Failed to delete probe'))
    },
  })
}

export function useRunChannelProbe() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: number) =>
      requireServerSuccess(await runChannelProbe(id)),
    onSuccess: (res) => {
      const failed = (res.data ?? []).filter((result) => !result.success)
      if (failed.length === 0) {
        toast.success(i18next.t('Probe run succeeded'))
      } else {
        toast.error(
          failed.map((result) => `${result.model}: ${result.error}`).join('\n')
        )
      }
      queryClient.invalidateQueries({ queryKey: channelProbeQueryKeys.all })
    },
    onError: (error: Error) => {
      handleServerError(error, i18next.t('Failed to run probe'))
    },
  })
}
