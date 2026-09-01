#!/usr/bin/env node
/**
 * M3.2 operators + Ops policy regression (local fiscal agent).
 */
import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, rmSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  loginOperator, envWithCookie, DEFAULT_PIN, runUat, uatJson,
  setFiscalProfileViaDb, ensureOwnerSession,
} from './fiscal-session-helper.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17883';
const base = `http://${bind}`;
const storeId = 'store-m32-001';
const dbPath = join(agent, 'data', 'fiscal-m32.db');
const dataDir = join(agent, 'data', 'fiscal-m32-secure');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

function startAgent() {
  return spawn('go', ['run', './cmd/fiscal-local', '-fiscal-standalone'], {
    cwd: agent,
    env: {
      ...process.env,
      FISCAL_BIND: bind,
      FISCAL_DB: dbPath,
      FISCAL_DATA_DIR: dataDir,
      FISCAL_STORE_ID: storeId,
      FISCAL_ALLOW_LOCAL_PROVISION: '1',
      FISCAL_AT_ENV: 'mock',
      FISCAL_SEED: '0',
    },
    stdio: 'ignore',
  });
}

async function waitHealth() {
  for (let i = 0; i < 40; i++) {
    try {
      const r = await fetch(`${base}/local/v1/health`);
      if (r.ok) return;
    } catch { /* retry */ }
    await new Promise((r) => setTimeout(r, 150));
  }
  throw new Error('agent health timeout');
}

async function main() {
  mkdirSync(join(agent, 'data'), { recursive: true });
  try { spawnSync('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
  await new Promise((r) => setTimeout(r, 400));
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true });

  const proc = startAgent();
  try {
    await waitHealth();

    // 1 cold start: operator_ok false until bootstrap
    const st0 = uatJson(['req', 'GET', '/local/v1/setup/status'], { FISCAL_UAT_BASE: base });
    record('operator_ok false before bootstrap', st0.operator_ok === false);

    const { cookie: ownerCookie } = ensureOwnerSession(base, 'Owner One', DEFAULT_PIN);
    let env = envWithCookie(base, ownerCookie);

    const unauth = await fetch(`${base}/local/v1/fiscal-documents`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ request_id: 'x', document_type: 'FS', snapshot: {} }),
    });
    record('unauthenticated issue → 401', unauth.status === 401, `status ${unauth.status}`);

    // setup minimal store
    runUat(['req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
      tax_registration_number: '517535009', legal_name: 'Demo', address_detail: 'Rua 1',
      city: 'Lisboa', postal_code: '1000-001', country: 'PT', timezone: 'Europe/Lisbon',
    })], env);
    runUat(['req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
      username: '517535009/37', password: 'demo-secret',
    })], env);
    setFiscalProfileViaDb(dbPath, 'retail', 2);
    const st = uatJson(['req', 'GET', '/local/v1/setup/status'], env);
    record('fiscal_profile_ok after Ops policy', st.fiscal_profile_ok && st.fiscal_profile === 'retail');
    record('operator_ok after bootstrap', st.operator_ok === true, `operator_ok=${st.operator_ok}`);

    // 2 create cashier
    const cashierBody = {
      id: 'cashier-test-1', role: 'cashier', display_name: 'Cashier A', pin: '654321',
    };
    uatJson(['req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify(cashierBody)], env);
    const cashierCookie = loginOperator(base, 'cashier-test-1', '654321');
    const cashierEnv = envWithCookie(base, cashierCookie);

    // cashier cannot enable nc on self via owner endpoint
    const forbid = spawnSync(process.execPath, [
      join(__dirname, 'fiscal-local-uat.mjs'), 'req', 'PUT', '/local/v1/setup/operator',
      '--body', JSON.stringify({ id: 'cashier-test-1', role: 'cashier', display_name: 'Cashier A', can_issue_nc: true }),
    ], { encoding: 'utf8', env: cashierEnv });
    record('cashier cannot owner PUT', forbid.status !== 0);

    const saftForbid = spawnSync(process.execPath, [
      join(__dirname, 'fiscal-local-uat.mjs'), 'req', 'POST', '/local/v1/saft/exports',
      '--body', JSON.stringify({ year: new Date().getFullYear(), month: new Date().getMonth() + 1 }),
    ], { encoding: 'utf8', env: cashierEnv });
    record('cashier cannot SAFT export', saftForbid.status !== 0);

    // owner resets cashier pin
    uatJson(['req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
      id: 'cashier-test-1', role: 'cashier', display_name: 'Cashier A', pin: '111111',
    })], env);
    let newCashierCookie = loginOperator(base, 'cashier-test-1', '111111');
    record('owner pin reset', !!newCashierCookie);

    const changePin = uatJson(['req', 'POST', '/local/v1/setup/change-pin', '--body', JSON.stringify({
      old_pin: '111111', new_pin: '222222',
    })], envWithCookie(base, newCashierCookie));
    record('cashier can change own pin', changePin.ok === true);
    newCashierCookie = loginOperator(base, 'cashier-test-1', '222222');
    record('cashier login after pin change', !!newCashierCookie);

    uatJson(['req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
      id: 'cashier-test-1', role: 'cashier', display_name: 'Cashier A', pin: '111111',
    })], env);
    newCashierCookie = loginOperator(base, 'cashier-test-1', '111111');

    // activate local signing
    const pem = await import('node:fs').then((m) => m.readFileSync(pemPath, 'utf8'));
    runUat(['req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `FS${new Date().getFullYear()}M32`, document_type: 'FS', fiscal_year: new Date().getFullYear(),
    })], env);
    uatJson(['req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({ product_private_key_pem: pem })], env);

    const issueBody = {
      request_id: `m32-fs-${Date.now()}`,
      document_type: 'FS',
      snapshot: {
        source_system: 'LOCAL', source_sale_id: 's1', scope_type: 'session', scope_id: 'sc1',
        fiscal_purpose: 'sale',
        lines: [{ product_code: 'P1', display_name: 'Item', saft_name: 'Item', quantity: '1', unit_price_gross: '10.00', vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN' }],
        customer: { tax_id: '999999990', company_name: 'CF', country: 'PT' },
        payments: [{ method: 'CASH', amount: '10.00' }],
      },
    };
    const issued = uatJson(['req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify(issueBody)], envWithCookie(base, newCashierCookie));
    record('cashier issue with session', !!issued.document_id);

    const failed = results.filter((r) => r.status === 'fail');
    console.log('\nSummary:', results.length - failed.length, '/', results.length, 'passed');
    if (failed.length) process.exit(1);
  } finally {
    proc.kill('SIGTERM');
  }
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
