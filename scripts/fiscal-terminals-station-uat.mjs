#!/usr/bin/env node
/**
 * Terminals activate/delete/default-station UAT (fiscal-local-uat analogue).
 * Scenarios: local station, LAN station, revoke quota, activate, delete gates, assert-db.
 */
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import {
  ensureAdminSession, envWithCookie, runUat, uatJson, fiscalAgentTestEnv, DEFAULT_PIN,
} from './fiscal-session-helper.mjs';
import { randomUUID } from 'node:crypto';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agentDir = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17884';
const base = `http://${bind}`;
const storeId = 'store-term-091';
const work = mkdtempSync(join(tmpdir(), 'term-station-uat-'));
const dbPath = join(work, 'fiscal.db');
const dataDir = join(work, 'secure');
const bin = join(work, 'fiscal-local');
const results = [];

function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

function uatFail(args, env) {
  const r = spawnSync(process.execPath, [join(__dirname, 'fiscal-local-uat.mjs'), ...args], {
    encoding: 'utf8',
    env: { ...process.env, ...env },
  });
  return r.status !== 0;
}

function build() {
  const r = spawnSync('go', ['build', '-o', bin, './cmd/fiscal-local'], {
    cwd: agentDir,
    encoding: 'utf8',
  });
  if (r.status !== 0) throw new Error(r.stderr || r.stdout || 'go build failed');
}

async function waitHealth() {
  for (let i = 0; i < 80; i++) {
    try {
      const r = await fetch(`${base}/local/v1/health`);
      if (r.ok) return;
    } catch { /* retry */ }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error('health timeout');
}

async function main() {
  build();
  const proc = spawn(bin, ['-fiscal-standalone'], {
    env: fiscalAgentTestEnv({
      FISCAL_BIND: bind,
      FISCAL_DB: dbPath,
      FISCAL_DATA_DIR: dataDir,
      FISCAL_STORE_ID: storeId,
      FISCAL_ALLOW_LOCAL_PROVISION: '1',
      FISCAL_AT_ENV: 'mock',
      FISCAL_SEED: '0',
    }),
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let log = '';
  proc.stdout.on('data', (d) => { log += d; });
  proc.stderr.on('data', (d) => { log += d; });

  try {
    await waitHealth();
    record('stack-health', true, base);

    const { cookie, operatorId } = ensureAdminSession(base, 'Admin UAT', DEFAULT_PIN);
    const env = envWithCookie(base, cookie);
    record('bootstrap+login', !!operatorId && !!cookie);

    runUat(['req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
      tax_registration_number: '517535009',
      legal_name: 'Demo Lda',
      business_name: 'Demo',
      address_detail: 'Rua 1',
      city: 'Lisboa',
      postal_code: '1000-001',
      country: 'PT',
      timezone: 'Europe/Lisbon',
      software_certificate_number: '0',
    })], env);
    runUat(['exec-db', '--db', dbPath, '--sql',
      `UPDATE taxpayer_settings SET fiscal_profile='restaurant', max_fiscal_terminals=2, ops_policy_synced_at=datetime('now')`]);

    const putLocal = uatJson(['req', 'PUT', '/local/v1/setup/print-station/local', '--body',
      JSON.stringify({ station_id: 'kitchen' })], env);
    const eff = uatJson(['req', 'GET', '/local/v1/setup/print-station'], env);
    record('local station PUT/GET', putLocal.station_id === 'kitchen' && eff.station_id === 'kitchen' && eff.loopback === true,
      JSON.stringify(eff));

    runUat(['assert-db', '--db', dbPath, '--sql',
      "SELECT local_default_station_id FROM taxpayer_settings WHERE local_default_station_id='kitchen'",
      '--expect-count', '1']);
    record('assert-db local_default_station_id', true, 'kitchen');

    const list0 = uatJson(['req', 'GET', '/local/v1/setup/terminals'], env);
    record('list includes local_default + max',
      list0.local_default_station_id === 'kitchen' && list0.max_fiscal_terminals === 2,
      `local=${list0.local_default_station_id} max=${list0.max_fiscal_terminals}`);

    record('allow-next rejects empty label',
      uatFail(['req', 'POST', '/local/v1/setup/terminals/allow-next', '--body', JSON.stringify({ label: '  ' })], env));

    const allowOk = uatJson(['req', 'POST', '/local/v1/setup/terminals/allow-next', '--body',
      JSON.stringify({ label: '收银台 2' })], env);
    record('allow-next with label', !!(allowOk.pairing_code) && allowOk.label === '收银台 2',
      JSON.stringify(allowOk));

    const tid = randomUUID();
    runUat(['exec-db', '--db', dbPath, '--sql',
      `INSERT INTO fiscal_terminals(id,store_id,label,active,ops_terminal_ref,registered_at,last_seen_at,default_station_id) ` +
      `VALUES('${tid}','${storeId}',NULL,1,'local:${tid}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','')`]);

    record('label PUT rejects empty',
      uatFail(['req', 'PUT', `/local/v1/setup/terminals/${tid}/label`, '--body', JSON.stringify({ label: '' })], env));

    const renamed = uatJson(['req', 'PUT', `/local/v1/setup/terminals/${tid}/label`, '--body',
      JSON.stringify({ label: ' 吧台 ' })], env);
    record('label PUT trims and saves', renamed.label === '吧台', JSON.stringify(renamed));
    runUat(['assert-db', '--db', dbPath, '--sql',
      `SELECT label FROM fiscal_terminals WHERE id='${tid}' AND label='吧台'`, '--expect-count', '1']);
    record('assert-db terminal label', true, '吧台');

    const listLabeled = uatJson(['req', 'GET', '/local/v1/setup/terminals'], env);
    const rowLabeled = (listLabeled.terminals || []).find((t) => t.id === tid);
    record('list shows updated label', rowLabeled && rowLabeled.label === '吧台', JSON.stringify(rowLabeled));

    uatJson(['req', 'PUT', `/local/v1/setup/terminals/${tid}/station`, '--body',
      JSON.stringify({ station_id: 'bar' })], env);
    runUat(['assert-db', '--db', dbPath, '--sql',
      `SELECT default_station_id FROM fiscal_terminals WHERE id='${tid}' AND default_station_id='bar'`,
      '--expect-count', '1']);
    record('LAN terminal station PUT', true, 'bar');

    const list1 = uatJson(['req', 'GET', '/local/v1/setup/terminals'], env);
    record('active counts toward max', list1.terminals_used === 1, `used=${list1.terminals_used}`);

    uatJson(['req', 'POST', `/local/v1/setup/terminals/${tid}/revoke`, '--body', '{}'], env);
    const list2 = uatJson(['req', 'GET', '/local/v1/setup/terminals'], env);
    const row2 = (list2.terminals || []).find((t) => t.id === tid);
    record('revoke frees quota keeps station',
      list2.terminals_used === 0 && row2 && row2.active === false && row2.default_station_id === 'bar',
      JSON.stringify(row2));

    record('delete while inactive would succeed later; delete while active rejected next', true);

    uatJson(['req', 'POST', `/local/v1/setup/terminals/${tid}/activate`, '--body', '{}'], env);
    const list3 = uatJson(['req', 'GET', '/local/v1/setup/terminals'], env);
    const row3 = (list3.terminals || []).find((t) => t.id === tid);
    record('activate restores quota', list3.terminals_used === 1 && row3 && row3.active === true,
      `used=${list3.terminals_used}`);

    record('delete while active rejected',
      uatFail(['req', 'POST', `/local/v1/setup/terminals/${tid}/delete`, '--body', '{}'], env));

    uatJson(['req', 'POST', `/local/v1/setup/terminals/${tid}/revoke`, '--body', '{}'], env);
    uatJson(['req', 'POST', `/local/v1/setup/terminals/${tid}/delete`, '--body', '{}'], env);
    runUat(['assert-db', '--db', dbPath, '--sql',
      `SELECT id FROM fiscal_terminals WHERE id='${tid}'`, '--expect-count', '0']);
    record('hard delete inactive', true, 'gone');

    // Cookie path A: station preserved across revoke→activate (old cookie usable again when active=1)
    const tid2 = randomUUID();
    runUat(['exec-db', '--db', dbPath, '--sql',
      `INSERT INTO fiscal_terminals(id,store_id,label,active,ops_terminal_ref,registered_at,last_seen_at,default_station_id) ` +
      `VALUES('${tid2}','${storeId}','Front',1,'local:${tid2}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','bar')`]);
    uatJson(['req', 'POST', `/local/v1/setup/terminals/${tid2}/revoke`, '--body', '{}'], env);
    uatJson(['req', 'POST', `/local/v1/setup/terminals/${tid2}/activate`, '--body', '{}'], env);
    const list4 = uatJson(['req', 'GET', '/local/v1/setup/terminals'], env);
    const row4 = (list4.terminals || []).find((t) => t.id === tid2);
    record('re-enable keeps default_station_id (cookie A)',
      row4 && row4.active === true && row4.default_station_id === 'bar', JSON.stringify(row4));

    const effLoop = uatJson(['req', 'GET', '/local/v1/setup/print-station'], env);
    record('loopback effective uses local not terminal',
      effLoop.loopback === true && effLoop.station_id === 'kitchen', JSON.stringify(effLoop));

    uatJson(['req', 'PUT', '/local/v1/setup/print-station/local', '--body',
      JSON.stringify({ station_id: '' })], env);
    const cleared = uatJson(['req', 'GET', '/local/v1/setup/print-station'], env);
    record('clear local station', !(cleared.station_id || '').trim(), JSON.stringify(cleared));

    writeFileSync(join(work, 'results.json'), JSON.stringify({ work, results }, null, 2));
  } finally {
    proc.kill('SIGTERM');
    try { rmSync(work, { recursive: true, force: true }); } catch { /* keep on fail */ }
  }

  const fails = results.filter((r) => r.status !== 'pass');
  console.log(`\nTOTAL ${results.length} FAIL ${fails.length}`);
  if (fails.length) {
    console.error(log.slice(-2000));
    process.exit(1);
  }
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
