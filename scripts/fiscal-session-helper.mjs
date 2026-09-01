#!/usr/bin/env node
/**
 * Shared fiscal session helpers for regression scripts.
 * Uses fiscal-local-uat.mjs with cookie jar (FISCAL_UAT_COOKIE).
 */
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const uat = join(__dirname, 'fiscal-local-uat.mjs');

export const DEFAULT_PIN = '123456';

export function runUat(args, env = {}) {
  const r = spawnSync(process.execPath, [uat, ...args], {
    env: { ...process.env, ...env },
    encoding: 'utf8',
  });
  if (r.status !== 0) {
    throw new Error((r.stderr || r.stdout || 'uat failed').trim());
  }
  return (r.stdout || '').trim();
}

export function uatJson(args, env = {}) {
  return JSON.parse(runUat(args, env));
}

export function extractCookieFromOutput(text) {
  const line = String(text).split('\n').find((l) => l.startsWith('FISCAL_UAT_COOKIE='));
  if (!line) return '';
  return line.slice('FISCAL_UAT_COOKIE='.length).trim();
}

export function bootstrapOwner(base, displayName = 'Owner UAT', pin = DEFAULT_PIN) {
  const json = uatJson(
    ['req', 'POST', '/local/v1/setup/bootstrap-owner', '--body', JSON.stringify({ display_name: displayName, pin })],
    { FISCAL_UAT_BASE: base },
  );
  return json.operator_id;
}

export function loginOperator(base, operatorId, pin = DEFAULT_PIN) {
  const out = runUat(
    ['login', operatorId, pin],
    { FISCAL_UAT_BASE: base },
  );
  const cookie = extractCookieFromOutput(out);
  if (!cookie) throw new Error('login did not return cookie');
  return cookie;
}

export function ensureOwnerSession(base, displayName = 'Owner UAT', pin = DEFAULT_PIN) {
  let operatorId;
  try {
    const ops = uatJson(['req', 'GET', '/local/v1/setup/operators'], { FISCAL_UAT_BASE: base });
    const owners = (ops.operators || []).filter((o) => o.role === 'owner' && o.has_pin);
    if (owners.length) {
      operatorId = owners[0].id;
    }
  } catch {
    operatorId = bootstrapOwner(base, displayName, pin);
  }
  if (!operatorId) {
    operatorId = bootstrapOwner(base, displayName, pin);
  }
  const cookie = loginOperator(base, operatorId, pin);
  return { cookie, operatorId };
}

export function envWithCookie(base, cookie) {
  return { FISCAL_UAT_BASE: base, FISCAL_UAT_COOKIE: cookie };
}

export function setFiscalProfileViaDb(dbPath, profile = 'restaurant', max = 3) {
  runUat(['exec-db', '--db', dbPath, '--sql',
    `UPDATE taxpayer_settings SET fiscal_profile='${profile}', max_fiscal_terminals=${max}, ops_policy_synced_at=datetime('now')`]);
}
