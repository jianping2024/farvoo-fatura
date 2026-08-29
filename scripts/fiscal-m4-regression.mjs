#!/usr/bin/env node
/**
 * M4 regression: issue FT → sync_outbox PENDING → mock Farvoo SENT;
 * §13 auth required: missing creds → 401; terminal + operator_token → 200.
 * No t.Skip / scenario skip.
 */
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { createHmac } from 'node:crypto';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = 'http://127.0.0.1:17887';
const mockPort = 17992;
const dataRoot = join(agent, 'data', 'm4-uat');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const pem = readFileSync(pemPath, 'utf8');
const jwtSecret = 'm4-uat-operator-jwt-secret';
const year = new Date().getFullYear();

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

function b64url(buf) {
  return Buffer.from(buf)
    .toString('base64')
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_');
}

function signOperatorToken(claims) {
  const header = b64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = b64url(JSON.stringify(claims));
  const body = `${header}.${payload}`;
  const sig = createHmac('sha256', jwtSecret).update(body).digest('base64url');
  return `${body}.${sig}`;
}

async function uatCmd(extraEnv, ...args) {
  return (
    await run(process.execPath, [uat, ...args], {
      env: { ...process.env, FISCAL_UAT_BASE: base, ...extraEnv },
    })
  ).trim();
}

async function uatReq(method, path, { body, headers, expectStatus } = {}) {
  const args = ['req', method, path];
  if (body) args.push('--body', typeof body === 'string' ? body : JSON.stringify(body));
  if (headers) {
    for (const [k, v] of Object.entries(headers)) {
      args.push('--header', `${k}: ${v}`);
    }
  }
  if (expectStatus != null) args.push('--expect-status', String(expectStatus));
  const out = await uatCmd({}, ...args);
  return JSON.parse(out);
}

async function main() {
  const copies = [];
  const mock = createServer(async (req, res) => {
    if (req.method === 'POST' && req.url === '/api/print-agent/fiscal-invoice-copies') {
      const chunks = [];
      for await (const c of req) chunks.push(c);
      const raw = Buffer.concat(chunks).toString('utf8');
      copies.push(JSON.parse(raw));
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ ok: true }));
      return;
    }
    res.writeHead(404);
    res.end();
  });
  await new Promise((r) => mock.listen(mockPort, '127.0.0.1', r));

  if (existsSync(dataRoot)) rmSync(dataRoot, { recursive: true, force: true });
  mkdirSync(dataRoot, { recursive: true });
  const dbPath = join(dataRoot, 'fiscal.db');

  const childEnv = {
    ...process.env,
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: join(dataRoot, 'secure'),
    FISCAL_BIND: '127.0.0.1:17887',
    FISCAL_STORE_ID: 'store-m4-001',
    FISCAL_SEED: '1',
    FISCAL_ALLOW_DEV_KEY: '1',
    FISCAL_KEY_PEM: pemPath,
    FISCAL_AUTH_MODE: 'required',
    FISCAL_OPERATOR_JWT_SECRET: jwtSecret,
    FARVOO_API: `http://127.0.0.1:${mockPort}`,
    FARVOO_JWT: 'mock-agent-jwt',
    FISCAL_STATION_PRINTERS_JSON: JSON.stringify({ 'st-uat': 'memory:st-uat' }),
  };
  // Clear conflicting FISCAL_ keys then apply
  for (const k of Object.keys(childEnv)) {
    /* keep */
  }

  const agentProc = spawn(
    'go',
    ['run', './cmd/fiscal-local'],
    { cwd: agent, env: childEnv, stdio: ['ignore', 'pipe', 'pipe'] },
  );
  let agentLog = '';
  agentProc.stderr.on('data', (d) => (agentLog += d));
  agentProc.stdout.on('data', (d) => (agentLog += d));

  const cleanup = async () => {
    agentProc.kill('SIGTERM');
    mock.close();
  };

  try {
    await new Promise((resolve, reject) => {
      const t0 = Date.now();
      const tick = async () => {
        try {
          const r = await fetch(base + '/local/v1/health');
          if (r.ok) return resolve();
        } catch {
          /* retry */
        }
        if (Date.now() - t0 > 45000) reject(new Error('agent start timeout\n' + agentLog));
        else setTimeout(tick, 200);
      };
      tick();
    });
    record('stack-health', true);

    // Pair terminal
    await uatReq('PUT', '/local/v1/setup/issue-terminal', {
      body: {
        id: 'term-m4-1',
        store_id: 'store-m4-001',
        display_name: 'UAT Terminal',
        secret: 'term-secret-m4',
        station_id: 'st-uat',
      },
    });
    record('setup-issue-terminal', true);

    const snap = {
      source_system: 'farvoo',
      source_sale_id: 'sale-m4-1',
      scope_type: 'whole_table',
      scope_id: 'sale-m4-1',
      fiscal_purpose: 'sale',
      lines: [
        {
          product_code: 'P1',
          display_name: 'Prato',
          saft_name: 'Prato',
          quantity: '1',
          unit_price_gross: '12.50',
          vat_rate: '0.23',
          product_type: 'P',
          unit_of_measure: 'UN',
        },
      ],
      customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
      payments: [{ method: 'CASH', amount: '12.50' }],
    };

    // Auth required: no headers → 401
    let denied = false;
    try {
      await uatReq('POST', '/local/v1/fiscal-documents', {
        body: {
          store_id: 'store-m4-001',
          request_id: 'req-m4-deny',
          operator_id: 'op-demo-cashier',
          station_id: 'st-uat',
          document_type: 'FT',
          snapshot: snap,
        },
        expectStatus: 401,
      });
      denied = true;
    } catch (e) {
      denied = String(e).includes('401') || String(e).includes('expect-status');
    }
    // fiscal-local-uat with expect-status should succeed when status matches
    const denyRes = await fetch(base + '/local/v1/fiscal-documents', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        store_id: 'store-m4-001',
        request_id: 'req-m4-deny',
        operator_id: 'op-demo-cashier',
        station_id: 'st-uat',
        document_type: 'FT',
        snapshot: snap,
      }),
    });
    record('auth-deny-without-creds', denyRes.status === 401, `status=${denyRes.status}`);

    const token = signOperatorToken({
      store_id: 'store-m4-001',
      mesa_user_id: 'mesa-m4-cashier',
      role: 'cashier',
      terminal_id: 'term-m4-1',
      display_name: 'M4 Cashier',
      iat: Math.floor(Date.now() / 1000),
      exp: Math.floor(Date.now() / 1000) + 900,
      jti: 'jti-m4-1',
    });

    const issued = await uatReq('POST', '/local/v1/fiscal-documents', {
      body: {
        store_id: 'store-m4-001',
        request_id: 'req-m4-ok',
        station_id: 'st-uat',
        document_type: 'FT',
        snapshot: snap,
      },
      headers: {
        'X-Fiscal-Terminal-Id': 'term-m4-1',
        'X-Fiscal-Terminal-Secret': 'term-secret-m4',
        'X-Fiscal-Operator-Token': token,
      },
    });
    record(
      'auth-issue-with-terminal-token',
      !!issued.invoice_no && issued.document_status === 'SIGNED',
      issued.invoice_no || JSON.stringify(issued),
    );

    // Outbox PENDING then SENT via worker
    await run(process.execPath, [
      uat,
      'assert-db',
      '--db',
      dbPath,
      '--sql',
      "SELECT id FROM sync_outbox WHERE event_type='INVOICE_ISSUED' AND status IN ('PENDING','SENT')",
      '--expect-count',
      '1',
    ], { env: { ...process.env, FISCAL_UAT_BASE: base } });
    record('outbox-row-exists', true);

    const sentDeadline = Date.now() + 15000;
    let sent = false;
    while (Date.now() < sentDeadline) {
      try {
        await run(process.execPath, [
          uat,
          'assert-db',
          '--db',
          dbPath,
          '--sql',
          "SELECT id FROM sync_outbox WHERE status='SENT'",
          '--expect-count',
          '1',
        ], { env: { ...process.env, FISCAL_UAT_BASE: base } });
        sent = true;
        break;
      } catch {
        await new Promise((r) => setTimeout(r, 200));
      }
    }
    record('outbox-sent', sent, sent ? `copies=${copies.length}` : 'timeout waiting SENT');
    record(
      'farvoo-mock-received-copy',
      copies.length >= 1 && copies[0].invoice_no === issued.invoice_no,
      copies[0]?.invoice_no || 'none',
    );

    // Idempotent: same request_id → no second outbox
    const hit = await uatReq('POST', '/local/v1/fiscal-documents', {
      body: {
        store_id: 'store-m4-001',
        request_id: 'req-m4-ok',
        station_id: 'st-uat',
        document_type: 'FT',
        snapshot: snap,
      },
      headers: {
        'X-Fiscal-Terminal-Id': 'term-m4-1',
        'X-Fiscal-Terminal-Secret': 'term-secret-m4',
        'X-Fiscal-Operator-Token': token,
      },
    });
    record('idempotent-hit', hit.idempotent_hit === true, JSON.stringify(hit.idempotent_hit));
    await run(process.execPath, [
      uat,
      'assert-db',
      '--db',
      dbPath,
      '--sql',
      "SELECT id FROM sync_outbox WHERE event_type='INVOICE_ISSUED'",
      '--expect-count',
      '1',
    ], { env: { ...process.env, FISCAL_UAT_BASE: base } });
    record('outbox-no-duplicate-on-idempotent', true);

    await uatCmd({}, 'wait-json', 'GET', `/local/v1/print-jobs/${issued.print_job_id}`, '--path', 'job_status', '--equals', 'PRINTED', '--timeout-ms', '10000');
    record('print-printed', true);
  } catch (e) {
    record('fatal', false, String(e.message || e));
    console.error(e);
  } finally {
    await cleanup();
  }

  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n=== M4 regression summary ===');
  for (const r of results) console.log(`${r.status.toUpperCase()}  ${r.name}${r.note ? ' — ' + r.note : ''}`);
  if (failed.length) {
    process.exit(1);
  }
}

main();
