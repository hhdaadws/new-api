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
  createErrorKeywordRetryRuleSchema,
  parseErrorKeywordRetryRules,
} from '../error-keyword-retry'

const rule = {
  group: 'default',
  channel_id: 1,
  keywords: ['context length'],
  retry_channel_id: 2,
}

describe('parseErrorKeywordRetryRules', () => {
  it('stored rules keep their saved order', () => {
    const second = { ...rule, keywords: ['quota'], retry_channel_id: 3 }
    expect(parseErrorKeywordRetryRules(JSON.stringify([rule, second]))).toEqual(
      [rule, second]
    )
  })

  it.each(['', 'not json', '{}', '"rules"', 'null'])(
    'a stored value that is not a JSON array (%j) gives no rules',
    (stored) => {
      expect(parseErrorKeywordRetryRules(stored)).toEqual([])
    }
  )

  it('entries without a group, both channels or a keyword are dropped', () => {
    const stored = JSON.stringify([
      { ...rule, group: '' },
      { ...rule, channel_id: 0 },
      { ...rule, retry_channel_id: '2' },
      { ...rule, keywords: [] },
      null,
      rule,
    ])
    expect(parseErrorKeywordRetryRules(stored)).toEqual([rule])
  })

  it('non-string keywords are dropped from a rule', () => {
    const stored = JSON.stringify([{ ...rule, keywords: [1, 'quota', null] }])
    expect(parseErrorKeywordRetryRules(stored)).toEqual([
      { ...rule, keywords: ['quota'] },
    ])
  })
})

describe('createErrorKeywordRetryRuleSchema', () => {
  const schema = createErrorKeywordRetryRuleSchema((key) => key)
  const valid = {
    group: 'default',
    channelId: '1',
    keywords: 'context length',
    retryChannelId: '2',
  }

  it('a complete rule is accepted', () => {
    expect(schema.safeParse(valid).success).toBe(true)
  })

  it('keywords made of blank lines are rejected', () => {
    const result = schema.safeParse({ ...valid, keywords: ' \n\n  ' })
    expect(result.success).toBe(false)
    expect(result.error?.issues[0]).toMatchObject({
      path: ['keywords'],
      message: 'Enter at least one keyword',
    })
  })

  it('a retry channel equal to the channel is rejected on the retry channel', () => {
    const result = schema.safeParse({ ...valid, retryChannelId: '1' })
    expect(result.success).toBe(false)
    expect(result.error?.issues[0]).toMatchObject({
      path: ['retryChannelId'],
      message: 'The retry channel must differ from the channel',
    })
  })

  it.each([
    ['group', 'Choose a group'],
    ['channelId', 'Choose a channel'],
    ['retryChannelId', 'Choose a retry channel'],
  ])('an empty %s is rejected with its message', (field, message) => {
    const result = schema.safeParse({ ...valid, [field]: '' })
    expect(result.success).toBe(false)
    expect(result.error?.issues[0]).toMatchObject({ path: [field], message })
  })
})
