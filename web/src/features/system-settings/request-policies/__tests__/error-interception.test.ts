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
import { describe, expect, it } from 'vitest'

import {
  createErrorInterceptionSchema,
  interceptionKeywordsFromText,
  interceptionKeywordsToText,
} from '../error-interception'

const schema = createErrorInterceptionSchema((key) => key)
const validValues = {
  enabled: true,
  keywords: 'quota exceeded',
  statusCode: 502,
  message: 'bad response',
  retryOnMatch: false,
}

describe('interceptionKeywordsToText', () => {
  it('a stored JSON array becomes one keyword per line', () => {
    expect(interceptionKeywordsToText('["quota exceeded","rate limit"]')).toBe(
      'quota exceeded\nrate limit'
    )
  })

  it('an empty stored array becomes an empty editor', () => {
    expect(interceptionKeywordsToText('[]')).toBe('')
  })

  it.each(['', 'not json', '{"keyword":"x"}', '"quota"'])(
    'a stored value that is not a JSON array (%j) becomes an empty editor',
    (stored) => {
      expect(interceptionKeywordsToText(stored)).toBe('')
    }
  )

  it('non-string entries in the stored array are dropped', () => {
    expect(interceptionKeywordsToText('["quota",1,null,"limit"]')).toBe(
      'quota\nlimit'
    )
  })
})

describe('interceptionKeywordsFromText', () => {
  it('lines are trimmed and blank lines are dropped', () => {
    expect(
      interceptionKeywordsFromText('  quota exceeded \n\n   \nrate limit\n')
    ).toBe('["quota exceeded","rate limit"]')
  })

  it('duplicates differing only in case keep the first spelling', () => {
    expect(
      interceptionKeywordsFromText('Quota Exceeded\nquota exceeded\nQUOTA')
    ).toBe('["Quota Exceeded","QUOTA"]')
  })

  it('Windows line endings split keywords like Unix ones', () => {
    expect(interceptionKeywordsFromText('quota\r\nlimit')).toBe(
      '["quota","limit"]'
    )
  })

  it('an empty editor is stored as an empty JSON array', () => {
    expect(interceptionKeywordsFromText('')).toBe('[]')
  })

  it('keywords with quotes and backslashes survive a round trip', () => {
    const stored = interceptionKeywordsFromText('say "no"\nC:\\path')
    expect(interceptionKeywordsToText(stored)).toBe('say "no"\nC:\\path')
  })
})

describe('createErrorInterceptionSchema', () => {
  it.each([400, 502, 599])('status code %i is accepted', (statusCode) => {
    expect(schema.safeParse({ ...validValues, statusCode }).success).toBe(true)
  })

  it.each([399, 600, 200, 502.5])(
    'status code %d is rejected with the range message',
    (statusCode) => {
      const result = schema.safeParse({ ...validValues, statusCode })
      expect(result.success).toBe(false)
      expect(result.error?.issues[0]).toMatchObject({
        path: ['statusCode'],
        message: 'Enter a status code between 400 and 599',
      })
    }
  )

  it('a blank error message is rejected', () => {
    const result = schema.safeParse({ ...validValues, message: '   ' })
    expect(result.success).toBe(false)
    expect(result.error?.issues[0]).toMatchObject({
      path: ['message'],
      message: 'Error message is required',
    })
  })

  it('the error message is trimmed before saving', () => {
    const result = schema.parse({ ...validValues, message: '  try later  ' })
    expect(result.message).toBe('try later')
  })
})
