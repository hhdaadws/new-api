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

import {
  formValuesToPayload,
  isValidProbeBody,
  isValidProbeHeaders,
  PROBE_FORM_DEFAULT_VALUES,
  ZERO_INJECTION_TEMPLATE,
} from '../lib/probe-form'

describe('probe headers validation', () => {
  test.each([
    ['', true],
    ['{"X-Probe":"1"}', true],
    ['{"X-Probe":1}', false],
    ['{" ":"1"}', false],
    ['["X-Probe"]', false],
    ['{bad json', false],
  ])('headers %j is valid: %s', (value, expected) => {
    expect(isValidProbeHeaders(value)).toBe(expected)
  })
})

describe('probe body validation', () => {
  test.each([
    ['{"model":"gpt-4o-mini","messages":[]}', true],
    ['{"messages":[]}', false],
    ['{"model":"  "}', false],
    ['{"model":"m","max_tokens":200,}', false],
    ['', false],
  ])('body %j is valid: %s', (value, expected) => {
    expect(isValidProbeBody(value)).toBe(expected)
  })
})

describe('zero injection template', () => {
  test('template body is valid JSON that asks for the system prompt', () => {
    const body = JSON.parse(ZERO_INJECTION_TEMPLATE.body)

    expect(isValidProbeBody(ZERO_INJECTION_TEMPLATE.body)).toBe(true)
    expect(body).toMatchObject({ model: 'claude-opus-5-5', max_tokens: 200 })
    expect(body.messages).toHaveLength(1)
    expect(body.messages[0].content).toContain('NONE')
    expect(ZERO_INJECTION_TEMPLATE.endpoint_type).toBe('openai')
  })
})

describe('probe form payload', () => {
  test('channel id becomes a number and text fields are trimmed', () => {
    const payload = formValuesToPayload({
      ...PROBE_FORM_DEFAULT_VALUES,
      name: '  zero  ',
      channel_id: '12',
      headers: '  ',
      body: ' {"model":"m"} ',
    })

    expect(payload).toEqual({
      name: 'zero',
      channel_id: 12,
      endpoint_type: 'openai',
      interval_seconds: 300,
      enabled: true,
      headers: '',
      body: '{"model":"m"}',
    })
  })
})
