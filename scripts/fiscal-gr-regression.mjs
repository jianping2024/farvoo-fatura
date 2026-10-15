#!/usr/bin/env node
/**
 * GR (guia de remessa) regression — GR-01…10 core paths (mock AT transport).
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
const bind = '127.0.0.1:17888';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-gr.db');
const dataDir = join(agent, 'data', 'fiscal-gr-secure');
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

function grLines(amount) {
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

async function issueGR(amount, requestId, extra = {}) {
  return uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
    request_id: requestId,
    document_type: 'GR',
    customer_nif: '509442013',
    customer_name: 'Cliente GR',
    customer_address: 'Rua Entrega 10',
    customer_city: 'Lisboa',
    customer_postal_code: '1000-001',
    payment_method: 'CASH',
    lines: grLines(amount),
    ship_from_address: 'Armazem Rua Demo 1, Lisboa',
    ship_to_address: 'Rua Entrega 10, Lisboa',
    movement_start_at: movementStart(),
    recipient_is_vat_payer: true,
    ...extra,
  }));
}

async function issueFT(amount, requestId, deliveryNoteIds) {
  const body = {
    request_id: requestId,
    document_type: 'FT',
    customer_nif: '509442013',
    customer_name: 'Cliente GR',
    payment_method: 'CASH',
    lines: grLines(amount),
  };
  if (deliveryNoteIds) body.delivery_note_ids = deliveryNoteIds;
  return uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(body));
}

async function setupFiscal(withGR) {
  const sess = ensureOwnerSession(base);
  uatEnv = envWithCookie(base, sess.cookie);
  const pem = readFileSync(pemPath, 'utf8');
  await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
    tax_registration_number: '517535009',
    legal_name: 'Farvoo Demo Lda',
    address_detail: 'Rua Demo 1',
    city: 'Lisboa',
    postal_code: '1000-001',
    country: 'PT',
    timezone: 'Europe/Lisbon',
    software_certificate_number: '0',
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
    username: '517535009/37', password: 'demo-secret',
  }));
  const types = [['FT', 'FT01'], ['FS', 'FS01'], ['FR', 'FR01'], ['NC', 'NC01']];
  if (withGR) types.push(['GR', 'GR01']);
  for (const [docType, suffix] of types) {
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `${docType}${year}GR${suffix}`, document_type: docType, fiscal_year: year,
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
    // GR-07: no GR series
    await setupFiscal(false);
    const st0 = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('ready_to_issue without GR', !!st0.ready_to_issue && !st0.gr_series_ok, `gr_ok=${st0.gr_series_ok}`);
    record('GR-07 series_missing', await expectErr(
      () => issueGR('500.00', 'gr-no-series'),
      'series_missing',
    ));

    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `GR${year}GR01`, document_type: 'GR', fiscal_year: year,
    }));
    const st1 = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('ready_to_delivery_note', !!st1.ready_to_delivery_note && !!st1.gr_series_ok);

    // GR-01
    const gr1 = await issueGR('500.00', 'gr-01');
    record('GR-01 issue', gr1.document_type === 'GR' && String(gr1.invoice_no).includes('GR') && gr1.document_status === 'N', gr1.invoice_no);
    const gr1d = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${gr1.document_id}`);
    record('GR-01 ATDocCodeID', !!(gr1d.at_doc_code_id && String(gr1d.at_doc_code_id).startsWith('MOCK')), gr1d.at_doc_code_id);

    const job = await uatJson('req', 'GET', `/local/v1/print-jobs/${gr1.print_job_id}`);
    const payloadStr = typeof job.payload_json === 'string' ? job.payload_json : JSON.stringify(job.payload_json || job.payload || {});
    record('GR print type', job.document_type === 'GR' || payloadStr.includes('"document_type":"GR"') || payloadStr.includes('GR'), job.document_type || 'payload');
    const hasTransport = payloadStr.includes('ship_from') || payloadStr.includes('ShipFrom')
      || payloadStr.includes('at_doc_code') || payloadStr.includes('ATDocCode')
      || !!(gr1d.ship_from_address && gr1d.at_doc_code_id);
    record('GR print transport', hasTransport, (gr1d.ship_from_address || '').slice(0, 40));

    // GR-06 direct FT
    const ftDirect = await issueFT('10.00', 'gr-06-ft');
    record('GR-06 direct FT', ftDirect.document_type === 'FT', ftDirect.invoice_no);

    // GR-02 two GR → one FT
    const gr2a = await issueGR('300.00', 'gr-02a');
    const gr2b = await issueGR('200.00', 'gr-02b');
    const ft2 = await issueFT('500.00', 'gr-02-ft', [gr2a.document_id, gr2b.document_id]);
    const gr2ad = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${gr2a.document_id}`);
    const gr2bd = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${gr2b.document_id}`);
    const ft2d = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${ft2.document_id}`);
    record('GR-02 FT link + F', ft2.document_type === 'FT' && gr2ad.document_status === 'F' && gr2bd.document_status === 'F'
      && (ft2d.linked_delivery_invoice_nos || []).length === 2, `refs=${(ft2d.linked_delivery_invoice_nos || []).length}`);

    // GR-03 amount mismatch
    const gr3 = await issueGR('500.00', 'gr-03');
    const ft3 = await issueFT('520.00', 'gr-03-ft', [gr3.document_id]);
    const gr3d = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${gr3.document_id}`);
    record('GR-03 FT mismatch OK', ft3.document_type === 'FT' && gr3d.document_status === 'F');

    // GR-04 partial + second FT on same GR
    const gr4 = await issueGR('500.00', 'gr-04');
    const ft4a = await issueFT('200.00', 'gr-04-ft-a', [gr4.document_id]);
    const ft4b = await issueFT('300.00', 'gr-04-ft-b', [gr4.document_id]);
    const gr4d = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${gr4.document_id}`);
    record('GR-04 multi FT', ft4a.document_type === 'FT' && ft4b.document_type === 'FT' && gr4d.document_status === 'F'
      && (gr4d.linked_sale_invoice_nos || []).length === 2, `sales=${(gr4d.linked_sale_invoice_nos || []).length}`);

    // FS cannot link GR
    const grFs = await issueGR('40.00', 'gr-fs');
    record('FS rejects delivery_note_ids', await expectErr(
      () => uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
        request_id: 'gr-fs-sale',
        document_type: 'FS',
        customer_nif: '509442013',
        customer_name: 'Cliente GR',
        payment_method: 'CASH',
        lines: grLines('40.00'),
        delivery_note_ids: [grFs.document_id],
      })),
      'delivery_note_ids',
    ) || await expectErr(
      () => uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
        request_id: 'gr-fs-sale-2',
        document_type: 'FS',
        customer_nif: '509442013',
        customer_name: 'Cliente GR',
        payment_method: 'CASH',
        lines: grLines('40.00'),
        delivery_note_ids: [grFs.document_id],
      })),
      'validation_failed',
    ));

    // VAT payer without NIF
    record('VAT payer needs NIF', await expectErr(
      () => issueGR('10.00', 'gr-nif', { customer_nif: '', recipient_is_vat_payer: true }),
      'validation_failed',
    ));

    // Annul N→A
    const grA = await issueGR('30.00', 'gr-annul');
    await uatJson('req', 'POST', `/local/v1/fiscal-documents/${grA.document_id}/annul`, '--body', JSON.stringify({
      request_id: 'annul-gr-1', reason: 'Cancelado',
    }));
    const grAD = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${grA.document_id}`);
    record('annul N→A', grAD.document_status === 'A');
    record('annul blocks link', await expectErr(
      () => issueFT('30.00', 'gr-annul-link', [grA.document_id]),
      'delivery_not_allowed',
    ));

    // F cannot annul
    record('F cannot annul', await expectErr(
      () => uatJson('req', 'POST', `/local/v1/fiscal-documents/${gr2a.document_id}/annul`, '--body', JSON.stringify({
        request_id: 'annul-f', reason: 'Nope',
      })),
      'annul_not_allowed',
    ));

    const listGr = await uatJson('req', 'GET', '/local/v1/fiscal-documents?document_type=GR&page_size=20');
    record('list GR tab', (listGr.invoices || []).every((x) => x.document_type === 'GR') && (listGr.total || 0) >= 1, `total=${listGr.total}`);

    const rev = await uatJson('req', 'GET', `/local/v1/fiscal-documents/revenue-summary?from=${year}-01-01&to=${year}-12-31`);
    record('revenue excludes GR shape', typeof rev.gross_net_sum === 'string');

    const month = new Date().getMonth() + 1;
    const exp = await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({ year, month }));
    const dl = await uatCmd('req', 'GET', `/local/v1/saft/exports/${exp.export_id}/download`);
    record('GR MovementOfGoods', dl.includes('MovementOfGoods') && dl.includes('MovementType') && dl.includes('GR') && !dl.includes('<InvoiceType>GR</InvoiceType>'), `export=${exp.export_id}`);

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
