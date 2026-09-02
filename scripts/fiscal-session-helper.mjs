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

/** UAT session secret (≥32 bytes); required when FISCAL_ALLOW_DEV_KEY is unset. */
export const FISCAL_UAT_SESSION_SECRET = 'farvoo-fiscal-uat-session-secret-32b!!';

export function fiscalAgentTestEnv(extra = {}) {
  return {
    ...process.env,
    FISCAL_SESSION_SECRET: FISCAL_UAT_SESSION_SECRET,
    ...extra,
  };
}

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

export function bootstrapAdmin(base, displayName = 'Admin UAT', pin = DEFAULT_PIN) {
  const json = uatJson(
    ['req', 'POST', '/local/v1/setup/bootstrap-owner', '--body', JSON.stringify({ display_name: displayName, pin })],
    { FISCAL_UAT_BASE: base },
  );
  return json.operator_id;
}

/** @deprecated use bootstrapAdmin */
export const bootstrapOwner = bootstrapAdmin;

export function loginOperator(base, operatorId, pin = DEFAULT_PIN) {
  const out = runUat(
    ['login', operatorId, pin],
    { FISCAL_UAT_BASE: base },
  );
  const cookie = extractCookieFromOutput(out);
  if (!cookie) throw new Error('login did not return cookie');
  return cookie;
}

export function ensureAdminSession(base, displayName = 'Admin UAT', pin = DEFAULT_PIN) {
  let operatorId;
  try {
    const ops = uatJson(['req', 'GET', '/local/v1/setup/operators'], { FISCAL_UAT_BASE: base });
    const admins = (ops.operators || []).filter((o) => o.role === 'admin' && o.has_pin);
    if (admins.length) {
      operatorId = admins[0].id;
    }
  } catch {
    operatorId = bootstrapAdmin(base, displayName, pin);
  }
  if (!operatorId) {
    operatorId = bootstrapAdmin(base, displayName, pin);
  }
  const cookie = loginOperator(base, operatorId, pin);
  return { cookie, operatorId };
}

/** @deprecated use ensureAdminSession */
export function ensureOwnerSession(base, displayName = 'Admin UAT', pin = DEFAULT_PIN) {
  return ensureAdminSession(base, displayName, pin);
}

export function envWithCookie(base, cookie) {
  return { FISCAL_UAT_BASE: base, FISCAL_UAT_COOKIE: cookie };
}

export function setFiscalProfileViaDb(dbPath, profile = 'restaurant', max = 3) {
  runUat(['exec-db', '--db', dbPath, '--sql',
    `UPDATE taxpayer_settings SET fiscal_profile='${profile}', max_fiscal_terminals=${max}, ops_policy_synced_at=datetime('now')`]);
}
