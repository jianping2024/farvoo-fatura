#!/usr/bin/env node
/**
 * Prep fiscal-local for Admin SSE UAT: leave server on 17880, then ingest via
 * in-process POST /local/v1/dev/bill-sync/pull (same DB + Hub as Admin SSE).
 *
 * Usage:
 *   node scripts/fiscal-admin-sse-uat-prep.mjs          # start only
 *   node scripts/fiscal-admin-sse-uat-prep.mjs ingest   # push one bill (server must already be up)
 */
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { mkdirSync, rmSync, existsSync, writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const dbPath = join(agent, 'data', 'fiscal-admin-sse-uat.db');
const mockPort = 17994;
const base = 'http://127.0.0.1:17880';
const pidFile = join(agent, 'data', 'fiscal-admin-sse-uat.pid');
const farvooAPI = `http://127.0.0.1:${mockPort}`;
const farvooJWT = 'test-jwt';

function jobPayload() {
  const ts = Date.now();
  return {
    id: 'job-sse-uat-' + ts,
    status: 'pending',
    payload: {
      request_id: 'req-sse-uat-' + ts,
      source_system: 'farvoo',
      source_sale_id: 'sale-sse-uat-' + ts,
      table_display_name: 'SSE-09',
      scope_type: 'whole_table',
      lines: [{
        item_code: 'SSE-TEA', name: 'SSE Tea', qty: '1',
        unit_price_gross: '4.50', line_gross: '4.50', vat_rate: '13.00',
      }],
      gross_total: '4.50',
    },
  };
}

function run(cmd, args, opts = {}) {
  return new Promise((resolve, reject) => {
    const p = spawn(cmd, args, { stdio: 'inherit', ...opts });
    p.on('close', (code) => (code === 0 ? resolve() : reject(new Error(`${cmd} exit ${code}`))));
  });
}

async function ingestOnce() {
  const job = jobPayload();
  let pending = [job];
  const mock = createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (req.method === 'GET' && url.pathname === '/api/print-agent/pending-bill-syncs') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ jobs: pending }));
      pending = [];
      return;
    }
    const m = url.pathname.match(/^\/api\/print-agent\/bill-syncs\/([^/]+)\/ack$/);
    if (req.method === 'POST' && m) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ ok: true }));
      return;
    }
    res.writeHead(404);
    res.end();
  });
  await new Promise((resolve) => mock.listen(mockPort, resolve));
  try {
    const res = await fetch(base + '/local/v1/dev/bill-sync/pull', { method: 'POST' });
    const text = await res.text();
    if (!res.ok) {
      throw new Error(`dev pull ${res.status}: ${text}`);
    }
    console.log('ingested', job.payload.table_display_name, job.payload.request_id, text);
  } finally {
    mock.close();
  }
}

async function start() {
  try { await run('pkill', ['-f', 'fiscal-local']); } catch { /* none */ }
  await new Promise((r) => setTimeout(r, 400));
  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);

  const childEnv = {
    ...process.env,
    PATH: `/opt/homebrew/bin:${process.env.PATH}`,
    FISCAL_DB: dbPath,
    FISCAL_BIND: '127.0.0.1:17880',
    FISCAL_SEED: '1',
    FISCAL_ALLOW_DEV_KEY: '1',
    FISCAL_AT_ENV: 'mock',
    FARVOO_API: farvooAPI,
    FARVOO_JWT: farvooJWT,
  };
  const child = spawn('go', ['run', './cmd/fiscal-local'], {
    cwd: agent,
    env: childEnv,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  writeFileSync(pidFile, String(child.pid));
  child.stdout.on('data', (d) => process.stdout.write(d));
  child.stderr.on('data', (d) => process.stderr.write(d));

  for (let i = 0; i < 100; i++) {
    try {
      const r = await fetch(base + '/local/v1/health');
      if (r.ok) break;
    } catch { /* wait */ }
    await new Promise((r) => setTimeout(r, 250));
  }
  console.log('fiscal-local ready', base);
  console.log('Admin SSE UAT: login restaurant → watch nav badge → run: node scripts/fiscal-admin-sse-uat-prep.mjs ingest');
  process.on('SIGINT', () => { child.kill('SIGTERM'); process.exit(0); });
}

const cmd = process.argv[2] || 'start';
if (cmd === 'ingest') ingestOnce().catch((e) => { console.error(e); process.exit(1); });
else start().catch((e) => { console.error(e); process.exit(1); });
