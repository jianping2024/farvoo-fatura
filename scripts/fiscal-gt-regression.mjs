#!/usr/bin/env node
/**
 * GT (guia de transporte) regression — GT-01…08 (mock AT transport).
 * No skips.
 */
import { ensureOwnerSession, setFiscalProfileViaDb, envWithCookie, fiscalAgentTestEnv } from './fiscal-session-helper.mjs';
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17889';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-gt.db');
const dataDir = join(agent, 'data', 'fiscal-gt-secure');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const year = new Date().getFullYear();
const STORE_NIF = '517535009';

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

let uatEnv = { FISCAL_UAT_BASE: base };

async function uatCmd(...args) {
  return (
    await run(process.execPath, [uat, ...args], {
      env: { ...process.env, ...uatEnv },
    })
  ).trim();
}

async function uatJson(...args) {
  return JSON.parse(await uatCmd(...args));
}

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

async function expectErr(fn, code) {
  try {
    await fn();
    return false;
  } catch (e) {
    const msg = String(e && e.message ? e.message : e);
    return msg.includes(code);
  }
}

function gtLines(amount) {
  return [{
    display_name: 'Mercadoria',
    saft_name: 'Mercadoria',
    unit_price_gross: amount,
    vat_rate_percent: '23.00',
    quantity: '1',
  }];
}

function movementStart() {
  const d = new Date(Date.now() + 60 * 60 * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}:00`;
}

async function issueGT(amount, requestId, extra = {}) {
  const purpose = extra.movement_purpose || 'transfer';
  const body = {
    request_id: requestId,
    document_type: 'GT',
    customer_nif: purpose === 'transfer' ? STORE_NIF : '509442013',
    customer_name: purpose === 'transfer' ? 'Loja B' : 'Cliente GT',
    customer_address: purpose === 'transfer' ? 'Rua Loja B 1' : 'Rua Entrega 10',
    customer_city: purpose === 'transfer' ? 'Porto' : 'Lisboa',
    customer_postal_code: purpose === 'transfer' ? '4000-001' : '1000-001',
    payment_method: 'CASH',
    lines: gtLines(amount),
    ship_from_address: 'Armazem Rua Demo 1, Lisboa',
    ship_to_address: purpose === 'transfer' ? 'Loja B Rua 1, Porto' : 'Rua Entrega 10, Lisboa',
    movement_start_at: movementStart(),
    movement_purpose: purpose,
    recipient_is_vat_payer: purpose === 'transfer' ? true : true,
    ...extra,
  };
  return uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(body));
}

async function setupFiscal(withGT) {
  const sess = ensureOwnerSession(base);
  uatEnv = envWithCookie(base, sess.cookie);
  const pem = readFileSync(pemPath, 'utf8');
  await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
    tax_registration_number: STORE_NIF,
    legal_name: 'Farvoo Demo Lda',
    address_detail: 'Rua Demo 1',
    city: 'Lisboa',
    postal_code: '1000-001',
    country: 'PT',
    timezone: 'Europe/Lisbon',
    software_certificate_number: '0',
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
    username: `${STORE_NIF}/37`, password: 'demo-secret',
  }));
  const types = [['FT', 'FT01'], ['FS', 'FS01'], ['FR', 'FR01'], ['NC', 'NC01']];
  if (withGT) types.push(['GT', 'GT01']);
  for (const [docType, suffix] of types) {
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `${docType}${year}GT${suffix}`, document_type: docType, fiscal_year: year,
    }));
  }
  setFiscalProfileViaDb(dbPath, 'restaurant', 3);
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({
    product_private_key_pem: pem,
  }));
}

async function main() {
  try { await run('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true, force: true });

  const childEnv = fiscalAgentTestEnv({
    PATH: `/opt/homebrew/bin:${process.env.PATH}`,
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: dataDir,
    FISCAL_BIND: bind,
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_AT_ENV: 'mock',
    FISCAL_ALLOW_LOCAL_PROVISION: '1',
  });
  const child = spawn('go', ['run', './cmd/fiscal-local'], { cwd: agent, env: childEnv, stdio: ['ignore', 'pipe', 'pipe'] });
  let boot = '';
  child.stdout.on('data', (d) => (boot += d));
  child.stderr.on('data', (d) => (boot += d));

  let healthy = false;
  for (let i = 0; i < 120; i++) {
    if (child.exitCode != null && child.exitCode !== 0) break;
    try { await uatCmd('stack-health'); healthy = true; break; }
    catch { await new Promise((r) => setTimeout(r, 250)); }
  }
  record('stack-health', healthy, healthy ? base : boot.slice(-500));
  if (!healthy) { child.kill('SIGTERM'); process.exit(1); }

  try {
    // GT-07: no GT series; ready_to_issue still OK
    await setupFiscal(false);
    const st0 = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('ready_to_issue without GT', !!st0.ready_to_issue && !st0.gt_series_ok, `gt_ok=${st0.gt_series_ok}`);
    record('GT-07 series_missing', await expectErr(
      () => issueGT('500.00', 'gt-no-series'),
      'series_missing',
    ));

    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `GT${year}GT01`, document_type: 'GT', fiscal_year: year,
    }));
    const st1 = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('ready_to_gt', !!st1.ready_to_gt && !!st1.gt_series_ok);

    // GT-01 transfer
    const gt1 = await issueGT('500.00', 'gt-01', { movement_purpose: 'transfer' });
    record('GT-01 transfer issue', gt1.document_type === 'GT' && String(gt1.invoice_no).includes('GT') && gt1.document_status === 'N', gt1.invoice_no);
    const gt1d = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${gt1.document_id}`);
    record('GT-01 purpose+AT', gt1d.movement_purpose === 'transfer' && !!(gt1d.at_doc_code_id && String(gt1d.at_doc_code_id).startsWith('MOCK')), gt1d.at_doc_code_id);

    const job = await uatJson('req', 'GET', `/local/v1/print-jobs/${gt1.print_job_id}`);
    const payloadStr = typeof job.payload_json === 'string' ? job.payload_json : JSON.stringify(job.payload_json || job.payload || {});
    record('GT print type', job.document_type === 'GT' || payloadStr.includes('"document_type":"GT"'), job.document_type || 'payload');
    record('GT print transport', payloadStr.includes('ship_from') || !!(gt1d.ship_from_address && gt1d.at_doc_code_id));

    // GT-02 customer
    const gt2 = await issueGT('80.00', 'gt-02', { movement_purpose: 'customer', customer_nif: '509442013', recipient_is_vat_payer: true });
    const gt2d = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${gt2.document_id}`);
    record('GT-02 customer', gt2.document_type === 'GT' && gt2d.movement_purpose === 'customer', gt2.invoice_no);

    // GT-03 transfer wrong NIF
    record('GT-03 transfer rejects guest NIF', await expectErr(
      () => issueGT('10.00', 'gt-03', { movement_purpose: 'transfer', customer_nif: '509442013' }),
      'validation_failed',
    ));

    // GT-04 customer with store NIF
    record('GT-04 customer rejects store NIF', await expectErr(
      () => issueGT('10.00', 'gt-04', { movement_purpose: 'customer', customer_nif: STORE_NIF }),
      'validation_failed',
    ));

    // GT-05 VAT payer needs NIF
    record('GT-05 VAT payer needs NIF', await expectErr(
      () => issueGT('10.00', 'gt-05', { movement_purpose: 'customer', customer_nif: '', recipient_is_vat_payer: true }),
      'validation_failed',
    ));

    // GT-06 annul N→A
    const gtA = await issueGT('30.00', 'gt-annul', { movement_purpose: 'transfer' });
    await uatJson('req', 'POST', `/local/v1/fiscal-documents/${gtA.document_id}/annul`, '--body', JSON.stringify({
      request_id: 'annul-gt-1', reason: 'Cancelado',
    }));
    const gtAD = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${gtA.document_id}`);
    record('GT-06 annul N→A', gtAD.document_status === 'A');
    // Same document + new request_id still hits business_key idempotency (returns prior A), not annul_not_allowed.
    const again = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${gtA.document_id}/annul`, '--body', JSON.stringify({
      request_id: 'annul-gt-2', reason: 'Again',
    }));
    record('GT-06 re-annul idempotent', again.document_status === 'A' && again.idempotent_hit === true);

    // No FT link field on GT
    record('GT has no FT links', !(gt1d.linked_sale_invoice_nos && gt1d.linked_sale_invoice_nos.length), 'ok');

    const listGt = await uatJson('req', 'GET', '/local/v1/fiscal-documents?document_type=GT&page_size=20');
    record('list GT tab', (listGt.invoices || []).every((x) => x.document_type === 'GT') && (listGt.total || 0) >= 1, `total=${listGt.total}`);

    const rev = await uatJson('req', 'GET', `/local/v1/fiscal-documents/revenue-summary?from=${year}-01-01&to=${year}-12-31`);
    record('revenue excludes GT shape', typeof rev.gross_net_sum === 'string');

    const month = new Date().getMonth() + 1;
    const exp = await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({ year, month }));
    const dl = await uatCmd('req', 'GET', `/local/v1/saft/exports/${exp.export_id}/download`);
    record('GT MovementOfGoods', dl.includes('MovementOfGoods') && dl.includes('<MovementType>GT</MovementType>') && !dl.includes('<InvoiceType>GT</InvoiceType>'), `export=${exp.export_id}`);

  } catch (e) {
    record('fatal', false, String(e && e.message ? e.message : e).slice(0, 400));
  } finally {
    child.kill('SIGTERM');
  }

  const failed = results.filter((r) => r.status !== 'pass');
  console.log('\n--- summary ---');
  for (const r of results) console.log(`${r.status.toUpperCase()} ${r.name}${r.note ? ' — ' + r.note : ''}`);
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
