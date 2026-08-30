#!/usr/bin/env node
/**
 * M3 NC regression: FT series + NC series → FT → full NC → idempotency → CREDITED_FULL → print payload.
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = process.env.FISCAL_UAT_BASE || 'http://127.0.0.1:17882';
const bind = '127.0.0.1:17882';
const dbPath = join(agent, 'data', 'fiscal-m3.db');
const dataDir = join(agent, 'data', 'fiscal-m3-secure');
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

async function uatCmd(...args) {
  return (
    await run(process.execPath, [uat, ...args], {
      env: { ...process.env, FISCAL_UAT_BASE: base },
    })
  ).trim();
}

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

async function setupFiscal(childEnv) {
  const pem = readFileSync(pemPath, 'utf8');
  await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
    tax_registration_number: '517535009',
    legal_name: 'Farvoo Demo Lda',
    address_detail: 'Rua Demo 1',
    city: 'Lisboa',
    postal_code: '1000-001',
    country: 'PT',
    timezone: 'Europe/Lisbon',
    software_certificate_number: '0',
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
    username: '517535009/37', password: 'demo-secret',
  }));
  await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
    series_code: `FT${year}M3FT01`, document_type: 'FT', fiscal_year: year,
  }));
  await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
    series_code: `NC${year}M3NC01`, document_type: 'NC', fiscal_year: year,
  }));
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({
    product_private_key_pem: pem,
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'op-demo-cashier', role: 'cashier', display_name: 'Demo Cashier',
  }));
}

async function main() {
  try { await run('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
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
    FISCAL_BIND: bind,
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_AT_ENV: 'mock',
    FISCAL_ALLOW_LOCAL_PROVISION: '1',
  });
  const child = spawn('go', ['run', './cmd/fiscal-local'], {
    cwd: agent, env: childEnv, stdio: ['ignore', 'pipe', 'pipe'],
  });
  let boot = '';
  child.stdout.on('data', (d) => (boot += d));
  child.stderr.on('data', (d) => (boot += d));

  let healthy = false;
  for (let i = 0; i < 120; i++) {
    if (child.exitCode != null && child.exitCode !== 0) break;
    try { await uatCmd('stack-health'); healthy = true; break; }
    catch { await new Promise((r) => setTimeout(r, 250)); }
  }
  record('stack-health', healthy, healthy ? base : boot.slice(-500));
  if (!healthy) { child.kill('SIGTERM'); process.exit(1); }

  try {
    await setupFiscal(childEnv);
    record('setup-ft-nc-series', true);
  } catch (e) {
    record('setup-ft-nc-series', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT 1 FROM series WHERE document_type='NC' AND status='ACTIVE' AND validation_code IS NOT NULL AND validation_code != '';`,
      '--expect-count', '1');
    record('sqlite-nc-series', true);
  } catch (e) {
    record('sqlite-nc-series', false, String(e));
  }

  const requestId = `m3-ft-${Date.now()}`;
  const ftBody = {
    request_id: requestId,
    operator_id: 'op-demo-cashier',
    document_type: 'FT',
    snapshot: {
      source_system: 'LOCAL',
      source_sale_id: `sale-${requestId}`,
      scope_type: 'session',
      scope_id: `scope-${requestId}`,
      fiscal_purpose: 'sale',
      lines: [{
        product_code: 'DEMO1', display_name: 'Prato Demo', saft_name: 'Prato Demo',
        quantity: '1', unit_price_gross: '12.50', vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN',
      }],
      customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
      payments: [{ method: 'CASH', amount: '12.50' }],
    },
  };

  let ft;
  try {
    ft = JSON.parse(await uatCmd('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify(ftBody)));
    record('issue-ft', ft.document_status === 'SIGNED', ft.invoice_no);
  } catch (e) {
    record('issue-ft', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT 1 FROM series WHERE document_type='FT' AND last_number=1 AND document_type='FT';`,
      '--expect-count', '1');
    record('ft-series-seq-1', true);
  } catch (e) {
    record('ft-series-seq-1', false, String(e));
  }

  const ncReqId = `m3-nc-${Date.now()}`;
  const ncBody = {
    request_id: ncReqId,
    operator_id: 'op-demo-cashier',
    reason: 'Devolucao total M3',
    credit_full: true,
  };
  let nc;
  try {
    nc = JSON.parse(await uatCmd('req', 'POST',
      `/local/v1/fiscal-documents/${ft.document_id}/credit-notes`, '--body', JSON.stringify(ncBody)));
    record('issue-nc-full', nc.document_type === 'NC' && String(nc.invoice_no).includes('NC'), nc.invoice_no);
  } catch (e) {
    record('issue-nc-full', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT 1 FROM series WHERE document_type='NC' AND last_number=1;`,
      '--expect-count', '1');
    record('nc-series-independent', true);
  } catch (e) {
    record('nc-series-independent', false, String(e));
  }

  try {
    const again = JSON.parse(await uatCmd('req', 'POST',
      `/local/v1/fiscal-documents/${ft.document_id}/credit-notes`, '--body', JSON.stringify(ncBody)));
    record('nc-idempotent', again.idempotent_hit === true && again.document_id === nc.document_id);
  } catch (e) {
    record('nc-idempotent', false, String(e));
  }

  try {
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT 1 FROM invoices WHERE id='${ft.document_id}' AND document_status='CREDITED_FULL' AND credited_gross_total='12.50';`,
      '--expect-count', '1');
    record('original-credited-full', true);
  } catch (e) {
    record('original-credited-full', false, String(e));
  }

  try {
    await uatCmd('req', 'POST',
      `/local/v1/fiscal-documents/${ft.document_id}/credit-notes`,
      '--body', JSON.stringify({
        request_id: `m3-nc-dup-${Date.now()}`,
        operator_id: 'op-demo-cashier',
        reason: 'Too late',
        credit_full: false,
        lines: [{ original_line_number: 1, line_gross: '1.00' }],
      }));
    record('credit-full-rejected', false, 'expected credit_not_allowed');
  } catch (e) {
    record('credit-full-rejected', String(e).includes('credit_not_allowed'), String(e).slice(-120));
  }

  try {
    await uatCmd('wait-json', 'GET', `/local/v1/print-jobs/${nc.print_job_id}`,
      '--path', 'job_status', '--equals', 'PRINTED', '--timeout-ms', '5000');
    record('nc-print-printed', true);
  } catch (e) {
    record('nc-print-printed', false, String(e));
  }

  try {
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT 1 FROM local_print_jobs WHERE invoice_id='${nc.document_id}' AND payload_json LIKE '%original_invoice_no%' AND payload_json LIKE '%credit_reason%';`,
      '--expect-count', '1');
    record('nc-print-payload-fields', true);
  } catch (e) {
    record('nc-print-payload-fields', false, String(e));
  }

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n=== M3 SUMMARY ===');
  for (const r of results) console.log(`${r.status}\t${r.name}\t${r.note}`);
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
