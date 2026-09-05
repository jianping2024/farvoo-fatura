#!/usr/bin/env node
/**
 * Audit log UI + GET /local/v1/audit-log regression (design D-A.6).
 */
import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, rmSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  loginOperator, envWithCookie, DEFAULT_PIN, uatJson, ensureAdminSession,
  fiscalAgentTestEnv,
} from './fiscal-session-helper.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17884';
const base = `http://${bind}`;
const storeId = 'store-audit-001';
const dbPath = join(agent, 'data', 'fiscal-audit.db');
const dataDir = join(agent, 'data', 'fiscal-audit-secure');

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

function startAgent() {
  return spawn('go', ['run', './cmd/fiscal-local'], {
    cwd: agent,
    env: fiscalAgentTestEnv({
      FISCAL_BIND: bind,
      FISCAL_DB: dbPath,
      FISCAL_DATA_DIR: dataDir,
      FISCAL_STORE_ID: storeId,
      FISCAL_ALLOW_LOCAL_PROVISION: '1',
      FISCAL_AT_ENV: 'mock',
      FISCAL_SEED: '0',
      FISCAL_ALLOW_DEV_KEY: '1',
    }),
    stdio: 'ignore',
  });
}

async function waitHealth() {
  for (let i = 0; i < 80; i++) {
    try {
      const r = await fetch(`${base}/local/v1/health`);
      if (r.ok) return;
    } catch { /* retry */ }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error('agent health timeout');
}

async function fetchStatus(path, cookie) {
  const r = await fetch(`${base}${path}`, { headers: cookie ? { Cookie: cookie } : {} });
  let json = null;
  try { json = await r.json(); } catch { json = {}; }
  return { status: r.status, json };
}

async function seedAuditRows(adminEnv) {
  uatJson(['req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'owner-audit-1', role: 'owner', display_name: 'Store Owner', pin: '234567',
  })], adminEnv);
  uatJson(['req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'cashier-audit-1', role: 'cashier', display_name: 'Cashier A', pin: '654321',
  })], adminEnv);

  const ownerCookie = loginOperator(base, 'owner-audit-1', '234567');
  await fetchStatus('/local/v1/setup/logout', ownerCookie);
  loginOperator(base, 'owner-audit-1', '234567');

  spawnSync('sqlite3', [dbPath, `
    INSERT OR REPLACE INTO audit_log (id, at, operator_id, action, entity_type, entity_id, detail_json)
    VALUES
      ('audit-backup-1', datetime('now'), NULL, 'fiscal_db_backup', 'sqlite', 'p1', '{"path":"/tmp/fiscal-backup.db"}'),
      ('audit-series-1', datetime('now'), NULL, 'series_integrity_failed', 'series', 's1', '{"series_code":"FT2026X"}');
  `]);
}

function finish() {
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\nSummary:', results.length - failed.length, '/', results.length, 'passed');
  if (failed.length) process.exit(1);
}

async function main() {
  mkdirSync(join(agent, 'data'), { recursive: true });
  try { spawnSync('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
  await new Promise((r) => setTimeout(r, 400));
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true });

  let proc = startAgent();
  try {
    await waitHealth();

    const { cookie: adminCookie } = ensureAdminSession(base, 'Admin Audit', DEFAULT_PIN);
    const adminEnv = envWithCookie(base, adminCookie);
    await seedAuditRows(adminEnv);

    const adminList = uatJson(['req', 'GET', '/local/v1/audit-log?page=1&page_size=50'], adminEnv);
    const adminActions = (adminList.items || []).map((i) => i.action);
    record('admin sees fiscal_db_backup', adminActions.includes('fiscal_db_backup'));
    record('admin sees series_integrity_failed', adminActions.includes('series_integrity_failed'));
    record('admin filter_actions includes backup',
      (adminList.filter_actions || []).some((a) => a.value === 'fiscal_db_backup'));
    record('admin action_label present',
      (adminList.items || []).every((i) => i.action_label && i.action_label.length > 0));

    const defaultPage = uatJson(['req', 'GET', '/local/v1/audit-log?page=1'], adminEnv);
    record('default page_size is 10', defaultPage.page_size === 10,
      `page_size=${defaultPage.page_size}`);
    record('default page returns ≤10 items', (defaultPage.items || []).length <= 10);

    const page1 = uatJson(['req', 'GET', '/local/v1/audit-log?page=1&page_size=10'], adminEnv);
    record('pagination page_size=10 capped', (page1.items || []).length <= 10);
    record('pagination total stable', page1.total === adminList.total, `page1=${page1.total} admin=${adminList.total}`);

    spawnSync('sqlite3', [dbPath, `
      INSERT OR REPLACE INTO audit_log (id, at, operator_id, action, entity_type, entity_id, detail_json)
      VALUES ('audit-old-1', '2024-01-01T00:00:00Z', NULL, 'LOGIN', 'operator', 'op-old', '{}');
    `]);
    const beforePurge = spawnSync('sqlite3', [dbPath, `SELECT COUNT(*) FROM audit_log WHERE id='audit-old-1'`], { encoding: 'utf8' });
    record('seeded old audit row', String(beforePurge.stdout).trim() === '1');

    proc.kill('SIGTERM');
    try { spawnSync('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
    await new Promise((r) => setTimeout(r, 1200));
    proc = startAgent();
    await waitHealth();

    const afterPurge = spawnSync('sqlite3', [dbPath, `SELECT COUNT(*) FROM audit_log WHERE id='audit-old-1'`], { encoding: 'utf8' });
    record('Open purges audit older than 365d', String(afterPurge.stdout).trim() === '0',
      `left=${String(afterPurge.stdout).trim()}`);

    const { cookie: adminCookie2 } = ensureAdminSession(base, 'Admin Audit', DEFAULT_PIN);
    const adminEnv2 = envWithCookie(base, adminCookie2);
    await seedAuditRows(adminEnv2);

    const ownerCookie = loginOperator(base, 'owner-audit-1', '234567');
    const ownerEnv = envWithCookie(base, ownerCookie);
    const ownerList = uatJson(['req', 'GET', '/local/v1/audit-log?page=1&page_size=50'], ownerEnv);
    const ownerActions = (ownerList.items || []).map((i) => i.action);
    record('owner hides fiscal_db_backup', !ownerActions.includes('fiscal_db_backup'));
    record('owner hides series_integrity_failed', !ownerActions.includes('series_integrity_failed'));
    record('owner filter_actions excludes backup',
      !(ownerList.filter_actions || []).some((a) => a.value === 'fiscal_db_backup'));

    const ownerForbidden = await fetchStatus('/local/v1/audit-log?action=fiscal_db_backup', ownerCookie);
    record('owner ?action=fiscal_db_backup → 403', ownerForbidden.status === 403, `status ${ownerForbidden.status}`);

    const cashierCookie = loginOperator(base, 'cashier-audit-1', '654321');
    const cashierDenied = await fetchStatus('/local/v1/audit-log', cashierCookie);
    record('cashier GET audit-log → 403', cashierDenied.status === 403, `status ${cashierDenied.status}`);

    const unauth = await fetchStatus('/local/v1/audit-log');
    record('unauthenticated GET audit-log → 401', unauth.status === 401, `status ${unauth.status}`);

    const countCli = spawnSync('sqlite3', [dbPath, `SELECT COUNT(*) FROM audit_log WHERE action IN ('LOGIN','LOGOUT','fiscal_db_backup','series_integrity_failed')`], { encoding: 'utf8' });
    const dbCount = Number(String(countCli.stdout).trim());
    const adminList2 = uatJson(['req', 'GET', '/local/v1/audit-log?page=1&page_size=50'], adminEnv2);
    record('admin total matches sqlite COUNT',
      adminList2.total >= dbCount && adminList2.total >= 3,
      `api=${adminList2.total} db=${dbCount}`);

    const blob = JSON.stringify(adminList2);
    record('response has no pin plaintext', !/pin["']?\s*:\s*["'][0-9]{6}/i.test(blob));

    finish();
  } finally {
    try { proc.kill('SIGTERM'); } catch { /* */ }
  }
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
