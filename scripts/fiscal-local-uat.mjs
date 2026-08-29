#!/usr/bin/env node
/**
 * Fiscal local UAT helper (mesa-local-uat analogue for farvoo-fatura).
 *
 * Usage (repo root):
 *   node scripts/fiscal-local-uat.mjs stack-health
 *   node scripts/fiscal-local-uat.mjs req POST /local/v1/fiscal-documents --body '...'
 *   node scripts/fiscal-local-uat.mjs req GET /local/v1/print-jobs/{id}
 *   node scripts/fiscal-local-uat.mjs wait-json GET /local/v1/print-jobs/{id} --path job_status --equals PRINTED
 *   node scripts/fiscal-local-uat.mjs assert-db --db path --sql 'SELECT ...' --expect-count 1
 *
 * Env:
 *   FISCAL_UAT_BASE (default http://127.0.0.1:17880)
 */
import { existsSync, readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';

const BASE = (process.env.FISCAL_UAT_BASE || 'http://127.0.0.1:17880').replace(/\/$/, '');

function usage(exit = 1) {
  console.error(`fiscal-local-uat:
  stack-health
  req METHOD PATH [--body JSON] [--header 'K: V']... [--expect-status N]
  wait-json METHOD PATH --path a.b [--equals v] [--timeout-ms N]
  assert-db --db PATH --sql 'SELECT ...' [--expect-count N]`);
  process.exit(exit);
}

function parseArgs(argv) {
  const out = { _: [], header: [] };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith('--')) {
      const key = a.slice(2);
      const next = argv[i + 1];
      if (key === 'header') {
        if (!next || next.startsWith('--')) usage();
        out.header.push(next);
        i++;
      } else if (!next || next.startsWith('--')) out[key] = true;
      else {
        out[key] = next;
        i++;
      }
    } else out._.push(a);
  }
  return out;
}

function parseHeaders(list) {
  const h = {};
  for (const raw of list || []) {
    const i = raw.indexOf(':');
    if (i < 0) continue;
    h[raw.slice(0, i).trim()] = raw.slice(i + 1).trim();
  }
  return h;
}

function getPath(obj, path) {
  return String(path || '')
    .split('.')
    .filter(Boolean)
    .reduce((o, k) => (o == null ? undefined : o[k]), obj);
}

async function req(method, path, body, headers) {
  const h = {};
  if (body) h['Content-Type'] = 'application/json';
  if (headers) Object.assign(h, headers);
  const r = await fetch(BASE + path, {
    method,
    headers: Object.keys(h).length ? h : undefined,
    body: body || undefined,
  });
  const text = await r.text();
  let json = null;
  try {
    json = JSON.parse(text);
  } catch {
    json = { raw: text };
  }
  return { status: r.status, json };
}

async function stackHealth() {
  const r = await req('GET', '/local/v1/health');
  if (r.status !== 200 || r.json?.status !== 'ok') {
    console.error(JSON.stringify(r, null, 2));
    process.exit(1);
  }
  console.log(JSON.stringify({ ok: true, base: BASE, health: r.json }));
}

async function waitJson(method, path, jsonPath, equals, timeoutMs) {
  const start = Date.now();
  let last;
  while (Date.now() - start < timeoutMs) {
    last = await req(method, path);
    const v = getPath(last.json, jsonPath);
    if (equals === undefined || String(v) === String(equals)) {
      if (last.status >= 200 && last.status < 300) {
        console.log(JSON.stringify(last.json));
        return;
      }
    }
    await new Promise((r) => setTimeout(r, 150));
  }
  console.error('timeout', { last, path: jsonPath, equals });
  process.exit(1);
}

function assertDb(dbPath, sql, expectCount) {
  if (!existsSync(dbPath)) {
    console.error('db missing', dbPath);
    process.exit(1);
  }
  // Prefer sqlite3 CLI; fallback to node better-sqlite3 if present
  const cli = spawnSync('sqlite3', [dbPath, sql], { encoding: 'utf8' });
  if (cli.error && cli.error.code === 'ENOENT') {
    try {
      const require = createRequire(import.meta.url);
      const Database = require('better-sqlite3');
      const db = new Database(dbPath, { readonly: true });
      const rows = db.prepare(sql).all();
      if (expectCount !== undefined && rows.length !== Number(expectCount)) {
        console.error('expect-count', expectCount, 'got', rows.length, rows);
        process.exit(1);
      }
      console.log(JSON.stringify({ ok: true, rows }));
      return;
    } catch (e) {
      console.error('need sqlite3 CLI or better-sqlite3', e.message);
      process.exit(1);
    }
  }
  if (cli.status !== 0) {
    console.error(cli.stderr || cli.stdout);
    process.exit(cli.status || 1);
  }
  const lines = cli.stdout.trim() === '' ? [] : cli.stdout.trim().split('\n');
  if (expectCount !== undefined && lines.length !== Number(expectCount)) {
    console.error('expect-count', expectCount, 'got', lines.length, lines);
    process.exit(1);
  }
  console.log(JSON.stringify({ ok: true, rows: lines }));
}

const args = parseArgs(process.argv.slice(2));
const cmd = args._[0];
if (!cmd) usage();

if (cmd === 'stack-health') await stackHealth();
else if (cmd === 'req') {
  const method = args._[1];
  const path = args._[2];
  if (!method || !path) usage();
  const r = await req(method, path, args.body, parseHeaders(args.header));
  console.log(JSON.stringify(r.json));
  const expect = args['expect-status'] != null ? Number(args['expect-status']) : null;
  if (expect != null) {
    if (r.status !== expect) {
      console.error('expect-status', expect, 'got', r.status);
      process.exit(1);
    }
  } else if (r.status >= 400) process.exit(1);
} else if (cmd === 'wait-json') {
  const method = args._[1];
  const path = args._[2];
  await waitJson(method, path, args.path, args.equals, Number(args['timeout-ms'] || 8000));
} else if (cmd === 'assert-db') {
  assertDb(args.db, args.sql, args['expect-count']);
} else usage();
