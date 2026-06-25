import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const paymentSettingPath = resolve(
  scriptDir,
  '../src/components/settings/PaymentSetting.jsx',
);
const source = readFileSync(paymentSettingPath, 'utf8');

assert.match(
  source,
  /WaffoPancakeWebhookPublicKey:\s*''/,
  'PaymentSetting should include a default WaffoPancakeWebhookPublicKey value.',
);
assert.match(
  source,
  /WaffoPancakeWebhookTestKey:\s*''/,
  'PaymentSetting should include a default WaffoPancakeWebhookTestKey value.',
);
assert.match(
  source,
  /case 'WaffoPancakeWebhookPublicKey':[\s\S]*case 'WaffoPancakeWebhookTestKey':/,
  'PaymentSetting should read Waffo Pancake webhook key options as string fields.',
);
assert.match(
  source,
  /<Tabs\.TabPane tab=\{t\('Waffo Pancake 设置'\)\} itemKey='waffo-pancake'>/,
  'PaymentSetting should expose the Waffo Pancake settings tab.',
);
assert.doesNotMatch(
  source,
  /\{\/\*<Tabs\.TabPane tab=\{t\('Waffo Pancake 设置'\)\}/,
  'The Waffo Pancake settings tab should not be commented out.',
);
