#!/usr/bin/env node
/**
 * Cash tendered / change regression (no skip).
 * fiscal-local-uat analogue: stack-health → manual issue paths → sqlite + print payload.
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { randomUUID } from 'node:crypto';
import {
  DEFAULT_PIN, ensureAdminSession, envWithCookie, fiscalAgentTestEnv,
} from './fiscal-session-helper.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17888';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-cash-tender.db');
const dataDir = join(agent, 'data', 'fiscal-cash-tender-secure');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const year = new Date().getFullYear();
let uatEnv = { FISCAL_UAT_BASE: base };

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
    env: { ...process.env, ...uatEnv },
  })).trim();
}

async function uatJson(...args) {
  return JSON.parse(await uatCmd(...args));
}

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

function manualBody(extra = {}) {
  return {
    request_id: `req-tender-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`,
    station_id: 'st-tender',
    document_type: 'FS',
    customer_nif: '999999990',
    customer_name: 'Consumidor Final',
    payment_method: 'CASH',
    lines: [{ product_code: 'TEND1', quantity: '1' }],
    ...extra,
  };
}

async function main() {
  try { await run('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true, force: true });

  const childEnv = fiscalAgentTestEnv({
    PATH: `/opt/homebrew/bin:${process.env.PATH}`,
  });
  for (const k of Object.keys(childEnv)) {
    if (k.startsWith('FISCAL_') && k !== 'FISCAL_SESSION_SECRET') delete childEnv[k];
  }
  Object.assign(childEnv, {
    FISCAL_SESSION_SECRET: childEnv.FISCAL_SESSION_SECRET || 'farvoo-fiscal-uat-session-secret-32b!!',
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: dataDir,
    FISCAL_BIND: bind,
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_ALLOW_DEV_KEY: '1',
    FISCAL_AT_ENV: 'mock',
    FISCAL_ALLOW_LOCAL_PROVISION: '1',
    FISCAL_SEED: '0',
    FISCAL_STATION_PRINTERS_JSON: JSON.stringify({ 'st-tender': 'tcp:127.0.0.1:9100' }),
    FISCAL_STATION_META_JSON: JSON.stringify([{ id: 'st-tender', label: 'Tender' }]),
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
  record('fiscal-local-uat stack-health', healthy, healthy ? base : boot.slice(-400));
  if (!healthy) {
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    const { cookie } = ensureAdminSession(base, 'Tender Admin', DEFAULT_PIN);
    uatEnv = envWithCookie(base, cookie);
    const pem = readFileSync(pemPath, 'utf8');
    await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
      tax_registration_number: '517535009', legal_name: 'Farvoo Demo Lda',
      address_detail: 'Rua Demo 1', city: 'Lisboa', postal_code: '1000-001',
      country: 'PT', timezone: 'Europe/Lisbon', software_certificate_number: '0',
    }));
    await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
      username: '517535009/37', password: 'demo-secret',
    }));
    for (const [docType, suffix] of [['FT', 'TND'], ['FS', 'TNS']]) {
      await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
        series_code: `${docType}${year}${suffix}01`, document_type: docType, fiscal_year: year,
      }));
    }
    await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({
      product_private_key_pem: pem.trim(),
    }));
    await uatCmd('req', 'POST', '/local/v1/products', '--body', JSON.stringify({
      product_code: 'TEND1', display_name: 'Tender Item', saft_name: 'Tender Item',
      unit_price_gross: '12.50', vat_rate: '23.00',
    }));
    record('setup', true, 'taxpayer/series/activate/product');
  } catch (e) {
    record('setup', false, String(e).slice(0, 240));
    child.kill('SIGTERM');
    process.exit(1);
  }

  // Admin unique writers (served HTML)
  try {
    const htmlPayload = await uatCmd('req', 'GET', '/');
    const html = (() => {
      try {
        const j = JSON.parse(htmlPayload);
        return j.raw || htmlPayload;
      } catch {
        return htmlPayload;
      }
    })();
    const checks = [
      ['bindCashTenderUI once', (html.match(/function bindCashTenderUI/g) || []).length === 1],
      ['syncCashTenderWrap once', (html.match(/function syncCashTenderWrap/g) || []).length === 1],
      ['readCashTenderedOrToast once', (html.match(/function readCashTenderedOrToast/g) || []).length === 1],
      ['invCashTenderWrap', html.includes('id="invCashTenderWrap"')],
      ['splitCashTenderWrap', html.includes('id="splitCashTenderWrap"')],
    ];
    for (const [name, ok] of checks) record(`admin-html ${name}`, ok);
  } catch (e) {
    record('admin-html', false, String(e).slice(0, 120));
  }

  // 1) CASH + tendered → DB + payload
  try {
    const issued = await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(manualBody({
      tendered: '20',
    })));
    const docId = issued.document_id;
    record('manual CASH+tendered issue', !!docId && issued.document_status === 'SIGNED', issued.invoice_no);
    const row = (await run('sqlite3', [dbPath,
      `SELECT amount||'|'||IFNULL(tendered,'')||'|'||IFNULL(change_due,'') FROM invoice_payments WHERE invoice_id='${docId}';`,
    ])).trim();
    record('sqlite tendered/change', row === '12.50|20.00|7.50', row);
    const payloadRaw = (await run('sqlite3', [dbPath,
      `SELECT payload_json FROM local_print_jobs WHERE invoice_id='${docId}';`,
    ])).trim();
    const payload = JSON.parse(payloadRaw);
    const pay = (payload.payments || [])[0] || {};
    record('print payload tendered', pay.tendered === '20.00' && pay.change_due === '7.50', JSON.stringify(pay));
  } catch (e) {
    record('manual CASH+tendered issue', false, String(e).slice(0, 200));
    record('sqlite tendered/change', false, '');
    record('print payload tendered', false, '');
  }

  // 2) CASH without tendered → empty columns
  try {
    const issued = await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(manualBody()));
    const docId = issued.document_id;
    const row = (await run('sqlite3', [dbPath,
      `SELECT amount||'|'||IFNULL(tendered,'')||'|'||IFNULL(change_due,'') FROM invoice_payments WHERE invoice_id='${docId}';`,
    ])).trim();
    record('CASH no-tendered empty cols', row === '12.50||', row);
  } catch (e) {
    record('CASH no-tendered empty cols', false, String(e).slice(0, 160));
  }

  // 3) exact tendered == amount → change 0.00
  try {
    const issued = await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(manualBody({
      tendered: '12.50',
    })));
    const row = (await run('sqlite3', [dbPath,
      `SELECT IFNULL(tendered,'')||'|'||IFNULL(change_due,'') FROM invoice_payments WHERE invoice_id='${issued.document_id}';`,
    ])).trim();
    record('exact tendered change 0.00', row === '12.50|0.00', row);
  } catch (e) {
    record('exact tendered change 0.00', false, String(e).slice(0, 160));
  }

  // 4) CARD + tendered → reject
  try {
    await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(manualBody({
      payment_method: 'CARD', tendered: '20.00',
    })));
    record('CARD+tendered rejected', false, 'expected error');
  } catch (e) {
    const msg = String(e);
    record('CARD+tendered rejected', /tendered only allowed for CASH/i.test(msg), msg.slice(0, 120));
  }

  // 5) underpay → reject
  try {
    await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(manualBody({
      tendered: '10.00',
    })));
    record('underpay rejected', false, 'expected error');
  } catch (e) {
    const msg = String(e);
    record('underpay rejected', /tendered less than amount/i.test(msg), msg.slice(0, 120));
  }

  // 6) bill-draft issue with tendered
  try {
    const draftId = randomUUID();
    const saleId = `sale-tender-${Date.now()}`;
    const now = new Date().toISOString();
    const payload = JSON.stringify({
      request_id: `req-${saleId}`, source_system: 'farvoo', source_sale_id: saleId,
      scope_type: 'whole_table', gross_total: '12.50', table_display_name: 'T-TEND',
      lines: [{
        item_code: 'TEND1', name: 'Tender Item', qty: '1',
        unit_price_gross: '12.50', line_gross: '12.50', vat_rate: '23.00',
      }],
    });
    await run('sqlite3', [dbPath, `
      INSERT INTO bill_sync_drafts(id, request_id, source_sale_id, payload_json, allocation_json, allocation_revision, status, cloud_job_id, created_at, updated_at)
      VALUES ('${draftId}', 'req-${saleId}', '${saleId}', '${payload.replace(/'/g, "''")}', '{}', 0, 'open', 'job-tend', '${now}', '${now}');
    `]);
    const issued = await uatJson('req', 'POST', `/local/v1/bill-drafts/${draftId}/issue`, '--body', JSON.stringify({
      station_id: 'st-tender',
      mode: 'whole_table',
      document_type: 'FS',
      payment_method: 'CASH',
      tendered: '15.00',
    }));
    const docId = issued.document_id || issued.DocumentID || '';
    const row = (await run('sqlite3', [dbPath,
      `SELECT amount||'|'||IFNULL(tendered,'')||'|'||IFNULL(change_due,'') FROM invoice_payments WHERE invoice_id='${docId}';`,
    ])).trim();
    record('draft-issue CASH+tendered', row === '12.50|15.00|2.50', row);
  } catch (e) {
    record('draft-issue CASH+tendered', false, String(e).slice(0, 200));
  }

  console.log('\n=== CASH-TENDER SUMMARY ===');
  for (const r of results) console.log(`${r.status}\t${r.name}\t${r.note}`);
  child.kill('SIGTERM');
  await new Promise((r) => {
    const done = () => r();
    child.once('close', done);
    setTimeout(() => {
      try { child.kill('SIGKILL'); } catch (_) { /* */ }
      done();
    }, 2000);
  });
  process.exit(results.some((x) => x.status === 'fail') ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
