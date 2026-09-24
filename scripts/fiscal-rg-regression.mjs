#!/usr/bin/env node
/**
 * RG (recibo) regression: ACCOUNT FT → partial/full RG, rejects, SAF-T Payments.
 */
import { ensureOwnerSession, setFiscalProfileViaDb, envWithCookie, fiscalAgentTestEnv } from './fiscal-session-helper.mjs';
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17886';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-rg.db');
const dataDir = join(agent, 'data', 'fiscal-rg-secure');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const year = new Date().getFullYear();
const month = new Date().getMonth() + 1;

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

let uatEnv = { FISCAL_UAT_BASE: base };

async function uatCmd(...args) {
  return (
    await run(process.execPath, [uat, ...args], {
      env: { ...process.env, ...uatEnv },
    })
  ).trim();
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
  const sess = ensureOwnerSession(base);
  uatEnv = envWithCookie(base, sess.cookie);
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
  for (const [docType, suffix] of [['FT', 'FT01'], ['FS', 'FS01'], ['NC', 'NC01'], ['RG', 'RG01']]) {
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `${docType}${year}RG${suffix}`, document_type: docType, fiscal_year: year,
    }));
  }
  setFiscalProfileViaDb(dbPath, 'restaurant', 3);
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({
    product_private_key_pem: pem,
  }));
  const ops = await uatJson('req', 'GET', '/local/v1/setup/operators');
  const op = (ops.operators || [])[0];
  if (op) {
    await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
      id: op.id, role: op.role, display_name: op.display_name, can_issue_nc: true,
    }));
  }
}

async function issueFTAccount(amount) {
  const requestId = `rg-ft-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
  return uatJson('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify({
    request_id: requestId,
    document_type: 'FT',
    snapshot: {
      source_system: 'LOCAL',
      source_sale_id: `sale-${requestId}`,
      scope_type: 'session',
      scope_id: `scope-${requestId}`,
      fiscal_purpose: 'sale',
      lines: [{
        product_code: 'DEMO1', display_name: 'Prato Demo', saft_name: 'Prato Demo',
        quantity: '1', unit_price_gross: amount, vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN',
      }],
      customer: { tax_id: '509442013', company_name: 'Cliente Conta', country: 'PT' },
      payments: [{ method: 'ACCOUNT', amount }],
    },
  }));
}

async function expectErr(fn, code) {
  try {
    await fn();
    return false;
  } catch (e) {
    const msg = String(e && e.message ? e.message : e);
    return msg.includes(code);
  }
}

async function main() {
  try { await run('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true, force: true });

  const childEnv = fiscalAgentTestEnv({
    PATH: `/opt/homebrew/bin:${process.env.PATH}`,
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: dataDir,
    FISCAL_BIND: bind,
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_AT_ENV: 'mock',
    FISCAL_ALLOW_LOCAL_PROVISION: '1',
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
    const st = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('ready_to_receipt', !!st.ready_to_receipt, `rg_series_ok=${st.rg_series_ok}`);

    const ft = await issueFTAccount('100.00');
    const detail0 = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${ft.document_id}`);
    record('RG-01 remaining 100', detail0.remaining_receivable_total === '100.00', detail0.remaining_receivable_total);

    const rg1 = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/receipts`, '--body', JSON.stringify({
      request_id: 'rg-partial-40', receive_full: false, amount: '40.00', payment_method: 'CARD',
    }));
    record('RG-02 partial 40', rg1.document_type === 'RG' && String(rg1.invoice_no).includes('RG'), rg1.invoice_no);

    const rg2 = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/receipts`, '--body', JSON.stringify({
      request_id: 'rg-partial-60', receive_full: false, amount: '60.00', payment_method: 'CASH',
    }));
    record('RG-02 partial 60', rg2.document_type === 'RG', rg2.invoice_no);

    const detail1 = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${ft.document_id}`);
    record('balance zero', detail1.remaining_receivable_total === '0.00' && detail1.received_gross_total === '100.00');

    record('RG-03 over reject', await expectErr(
      () => uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/receipts`, '--body', JSON.stringify({
        request_id: 'rg-over', receive_full: false, amount: '1.00', payment_method: 'CASH',
      })),
      'receipt_amount_exceeded',
    ));

    const fsIssue = await uatJson('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify({
      request_id: 'rg-fs-' + Date.now(), document_type: 'FS',
      snapshot: {
        source_system: 'LOCAL', source_sale_id: 'sale-fs', scope_type: 'session', scope_id: 's-fs', fiscal_purpose: 'sale',
        lines: [{ product_code: 'DEMO1', display_name: 'X', saft_name: 'X', quantity: '1', unit_price_gross: '10.00', vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN' }],
        customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
        payments: [{ method: 'CASH', amount: '10.00' }],
      },
    }));
    record('RG-04 FS reject', await expectErr(
      () => uatJson('req', 'POST', `/local/v1/fiscal-documents/${fsIssue.document_id}/receipts`, '--body', JSON.stringify({
        request_id: 'rg-on-fs', receive_full: true, payment_method: 'CASH',
      })),
      'receipt_not_allowed',
    ));

    const ft2 = await issueFTAccount('70.00');
    await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft2.document_id}/credit-notes`, '--body', JSON.stringify({
      request_id: 'nc-rg-' + Date.now(), reason: 'Desconto', credit_full: false,
      lines: [{ original_line_number: 1, line_gross: '20.00' }],
    }));
    const d2 = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${ft2.document_id}`);
    record('RG-07 remaining after NC', d2.remaining_receivable_total === '50.00', d2.remaining_receivable_total);

    record('RG-07 over after NC', await expectErr(
      () => uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft2.document_id}/receipts`, '--body', JSON.stringify({
        request_id: 'rg-71', amount: '51.00', payment_method: 'CASH',
      })),
      'receipt_amount_exceeded',
    ));

    const rgIdem = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft2.document_id}/receipts`, '--body', JSON.stringify({
      request_id: 'rg-idem-same', receive_full: true, payment_method: 'CASH',
    }));
    const rgIdem2 = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft2.document_id}/receipts`, '--body', JSON.stringify({
      request_id: 'rg-idem-same', receive_full: true, payment_method: 'CASH',
    }));
    record('RG-09 idempotent', rgIdem.document_id === rgIdem2.document_id && rgIdem2.idempotent_hit === true);

    const exp = await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({ year, month }));
    const dl = await uatCmd('req', 'GET', `/local/v1/saft/exports/${exp.export_id}/download`);
    record('SAF-T has Payments RG', dl.includes('<PaymentType>RG</PaymentType>') && dl.includes('<Payments>'));
    record('SAF-T sales exclude RG', !dl.includes('<InvoiceType>RG</InvoiceType>'));
  } finally {
    child.kill('SIGTERM');
  }

  const failed = results.filter((r) => r.status !== 'pass');
  if (failed.length) {
    console.error(`FAILED ${failed.length}/${results.length}`);
    process.exit(1);
  }
  console.log(`OK ${results.length}/${results.length}`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
