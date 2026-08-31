#!/usr/bin/env node
/**
 * M6 regression: FS/FR issue + ND debit + NC still works + SAF-T includes ND.
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17884';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-m6.db');
const dataDir = join(agent, 'data', 'fiscal-m6-secure');
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
  for (const [docType, suffix] of [['FT', 'FT01'], ['NC', 'NC01'], ['ND', 'ND01'], ['FS', 'FS01'], ['FR', 'FR01']]) {
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `${docType}${year}M6${suffix}`, document_type: docType, fiscal_year: year,
    }));
  }
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({ product_private_key_pem: pem }));
  await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'op-demo-cashier', role: 'cashier', display_name: 'Demo', can_issue_nc: true,
  }));
}

function saleLine(name) {
  return {
    product_code: 'DEMO1', display_name: name, saft_name: name,
    quantity: '1', unit_price_gross: '10.00', vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN',
  };
}

async function issueDoc(docType, saleId) {
  const requestId = `m6-${docType}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
  return uatJson('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify({
    request_id: requestId, operator_id: 'op-demo-cashier', document_type: docType,
    snapshot: {
      source_system: 'LOCAL', source_sale_id: saleId, scope_type: 'session', scope_id: `scope-${requestId}`,
      fiscal_purpose: 'sale',
      lines: [saleLine(`Item ${docType}`)],
      customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
      payments: [{ method: 'CASH', amount: '10.00' }],
    },
  }));
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
    record('setup-fiscal', true);
  } catch (e) {
    record('setup-fiscal', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  let ft, fs, fr;
  try {
    ft = await issueDoc('FT', 'sale-ft');
    fs = await issueDoc('FS', 'sale-fs');
    fr = await issueDoc('FR', 'sale-fr');
    record('issue-ft-fs-fr', ft.document_type === 'FT' && fs.document_type === 'FS' && fr.document_type === 'FR');
  } catch (e) {
    record('issue-ft-fs-fr', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    const nc = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/credit-notes`, '--body', JSON.stringify({
      request_id: 'm6-nc-1', operator_id: 'op-demo-cashier', reason: 'Devolucao', credit_full: true,
    }));
    record('nc-on-ft', nc.document_type === 'NC');
  } catch (e) {
    record('nc-on-ft', false, String(e));
  }

  try {
    const nd = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${fs.document_id}/debit-notes`, '--body', JSON.stringify({
      request_id: 'm6-nd-1', operator_id: 'op-demo-cashier', reason: 'Ajuste', debit_full: true,
    }));
    const orig = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${fs.document_id}`);
    record('nd-on-fs', nd.document_type === 'ND' && orig.document_status === 'DEBITED_PARTIAL');
  } catch (e) {
    record('nd-on-fs', false, String(e));
  }

  try {
    const ftPartial = await issueDoc('FT', 'sale-ft-partial');
    const ndp = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ftPartial.document_id}/debit-notes`, '--body', JSON.stringify({
      request_id: 'm6-nd-partial', operator_id: 'op-demo-cashier', reason: 'Partial',
      debit_full: false, lines: [{ original_line_number: 1, line_gross: '5.00' }],
    }));
    const origP = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${ftPartial.document_id}`);
    record('nd-partial-on-ft', ndp.document_type === 'ND' && origP.document_status === 'DEBITED_PARTIAL');
  } catch (e) {
    record('nd-partial-on-ft', false, String(e));
  }

  try {
    const ftOver = await issueDoc('FT', 'sale-ft-over');
    const ndOver = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ftOver.document_id}/debit-notes`, '--body', JSON.stringify({
      request_id: 'm6-nd-over', operator_id: 'op-demo-cashier', reason: 'Above original',
      debit_full: false, lines: [{ original_line_number: 1, line_gross: '20.00' }],
    }));
    const origOver = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${ftOver.document_id}`);
    record('nd-allows-above-original',
      ndOver.document_type === 'ND' && origOver.debited_gross_total === '20.00' && origOver.document_status === 'DEBITED_PARTIAL',
      `debited=${origOver.debited_gross_total}`);
  } catch (e) {
    record('nd-allows-above-original', false, String(e));
  }

  try {
    const ftZero = await issueDoc('FT', 'sale-ft-zero');
    await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ftZero.document_id}/debit-notes`, '--body', JSON.stringify({
      request_id: 'm6-nd-zero', operator_id: 'op-demo-cashier', reason: 'Zero',
      debit_full: false, lines: [{ original_line_number: 1, line_gross: '0.00' }],
    }));
    record('nd-zero-rejected', false, 'expected 4xx');
  } catch (e) {
    const msg = String(e);
    record('nd-zero-rejected', msg.includes('debit_amount_exceeded') || msg.includes('409') || msg.includes('400'), msg.slice(0, 120));
  }

  try {
    const exp = await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({
      operator_id: 'op-demo-cashier', year, month,
    }));
    const xml = await uatCmd('req', 'GET', `/local/v1/saft/exports/${exp.export_id}/download`, '--raw');
    const hasTypes = ['<InvoiceType>FT</InvoiceType>', '<InvoiceType>NC</InvoiceType>',
      '<InvoiceType>FS</InvoiceType>', '<InvoiceType>FR</InvoiceType>', '<InvoiceType>ND</InvoiceType>']
      .every((t) => xml.includes(t));
    record('saft-includes-m6-types', hasTypes && exp.invoice_count >= 5, `count=${exp.invoice_count}`);
  } catch (e) {
    record('saft-includes-m6-types', false, String(e));
  }

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n--- summary ---');
  console.log(JSON.stringify({ pass: results.length - failed.length, fail: failed.length, results }, null, 2));
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(1); });
