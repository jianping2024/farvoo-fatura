#!/usr/bin/env node
/**
 * M6 product rules regression: FS default, FT+FS scope, NC/ND on FS, 6 payments, ready_to_issue.
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17885';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-m6-product.db');
const dataDir = join(agent, 'data', 'fiscal-m6-product-secure');
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
  return (await run(process.execPath, [uat, ...args], { env: { ...process.env, FISCAL_UAT_BASE: base } })).trim();
}

async function uatJson(...args) {
  return JSON.parse(await uatCmd(...args));
}

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

async function setupFiscal() {
  const pem = readFileSync(pemPath, 'utf8');
  await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
    tax_registration_number: '517535009', legal_name: 'Farvoo Demo Lda',
    address_detail: 'Rua Demo 1', city: 'Lisboa', postal_code: '1000-001',
    country: 'PT', timezone: 'Europe/Lisbon', software_certificate_number: '0',
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
    username: '517535009/37', password: 'demo-secret',
  }));
  for (const [docType, suffix] of [['FT', 'PFT'], ['FS', 'PFS'], ['NC', 'PNC'], ['ND', 'PND']]) {
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `${docType}${year}M6${suffix}`, document_type: docType, fiscal_year: year,
    }));
  }
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({ product_private_key_pem: pem }));
  await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'op-demo-cashier', role: 'cashier', display_name: 'Demo', can_issue_nc: true,
  }));
}

function saleSnapshot(saleId, payMethod = 'CASH') {
  return {
    source_system: 'LOCAL', source_sale_id: saleId, scope_type: 'session', scope_id: `scope-${saleId}`,
    fiscal_purpose: 'sale',
    lines: [{
      product_code: 'DEMO1', display_name: 'Item', saft_name: 'Item',
      quantity: '1', unit_price_gross: '10.00', vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN',
    }],
    customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
    payments: [{ method: payMethod, amount: '10.00' }],
  };
}

async function main() {
  try { await run('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true, force: true });

  const childEnv = { ...process.env, PATH: `/opt/homebrew/bin:${process.env.PATH}` };
  for (const k of Object.keys(childEnv)) if (k.startsWith('FISCAL_')) delete childEnv[k];
  Object.assign(childEnv, {
    FISCAL_DB: dbPath, FISCAL_DATA_DIR: dataDir, FISCAL_BIND: bind,
    FISCAL_STORE_ID: 'store-demo-001', FISCAL_AT_ENV: 'mock', FISCAL_ALLOW_LOCAL_PROVISION: '1',
  });
  const child = spawn('go', ['run', './cmd/fiscal-local'], { cwd: agent, env: childEnv, stdio: ['ignore', 'pipe', 'pipe'] });
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
    await setupFiscal();
    await uatCmd('req', 'POST', '/local/v1/products', '--body', JSON.stringify({
      product_code: 'DEMO1', display_name: 'Demo Item', saft_name: 'Demo Item',
      unit_price_gross: '10.00', vat_rate: '23.00',
    }));
    record('setup-fiscal', true);
  } catch (e) {
    record('setup-fiscal', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    const stFtOnly = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('ready-to-issue-needs-fs', stFtOnly.ready_to_issue === true, `ft=${stFtOnly.series_ok} fs=${stFtOnly.fs_series_ok}`);
  } catch (e) {
    record('ready-to-issue-needs-fs', false, String(e));
  }

  try {
    const def = await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
      request_id: `m6p-default-fs-${Date.now()}`, operator_id: 'op-demo-cashier',
      customer_nif: '999999990', customer_name: 'Consumidor Final', payment_method: 'CASH',
      lines: [{ product_code: 'DEMO1', quantity: '1' }],
    }));
    record('manual-default-fs', def.document_type === 'FS', def.document_type);
  } catch (e) {
    record('manual-default-fs', false, String(e));
  }

  try {
    const bad = await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
      request_id: `m6p-bad-fr-${Date.now()}`, operator_id: 'op-demo-cashier', document_type: 'FR',
      customer_nif: '999999990', lines: [{ product_code: 'DEMO1', quantity: '1' }],
    }));
    record('manual-rejects-fr', false, bad.document_type);
  } catch (e) {
    record('manual-rejects-fr', String(e).includes('document_type') || String(e).includes('400'), String(e).slice(0, 80));
  }

  for (const pay of ['CASH', 'CARD', 'MBWAY', 'MULTIBANCO', 'MIXED', 'OTHER']) {
    try {
      const r = await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
        request_id: `m6p-pay-${pay}-${Date.now()}`, operator_id: 'op-demo-cashier', document_type: 'FS',
        payment_method: pay, customer_nif: '999999990',
        lines: [{ product_code: 'DEMO1', quantity: '1' }],
      }));
      record(`payment-${pay}`, r.document_type === 'FS', r.invoice_no);
    } catch (e) {
      record(`payment-${pay}`, false, String(e).slice(0, 80));
    }
  }

  try {
    const badPay = await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
      request_id: `m6p-bad-pay-${Date.now()}`, operator_id: 'op-demo-cashier',
      payment_method: 'BITCOIN', customer_nif: '999999990',
      lines: [{ product_code: 'DEMO1', quantity: '1' }],
    }));
    record('payment-rejects-unknown', false, JSON.stringify(badPay).slice(0, 60));
  } catch (e) {
    record('payment-rejects-unknown', String(e).includes('payment') || String(e).includes('400'), String(e).slice(0, 80));
  }

  let fs;
  try {
    fs = await uatJson('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify({
      request_id: `m6p-fs-nc-${Date.now()}`, operator_id: 'op-demo-cashier', document_type: 'FS',
      snapshot: saleSnapshot('sale-fs-nc'),
    }));
    const nc = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${fs.document_id}/credit-notes`, '--body', JSON.stringify({
      request_id: 'm6p-nc-fs', operator_id: 'op-demo-cashier', reason: 'Devolucao', credit_full: true,
    }));
    const orig = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${fs.document_id}`);
    record('nc-on-fs', nc.document_type === 'NC' && orig.document_status === 'CREDITED_FULL',
      `orig=${orig.document_status} remaining=${orig.remaining_gross_total}`);
  } catch (e) {
    record('nc-on-fs', false, String(e));
  }

  try {
    const ft = await uatJson('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify({
      request_id: `m6p-ft-nd-${Date.now()}`, operator_id: 'op-demo-cashier', document_type: 'FT',
      snapshot: saleSnapshot('sale-ft-nd'),
    }));
    const nd = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/debit-notes`, '--body', JSON.stringify({
      request_id: 'm6p-nd-ft', operator_id: 'op-demo-cashier', reason: 'Ajuste', debit_full: true,
    }));
    record('nd-on-ft', nd.document_type === 'ND', ft.invoice_no);
  } catch (e) {
    record('nd-on-ft', false, String(e));
  }

  try {
    const emptyType = await uatJson('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify({
      request_id: `m6p-api-default-${Date.now()}`, operator_id: 'op-demo-cashier',
      snapshot: saleSnapshot('sale-api-default'),
    }));
    record('api-empty-doc-type-default-fs', emptyType.document_type === 'FS', emptyType.document_type);
  } catch (e) {
    record('api-empty-doc-type-default-fs', false, String(e));
  }

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n--- summary ---');
  console.log(JSON.stringify({ pass: results.length - failed.length, fail: failed.length, results }, null, 2));
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(1); });
