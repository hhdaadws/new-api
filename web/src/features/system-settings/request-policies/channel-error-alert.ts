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

type Translate = (key: string) => string

export const MAX_ALERT_COOLDOWN_MINUTES = 1440
export const MAX_PUSHPLUS_TOKENS = 50

/**
 * Split the token editor text into distinct tokens. Tokens are separated by
 * line breaks, commas, or spaces; blank entries and repeats are dropped.
 */
export function parsePushPlusTokens(text: string): string[] {
  return [...new Set(text.split(/[\s,，]+/).filter(Boolean))]
}

export function createChannelErrorAlertSchema(t: Translate) {
  const thresholdMessage = t('Enter a positive integer')
  const cooldownMessage = t('Enter a whole number of minutes from 0 to 1440')
  return z.object({
    enabled: z.boolean(),
    threshold: z
      .number(thresholdMessage)
      .int(thresholdMessage)
      .min(1, thresholdMessage),
    cooldownMinutes: z
      .number(cooldownMessage)
      .int(cooldownMessage)
      .min(0, cooldownMessage)
      .max(MAX_ALERT_COOLDOWN_MINUTES, cooldownMessage),
    tokens: z
      .string()
      .refine(
        (value) => parsePushPlusTokens(value).length <= MAX_PUSHPLUS_TOKENS,
        t('Enter at most 50 PushPlus tokens')
      ),
  })
}

export type ChannelErrorAlertFormInput = z.input<
  ReturnType<typeof createChannelErrorAlertSchema>
>
export type ChannelErrorAlertFormValues = z.output<
  ReturnType<typeof createChannelErrorAlertSchema>
>
