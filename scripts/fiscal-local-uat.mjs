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
let cookieJar = process.env.FISCAL_UAT_COOKIE || '';

function mergeCookies(existing, setCookies) {
  const jar = new Map();
  for (const part of (existing || '').split(';')) {
    const p = part.trim();
    if (!p) continue;
    const eq = p.indexOf('=');
    if (eq > 0) jar.set(p.slice(0, eq), p.slice(eq + 1));
  }
  for (const sc of setCookies || []) {
    const bit = String(sc).split(';')[0];
    const eq = bit.indexOf('=');
    if (eq > 0) jar.set(bit.slice(0, eq).trim(), bit.slice(eq + 1).trim());
  }
  return [...jar.entries()].map(([k, v]) => `${k}=${v}`).join('; ');
}

function usage(exit = 1) {
  console.error(`fiscal-local-uat:
  stack-health
  req METHOD PATH [--body JSON]
  login OPERATOR_ID PIN
  wait-json METHOD PATH --path a.b [--equals v] [--timeout-ms N]
  assert-db --db PATH --sql 'SELECT ...' [--expect-count N]
  exec-db --db PATH --sql 'UPDATE ...'`);
  process.exit(exit);
}

function parseArgs(argv) {
  const out = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith('--')) {
      const key = a.slice(2);
      const next = argv[i + 1];
      if (!next || next.startsWith('--')) out[key] = true;
      else {
        out[key] = next;
        i++;
      }
    } else out._.push(a);
  }
  return out;
}

function getPath(obj, path) {
  return String(path || '')
    .split('.')
    .filter(Boolean)
    .reduce((o, k) => (o == null ? undefined : o[k]), obj);
}

async function req(method, path, body) {
  const headers = {};
  if (body) headers['Content-Type'] = 'application/json';
  if (cookieJar) headers.Cookie = cookieJar;
  const r = await fetch(BASE + path, {
    method,
    headers: Object.keys(headers).length ? headers : undefined,
    body: body || undefined,
  });
  const setCookies = typeof r.headers.getSetCookie === 'function' ? r.headers.getSetCookie() : [];
  if (setCookies.length) cookieJar = mergeCookies(cookieJar, setCookies);
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

function execDb(dbPath, sql) {
  if (!existsSync(dbPath)) {
    console.error('db missing', dbPath);
    process.exit(1);
  }
  const cli = spawnSync('sqlite3', [dbPath, sql], { encoding: 'utf8' });
  if (cli.status !== 0) {
    console.error(cli.stderr || cli.stdout);
    process.exit(cli.status || 1);
  }
  console.log(JSON.stringify({ ok: true }));
}

const args = parseArgs(process.argv.slice(2));
const cmd = args._[0];
if (!cmd) usage();

if (cmd === 'stack-health') await stackHealth();
else if (cmd === 'req') {
  const method = args._[1];
  const path = args._[2];
  if (!method || !path) usage();
  const r = await req(method, path, args.body);
  console.log(JSON.stringify(r.json));
  if (r.status >= 400) process.exit(1);
} else if (cmd === 'login') {
  const operatorId = args._[1];
  const pin = args._[2];
  if (!operatorId || !pin) usage();
  const r = await req('POST', '/local/v1/setup/login', JSON.stringify({ operator_id: operatorId, pin }));
  console.log(JSON.stringify(r.json));
  if (r.status >= 400) process.exit(1);
  console.log(`FISCAL_UAT_COOKIE=${cookieJar}`);
} else if (cmd === 'wait-json') {
  const method = args._[1];
  const path = args._[2];
  await waitJson(method, path, args.path, args.equals, Number(args['timeout-ms'] || 8000));
} else if (cmd === 'assert-db') {
  assertDb(args.db, args.sql, args['expect-count']);
} else if (cmd === 'exec-db') {
  execDb(args.db, args.sql);
} else usage();
