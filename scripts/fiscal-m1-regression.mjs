#!/usr/bin/env node
/**
 * M1 full regression (no SeedDemo):
 * health → issue fails (series_missing) → taxpayer → AT creds → register series →
 * activate → operator → issue FT → PRINTED → idempotent
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = process.env.FISCAL_UAT_BASE || 'http://127.0.0.1:17881';
const bind = '127.0.0.1:17881';
const dbPath = join(agent, 'data', 'fiscal-m1.db');
const dataDir = join(agent, 'data', 'fiscal-m1-secure');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const year = new Date().getFullYear();

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

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

async function main() {
  try {
    await run('pkill', ['-f', 'fiscal-local']);
  } catch {
    /* */
  }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true, force: true });

  const pem = readFileSync(pemPath, 'utf8');
  const childEnv = { ...process.env, PATH: `/opt/homebrew/bin:${process.env.PATH}` };
  for (const k of Object.keys(childEnv)) {
    if (k.startsWith('FISCAL_')) delete childEnv[k];
  }
  Object.assign(childEnv, {
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: dataDir,
    FISCAL_BIND: bind,
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
    if (child.exitCode != null && child.exitCode !== 0) break;
    try {
      await uatCmd('stack-health');
      healthy = true;
      break;
    } catch {
      await new Promise((r) => setTimeout(r, 250));
    }
  }
  record('stack-health', healthy, healthy ? base : boot.slice(-500));
  if (!healthy) {
    child.kill('SIGTERM');
    process.exit(1);
  }

  // 1) issue without setup → series_missing
  try {
    await uatCmd(
      'req',
      'POST',
      '/local/v1/fiscal-documents',
      '--body',
      JSON.stringify({
        request_id: 'early-1',
        operator_id: 'op-demo-cashier',
        snapshot: {
          source_system: 'farvoo',
          source_sale_id: 'x',
          scope_type: 's',
          scope_id: 's',
          fiscal_purpose: 'sale',
          lines: [
            {
              product_code: 'P',
              display_name: 'X',
              saft_name: 'X',
              quantity: '1',
              unit_price_gross: '1.00',
              vat_rate: '0.23',
            },
          ],
        },
      }),
    );
    record('issue-before-setup-fails', false, 'expected error');
  } catch (e) {
    const ok = String(e).includes('series_missing') || String(e).includes('series_missing');
    // uat req exits 1 and throws with JSON in message
    const msg = String(e);
    record('issue-before-setup-fails', msg.includes('series_missing') || msg.includes('signer_not_ready'), msg.slice(-200));
  }

  try {
    await uatCmd(
      'req',
      'PUT',
      '/local/v1/setup/taxpayer',
      '--body',
      JSON.stringify({
        tax_registration_number: '517535009',
        legal_name: 'Farvoo Demo Lda',
        address_detail: 'Rua Demo 1',
        city: 'Lisboa',
        postal_code: '1000-001',
        country: 'PT',
        timezone: 'Europe/Lisbon',
        software_certificate_number: '0',
      }),
    );
    record('setup-taxpayer', true);
  } catch (e) {
    record('setup-taxpayer', false, String(e));
  }

  try {
    await uatCmd(
      'req',
      'PUT',
      '/local/v1/setup/at-credentials',
      '--body',
      JSON.stringify({ username: '517535009/37', password: 'demo-secret' }),
    );
    record('setup-at-credentials', true);
  } catch (e) {
    record('setup-at-credentials', false, String(e));
  }

  let status;
  try {
    const raw = await uatCmd(
      'req',
      'POST',
      '/local/v1/setup/series/register',
      '--body',
      JSON.stringify({ series_code: `FT${year}DEMO01`, document_type: 'FT', fiscal_year: year }),
    );
    status = JSON.parse(raw);
    record('setup-series-register', status.series_ok === true && !!status.validation_code, status.validation_code);
  } catch (e) {
    record('setup-series-register', false, String(e));
  }

  try {
    await uatCmd(
      'assert-db',
      '--db',
      dbPath,
      '--sql',
      `SELECT validation_code FROM series WHERE status='ACTIVE';`,
      '--expect-count',
      '1',
    );
    record('sqlite-series-validation', true);
  } catch (e) {
    record('sqlite-series-validation', false, String(e));
  }

  try {
    const raw = await uatCmd(
      'req',
      'POST',
      '/local/v1/setup/activate',
      '--body',
      JSON.stringify({ product_private_key_pem: pem }),
    );
    status = JSON.parse(raw);
    record('setup-activate', status.activated_ok === true);
  } catch (e) {
    record('setup-activate', false, String(e));
  }

  try {
    await uatCmd(
      'req',
      'PUT',
      '/local/v1/setup/operator',
      '--body',
      JSON.stringify({ id: 'op-demo-cashier', role: 'cashier', display_name: 'Demo Cashier' }),
    );
    record('setup-operator', true);
  } catch (e) {
    record('setup-operator', false, String(e));
  }

  const requestId = `m1-${Date.now()}`;
  const body = {
    request_id: requestId,
    operator_id: 'op-demo-cashier',
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
  };

  let issue;
  try {
    const raw = await uatCmd('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify(body));
    issue = JSON.parse(raw);
    const hashOk = true; // hash not in response; check via length of invoice
    record(
      'issue-ft',
      issue.document_status === 'SIGNED' && String(issue.invoice_no).includes('/1'),
      issue.invoice_no,
    );
  } catch (e) {
    record('issue-ft', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
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
    record('print-job-printed', true);
  } catch (e) {
    record('print-job-printed', false, String(e));
  }

  try {
    const raw2 = await uatCmd('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify(body));
    const again = JSON.parse(raw2);
    record('idempotent-reissue', again.idempotent_hit === true && again.document_id === issue.document_id);
  } catch (e) {
    record('idempotent-reissue', false, String(e));
  }

  // hash length via sqlite
  try {
    await uatCmd(
      'assert-db',
      '--db',
      dbPath,
      '--sql',
      `SELECT 1 FROM invoices WHERE id='${issue.document_id}' AND length(hash) >= 100;`,
      '--expect-count',
      '1',
    );
    record('hash-length', true);
  } catch (e) {
    record('hash-length', false, String(e));
  }

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n=== M1 SUMMARY ===');
  for (const r of results) console.log(`${r.status}\t${r.name}\t${r.note}`);
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
