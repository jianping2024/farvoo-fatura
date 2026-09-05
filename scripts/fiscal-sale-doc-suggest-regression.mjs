#!/usr/bin/env node
/**
 * Soft FT/FS suggestion + split payment_method regression (no skip).
 * Covers: unique Admin helpers, draft issue payment override, manual still issues.
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
const bind = '127.0.0.1:17887';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-sale-doc-suggest.db');
const dataDir = join(agent, 'data', 'fiscal-sale-doc-suggest-secure');
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
    if (k.startsWith('FISCAL_') && !['FISCAL_SESSION_SECRET'].includes(k)) delete childEnv[k];
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
    child.kill();
    process.exit(1);
  }

  try {
    const { cookie } = ensureAdminSession(base, 'Suggest Admin', DEFAULT_PIN);
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
    for (const [docType, suffix] of [['FT', 'SFT'], ['FS', 'SFS']]) {
      await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
        series_code: `${docType}${year}${suffix}01`, document_type: docType, fiscal_year: year,
      }));
    }
    await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({
      product_private_key_pem: pem.trim(),
    }));
    await uatCmd('req', 'POST', '/local/v1/products', '--body', JSON.stringify({
      product_code: 'SUG1', display_name: 'Suggest Item', saft_name: 'Suggest Item',
      unit_price_gross: '10.00', vat_rate: '23.00',
    }));
    record('setup', true, 'taxpayer/series/activate/product');
  } catch (e) {
    record('setup', false, String(e).slice(0, 200));
    child.kill();
    process.exit(1);
  }

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
      ['suggestedSaleDocumentType once', (html.match(/function suggestedSaleDocumentType/g) || []).length === 1],
      ['syncSaleDocTypeSuggestion once', (html.match(/function syncSaleDocTypeSuggestion/g) || []).length === 1],
      ['bindSaleDocTypeSuggestion once', (html.match(/function bindSaleDocTypeSuggestion/g) || []).length === 1],
      ['DEFAULT_SALE_DOC FS once', (html.match(/const DEFAULT_SALE_DOC = 'FS'/g) || []).length === 1],
      ['fmtVatPercent once', (html.match(/function fmtVatPercent/g) || []).length === 1],
      ['lineFromGrossPreview once', (html.match(/function lineFromGrossPreview/g) || []).length === 1],
      ['orderMoneyBreakdown once', (html.match(/function orderMoneyBreakdown/g) || []).length === 1],
      ['splitPay present', html.includes('id="splitPay"')],
      ['order bind', html.includes("bindSaleDocTypeSuggestion($('#docType'")],
      ['split bind', html.includes("bindSaleDocTypeSuggestion($('#splitDocType'")],
    ];
    for (const [name, ok] of checks) record(`admin-html ${name}`, ok);
  } catch (e) {
    record('admin-html', false, String(e).slice(0, 120));
  }

  try {
    const draftId = randomUUID();
    const saleId = `sale-pay-${Date.now()}`;
    const now = new Date().toISOString();
    const payload = JSON.stringify({
      request_id: `req-${saleId}`, source_system: 'farvoo', source_sale_id: saleId,
      scope_type: 'whole_table', gross_total: '10.00', table_display_name: 'T-SUG',
      lines: [{
        item_code: 'SUG1', name: 'Suggest Item', qty: '1',
        unit_price_gross: '10.00', line_gross: '10.00', vat_rate: '23.00',
      }],
    });
    await run('sqlite3', [dbPath, `
      INSERT INTO bill_sync_drafts(id, request_id, source_sale_id, payload_json, allocation_json, allocation_revision, status, cloud_job_id, created_at, updated_at)
      VALUES ('${draftId}', 'req-${saleId}', '${saleId}', '${payload.replace(/'/g, "''")}', '{}', 0, 'open', 'job-sug', '${now}', '${now}');
    `]);
    const issued = await uatJson('req', 'POST', `/local/v1/bill-drafts/${draftId}/issue`, '--body', JSON.stringify({
      station_id: 'st-uat',
      mode: 'whole_table',
      document_type: 'FT',
      customer_nif: '502757191',
      customer_name: 'Acme Lda',
      payment_method: 'CARD',
    }));
    const docId = issued.document_id || issued.DocumentID || '';
    const docType = issued.document_type || issued.DocumentType;
    record('draft-issue FT+CARD', docType === 'FT' && !!docId, `type=${docType} id=${docId.slice(0, 8)}`);
    const pay = (await run('sqlite3', [dbPath,
      `SELECT method FROM invoice_payments WHERE invoice_id='${docId}';`])).trim();
    record('draft payment_method CARD', pay === 'CARD', pay || 'missing');
  } catch (e) {
    record('draft-issue FT+CARD', false, String(e).slice(0, 200));
    record('draft payment_method CARD', false, String(e).slice(0, 120));
  }

  try {
    const draftId = randomUUID();
    const saleId = `sale-cash-${Date.now()}`;
    const now = new Date().toISOString();
    const payload = JSON.stringify({
      request_id: `req-${saleId}`, source_system: 'farvoo', source_sale_id: saleId,
      scope_type: 'whole_table', gross_total: '10.00',
      lines: [{
        item_code: 'SUG1', name: 'Suggest Item', qty: '1',
        unit_price_gross: '10.00', line_gross: '10.00', vat_rate: '23.00',
      }],
    });
    await run('sqlite3', [dbPath, `
      INSERT INTO bill_sync_drafts(id, request_id, source_sale_id, payload_json, allocation_json, allocation_revision, status, cloud_job_id, created_at, updated_at)
      VALUES ('${draftId}', 'req-${saleId}', '${saleId}', '${payload.replace(/'/g, "''")}', '{}', 0, 'open', 'job-sug2', '${now}', '${now}');
    `]);
    const issued = await uatJson('req', 'POST', `/local/v1/bill-drafts/${draftId}/issue`, '--body', JSON.stringify({
      station_id: 'st-uat', mode: 'whole_table', document_type: 'FS',
    }));
    const docId = issued.document_id || issued.DocumentID || '';
    const pay = (await run('sqlite3', [dbPath,
      `SELECT method FROM invoice_payments WHERE invoice_id='${docId}';`])).trim();
    record('draft default payment CASH', pay === 'CASH', pay || 'missing');
  } catch (e) {
    record('draft default payment CASH', false, String(e).slice(0, 160));
  }

  try {
    const man = await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
      request_id: `req-man-sug-${Date.now()}`,
      document_type: 'FS',
      payment_method: 'CASH',
      lines: [{ product_code: 'SUG1', quantity: '1' }],
    }));
    record('manual FS still works', man.document_type === 'FS', man.invoice_no || '');
  } catch (e) {
    record('manual FS still works', false, String(e).slice(0, 160));
  }

  console.log('\n=== SALE-DOC-SUGGEST SUMMARY ===');
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
