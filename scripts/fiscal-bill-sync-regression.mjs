#!/usr/bin/env node
/**
 * Bill sync Agent regression (farvoo-fiscal-bill-sync-api):
 * mock Farvoo pending-bill-syncs → PullAndIngest (unique path) → SQLite drafts + products → ack.
 */
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { mkdirSync, rmSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = 'http://127.0.0.1:17886';
const mockPort = 17991;
const dataRoot = join(agent, 'data', 'bill-sync-uat');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

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

function snap(overrides = {}) {
  return {
    request_id: 'req-bill-1',
    source_system: 'farvoo',
    source_sale_id: 'sale-bill-1',
    table_display_name: '018',
    scope_type: 'whole_table',
    gross_total: '47.10',
    lines: [
      {
        item_code: 'BF-LUNCH-ADULT',
        name: 'BUFFET ADULTO ALMOCO',
        qty: '3',
        unit_price_gross: '14.95',
        line_gross: '44.85',
        vat_rate: '13.00',
      },
      {
        item_code: '006',
        name: 'CERVEJA SUPERBOCK',
        qty: '1',
        unit_price_gross: '2.25',
        line_gross: '2.25',
        vat_rate: '23.00',
      },
    ],
    ...overrides,
  };
}

async function main() {
  const acks = [];
  let pendingJobs = [];

  const mock = createServer(async (req, res) => {
    const url = new URL(req.url, `http://127.0.0.1:${mockPort}`);
    if (req.method === 'GET' && url.pathname === '/api/print-agent/pending-bill-syncs') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ jobs: pendingJobs }));
      return;
    }
    const m = url.pathname.match(/^\/api\/print-agent\/bill-syncs\/([^/]+)\/ack$/);
    if (req.method === 'POST' && m) {
      let body = '';
      for await (const c of req) body += c;
      acks.push({ id: m[1], ...JSON.parse(body || '{}') });
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ ok: true }));
      return;
    }
    res.writeHead(404);
    res.end('no');
  });
  await new Promise((r) => mock.listen(mockPort, '127.0.0.1', r));

  if (existsSync(dataRoot)) rmSync(dataRoot, { recursive: true, force: true });
  mkdirSync(dataRoot, { recursive: true });
  const dbPath = join(dataRoot, 'fiscal.db');
  const secure = join(dataRoot, 'secure');

  const child = spawn('go', ['run', './cmd/fiscal-local'], {
    cwd: agent,
    env: {
      ...process.env,
      PATH: `/opt/homebrew/bin:${process.env.PATH}`,
      FISCAL_DB: dbPath,
      FISCAL_DATA_DIR: secure,
      FISCAL_BIND: '127.0.0.1:17886',
      FISCAL_STORE_ID: 'store-demo-001',
      FISCAL_AT_ENV: 'mock',
      FISCAL_ALLOW_LOCAL_PROVISION: '1',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let boot = '';
  child.stdout.on('data', (d) => (boot += d));
  child.stderr.on('data', (d) => (boot += d));

  let healthy = false;
  for (let i = 0; i < 80; i++) {
    try {
      await uatCmd('stack-health');
      healthy = true;
      break;
    } catch {
      await new Promise((r) => setTimeout(r, 250));
    }
  }
  record('fiscal-local-health', healthy, healthy ? base : boot.slice(-400));
  if (!healthy) {
    child.kill();
    mock.close();
    process.exit(1);
  }

  const runPull = async () =>
    (
      await run('go', ['run', './dev/bill-sync-pull-once'], {
        cwd: agent,
        env: {
          ...process.env,
          PATH: `/opt/homebrew/bin:${process.env.PATH}`,
          FISCAL_DB: dbPath,
          FARVOO_API: `http://127.0.0.1:${mockPort}`,
          FARVOO_JWT: 'test-jwt',
        },
      })
    ).trim();

  pendingJobs = [{ id: 'job-1', status: 'pending', payload: snap() }];
  acks.length = 0;
  try {
    await runPull();
    const drafts = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    const ok =
      drafts.drafts?.length === 1 &&
      drafts.drafts[0].status === 'open' &&
      drafts.drafts[0].source_sale_id === 'sale-bill-1' &&
      acks[0]?.status === 'succeeded';
    record('ingest-happy-ack', ok, JSON.stringify({ drafts: drafts.drafts?.[0], ack: acks[0] }));
  } catch (e) {
    record('ingest-happy-ack', false, String(e));
  }

  try {
    const out = await run('sqlite3', [
      dbPath,
      'SELECT product_code||\":\"||vat_rate FROM fiscal_products ORDER BY product_code;',
    ]);
    const lines = out.trim().split('\n').filter(Boolean);
    record(
      'sqlite-products-upsert',
      lines.includes('006:23.00') && lines.includes('BF-LUNCH-ADULT:13.00'),
      out.trim(),
    );
  } catch (e) {
    record('sqlite-products-upsert', false, String(e));
  }

  pendingJobs = [{ id: 'job-1b', status: 'pending', payload: snap() }];
  acks.length = 0;
  try {
    await runPull();
    const drafts = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    record(
      'idempotent-request-id',
      drafts.drafts?.length === 1 && acks[0]?.status === 'succeeded',
      `n=${drafts.drafts?.length}`,
    );
  } catch (e) {
    record('idempotent-request-id', false, String(e));
  }

  pendingJobs = [
    {
      id: 'job-bad-vat',
      status: 'pending',
      payload: snap({
        request_id: 'req-bad-vat',
        source_sale_id: 'sale-bad-vat',
        lines: [
          {
            item_code: 'X',
            name: 'X',
            qty: '1',
            unit_price_gross: '1.00',
            line_gross: '1.00',
            vat_rate: '0.23',
          },
        ],
      }),
    },
  ];
  acks.length = 0;
  try {
    await runPull();
    record(
      'reject-decimal-vat',
      acks[0]?.status === 'failed' && acks[0]?.error_code === 'invalid_vat_rate',
      JSON.stringify(acks[0]),
    );
  } catch (e) {
    record('reject-decimal-vat', false, String(e));
  }

  pendingJobs = [
    {
      id: 'job-conflict',
      status: 'pending',
      payload: snap({
        request_id: 'req-conflict',
        source_sale_id: 'sale-conflict',
        lines: [
          {
            item_code: 'A',
            name: 'One',
            qty: '1',
            unit_price_gross: '1.00',
            line_gross: '1.00',
            vat_rate: '23.00',
          },
          {
            item_code: 'A',
            name: 'Two',
            qty: '1',
            unit_price_gross: '1.00',
            line_gross: '1.00',
            vat_rate: '23.00',
          },
        ],
      }),
    },
  ];
  acks.length = 0;
  try {
    await runPull();
    record(
      'reject-item-conflict',
      acks[0]?.status === 'failed' && acks[0]?.error_code === 'item_code_conflict',
      JSON.stringify(acks[0]),
    );
  } catch (e) {
    record('reject-item-conflict', false, String(e));
  }

  try {
    await run('sqlite3', [dbPath, `UPDATE bill_sync_drafts SET status='invoiced' WHERE source_sale_id='sale-bill-1';`]);
    pendingJobs = [
      {
        id: 'job-re',
        status: 'pending',
        payload: snap({ request_id: 'req-re', source_sale_id: 'sale-bill-1' }),
      },
    ];
    acks.length = 0;
    await runPull();
    record(
      'already-invoiced-ack-failed',
      acks[0]?.status === 'failed' && acks[0]?.error_code === 'already_invoiced',
      JSON.stringify(acks[0]),
    );
  } catch (e) {
    record('already-invoiced-ack-failed', false, String(e));
  }

  child.kill('SIGTERM');
  mock.close();

  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n=== BILL-SYNC SUMMARY ===');
  for (const r of results) console.log(`${r.status}\t${r.name}\t${r.note}`);
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
