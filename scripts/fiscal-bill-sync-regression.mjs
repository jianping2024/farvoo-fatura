#!/usr/bin/env node
/**
 * Bill sync Agent regression (farvoo-fiscal-bill-sync-api):
 * mock Farvoo pending-bill-syncs → PullAndIngest (unique path) → SQLite drafts + products → ack.
 */
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = 'http://127.0.0.1:17886';
const mockPort = 17991;
const dataRoot = join(agent, 'data', 'bill-sync-uat');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const pem = readFileSync(pemPath, 'utf8');
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

async function uatCmd(...args) {
  return (
    await run(process.execPath, [uat, ...args], {
      env: { ...process.env, FISCAL_UAT_BASE: base },
    })
  ).trim();
}

/** Person issue body with OCC revision from GET detail (ONLY path). */
async function personIssuePayload(draftId, extra = {}) {
  const detail = JSON.parse(await uatCmd('req', 'GET', `/local/v1/bill-drafts/${draftId}`));
  return {
    station_id: 'st-uat',
    operator_id: 'op-demo-cashier',
    mode: 'person',
    allocation_revision: detail.allocation_revision,
    ...extra,
  };
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

  const childEnv = { ...process.env, PATH: `/opt/homebrew/bin:${process.env.PATH}` };
  for (const k of Object.keys(childEnv)) {
    if (k.startsWith('FISCAL_') || k.startsWith('FARVOO_')) delete childEnv[k];
  }
  Object.assign(childEnv, {
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: secure,
    FISCAL_BIND: '127.0.0.1:17886',
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_AT_ENV: 'mock',
    FISCAL_ALLOW_LOCAL_PROVISION: '1',
  });

  const child = spawn('go', ['run', './cmd/fiscal-local'], {
    cwd: agent,
    env: childEnv,
    stdio: ['ignore', 'pipe', 'pipe'],
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
    record(
      'list-draft-gross-total',
      drafts.drafts?.[0]?.gross_total === '47.10',
      drafts.drafts?.[0]?.gross_total || 'missing',
    );
  } catch (e) {
    record('ingest-happy-ack', false, String(e));
    record('list-draft-gross-total', false, String(e));
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

  // Setup fiscal (taxpayer/AT/series/activate/operator) then issue-from-draft.
  try {
    await uatCmd(
      'req',
      'PUT',
      '/local/v1/setup/taxpayer',
      '--body',
      JSON.stringify({
        tax_registration_number: '517535009',
        legal_name: 'Demo Lda',
        address_detail: 'Rua 1',
        city: 'Lisboa',
        postal_code: '1000-001',
        country: 'PT',
        timezone: 'Europe/Lisbon',
        software_certificate_number: '0',
      }),
    );
    await uatCmd(
      'req',
      'PUT',
      '/local/v1/setup/at-credentials',
      '--body',
      JSON.stringify({ username: '517535009/37', password: 'demo' }),
    );
    const y = year;
    await uatCmd(
      'req',
      'POST',
      '/local/v1/setup/series/register',
      '--body',
      JSON.stringify({ series_code: `FT${y}DEMO01`, document_type: 'FT', fiscal_year: y }),
    );
    await uatCmd(
      'req',
      'POST',
      '/local/v1/setup/series/register',
      '--body',
      JSON.stringify({ series_code: `FS${y}DEMO01`, document_type: 'FS', fiscal_year: y }),
    );
    await uatCmd(
      'req',
      'POST',
      '/local/v1/setup/activate',
      '--body',
      JSON.stringify({ product_private_key_pem: pem }),
    );
    await uatCmd(
      'req',
      'PUT',
      '/local/v1/setup/operator',
      '--body',
      JSON.stringify({ id: 'op-demo-cashier', role: 'cashier', display_name: 'Cashier' }),
    );
    record('fiscal-setup', true, 'taxpayer/at/series/activate/op');
  } catch (e) {
    record('fiscal-setup', false, String(e));
  }

  let openDraftId = null;
  try {
    pendingJobs = [
      {
        id: 'job-issue',
        status: 'pending',
        payload: snap({ request_id: 'req-issue-1', source_sale_id: 'sale-issue-1' }),
      },
    ];
    acks.length = 0;
    await runPull();
    pendingJobs = [
      {
        id: 'job-issue-2',
        status: 'pending',
        payload: snap({ request_id: 'req-issue-2', source_sale_id: 'sale-issue-1' }),
      },
    ];
    await runPull();
    const drafts = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    const open = (drafts.drafts || []).filter((d) => d.source_sale_id === 'sale-issue-1' && d.status === 'open');
    const discarded = (drafts.drafts || []).filter(
      (d) => d.source_sale_id === 'sale-issue-1' && d.status === 'discarded',
    );
    openDraftId = open[0]?.id;
    record(
      'cover-keeps-one-open',
      open.length === 1 && discarded.length >= 1 && !!openDraftId,
      `open=${open.length} discarded=${discarded.length}`,
    );
  } catch (e) {
    record('cover-keeps-one-open', false, String(e));
  }

  try {
    if (!openDraftId) throw new Error('no open draft');
    const issued = JSON.parse(
      await uatCmd(
        'req',
        'POST',
        `/local/v1/bill-drafts/${openDraftId}/issue`,
        '--body',
        JSON.stringify({ station_id: "st-uat", operator_id: 'op-demo-cashier', mode: 'whole_table' }),
      ),
    );
    const drafts = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    const left = (drafts.drafts || []).filter((d) => d.source_sale_id === 'sale-issue-1');
    const invoiceNo = issued.InvoiceNo || issued.invoice_no;
    const printJobID = issued.PrintJobID || issued.print_job_id;
    const ok = !!invoiceNo && left.length === 0;
    record('issue-from-draft-hard-delete', ok, JSON.stringify({ InvoiceNo: invoiceNo, left: left.length }));
    if (printJobID) {
      await uatCmd(
        'wait-json',
        'GET',
        `/local/v1/print-jobs/${printJobID}`,
        '--path',
        'job_status',
        '--equals',
        'PRINTED',
        '--timeout-ms',
        '8000',
      );
      record('issue-from-draft-print-job', true, printJobID);
    } else {
      record('issue-from-draft-print-job', false, 'no PrintJobID');
    }
  } catch (e) {
    record('issue-from-draft-hard-delete', false, String(e));
    record('issue-from-draft-print-job', false, String(e));
  }

  try {
    pendingJobs = [
      {
        id: 'job-nif',
        status: 'pending',
        payload: snap({ request_id: 'req-nif', source_sale_id: 'sale-nif-1' }),
      },
    ];
    acks.length = 0;
    await runPull();
    const drafts = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    const d = (drafts.drafts || []).find((x) => x.source_sale_id === 'sale-nif-1' && x.status === 'open');
    if (!d) throw new Error('nif draft missing');
    const issued = JSON.parse(
      await uatCmd(
        'req',
        'POST',
        `/local/v1/bill-drafts/${d.id}/issue`,
        '--body',
        JSON.stringify({ station_id: "st-uat",
          operator_id: 'op-demo-cashier',
          mode: 'whole_table',
          customer_nif: '123456789',
          customer_name: 'Cliente Teste',
        }),
      ),
    );
    const docID = issued.DocumentID || issued.document_id;
    const tax = (
      await run('sqlite3', [
        dbPath,
        `SELECT customer_tax_id FROM invoice_customer_snapshots WHERE invoice_id='${docID}';`,
      ])
    ).trim();
    record('whole-table-custom-nif', tax === '123456789', `tax=${tax}`);
  } catch (e) {
    record('whole-table-custom-nif', false, String(e));
  }

  try {
    pendingJobs = [
      {
        id: 'job-re',
        status: 'pending',
        payload: snap({ request_id: 'req-re', source_sale_id: 'sale-issue-1' }),
      },
    ];
    acks.length = 0;
    await runPull();
    record(
      'already-invoiced-via-tax-db',
      acks[0]?.status === 'failed' && acks[0]?.error_code === 'already_invoiced',
      JSON.stringify(acks[0]),
    );
  } catch (e) {
    record('already-invoiced-via-tax-db', false, String(e));
  }

  const scopeA = '11111111-1111-1111-1111-111111111111';
  const scopeB = '22222222-2222-2222-2222-222222222222';
  let splitDraftId = null;

  try {
    const splitPayload = snap({
      request_id: 'req-split-work',
      source_sale_id: 'sale-split-work',
      scope_type: 'split',
      lines: [],
      gross_total: undefined,
      splits: [
        {
          scope_id: scopeA,
          name: 'Ana',
          lines: [
            {
              item_code: '006',
              name: 'CERVEJA',
              qty: '1.00',
              unit_price_gross: '2.25',
              line_gross: '2.25',
              vat_rate: '23.00',
            },
          ],
          gross_total: '2.25',
        },
        {
          scope_id: scopeB,
          name: 'Bruno',
          lines: [
            {
              item_code: 'BF-LUNCH-ADULT',
              name: 'BUFFET',
              qty: '1.00',
              unit_price_gross: '14.95',
              line_gross: '14.95',
              vat_rate: '13.00',
            },
          ],
          gross_total: '14.95',
        },
      ],
    });
    delete splitPayload.gross_total;
    delete splitPayload.lines;
    pendingJobs = [{ id: 'job-split-work', status: 'pending', payload: splitPayload }];
    acks.length = 0;
    await runPull();
    const drafts = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    const d = (drafts.drafts || []).find((x) => x.source_sale_id === 'sale-split-work' && x.status === 'open');
    splitDraftId = d?.id;
    record('ingest-split-draft', !!splitDraftId && acks[0]?.status === 'succeeded', splitDraftId || '');
  } catch (e) {
    record('ingest-split-draft', false, String(e));
  }

  try {
    if (!splitDraftId) throw new Error('no split draft');
    const r = await fetch(`${base}/local/v1/bill-drafts/${splitDraftId}/issue`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ station_id: "st-uat", operator_id: 'op-demo-cashier', mode: 'whole_table' }),
    });
    const body = await r.json();
    record(
      'reject-split-mode-whole-table',
      r.status >= 400 && body.error === 'validation_failed',
      JSON.stringify(body),
    );
  } catch (e) {
    record('reject-split-mode-whole-table', false, String(e));
  }

  let invA = null;
  try {
    if (!splitDraftId) throw new Error('no split draft');
    const issued = JSON.parse(
      await uatCmd(
        'req',
        'POST',
        `/local/v1/bill-drafts/${splitDraftId}/issue`,
        '--body',
        JSON.stringify(await personIssuePayload(splitDraftId, {
          scope_id: scopeA,
          customer_nif: '123456789',
          customer_name: 'Ana',
        })),
      ),
    );
    invA = issued.InvoiceNo || issued.invoice_no;
    const detail = JSON.parse(await uatCmd('req', 'GET', `/local/v1/bill-drafts/${splitDraftId}`));
    const scopes = detail.issued_scopes || [];
    const drafts = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    const still = (drafts.drafts || []).find((x) => x.id === splitDraftId && x.status === 'open');
    const tax = (
      await run('sqlite3', [
        dbPath,
        `SELECT customer_tax_id FROM invoice_customer_snapshots WHERE invoice_id='${issued.DocumentID || issued.document_id}';`,
      ])
    ).trim();
    record(
      'person-issue-a-keeps-draft',
      !!invA && !!still && scopes.length === 1 && tax === '123456789',
      JSON.stringify({ invA, scopes: scopes.length, tax }),
    );
  } catch (e) {
    record('person-issue-a-keeps-draft', false, String(e));
  }

  try {
    if (!splitDraftId) throw new Error('no split draft');
    const r = await fetch(`${base}/local/v1/bill-drafts/${splitDraftId}/issue`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ station_id: "st-uat", operator_id: 'op-demo-cashier', mode: 'whole_table' }),
    });
    const body = await r.json();
    record('scope-mutex-after-person', r.status >= 400 && body.error === 'scope_mutex', JSON.stringify(body));
  } catch (e) {
    record('scope-mutex-after-person', false, String(e));
  }

  try {
    if (!splitDraftId) throw new Error('no split draft');
    const again = JSON.parse(
      await uatCmd(
        'req',
        'POST',
        `/local/v1/bill-drafts/${splitDraftId}/issue`,
        '--body',
        JSON.stringify(await personIssuePayload(splitDraftId, { scope_id: scopeA })),
      ),
    );
    const hit = again.IdempotentHit || again.idempotent_hit;
    const no = again.InvoiceNo || again.invoice_no;
    record('person-idempotent-a', hit === true && no === invA, JSON.stringify(again));
  } catch (e) {
    record('person-idempotent-a', false, String(e));
  }

  try {
    if (!splitDraftId) throw new Error('no split draft');
    const issued = JSON.parse(
      await uatCmd(
        'req',
        'POST',
        `/local/v1/bill-drafts/${splitDraftId}/issue`,
        '--body',
        JSON.stringify(await personIssuePayload(splitDraftId, { scope_id: scopeB })),
      ),
    );
    const tax = (
      await run('sqlite3', [
        dbPath,
        `SELECT customer_tax_id FROM invoice_customer_snapshots WHERE invoice_id='${issued.DocumentID || issued.document_id}';`,
      ])
    ).trim();
    const drafts = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    const left = (drafts.drafts || []).filter((x) => x.source_sale_id === 'sale-split-work');
    record(
      'person-issue-b-deletes-draft',
      tax === '999999990' && left.length === 0 && !!(issued.InvoiceNo || issued.invoice_no),
      JSON.stringify({ tax, left: left.length }),
    );
  } catch (e) {
    record('person-issue-b-deletes-draft', false, String(e));
  }

  try {
    const discPayload = snap({
      request_id: 'req-disc',
      source_sale_id: 'sale-disc',
      scope_type: 'split',
      lines: [],
      splits: [
        {
          scope_id: scopeA,
          name: 'A',
          lines: [
            {
              item_code: 'A',
              name: 'X',
              qty: '1',
              unit_price_gross: '1.00',
              line_gross: '1.00',
              vat_rate: '23.00',
            },
          ],
          gross_total: '1.00',
        },
        {
          scope_id: scopeB,
          name: 'B',
          lines: [
            {
              item_code: 'B',
              name: 'Y',
              qty: '1',
              unit_price_gross: '1.00',
              line_gross: '1.00',
              vat_rate: '23.00',
            },
          ],
          gross_total: '1.00',
        },
      ],
    });
    delete discPayload.gross_total;
    delete discPayload.lines;
    pendingJobs = [{ id: 'job-disc', status: 'pending', payload: discPayload }];
    await runPull();
    const drafts = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    const d = (drafts.drafts || []).find((x) => x.source_sale_id === 'sale-disc' && x.status === 'open');
    if (!d) throw new Error('disc draft missing');
    const issued = JSON.parse(
      await uatCmd(
        'req',
        'POST',
        `/local/v1/bill-drafts/${d.id}/issue`,
        '--body',
        JSON.stringify(await personIssuePayload(d.id, { scope_id: scopeA })),
      ),
    );
    await uatCmd('req', 'POST', `/local/v1/bill-drafts/${d.id}/discard`, '--body', '{}');
    const after = JSON.parse(await uatCmd('req', 'GET', '/local/v1/bill-drafts'));
    const left = (after.drafts || []).filter((x) => x.source_sale_id === 'sale-disc');
    const nInv = (
      await run('sqlite3', [dbPath, `SELECT COUNT(1) FROM invoices WHERE source_sale_id='sale-disc';`])
    ).trim();
    record(
      'discard-keeps-invoices',
      left.length === 0 && nInv === '1' && !!(issued.InvoiceNo || issued.invoice_no),
      `left=${left.length} inv=${nInv}`,
    );
  } catch (e) {
    record('discard-keeps-invoices', false, String(e));
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
