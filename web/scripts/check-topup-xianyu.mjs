import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const rechargeCardPath = resolve(
  scriptDir,
  '../src/components/topup/RechargeCard.jsx',
);
const source = readFileSync(rechargeCardPath, 'utf8');

assert.match(
  source,
  /description=\{\s*xianyuLink\s*\?/s,
  'RechargeCard Banner should branch on xianyuLink before using the generic disabled-topup message.',
);
assert.match(
  source,
  /管理员未开启在线充值功能，请到以下闲鱼店铺充值：/,
  'RechargeCard should show the xianyu recharge prompt when xianyuLink is configured.',
);
assert.match(
  source,
  /onClick=\{openXianyuLink\}/,
  'RechargeCard should use the provided xianyu link opener from the disabled-topup banner.',
);
assert.match(
  source,
  /管理员未开启在线充值功能，请联系管理员开启或使用兑换码充值。/,
  'RechargeCard should keep the generic disabled-topup fallback when no xianyuLink is configured.',
);
