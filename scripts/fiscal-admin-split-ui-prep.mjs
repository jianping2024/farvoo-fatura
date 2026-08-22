#!/usr/bin/env node
/**
 * Prep fiscal-local + one split bill-sync draft for Admin UI browser UAT.
 * Leaves fiscal-local running on 17880. Mock Farvoo on 17993.
 */
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { mkdirSync, rmSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const dbPath = join(agent, 'data', 'fiscal-admin-split-ui.db');
const mockPort = 17993;
const base = 'http://127.0.0.1:17880';

const scopeA = '11111111-1111-1111-1111-111111111111';
const scopeB = '22222222-2222-2222-2222-222222222222';

const splitJob = {
  id: 'job-split-ui',
  status: 'pending',
  payload: {
    request_id: 'req-split-ui',
    source_system: 'farvoo',
    source_sale_id: 'sale-split-ui',
    table_display_name: 'A-05',
    scope_type: 'split',
    lines: [],
    splits: [
      {
        scope_id: scopeA,
        name: 'Ana',
        lines: [{
          item_code: 'TEA-01', name: 'Tea', qty: '1', unit_price_gross: '4.50',
          line_gross: '4.50', vat_rate: '13.00',
        }],
        gross_total: '4.50',
      },
      {
        scope_id: scopeB,
        name: 'Bruno',
        lines: [{
          item_code: '006', name: 'Beer', qty: '1', unit_price_gross: '2.25',
          line_gross: '2.25', vat_rate: '23.00',
        }],
        gross_total: '2.25',
      },
    ],
  },
};

let pending = [splitJob];
const acks = [];

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
    let body = '';
    req.on('data', (c) => (body += c));
    req.on('end', () => {
      acks.push(JSON.parse(body || '{}'));
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ ok: true }));
    });
    return;
  }
  res.writeHead(404);
  res.end();
});

function run(cmd, args, opts = {}) {
  return new Promise((resolve, reject) => {
    const p = spawn(cmd, args, { stdio: 'inherit', ...opts });
    p.on('close', (code) => (code === 0 ? resolve() : reject(new Error(`${cmd} exit ${code}`))));
  });
}

async function main() {
  try { await run('pkill', ['-f', 'fiscal-local']); } catch { /* none */ }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);

  await new Promise((resolve) => mock.listen(mockPort, resolve));
  console.log('mock Farvoo on', mockPort);

  const childEnv = {
    ...process.env,
    PATH: `/opt/homebrew/bin:${process.env.PATH}`,
    FISCAL_DB: dbPath,
    FISCAL_BIND: '127.0.0.1:17880',
    FISCAL_SEED: '1',
    FISCAL_ALLOW_DEV_KEY: '1',
    FISCAL_AT_ENV: 'mock',
  };

  const child = spawn('go', ['run', './cmd/fiscal-local'], {
    cwd: agent,
    env: childEnv,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stdout.on('data', (d) => process.stdout.write(d));
  child.stderr.on('data', (d) => process.stderr.write(d));

  for (let i = 0; i < 80; i++) {
    try {
      const r = await fetch(base + '/local/v1/health');
      if (r.ok) break;
    } catch { /* wait */ }
    await new Promise((r) => setTimeout(r, 250));
  }

  await run('go', ['run', './dev/bill-sync-pull-once'], {
    cwd: agent,
    env: { ...childEnv, FARVOO_API: `http://127.0.0.1:${mockPort}`, FARVOO_JWT: 'test-jwt' },
  });

  console.log('split draft ingested; fiscal-local at', base);
  console.log('Admin: 餐馆模式 → 待开票账单 → A-05 按人 → 应见 Ana / Bruno');
  process.on('SIGINT', () => { child.kill('SIGTERM'); mock.close(); process.exit(0); });
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
