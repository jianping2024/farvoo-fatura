#!/usr/bin/env node
/**
 * Full P0 FT local regression (no skip):
 * stack-health → issue FT → SQLite assert → wait print PRINTED → idempotent reissue
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ensureOwnerSession, envWithCookie } from './fiscal-session-helper.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = process.env.FISCAL_UAT_BASE || 'http://127.0.0.1:17880';
const dbPath = join(agent, 'data', 'fiscal-regression.db');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');

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
  const out = await run(process.execPath, [uat, ...args], {
    env: { ...process.env, FISCAL_UAT_BASE: base, ...uatEnv },
  });
  return out.trim();
}

const results = [];
let uatEnv = {};
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

async function main() {
  // Ensure port free (stale fiscal-local from prior runs)
  try {
    await run('pkill', ['-f', 'fiscal-local']);
  } catch {
    /* none */
  }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);

  const childEnv = { ...process.env, PATH: `/opt/homebrew/bin:${process.env.PATH}` };
  for (const k of Object.keys(childEnv)) {
    if (k.startsWith('FISCAL_')) delete childEnv[k];
  }
  Object.assign(childEnv, {
    FISCAL_DB: dbPath,
    FISCAL_BIND: '127.0.0.1:17880',
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_SEED: '1',
    FISCAL_ALLOW_DEV_KEY: '1',
    FISCAL_AT_ENV: 'mock',
    FISCAL_STATION_PRINTERS_JSON: JSON.stringify({
      '2951b0b2-aaaa-bbbb-cccc-ddddeeeeffff': 'tcp:172.20.10.3:9100',
      '7e8facc6-1111-2222-3333-444455556666': 'tcp:172.20.10.3:9100',
    }),
    FISCAL_STATION_META_JSON: JSON.stringify([
      { id: '2951b0b2-aaaa-bbbb-cccc-ddddeeeeffff', name_zh: '厨房', sort_order: 0 },
      { id: '7e8facc6-1111-2222-3333-444455556666', name_zh: '吧台', sort_order: 1 },
    ]),
  });

  const child = spawn(
    'go',
    ['run', './cmd/fiscal-local'],
    {
      cwd: agent,
      env: childEnv,
      stdio: ['ignore', 'pipe', 'pipe'],
    },
  );
  let boot = '';
  child.stdout.on('data', (d) => (boot += d));
  child.stderr.on('data', (d) => (boot += d));
  child.on('exit', (code) => {
    if (code && code !== 0) boot += `\n[fiscal-local exited ${code}]`;
  });

  // wait health (go run compile can exceed 15s on cold cache)
  let healthy = false;
  for (let i = 0; i < 120; i++) {
    if (child.exitCode != null && child.exitCode !== 0) break;
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

  try {
    await uatCmd('exec-db', '--db', dbPath, '--sql', 'DELETE FROM operators;');
    const { cookie } = ensureOwnerSession(base, 'Regression Owner', '123456');
    uatEnv = envWithCookie(base, cookie);
    record('owner-session', !!cookie);
  } catch (e) {
    record('owner-session', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  const requestId = `reg-${Date.now()}`;
  const body = JSON.stringify({
    request_id: requestId,
    document_type: 'FT',
    snapshot: {
      source_system: 'farvoo',
      source_sale_id: `sale-${requestId}`,
      scope_type: 'session',
      scope_id: `scope-${requestId}`,
      fiscal_purpose: 'sale',
      lines: [
        {
          product_code: 'DEMO1',
          display_name: 'Prato Demo',
          saft_name: 'Prato Demo',
          quantity: '1',
          unit_price_gross: '12.50',
          vat_rate: '0.23',
          product_type: 'P',
          unit_of_measure: 'UN',
        },
      ],
      customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
      payments: [{ method: 'CASH', amount: '12.50' }],
    },
  });

  let issue;
  try {
    const raw = await uatCmd('req', 'POST', '/local/v1/fiscal-documents', '--body', body);
    issue = JSON.parse(raw);
    record('issue-ft', !!issue.document_id && issue.document_status === 'SIGNED', issue.invoice_no);
  } catch (e) {
    record('issue-ft', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    await uatCmd(
      'assert-db',
      '--db',
      dbPath,
      '--sql',
      `SELECT invoice_no FROM invoices WHERE id='${issue.document_id}';`,
      '--expect-count',
      '1',
    );
    record('sqlite-invoice-row', true);
  } catch (e) {
    record('sqlite-invoice-row', false, String(e));
  }

  try {
    await uatCmd(
      'wait-json',
      'GET',
      `/local/v1/print-jobs/${issue.print_job_id}`,
      '--path',
      'job_status',
      '--equals',
      'PRINTED',
      '--timeout-ms',
      '5000',
    );
    record('print-job-printed', true, issue.print_job_id);
  } catch (e) {
    record('print-job-printed', false, String(e));
  }

  try {
    const raw2 = await uatCmd('req', 'POST', '/local/v1/fiscal-documents', '--body', body);
    const again = JSON.parse(raw2);
    const ok = again.idempotent_hit === true && again.document_id === issue.document_id;
    record('idempotent-reissue', ok, JSON.stringify(again.idempotent_hit));
  } catch (e) {
    record('idempotent-reissue', false, String(e));
  }

  try {
    const raw = await uatCmd(
      'req',
      'GET',
      `/local/v1/fiscal-documents/by-request/${requestId}?store_id=store-demo-001`,
    );
    const j = JSON.parse(raw);
    record('get-by-request', j.document_id === issue.document_id);
  } catch (e) {
    record('get-by-request', false, String(e));
  }

  try {
    const raw = await uatCmd('req', 'GET', '/local/v1/fiscal-documents?page=1&page_size=10');
    const j = JSON.parse(raw);
    const row = (j.invoices || []).find((x) => x.document_id === issue.document_id);
    const ok =
      !!row &&
      !!row.system_entry_date &&
      !!row.hash &&
      row.customer_tax_id === '999999990' &&
      !!row.atcud &&
      row.document_status === 'SIGNED' &&
      row.payment_method === 'CASH' &&
      row.invoice_date === undefined &&
      j.total >= 1 &&
      j.page === 1 &&
      j.page_size === 10;
    record(
      'list-invoices-hash-customer',
      ok,
      row
        ? `sed=${!!row.system_entry_date} hash=${!!row.hash} nif=${row.customer_tax_id} pay=${row.payment_method} total=${j.total}`
        : 'row missing',
    );
  } catch (e) {
    record('list-invoices-hash-customer', false, String(e));
  }

  try {
    const raw = await uatCmd(
      'req',
      'GET',
      `/local/v1/fiscal-documents?q=${encodeURIComponent('FT2026')}`,
    );
    const j = JSON.parse(raw);
    const found = (j.invoices || []).some((x) => x.document_id === issue.document_id);
    record('list-invoices-search', found, issue.invoice_no);
  } catch (e) {
    record('list-invoices-search', false, String(e));
  }

  try {
    const raw = await uatCmd('req', 'GET', '/local/v1/fiscal-documents?from=2020-01-01&to=2020-01-02&page=1&page_size=10');
    const j = JSON.parse(raw);
    record(
      'list-invoices-date-empty',
      j.total === 0 && (j.invoices || []).length === 0 && j.from === '2020-01-01',
    );
  } catch (e) {
    record('list-invoices-date-empty', false, String(e));
  }

  try {
    const raw = await uatCmd('req', 'GET', '/local/v1/fiscal-documents?from=2026-01-01&to=2099-12-31&page=1&page_size=10');
    const j = JSON.parse(raw);
    const found = (j.invoices || []).some((x) => x.document_id === issue.document_id);
    record('list-invoices-date-range', found && j.total >= 1, 'wide-range');
  } catch (e) {
    record('list-invoices-date-range', false, String(e));
  }

  try {
    const raw = await uatCmd('req', 'GET', '/local/v1/fiscal-documents?page=2&page_size=10');
    const j = JSON.parse(raw);
    record(
      'list-invoices-pagination-clamp',
      j.page === 1 && j.page_size === 10 && j.total === 1 && (j.invoices || []).length === 1,
      `page=${j.page} items=${(j.invoices || []).length} total=${j.total}`,
    );
  } catch (e) {
    record('list-invoices-pagination-clamp', false, String(e));
  }

  try {
    const raw = await uatCmd('req', 'GET', '/local/v1/fiscal-documents?page=1&page_size=20');
    const j = JSON.parse(raw);
    const found = (j.invoices || []).some((x) => x.document_id === issue.document_id);
    record(
      'list-invoices-pagination-page-size-20',
      j.page_size === 20 && found && (j.invoices || []).length <= 20,
      `page_size=${j.page_size}`,
    );
  } catch (e) {
    record('list-invoices-pagination-page-size-20', false, String(e));
  }

  try {
    const raw = await uatCmd('req', 'GET', `/local/v1/fiscal-documents/${issue.document_id}`);
    const j = JSON.parse(raw);
    record(
      'get-invoice-detail-payment',
      j.document_id === issue.document_id && j.payment_method === 'CASH',
      `pay=${j.payment_method}`,
    );
  } catch (e) {
    record('get-invoice-detail-payment', false, String(e));
  }

  try {
    const raw = await uatCmd('req', 'GET', '/local/v1/printers');
    const j = JSON.parse(raw);
    const stations = j.stations || [];
    const kitchen = stations.find((s) => s.id === '2951b0b2-aaaa-bbbb-cccc-ddddeeeeffff');
    const bar = stations.find((s) => s.id === '7e8facc6-1111-2222-3333-444455556666');
    const ok =
      stations.length === 2 &&
      kitchen &&
      kitchen.label === '厨房' &&
      kitchen.printer === 'tcp:172.20.10.3:9100' &&
      bar &&
      bar.label === '吧台' &&
      stations[0].label === '厨房' &&
      stations[1].label === '吧台';
    record(
      'list-printers-station-labels',
      ok,
      ok ? `${stations[0].label} · ${stations[1].label}` : JSON.stringify(stations),
    );
  } catch (e) {
    record('list-printers-station-labels', false, String(e));
  }

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n=== SUMMARY ===');
  for (const r of results) console.log(`${r.status}\t${r.name}\t${r.note}`);
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
