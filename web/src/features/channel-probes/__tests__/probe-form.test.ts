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
import type { TFunction } from 'i18next'
import { describe, expect, test } from 'vitest'

import {
  formValuesToPayload,
  getProbeFormSchema,
  isValidProbeHeaders,
  PROBE_FORM_DEFAULT_VALUES,
  SIGNATURE_TAMPER_TEMPLATE,
  ZERO_INJECTION_TEMPLATE,
  type ProbeFormValues,
} from '../lib/probe-form'

const schema = getProbeFormSchema(((key: string) => key) as TFunction)

function issues(overrides: Partial<ProbeFormValues>) {
  const parsed = schema.safeParse({
    ...PROBE_FORM_DEFAULT_VALUES,
    name: 'probe',
    channel_id: '1',
    ...overrides,
  })
  return parsed.success
    ? []
    : parsed.error.issues.map((issue) => [issue.path.join('.'), issue.message])
}

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

describe('probe request validation', () => {
  test.each([
    {
      name: 'body with a model',
      values: { body: '{"model":"m","messages":[]}' },
      expected: [],
    },
    {
      name: 'selected models stand in for the body model',
      values: { body: '{"messages":[]}', models: ['a', 'b'] },
      expected: [],
    },
    {
      name: 'no model anywhere',
      values: { body: '{"model":"  "}' },
      expected: [
        [
          'models',
          'Select at least one model or set a model in the request body',
        ],
      ],
    },
    {
      name: 'trailing comma',
      values: { body: '{"model":"m","max_tokens":200,}' },
      expected: [['body', 'Request body must be a JSON object']],
    },
    {
      name: 'signature probe in another format',
      values: {
        probe_type: 'signature' as const,
        endpoint_type: 'openai' as const,
        body: '{"model":"m"}',
      },
      expected: [
        [
          'endpoint_type',
          'Signature probes must use the Anthropic Messages format',
        ],
      ],
    },
    {
      name: 'streaming signature probe',
      values: {
        probe_type: 'signature' as const,
        endpoint_type: 'anthropic' as const,
        body: '{"model":"m","stream":true}',
      },
      expected: [['body', 'Signature probes must not stream']],
    },
  ])('$name', (testCase) => {
    expect(issues(testCase.values)).toEqual(testCase.expected)
  })
})

describe('probe templates', () => {
  test('zero injection template asks for the system prompt', () => {
    const body = JSON.parse(ZERO_INJECTION_TEMPLATE.body)

    expect(issues({ body: ZERO_INJECTION_TEMPLATE.body })).toEqual([])
    expect(body).toMatchObject({ model: 'claude-opus-5-5', max_tokens: 200 })
    expect(body.messages[0].content).toContain('NONE')
    expect(ZERO_INJECTION_TEMPLATE.probe_type).toBe('custom')
  })

  test('signature tamper template is a valid non-streaming thinking request', () => {
    const body = JSON.parse(SIGNATURE_TAMPER_TEMPLATE.body)

    expect(
      issues({
        probe_type: SIGNATURE_TAMPER_TEMPLATE.probe_type,
        endpoint_type: SIGNATURE_TAMPER_TEMPLATE.endpoint_type,
        body: SIGNATURE_TAMPER_TEMPLATE.body,
      })
    ).toEqual([])
    expect(SIGNATURE_TAMPER_TEMPLATE.probe_type).toBe('signature')
    expect(body.thinking).toEqual({ type: 'adaptive' })
    expect(body.stream).toBeUndefined()
  })
})

describe('probe form payload', () => {
  test('channel id becomes a number, models are joined and text is trimmed', () => {
    const payload = formValuesToPayload({
      ...PROBE_FORM_DEFAULT_VALUES,
      name: '  zero  ',
      channel_id: '12',
      models: ['claude-opus-5-5', 'claude-sonnet-5-5'],
      headers: '  ',
      body: ' {"model":"m"} ',
    })

    expect(payload).toEqual({
      name: 'zero',
      channel_id: 12,
      probe_type: 'custom',
      endpoint_type: 'openai',
      models: 'claude-opus-5-5,claude-sonnet-5-5',
      interval_seconds: 300,
      enabled: true,
      headers: '',
      body: '{"model":"m"}',
    })
  })
})
