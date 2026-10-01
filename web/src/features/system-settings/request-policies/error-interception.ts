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

export const MIN_INTERCEPTION_STATUS_CODE = 400
export const MAX_INTERCEPTION_STATUS_CODE = 599

/**
 * Convert the stored keyword list (a JSON array of strings) into the
 * one-keyword-per-line text shown in the editor. Values that are not a JSON
 * array produce an empty list; non-string entries are dropped.
 */
export function interceptionKeywordsToText(stored: string): string {
  let parsed: unknown
  try {
    parsed = JSON.parse(stored)
  } catch {
    return ''
  }
  if (!Array.isArray(parsed)) return ''
  return parsed
    .filter((keyword): keyword is string => typeof keyword === 'string')
    .join('\n')
}

/**
 * Convert editor text into the stored JSON array. Lines are trimmed, blank
 * lines are dropped, and duplicates are removed ignoring case (matching is
 * case-insensitive), keeping the first spelling.
 */
export function interceptionKeywordsFromText(text: string): string {
  const seen = new Set<string>()
  const keywords: string[] = []
  for (const line of text.split(/\r?\n/)) {
    const keyword = line.trim()
    const normalized = keyword.toLowerCase()
    if (!keyword || seen.has(normalized)) continue
    seen.add(normalized)
    keywords.push(keyword)
  }
  return JSON.stringify(keywords)
}

export function createErrorInterceptionSchema(t: Translate) {
  const statusCodeMessage = t('Enter a status code between 400 and 599')
  return z.object({
    enabled: z.boolean(),
    keywords: z.string(),
    statusCode: z
      .number(statusCodeMessage)
      .int(statusCodeMessage)
      .min(MIN_INTERCEPTION_STATUS_CODE, statusCodeMessage)
      .max(MAX_INTERCEPTION_STATUS_CODE, statusCodeMessage),
    message: z.string().trim().min(1, t('Error message is required')),
    retryOnMatch: z.boolean(),
  })
}

export type ErrorInterceptionFormInput = z.input<
  ReturnType<typeof createErrorInterceptionSchema>
>
export type ErrorInterceptionFormValues = z.output<
  ReturnType<typeof createErrorInterceptionSchema>
>
