#!/usr/bin/env node
/**
 * Prep fiscal-local + whole_table draft with qty≥2 for Admin split merge/amount UAT.
 * Leaves fiscal-local on 17880. Mock Farvoo on 17993.
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

const wholeJob = {
  id: 'job-split-merge-ui',
  status: 'pending',
  payload: {
    request_id: 'req-split-merge-ui',
    source_system: 'farvoo',
    source_sale_id: 'sale-split-merge-ui',
    table_display_name: 'A-01',
    scope_type: 'whole_table',
    gross_total: '44.90',
    lines: [
      {
        item_code: 'BF1', name: 'Buffet livre', qty: '2',
        unit_price_gross: '19.95', line_gross: '39.90', vat_rate: '23.00',
      },
      {
        item_code: 'W1', name: 'Água 500ml', qty: '1',
        unit_price_gross: '1.50', line_gross: '1.50', vat_rate: '13.00',
      },
      {
        item_code: 'V1', name: 'Vitalis 750ml', qty: '1',
        unit_price_gross: '3.50', line_gross: '3.50', vat_rate: '13.00',
      },
    ],
  },
};

let pending = [wholeJob];

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
    FISCAL_STATION_PRINTERS_JSON: JSON.stringify({
      '2951b0b2-aaaa-bbbb-cccc-ddddeeeeffff': 'tcp:127.0.0.1:9100',
    }),
    FISCAL_STATION_META_JSON: JSON.stringify([
      { id: '2951b0b2-aaaa-bbbb-cccc-ddddeeeeffff', name_zh: '厨房', sort_order: 0 },
    ]),
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

  const list = await fetch(base + '/local/v1/bill-drafts').then((r) => r.json());
  const draft = (list.drafts || list || []).find?.(Boolean) || list?.[0];
  console.log('whole_table draft ready; fiscal-local at', base);
  console.log('Admin: 餐馆模式 → 待开票账单 → A-01 → 分单；Buffet×2 测累加 + 本票预估');
  if (draft?.id) console.log('draft_id', draft.id);
  process.on('SIGINT', () => { child.kill('SIGTERM'); mock.close(); process.exit(0); });
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
