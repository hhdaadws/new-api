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
import * as z from 'zod'

import { parseKeywordLines } from './error-interception'

type Translate = (key: string) => string

/**
 * When a request in `group` fails on `channel_id` with an error containing any
 * of `keywords` (case insensitive), its next attempt runs on
 * `retry_channel_id`, and that attempt is the last one.
 */
export type ErrorKeywordRetryRule = {
  group: string
  channel_id: number
  keywords: string[]
  retry_channel_id: number
}

function isChannelId(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

/**
 * Read the stored rule list (a JSON array) in its saved order. A value that is
 * not a JSON array gives no rules, and entries without a group, both channels
 * and a keyword are dropped.
 */
export function parseErrorKeywordRetryRules(
  stored: string
): ErrorKeywordRetryRule[] {
  let parsed: unknown
  try {
    parsed = JSON.parse(stored)
  } catch {
    return []
  }
  if (!Array.isArray(parsed)) return []
  const rules: ErrorKeywordRetryRule[] = []
  for (const entry of parsed) {
    if (typeof entry !== 'object' || entry === null) continue
    const rule = entry as Record<string, unknown>
    const keywords = Array.isArray(rule.keywords)
      ? rule.keywords.filter(
          (keyword): keyword is string => typeof keyword === 'string'
        )
      : []
    if (
      typeof rule.group !== 'string' ||
      !rule.group ||
      !isChannelId(rule.channel_id) ||
      !isChannelId(rule.retry_channel_id) ||
      keywords.length === 0
    ) {
      continue
    }
    rules.push({
      group: rule.group,
      channel_id: rule.channel_id,
      keywords,
      retry_channel_id: rule.retry_channel_id,
    })
  }
  return rules
}

export function createErrorKeywordRetryRuleSchema(t: Translate) {
  return z
    .object({
      group: z.string().min(1, t('Choose a group')),
      channelId: z.string().min(1, t('Choose a channel')),
      keywords: z
        .string()
        .refine(
          (text) => parseKeywordLines(text).length > 0,
          t('Enter at least one keyword')
        ),
      retryChannelId: z.string().min(1, t('Choose a retry channel')),
    })
    .refine((values) => values.retryChannelId !== values.channelId, {
      path: ['retryChannelId'],
      message: t('The retry channel must differ from the channel'),
    })
}

export type ErrorKeywordRetryRuleFormValues = z.infer<
  ReturnType<typeof createErrorKeywordRetryRuleSchema>
>
