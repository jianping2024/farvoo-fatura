#!/usr/bin/env node
/**
 * M5 SAF-T regression: FT + NC in period → export → file + sqlite asserts.
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17883';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-m5.db');
const dataDir = join(agent, 'data', 'fiscal-m5-secure');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const year = new Date().getFullYear();
const month = new Date().getMonth() + 1;

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
  return (await run(process.execPath, [uat, ...args], { env: { ...process.env, FISCAL_UAT_BASE: base } })).trim();
}

async function uatJson(...args) {
  return JSON.parse(await uatCmd(...args));
}

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

async function setupFiscal() {
  const pem = readFileSync(pemPath, 'utf8');
  await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
    tax_registration_number: '517535009', legal_name: 'Farvoo Demo Lda',
    address_detail: 'Rua Demo 1', city: 'Lisboa', postal_code: '1000-001',
    country: 'PT', timezone: 'Europe/Lisbon', software_certificate_number: '0',
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
    username: '517535009/37', password: 'demo-secret',
  }));
  await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
    series_code: `FT${year}M5FT01`, document_type: 'FT', fiscal_year: year,
  }));
  await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
    series_code: `NC${year}M5NC01`, document_type: 'NC', fiscal_year: year,
  }));
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({ product_private_key_pem: pem }));
  await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'op-demo-cashier', role: 'cashier', display_name: 'Demo', can_issue_nc: true,
  }));
}

async function issueFT() {
  const requestId = `m5-ft-${Date.now()}`;
  return uatJson('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify({
    request_id: requestId, operator_id: 'op-demo-cashier', document_type: 'FT',
    snapshot: {
      source_system: 'LOCAL', source_sale_id: `sale-${requestId}`, scope_type: 'session', scope_id: `scope-${requestId}`,
      fiscal_purpose: 'sale',
      lines: [{ product_code: 'DEMO1', display_name: 'Prato Demo', saft_name: 'Prato Demo',
        quantity: '1', unit_price_gross: '12.50', vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN' }],
      customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
      payments: [{ method: 'CASH', amount: '12.50' }],
    },
  }));
}

async function main() {
  try { await run('pkill', ['-f', 'fiscal-local']); } catch { /* */ }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true, force: true });

  const childEnv = { ...process.env, PATH: `/opt/homebrew/bin:${process.env.PATH}` };
  for (const k of Object.keys(childEnv)) if (k.startsWith('FISCAL_')) delete childEnv[k];
  Object.assign(childEnv, {
    FISCAL_DB: dbPath, FISCAL_DATA_DIR: dataDir, FISCAL_BIND: bind,
    FISCAL_STORE_ID: 'store-demo-001', FISCAL_AT_ENV: 'mock', FISCAL_ALLOW_LOCAL_PROVISION: '1',
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
    await setupFiscal();
    record('setup-fiscal', true);
  } catch (e) {
    record('setup-fiscal', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  let ft;
  try {
    ft = await issueFT();
    record('issue-ft', ft.document_status === 'SIGNED', ft.invoice_no);
  } catch (e) {
    record('issue-ft', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  let nc;
  try {
    nc = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/credit-notes`, '--body', JSON.stringify({
      request_id: `m5-nc-${Date.now()}`, operator_id: 'op-demo-cashier', reason: 'Devolucao M5', credit_full: true,
    }));
    record('issue-nc', nc.document_type === 'NC', nc.invoice_no);
  } catch (e) {
    record('issue-nc', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  let exp;
  try {
    exp = await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({
      year, month, operator_id: 'op-demo-cashier',
    }));
    record('export-saft', exp.validation_status === 'VALID' && exp.invoice_count >= 2, `${exp.invoice_count} docs`);
  } catch (e) {
    record('export-saft', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT 1 FROM saft_exports WHERE id='${exp.export_id}' AND validation_status='VALID' AND invoice_count>=2;`,
      '--expect-count', '1');
    record('sqlite-saft-export', true);
  } catch (e) {
    record('sqlite-saft-export', false, String(e));
  }

  try {
    const list = await uatJson('req', 'GET', `/local/v1/saft/exports?year=${year}&month=${month}`);
    record('list-saft-exports', Array.isArray(list.exports) && list.exports.length >= 1);
  } catch (e) {
    record('list-saft-exports', false, String(e));
  }

  try {
    const dl = await fetch(`${base}/local/v1/saft/exports/${exp.export_id}/download`);
    const xml = await dl.text();
    record('download-saft-xml', xml.includes('<AuditFile') && xml.includes(ft.invoice_no) && xml.includes(nc.invoice_no));
    record('saft-xml-nc-ref', xml.includes('Devolucao M5'));
  } catch (e) {
    record('download-saft-xml', false, String(e));
    record('saft-xml-nc-ref', false, String(e));
  }

  try {
    await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({ year: 2000, month: 1 }));
    record('export-no-invoices', false, 'expected no_invoices');
  } catch (e) {
    record('export-no-invoices', String(e).includes('no_invoices'), String(e).slice(-100));
  }

  try {
    const again = await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({
      year, month, operator_id: 'op-demo-cashier',
    }));
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT id FROM saft_exports WHERE period_year=${year} AND period_month=${month};`,
      '--expect-count', '2');
    record('repeat-export-new-row', !!again.export_id);
  } catch (e) {
    record('repeat-export-new-row', false, String(e));
  }

  try {
    await uatCmd('assert-db', '--db', dbPath, '--sql',
      `SELECT 1 FROM audit_log WHERE action='EXPORT_SAFT' LIMIT 1;`, '--expect-count', '1');
    record('audit-export-saft', true);
  } catch (e) {
    record('audit-export-saft', false, String(e));
  }

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n=== M5 SUMMARY ===');
  for (const r of results) console.log(`${r.status}\t${r.name}\t${r.note}`);
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(1); });
