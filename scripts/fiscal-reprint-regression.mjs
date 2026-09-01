#!/usr/bin/env node
/**
 * Reprint API regression (no skip):
 * issue FT → wait PRINTED → reprint → assert REPRINT job + invoice hash unchanged
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = process.env.FISCAL_UAT_BASE || 'http://127.0.0.1:17885';
const dbPath = join(agent, 'data', 'fiscal-reprint-regression.db');
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
  return (await run(process.execPath, [uat, ...args], {
    env: { ...process.env, FISCAL_UAT_BASE: base },
  })).trim();
}

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

async function main() {
  try {
    await run('pkill', ['-f', 'fiscal-local']);
  } catch { /* none */ }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);

  const childEnv = { ...process.env, PATH: `/opt/homebrew/bin:${process.env.PATH}` };
  for (const k of Object.keys(childEnv)) {
    if (k.startsWith('FISCAL_')) delete childEnv[k];
  }
  Object.assign(childEnv, {
    FISCAL_DB: dbPath,
    FISCAL_BIND: '127.0.0.1:17885',
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_SEED: '1',
    FISCAL_ALLOW_DEV_KEY: '1',
    FISCAL_AT_ENV: 'mock',
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

  const requestId = `reprint-${Date.now()}`;
  const issueBody = JSON.stringify({
    request_id: requestId,
    operator_id: 'op-demo-cashier',
    document_type: 'FT',
    snapshot: {
      source_system: 'farvoo',
      source_sale_id: `sale-${requestId}`,
      scope_type: 'session',
      scope_id: `scope-${requestId}`,
      fiscal_purpose: 'sale',
      lines: [{
        product_code: 'DEMO1',
        display_name: 'Prato Demo',
        saft_name: 'Prato Demo',
        quantity: '1',
        unit_price_gross: '12.50',
        vat_rate: '0.23',
        product_type: 'P',
        unit_of_measure: 'UN',
      }],
      customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
      payments: [{ method: 'CASH', amount: '12.50' }],
    },
  });

  let docId = '';
  let hashBefore = '';
  let printJobId = '';
  try {
    const issueRaw = await uatCmd('req', 'POST', '/local/v1/fiscal-documents', '--body', issueBody);
    const issue = JSON.parse(issueRaw.split('\n').pop() || issueRaw);
    docId = issue.document_id;
    printJobId = issue.print_job_id;
    record('issue-ft', !!docId, docId || issueRaw);
    if (!docId) throw new Error('no document_id');

    await uatCmd('wait-json', 'GET', `/local/v1/print-jobs/${printJobId}`, '--path', 'job_status', '--equals', 'PRINTED', '--timeout-ms', '15000');
    record('wait-original-printed', true, printJobId);

    const detailRaw = await uatCmd('req', 'GET', `/local/v1/fiscal-documents/${docId}`);
    const detail = JSON.parse(detailRaw.split('\n').pop() || detailRaw);
    hashBefore = detail.hash;
    record('get-invoice-detail', !!hashBefore, hashBefore ? 'hash ok' : detailRaw);

    const reprintRaw = await uatCmd('req', 'POST', `/local/v1/fiscal-documents/${docId}/reprints`, '--body', '{"operator_id":"op-demo-cashier"}');
    const reprint = JSON.parse(reprintRaw.split('\n').pop() || reprintRaw);
    const reprintJobId = reprint.print_job_id;
    record('reprint-api', reprint.print_purpose === 'REPRINT' || !!reprintJobId, reprintJobId || reprintRaw);

    await uatCmd('wait-json', 'GET', `/local/v1/print-jobs/${reprintJobId}`, '--path', 'job_status', '--equals', 'PRINTED', '--timeout-ms', '15000');
    record('wait-reprint-printed', true, reprintJobId);

    const detail2Raw = await uatCmd('req', 'GET', `/local/v1/fiscal-documents/${docId}`);
    const detail2 = JSON.parse(detail2Raw.split('\n').pop() || detail2Raw);
    const hashSame = detail2.hash === hashBefore;
    record('invoice-hash-unchanged', hashSame, `${hashBefore} vs ${detail2.hash}`);
    record('print-status-reprinted', detail2.print_status === 'REPRINTED', detail2.print_status);

    await uatCmd('assert-db', '--db', dbPath, '--sql', `SELECT COUNT(*) FROM local_print_jobs WHERE invoice_id='${docId}' AND print_purpose='REPRINT'`, '--expect-count', '1');
    record('sqlite-reprint-row', true, '1 REPRINT job');

    const listRaw = await uatCmd('req', 'GET', '/local/v1/fiscal-documents?limit=5');
    const list = JSON.parse(listRaw.split('\n').pop() || listRaw);
    const found = (list.invoices || []).some((i) => i.document_id === docId);
    record('list-invoices', found, `count=${(list.invoices || []).length}`);

    const year = new Date().getFullYear();
    const ndSeries = `ND${year}REPRINT01`;
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `INSERT OR IGNORE INTO series (id, store_id, document_type, series_code, validation_code, fiscal_year, last_number, last_hash, status, registered_at, created_at, updated_at) VALUES ('series-nd-reprint', 'store-demo-001', 'ND', '${ndSeries}', 'NDVAL1234', ${year}, 0, '', 'ACTIVE', datetime('now'), datetime('now'), datetime('now'))`);
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT id FROM series WHERE store_id='store-demo-001' AND document_type='ND' AND status='ACTIVE'`,
      '--expect-count', '1');
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `UPDATE operators SET can_issue_nc=1 WHERE store_id='store-demo-001' AND id='op-demo-cashier'`);
    record('seed-nd-series', true, ndSeries);

    const ndRaw = await uatCmd('req', 'POST', `/local/v1/fiscal-documents/${docId}/debit-notes`, '--body', JSON.stringify({
      request_id: `nd-${requestId}`,
      operator_id: 'op-demo-cashier',
      reason: 'Partial debit for reprint test',
      debit_full: false,
      lines: [{ original_line_number: 1, line_gross: '5.00' }],
    }));
    const nd = JSON.parse(ndRaw.split('\n').pop() || ndRaw);
    record('issue-nd-partial', nd.document_type === 'ND', nd.invoice_no || ndRaw);

    const origAfterNdRaw = await uatCmd('req', 'GET', `/local/v1/fiscal-documents/${docId}`);
    const origAfterNd = JSON.parse(origAfterNdRaw.split('\n').pop() || origAfterNdRaw);
    record('original-debited-partial', origAfterNd.document_status === 'DEBITED_PARTIAL', origAfterNd.document_status);

    const reprint2Raw = await uatCmd('req', 'POST', `/local/v1/fiscal-documents/${docId}/reprints`, '--body', '{"operator_id":"op-demo-cashier"}');
    const reprint2 = JSON.parse(reprint2Raw.split('\n').pop() || reprint2Raw);
    const reprint2JobId = reprint2.print_job_id;
    record('reprint-after-nd', !!reprint2JobId && reprint2.print_purpose === 'REPRINT', reprint2JobId || reprint2Raw);

    await uatCmd('wait-json', 'GET', `/local/v1/print-jobs/${reprint2JobId}`, '--path', 'job_status', '--equals', 'PRINTED', '--timeout-ms', '15000');
    record('wait-reprint-after-nd-printed', true, reprint2JobId);

    const detail3Raw = await uatCmd('req', 'GET', `/local/v1/fiscal-documents/${docId}`);
    const detail3 = JSON.parse(detail3Raw.split('\n').pop() || detail3Raw);
    record('hash-unchanged-after-nd-reprint', detail3.hash === hashBefore, detail3.hash);
  } catch (e) {
    record('reprint-flow', false, String(e.message || e).slice(0, 200));
  } finally {
    child.kill('SIGTERM');
  }

  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n--- fiscal-reprint-regression summary ---');
  console.log(JSON.stringify(results, null, 2));
  if (failed.length) {
    console.error(`\n${failed.length} failed`);
    process.exit(1);
  }
  console.log('\nAll passed.');
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
