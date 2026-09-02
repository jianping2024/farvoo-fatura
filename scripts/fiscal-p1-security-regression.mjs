#!/usr/bin/env node
/**
 * P1-S security hardening regression (IP rate limit, session secret, anonymous status).
 */
import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, rmSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  loginOperator, envWithCookie, DEFAULT_PIN, runUat, uatJson,
  ensureAdminSession, fiscalAgentTestEnv, FISCAL_UAT_SESSION_SECRET,
} from './fiscal-session-helper.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17884';
const base = `http://${bind}`;
const storeId = 'store-p1s-001';
const dbPath = join(agent, 'data', 'fiscal-p1s.db');
const dataDir = join(agent, 'data', 'fiscal-p1s-secure');
const uat = join(__dirname, 'fiscal-local-uat.mjs');

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

function startAgent(extraEnv = {}) {
  return spawn('go', ['run', './cmd/fiscal-local', '-fiscal-standalone'], {
    cwd: agent,
    env: fiscalAgentTestEnv({
      FISCAL_BIND: bind,
      FISCAL_DB: dbPath,
      FISCAL_DATA_DIR: dataDir,
      FISCAL_STORE_ID: storeId,
      FISCAL_ALLOW_LOCAL_PROVISION: '1',
      FISCAL_AT_ENV: 'mock',
      FISCAL_SEED: '0',
      ...extraEnv,
    }),
    stdio: 'ignore',
  });
}

async function waitHealth(ms = 8000) {
  for (let i = 0; i < 40; i++) {
    try {
      const r = await fetch(`${base}/local/v1/health`);
      if (r.ok) return true;
    } catch { /* retry */ }
    await new Promise((r) => setTimeout(r, 200));
  }
  return false;
}

async function loginFail(operatorId, pin = '000000') {
  const r = await fetch(`${base}/local/v1/setup/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ operator_id: operatorId, pin }),
  });
  let json = {};
  try { json = await r.json(); } catch { /* */ }
  return { status: r.status, json };
}

async function main() {
  mkdirSync(join(agent, 'data'), { recursive: true });
  try { spawnSync('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
  await new Promise((r) => setTimeout(r, 400));
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true });

  const prodProc = spawn('go', ['run', './cmd/fiscal-local'], {
    cwd: agent,
    env: {
      ...process.env,
      FISCAL_BIND: '127.0.0.1:17885',
      FISCAL_DB: join(agent, 'data', 'fiscal-p1s-prod-fail.db'),
      FISCAL_DATA_DIR: join(agent, 'data', 'fiscal-p1s-prod-fail-secure'),
      FISCAL_STORE_ID: 'store-prod-fail',
      FISCAL_AT_ENV: 'mock',
      FISCAL_SEED: '0',
    },
    stdio: 'ignore',
  });
  await new Promise((r) => setTimeout(r, 2500));
  let prodHealthy = false;
  try {
    const r = await fetch('http://127.0.0.1:17885/local/v1/health');
    prodHealthy = r.ok;
  } catch { /* expected */ }
  record('production without SESSION_SECRET does not serve health', !prodHealthy);
  try { prodProc.kill('SIGTERM'); } catch { /* */ }

  const proc = startAgent({ FISCAL_ALLOW_DEV_KEY: '1' });
  try {
    const healthy = await waitHealth();
    record('agent health with UAT secret', healthy);
    if (!healthy) throw new Error('agent not healthy');

    const anon = uatJson(['req', 'GET', '/local/v1/setup/status'], { FISCAL_UAT_BASE: base });
    const forbidden = ['taxpayer_ok', 'series_ok', 'series_code', 'at_credentials_ok', 'validation_code'];
    const anonOk = forbidden.every((k) => anon[k] === undefined);
    record('anonymous status omits sensitive fields', anonOk, forbidden.filter((k) => anon[k] !== undefined).join(','));

    const { cookie: adminCookie, operatorId: adminId } = ensureAdminSession(base, 'P1S Admin', DEFAULT_PIN);
    const adminEnv = envWithCookie(base, adminCookie);
    const full = uatJson(['req', 'GET', '/local/v1/setup/status'], adminEnv);
    record('authenticated status has taxpayer_ok', full.taxpayer_ok !== undefined);

    for (let i = 0; i < 6; i++) {
      await runUat(['req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
        id: `cashier-p1s-${i}`,
        store_id: storeId,
        role: 'cashier',
        display_name: `Cashier P1S ${i}`,
        pin: DEFAULT_PIN,
      })], adminEnv);
    }
    const manage = uatJson(['req', 'GET', '/local/v1/setup/operators/manage'], adminEnv);
    const ids = (manage.operators || []).filter((o) => o.has_pin).map((o) => o.id);
    let hit429 = false;
    let attempts = 0;
    for (const op of ids) {
      for (let i = 0; i < 5; i++) {
        attempts++;
        const res = await loginFail(op);
        if (res.status === 429 && res.json?.error === 'ip_rate_limited') {
          hit429 = true;
          break;
        }
      }
      if (hit429) break;
    }
    if (!hit429 && ids.length > 0) {
      const res = await loginFail(ids[0]);
      attempts++;
      hit429 = res.status === 429 && res.json?.error === 'ip_rate_limited';
    }
    record('31st cross-operator failures → 429 ip_rate_limited', hit429, `attempts ${attempts}`);

    spawnSync('sqlite3', [dbPath, `DELETE FROM audit_log WHERE entity_type='client_ip';`]);
    spawnSync('sqlite3', [dbPath,
      `DELETE FROM audit_log WHERE entity_type='operator' AND entity_id LIKE 'login_fail:%';`]);
    const afterClear = await loginFail(adminId);
    record('after IP audit clear not ip_rate_limited', afterClear.json?.error !== 'ip_rate_limited',
      `status ${afterClear.status} err ${afterClear.json?.error}`);

    for (let i = 0; i < 5; i++) {
      await loginFail(adminId);
    }
    const locked = await loginFail(adminId);
    record('operator locked after 5 failures', locked.status === 429 && locked.json?.error === 'operator_locked',
      `status ${locked.status} err ${locked.json?.error}`);

    spawnSync('sqlite3', [dbPath,
      `DELETE FROM audit_log WHERE entity_type='operator' AND entity_id LIKE 'login_fail:%';`]);
  } finally {
    try { proc.kill('SIGTERM'); } catch { /* */ }
  }

  const failed = results.filter((r) => r.status === 'fail');
  console.log(`\n${results.length - failed.length}/${results.length} passed`);
  if (failed.length) {
    console.error('Failures:', failed.map((f) => f.name).join(', '));
    process.exit(1);
  }
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
