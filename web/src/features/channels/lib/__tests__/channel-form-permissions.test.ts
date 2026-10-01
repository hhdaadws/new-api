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
import { describe, expect, test } from 'vitest'

import { CHANNEL_TYPE_VLLM } from '../../constants'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
  type ChannelFormValues,
} from '../channel-form'

function formValues(overrides: Partial<ChannelFormValues>): ChannelFormValues {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'Primary',
    key: 'sk-test',
    models: 'gpt-4o',
    ...overrides,
  }
}

describe('channel alias payload', () => {
  test('update payload carries the trimmed alias when the super admin saves', () => {
    const payload = transformFormDataToUpdatePayload(
      formValues({ alias: '  Vendor A  ' }),
      7,
      { includeAlias: true }
    )

    expect(payload.alias).toBe('Vendor A')
  })

  test('update payload sends an empty alias so the super admin can clear it', () => {
    const payload = transformFormDataToUpdatePayload(
      formValues({ alias: '' }),
      7,
      { includeAlias: true }
    )

    expect(payload.alias).toBe('')
  })

  test('update payload omits the alias key for other administrators', () => {
    const payload = transformFormDataToUpdatePayload(
      formValues({ alias: 'Vendor A' }),
      7
    )

    expect(payload).not.toHaveProperty('alias')
  })

  test('create payload carries the alias only when the super admin creates', () => {
    const values = formValues({ alias: 'Vendor A' })

    expect(
      transformFormDataToCreatePayload(values, { includeAlias: true }).channel
        .alias
    ).toBe('Vendor A')
    expect(
      transformFormDataToCreatePayload(values, { includeAlias: false }).channel
    ).not.toHaveProperty('alias')
  })

  test('rejects an alias longer than 255 characters', () => {
    const result = channelFormSchema.safeParse(
      formValues({ alias: 'a'.repeat(256) })
    )

    expect(result.success).toBe(false)
    expect(result.error?.issues.map((issue) => issue.path[0])).toContain(
      'alias'
    )
  })
})

describe('hidden base URL validation', () => {
  test('requires a base URL for vLLM when the stored value is visible', () => {
    const result = channelFormSchema.safeParse(
      formValues({ type: CHANNEL_TYPE_VLLM, base_url: '' })
    )

    expect(result.success).toBe(false)
    expect(result.error?.issues.map((issue) => issue.path[0])).toContain(
      'base_url'
    )
  })

  test('accepts an empty base URL when the stored value is hidden', () => {
    const result = channelFormSchema.safeParse(
      formValues({
        type: CHANNEL_TYPE_VLLM,
        base_url: '',
        base_url_hidden: true,
      })
    )

    expect(result.success).toBe(true)
  })
})
