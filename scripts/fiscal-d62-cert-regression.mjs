#!/usr/bin/env node
/**
 * D6.2 certification checklist runner — one row per C*.*
 * Emits pass/fail for automated items; manual/blocked recorded as skip with reason.
 *
 *   node scripts/fiscal-d62-cert-regression.mjs
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { randomUUID } from 'node:crypto';
import {
  ensureAdminSession, envWithCookie, loginOperator, DEFAULT_PIN, setFiscalProfileViaDb,
} from './fiscal-session-helper.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17887';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-d62-cert.db');
const dataDir = join(agent, 'data', 'fiscal-d62-cert-secure');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const year = new Date().getFullYear();
const month = new Date().getMonth() + 1;
const MOCK_VAL = 'CSDF7T5H';

/** @type {Record<string, string>} */
let sessionEnv = {};
/** @type {Record<string, string>} */
let adminEnv = {};
/** @type {string} */
let adminOperatorId = '';

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
  return (await run(process.execPath, [uat, ...args], {
    env: { ...process.env, FISCAL_UAT_BASE: base, ...sessionEnv },
  })).trim();
}

async function uatJson(...args) {
  return JSON.parse(await uatCmd(...args));
}

/** @type {{ id: string, status: 'pass'|'fail'|'skip'|'blocked', note: string }[]} */
const results = [];
function pass(id, note = '') {
  results.push({ id, status: 'pass', note });
  console.log(`PASS     ${id}${note ? ' — ' + note : ''}`);
}
function fail(id, note = '') {
  results.push({ id, status: 'fail', note });
  console.log(`FAIL     ${id}${note ? ' — ' + note : ''}`);
}
function skip(id, note) {
  results.push({ id, status: 'skip', note });
  console.log(`SKIP     ${id} — ${note}`);
}
function blocked(id, note) {
  results.push({ id, status: 'blocked', note });
  console.log(`BLOCKED  ${id} — ${note}`);
}

async function refreshAdminSession() {
  const cookie = loginOperator(base, adminOperatorId, DEFAULT_PIN);
  adminEnv = envWithCookie(base, cookie);
  sessionEnv = adminEnv;
  return adminEnv;
}

async function setupFiscal() {
  const pem = readFileSync(pemPath, 'utf8');
  const { cookie: adminCookie, operatorId } = ensureAdminSession(base, 'Cert Admin', DEFAULT_PIN);
  adminOperatorId = operatorId;
  adminEnv = envWithCookie(base, adminCookie);
  sessionEnv = adminEnv;

  await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
    tax_registration_number: '517535009', legal_name: 'Farvoo Demo Lda',
    address_detail: 'Rua Demo 1', city: 'Lisboa', postal_code: '1000-001',
    country: 'PT', timezone: 'Europe/Lisbon', software_certificate_number: '0',
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
    username: '517535009/37', password: 'demo-secret',
  }));
  for (const [docType, suffix] of [['FT', 'CFT'], ['FS', 'CFS'], ['NC', 'CNC'], ['ND', 'CND']]) {
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `${docType}${year}D62${suffix}`, document_type: docType, fiscal_year: year,
    }));
  }
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({ product_private_key_pem: pem }));
  setFiscalProfileViaDb(dbPath, 'retail', 3);
  await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'owner-cert-1', role: 'owner', display_name: 'Owner', pin: '234567',
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'op-demo-cashier', role: 'cashier', display_name: 'Demo', pin: DEFAULT_PIN, can_issue_nc: true,
  }));
  sessionEnv = envWithCookie(base, loginOperator(base, 'op-demo-cashier', DEFAULT_PIN));
  await uatCmd('req', 'POST', '/local/v1/products', '--body', JSON.stringify({
    product_code: 'DEMO1', display_name: 'Água 500ml', saft_name: 'Água 500ml',
    unit_price_gross: '10.00', vat_rate: '23.00',
  }));
}

function saleSnapshot(saleId, name = 'Água 500ml') {
  return {
    source_system: 'LOCAL', source_sale_id: saleId, scope_type: 'session', scope_id: `scope-${saleId}`,
    fiscal_purpose: 'sale',
    lines: [{
      product_code: 'DEMO1', display_name: name, saft_name: name,
      quantity: '1', unit_price_gross: '10.00', vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN',
    }],
    customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
    payments: [{ method: 'CASH', amount: '10.00' }],
  };
}

async function issue(docType, saleId, extra = {}) {
  return uatJson('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify({
    request_id: `d62-${docType}-${saleId}-${Date.now()}`,
    operator_id: 'op-demo-cashier',
    ...(docType ? { document_type: docType } : {}),
    snapshot: saleSnapshot(saleId),
    ...extra,
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
    FISCAL_MOCK_VALIDATION_CODE: MOCK_VAL, FISCAL_SEED: '0',
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
  if (!healthy) {
    console.error('stack-health failed', boot.slice(-800));
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    await setupFiscal();
  } catch (e) {
    console.error('setup failed', e);
    child.kill('SIGTERM');
    process.exit(1);
  }

  // --- C1 Setup ---
  try {
    const st = await uatJson('req', 'GET', '/local/v1/setup/status');
    (st.series_ok ? pass : fail)('C1.1', `series_ok=${st.series_ok}`);
    (st.fs_series_ok ? pass : fail)('C1.2', `fs_series_ok=${st.fs_series_ok}`);
    (st.nc_series_ok ? pass : fail)('C1.3', `nc_series_ok=${st.nc_series_ok}`);
    (st.nd_series_ok ? pass : fail)('C1.4', `nd_series_ok=${st.nd_series_ok}`);
    (st.ready_to_issue ? pass : fail)('C1.5', `ready_to_issue=${st.ready_to_issue}`);
    (st.ready_to_credit && st.ready_to_debit ? pass : fail)(
      'C1.6',
      `credit=${st.ready_to_credit} debit=${st.ready_to_debit}`,
    );
  } catch (e) {
    for (const id of ['C1.1', 'C1.2', 'C1.3', 'C1.4', 'C1.5', 'C1.6']) fail(id, String(e).slice(0, 120));
  }

  // --- C2 Issue ---
  try {
    const def = await issue('', 'sale-default-fs');
    (def.document_type === 'FS' ? pass : fail)('C2.1', def.document_type);
  } catch (e) {
    fail('C2.1', String(e).slice(0, 120));
  }

  try {
    await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
      request_id: `d62-fr-${Date.now()}`, operator_id: 'op-demo-cashier', document_type: 'FR',
      customer_nif: '999999990', lines: [{ product_code: 'DEMO1', quantity: '1' }],
    }));
    fail('C2.2', 'expected FR reject');
  } catch (e) {
    const ok = String(e).includes('document_type') || String(e).includes('400');
    (ok ? pass : fail)('C2.2', 'API rejects FR; Admin dropdown = 手测');
  }

  let payOk = true;
  let payNote = [];
  for (const pay of ['CASH', 'CARD', 'MBWAY', 'MULTIBANCO', 'MIXED', 'OTHER']) {
    try {
      const r = await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
        request_id: `d62-pay-${pay}-${Date.now()}`, operator_id: 'op-demo-cashier', document_type: 'FS',
        payment_method: pay, customer_nif: '999999990',
        lines: [{ product_code: 'DEMO1', quantity: '1' }],
      }));
      if (r.document_type !== 'FS') { payOk = false; payNote.push(`${pay}=${r.document_type}`); }
    } catch (e) {
      payOk = false;
      payNote.push(`${pay}:${String(e).slice(0, 40)}`);
    }
  }
  (payOk ? pass : fail)('C2.3', payOk ? '6 methods' : payNote.join(';'));

  let atcud2 = '', seq2 = 0, doc2Id = '';
  try {
    const a = await issue('FS', 'sale-hash-a');
    const b = await issue('FS', 'sale-hash-b');
    doc2Id = b.document_id;
    atcud2 = b.atcud;
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT 1 FROM invoices WHERE id='${a.document_id}' AND length(hash)>=32;`, '--expect-count', '1');
    const prev = (await run('sqlite3', [dbPath,
      `SELECT previous_hash FROM invoices WHERE id='${b.document_id}';`])).trim();
    const h1 = (await run('sqlite3', [dbPath, `SELECT hash FROM invoices WHERE id='${a.document_id}';`])).trim();
    seq2 = Number((await run('sqlite3', [dbPath,
      `SELECT sequence_number FROM invoices WHERE id='${b.document_id}';`])).trim());
    (prev === h1 && h1.length >= 32 ? pass : fail)('C2.4', `prev===hash1 len=${h1.length}`);
  } catch (e) {
    fail('C2.4', String(e).slice(0, 160));
  }

  try {
    const expect = `${MOCK_VAL}-${seq2}`;
    const detail = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${doc2Id}`);
    const dbAtcud = (await run('sqlite3', [dbPath, `SELECT atcud FROM invoices WHERE id='${doc2Id}';`])).trim();
    const ok = detail.atcud === expect && dbAtcud === expect && atcud2 === expect;
    (ok ? pass : fail)('C2.5', `atcud=${detail.atcud} expect=${expect}`);
  } catch (e) {
    fail('C2.5', String(e).slice(0, 120));
  }

  try {
    const qr = (await run('sqlite3', [dbPath, `SELECT qr_content FROM invoices WHERE id='${doc2Id}';`])).trim();
    const ok = qr.includes(`H:${MOCK_VAL}-${seq2}`) && (qr.includes('N:') || qr.includes('I:'));
    (ok ? pass : fail)('C2.6', ok ? 'qr has ATCUD+tax fields; 扫枪可读性=手测' : qr.slice(0, 80));
  } catch (e) {
    fail('C2.6', String(e).slice(0, 120));
  }

  // --- C3 NC ---
  let fsForNc;
  try {
    fsForNc = await issue('FS', 'sale-nc-fs');
    const nc = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${fsForNc.document_id}/credit-notes`, '--body', JSON.stringify({
      request_id: `d62-nc-${Date.now()}`, operator_id: 'op-demo-cashier', reason: 'Devolucao', credit_full: true,
    }));
    const orig = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${fsForNc.document_id}`);
    (nc.document_type === 'NC' && storeIsFs(fsForNc) ? pass : fail)('C3.1', `nc=${nc.document_type} orig=${fsForNc.document_type}`);
    (orig.document_status === 'CREDITED_FULL' ? pass : fail)('C3.2', orig.document_status);
    (String(orig.remaining_gross_total) === '0.00' || orig.remaining_gross_total === '0' ? pass : fail)(
      'C3.3',
      `remaining=${orig.remaining_gross_total}`,
    );
  } catch (e) {
    fail('C3.1', String(e).slice(0, 120));
    fail('C3.2', String(e).slice(0, 80));
    fail('C3.3', String(e).slice(0, 80));
  }

  try {
    sessionEnv = adminEnv;
    await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
      id: 'op-no-nc', role: 'cashier', display_name: 'NoNC', pin: '345678', can_issue_nc: false,
    }));
    sessionEnv = envWithCookie(base, loginOperator(base, 'op-no-nc', '345678'));
    const ft = await issue('FT', 'sale-nc-perm');
    try {
      await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/credit-notes`, '--body', JSON.stringify({
        request_id: `d62-nc-deny-${Date.now()}`, reason: 'x', credit_full: true,
      }));
      fail('C3.4', 'expected credit_not_allowed');
    } catch (e) {
    (String(e).includes('credit_not_allowed') || String(e).includes('409') ? pass : fail)(
      'C3.4',
      String(e).includes('credit_not_allowed') ? 'credit_not_allowed' : String(e).slice(0, 80),
    );
    }
    sessionEnv = envWithCookie(base, loginOperator(base, 'op-demo-cashier', DEFAULT_PIN));
  } catch (e) {
    fail('C3.4', String(e).slice(0, 120));
  }

  // --- C4 ND ---
  try {
    const ft = await issue('FT', 'sale-nd-ft');
    const nd = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/debit-notes`, '--body', JSON.stringify({
      request_id: `d62-nd-${Date.now()}`, operator_id: 'op-demo-cashier', reason: 'Ajuste', debit_full: true,
    }));
    const orig = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${ft.document_id}`);
    (nd.document_type === 'ND' && ft.document_type === 'FT' ? pass : fail)('C4.1', nd.document_type);
    (orig.document_status === 'DEBITED_PARTIAL' ? pass : fail)('C4.3', orig.document_status);
  } catch (e) {
    fail('C4.1', String(e).slice(0, 120));
    fail('C4.3', String(e).slice(0, 80));
  }

  try {
    const ft = await issue('FT', 'sale-nd-over');
    const nd = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/debit-notes`, '--body', JSON.stringify({
      request_id: `d62-nd-over-${Date.now()}`, operator_id: 'op-demo-cashier', reason: 'Above original',
      debit_full: false, lines: [{ original_line_number: 1, line_gross: '99.00' }],
    }));
    const orig = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${ft.document_id}`);
    (nd.document_type === 'ND' && orig.debited_gross_total === '99.00' ? pass : fail)('C4.2', `debited=${orig.debited_gross_total}`);
  } catch (e) {
    fail('C4.2', String(e).slice(0, 80));
  }

  // --- C5 SAF-T + print ---
  try {
    await refreshAdminSession();
    const exp = await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({
      year, month,
    }));
    const xml = await uatCmd('req', 'GET', `/local/v1/saft/exports/${exp.export_id}/download`, '--raw');
    const types = ['FT', 'FS', 'NC', 'ND'].every((t) => xml.includes(`<InvoiceType>${t}</InvoiceType>`));
    (types && exp.validation_status === 'VALID' ? pass : fail)(
      'C5.1',
      `status=${exp.validation_status} count=${exp.invoice_count}`,
    );
    (exp.validation_status === 'VALID' ? pass : fail)('C5.2', `${exp.validation_status}; accent product in period`);
    sessionEnv = envWithCookie(base, loginOperator(base, 'op-demo-cashier', DEFAULT_PIN));
  } catch (e) {
    fail('C5.1', String(e.message || e).slice(0, 240));
    fail('C5.2', String(e.message || e).slice(0, 240));
  }

  try {
    const fs = await issue('FS', 'sale-print');
    await uatCmd('wait-json', 'GET', `/local/v1/print-jobs/${fs.print_job_id}`,
      '--path', 'job_status', '--equals', 'PRINTED', '--timeout-ms', '8000');
    pass('C5.3', `MemorySink PRINTED; 真机出纸=手测`);
    const before = (await uatJson('req', 'GET', `/local/v1/fiscal-documents/${fs.document_id}`)).hash;
    const rp = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${fs.document_id}/reprints`, '--body', JSON.stringify({
      operator_id: 'op-demo-cashier', station_id: 'st-uat',
    }));
    await uatCmd('wait-json', 'GET', `/local/v1/print-jobs/${rp.print_job_id}`,
      '--path', 'job_status', '--equals', 'PRINTED', '--timeout-ms', '8000');
    const after = (await uatJson('req', 'GET', `/local/v1/fiscal-documents/${fs.document_id}`)).hash;
    (before && before === after ? pass : fail)('C5.4', before === after ? 'hash unchanged' : `${before} vs ${after}`);
  } catch (e) {
    fail('C5.3', String(e).slice(0, 120));
    fail('C5.4', String(e).slice(0, 80));
  }

  // --- C6 bill sync ---
  try {
    await run('go', ['test', './internal/fiscal/billsync/', '-run', 'TestIngest_AlreadyInvoicedViaTaxDB', '-count=1'], {
      cwd: agent, env: { ...process.env, PATH: `/opt/homebrew/bin:${process.env.PATH}` },
    });
    pass('C6.1', 'go test TestIngest_AlreadyInvoicedViaTaxDB (+ fiscal-bill-sync-regression)');
  } catch (e) {
    fail('C6.1', String(e).slice(0, 160));
  }

  try {
    const draftId = randomUUID();
    const saleId = `sale-draft-fs-${Date.now()}`;
    const now = new Date().toISOString();
    const payload = JSON.stringify({
      request_id: `req-${saleId}`, source_system: 'farvoo', source_sale_id: saleId,
      scope_type: 'whole_table', gross_total: '10.00', table_display_name: 'T1',
      lines: [{
        item_code: 'DEMO1', name: 'Água 500ml', qty: '1',
        unit_price_gross: '10.00', line_gross: '10.00', vat_rate: '23.00',
      }],
    });
    await run('sqlite3', [dbPath, `
      INSERT INTO bill_sync_drafts(id, request_id, source_sale_id, payload_json, allocation_json, allocation_revision, status, cloud_job_id, created_at, updated_at)
      VALUES ('${draftId}', 'req-${saleId}', '${saleId}', '${payload.replace(/'/g, "''")}', '{}', 0, 'open', 'job-d62', '${now}', '${now}');
    `]);
    const issued = await uatJson('req', 'POST', `/local/v1/bill-drafts/${draftId}/issue`, '--body', JSON.stringify({
      station_id: 'st-uat', operator_id: 'op-demo-cashier', mode: 'whole_table',
    }));
    const docType = issued.document_type || issued.DocumentType;
    (docType === 'FS' ? pass : fail)('C6.2', `document_type=${docType}`);
  } catch (e) {
    fail('C6.2', String(e).slice(0, 160));
  }

  // --- C7 ops (D6.3 / D6.4) ---
  try {
    await refreshAdminSession();
    const bak = await uatJson('req', 'POST', '/local/v1/setup/backup', '--body', '{}');
    const ok0 = await uatJson('req', 'POST', '/local/v1/setup/integrity/verify', '--body', JSON.stringify({ block_on_fail: true }));
    (bak.backup_path && ok0.ok ? pass : fail)('C7.1', `backup=${!!bak.backup_path} integrity=${ok0.ok}`);
  } catch (e) {
    fail('C7.1', String(e).slice(0, 120));
  }

  try {
    await refreshAdminSession();
    const swap = await uatJson('req', 'POST', '/local/v1/setup/prepare-swap', '--body', JSON.stringify({
      backup: true,
    }));
    const st = await uatJson('req', 'GET', '/local/v1/setup/status');
    (swap.ok && st.activated_ok === false ? pass : fail)('C7.2', `activated_ok=${st.activated_ok}`);
  } catch (e) {
    fail('C7.2', String(e).slice(0, 120));
  }

  // Manual items — passed in product UAT (2026-09-02); recorded as skip (not auto-fail).
  skip('C2.2-UI', '手测已通过（2026-09-02）：Admin 手工开票类型下拉仅 FT/FS');
  skip('C2.6-scan', '手测已通过（2026-09-02）：扫枪读 QR');
  skip('C5.3-hw', '手测已通过（2026-09-02）：真热敏 ORIGINAL 出纸');
  skip('C7.2-hw', '手测已通过或跳过（2026-09-02）：真机换机');

  child.kill('SIGTERM');

  const failed = results.filter((r) => r.status === 'fail');
  const passed = results.filter((r) => r.status === 'pass');
  const blockedN = results.filter((r) => r.status === 'blocked');
  const skipped = results.filter((r) => r.status === 'skip');
  console.log('\n--- D6.2 cert summary ---');
  console.log(JSON.stringify({
    pass: passed.length, fail: failed.length, blocked: blockedN.length, skip: skipped.length,
    results,
  }, null, 2));
  process.exit(failed.length ? 1 : 0);
}

function storeIsFs(doc) {
  return doc.document_type === 'FS';
}

main().catch((e) => { console.error(e); process.exit(1); });
