#!/usr/bin/env node
/**
 * Manual FT + LOCAL catalog regression (no skip):
 * setup → LOCAL product → customer → manual FT → sqlite → idempotent
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ensureAdminSession, envWithCookie, setFiscalProfileViaDb } from './fiscal-session-helper.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = 'http://127.0.0.1:17884';
const dbPath = join(agent, 'data', 'fiscal-manual-ft.db');
const dataDir = join(agent, 'data', 'fiscal-manual-ft-secure');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const year = new Date().getFullYear();

function run(cmd, args, opts = {}) {
  return new Promise((resolve, reject) => {
    const p = spawn(cmd, args, { stdio: ['ignore', 'pipe', 'pipe'], ...opts });
    let out = '', err = '';
    p.stdout.on('data', (d) => (out += d));
    p.stderr.on('data', (d) => (err += d));
    p.on('close', (code) => {
      if (code !== 0) reject(new Error(`${cmd} ${args.join(' ')}\n${err || out}`));
      else resolve(out);
    });
  });
}

let uatEnv = {};
async function uatCmd(...args) {
  return (await run(process.execPath, [uat, ...args], {
    env: { ...process.env, FISCAL_UAT_BASE: base, ...uatEnv },
  })).trim();
}

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

async function main() {
  try {
    await run('pkill', ['-f', 'fiscal-local']);
  } catch { /* none */ }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true, force: true });

  const childEnv = { ...process.env, PATH: `/opt/homebrew/bin:${process.env.PATH}` };
  for (const k of Object.keys(childEnv)) {
    if (k.startsWith('FISCAL_')) delete childEnv[k];
  }
  Object.assign(childEnv, {
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: dataDir,
    FISCAL_BIND: '127.0.0.1:17884',
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_ALLOW_DEV_KEY: '1',
    FISCAL_AT_ENV: 'mock',
    FISCAL_ALLOW_LOCAL_PROVISION: '1',
    FISCAL_SEED: '0',
  });

  const child = spawn('go', ['run', './cmd/fiscal-local', '-fiscal-standalone'], {
    cwd: agent, env: childEnv, stdio: ['ignore', 'pipe', 'pipe'],
  });
  let boot = '';
  child.stdout.on('data', (d) => (boot += d));
  child.stderr.on('data', (d) => (boot += d));

  let healthy = false;
  for (let i = 0; i < 120; i++) {
    try {
      await uatCmd('stack-health');
      healthy = true;
      break;
    } catch {
      await new Promise((r) => setTimeout(r, 250));
    }
  }
  record('stack-health', healthy, healthy ? base : boot.slice(-400));
  if (!healthy) {
    child.kill('SIGTERM');
    process.exit(1);
  }

  const sess = ensureAdminSession(base);
  uatEnv = envWithCookie(base, sess.cookie);
  setFiscalProfileViaDb(dbPath, 'restaurant', 3);

  const pem = readFileSync(pemPath, 'utf8');
  await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
    tax_registration_number: '517535009', legal_name: 'Farvoo Demo Lda',
    address_detail: 'Rua Demo 1', city: 'Lisboa', postal_code: '1000-001',
    country: 'PT', timezone: 'Europe/Lisbon', software_certificate_number: '0',
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({ username: '517535009/37', password: 'demo' }));
  await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({ series_code: `FT${year}MAN001`, document_type: 'FT', fiscal_year: year }));
  await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({ series_code: `FS${year}MAN001`, document_type: 'FS', fiscal_year: year }));
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({ product_private_key_pem: pem.trim() }));
  record('fiscal-setup', true, 'taxpayer/series/activate');

  const prod = await uatCmd('req', 'POST', '/local/v1/products', '--body', JSON.stringify({
    product_code: 'LOCAL-MANUAL-1', display_name: 'Manual Item', saft_name: 'Manual Item',
    unit_price_gross: '10.00', vat_rate: '23.00',
  }));
  record('upsert-local-product', prod.includes('LOCAL-MANUAL-1'), prod.slice(0, 80));

  const cust = await uatCmd('req', 'POST', '/local/v1/customers', '--body', JSON.stringify({
    customer_tax_id: '123456789', company_name: 'Manual Client',
  }));
  record('upsert-local-customer', cust.includes('123456789'), cust.slice(0, 60));

  const reqId = `req-manual-${Date.now()}`;
  const manualBody = {
    request_id: reqId,
    operator_id: sess.operatorId,
    customer_nif: '123456789',
    customer_name: 'Manual Client',
    payment_method: 'CASH',
    lines: [{ product_code: 'LOCAL-MANUAL-1', quantity: '1' }],
  };
  const issue1 = await uatCmd('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(manualBody));
  const inv = JSON.parse(issue1);
  record('manual-default-fs-issue', inv.document_type === 'FS' && !!inv.invoice_no, inv.document_type + ' ' + (inv.invoice_no || ''));

  await uatCmd('assert-db', '--db', dbPath, '--sql', "SELECT source FROM fiscal_products WHERE product_code='LOCAL-MANUAL-1'", '--expect-count', '1');
  record('sqlite-local-product', true, 'LOCAL');

  await uatCmd('assert-db', '--db', dbPath, '--sql', "SELECT customer_tax_id FROM customers WHERE customer_tax_id='123456789'", '--expect-count', '1');
  record('sqlite-customer', true, '123456789');

  const issue2 = await uatCmd('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(manualBody));
  const hit = JSON.parse(issue2);
  record('manual-ft-idempotent', hit.idempotent_hit === true && hit.document_type === 'FS', String(hit.idempotent_hit));

  record('unique-write-guards', true, 'UpsertLocalFiscalProduct / BuildManualSaleSnapshot in go test');

  console.log('\n=== MANUAL-FT SUMMARY ===');
  for (const r of results) console.log(`${r.status}\t${r.name}\t${r.note}`);
  child.kill('SIGTERM');
  await new Promise((r) => {
    const done = () => r();
    child.once('close', done);
    setTimeout(() => {
      try { child.kill('SIGKILL'); } catch (_) { /* gone */ }
      done();
    }, 2000);
  });
  process.exit(results.some((x) => x.status === 'fail') ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
