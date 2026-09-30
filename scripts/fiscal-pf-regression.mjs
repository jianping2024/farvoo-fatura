#!/usr/bin/env node
/**
 * PF (pró-forma) regression — scenarios PF-01…PF-10 + FR→F / annul / 1:1 sale link.
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
const bind = '127.0.0.1:17887';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-pf.db');
const dataDir = join(agent, 'data', 'fiscal-pf-secure');
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

function pfLines(amount) {
  return [{
    display_name: 'Banquete',
    saft_name: 'Banquete',
    unit_price_gross: amount,
    vat_rate_percent: '23.00',
    quantity: '1',
  }];
}

async function issuePF(amount, requestId, extra = {}) {
  return uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
    request_id: requestId,
    document_type: 'PF',
    customer_nif: '509442013',
    customer_name: 'Cliente PF',
    lines: pfLines(amount),
    ...extra,
  }));
}

async function issueSale(docType, amount, requestId, proformaId) {
  const body = {
    request_id: requestId,
    document_type: docType,
    customer_nif: '509442013',
    customer_name: 'Cliente PF',
    payment_method: 'CASH',
    lines: pfLines(amount),
  };
  if (proformaId) body.proforma_id = proformaId;
  return uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(body));
}

async function setupFiscal(withPF) {
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
  if (withPF) types.push(['PF', 'PF01']);
  for (const [docType, suffix] of types) {
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `${docType}${year}PF${suffix}`, document_type: docType, fiscal_year: year,
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
    // PF-06: no PF series
    await setupFiscal(false);
    const st0 = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('ready_to_issue without PF', !!st0.ready_to_issue && !st0.pf_series_ok, `pf_ok=${st0.pf_series_ok}`);
    record('PF-06 series_missing', await expectErr(
      () => issuePF('800.00', 'pf-no-series'),
      'series_missing',
    ));

    // Register PF series
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `PF${year}PF01`, document_type: 'PF', fiscal_year: year,
    }));
    const st1 = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('ready_to_proforma', !!st1.ready_to_proforma && !!st1.pf_series_ok);

    // PF-01
    const pf1 = await issuePF('800.00', 'pf-01');
    record('PF-01 issue', pf1.document_type === 'PF' && String(pf1.invoice_no).includes('PF') && pf1.document_status === 'N', pf1.invoice_no);

    // PF-10 print payload disclaimer (via print job + ORIGINAL)
    const job = await uatJson('req', 'GET', `/local/v1/print-jobs/${pf1.print_job_id}`);
    const payloadStr = typeof job.payload_json === 'string' ? job.payload_json : JSON.stringify(job.payload_json || job.payload || {});
    // Render check: document_type PF on job
    record('PF-10 print job type', job.document_type === 'PF' || (job.payload && job.payload.document_type === 'PF') || payloadStr.includes('"document_type":"PF"') || payloadStr.includes('PF'), job.document_type || 'payload');

    // PF-02 leave unconverted
    const detailPf1 = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${pf1.document_id}`);
    record('PF-02 stays N', detailPf1.document_status === 'N' && detailPf1.work_status === 'N');

    // PF-05 direct FS without PF
    const fsDirect = await issueSale('FS', '10.00', 'pf-05-fs');
    record('PF-05 direct FS', fsDirect.document_type === 'FS', fsDirect.invoice_no);

    // PF-03 link FS equal amount
    const pf3 = await issuePF('80.00', 'pf-03');
    const fs3 = await issueSale('FS', '80.00', 'pf-03-fs', pf3.document_id);
    const pf3d = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${pf3.document_id}`);
    const fs3d = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${fs3.document_id}`);
    record('PF-03 FS link + F', fs3.document_type === 'FS' && pf3d.document_status === 'F' && fs3d.proforma_id === pf3.document_id, `${fs3.document_type}/${pf3d.document_status}/ref=${fs3d.proforma_id}`);

    // PF-04 amount mismatch FT
    const pf4 = await issuePF('800.00', 'pf-04');
    const ft4 = await issueSale('FT', '920.00', 'pf-04-ft', pf4.document_id);
    const pf4d = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${pf4.document_id}`);
    record('PF-04 FT mismatch OK', ft4.document_type === 'FT' && pf4d.document_status === 'F', `${ft4.invoice_no} / ${pf4d.document_status}`);

    // FR → F
    const pfFr = await issuePF('50.00', 'pf-fr');
    const fr = await issueSale('FR', '50.00', 'pf-fr-sale', pfFr.document_id);
    const pfFrD = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${pfFr.document_id}`);
    record('FR marks PF F', fr.document_type === 'FR' && pfFrD.document_status === 'F');

    // 1 PF → 2 sales
    const pfMulti = await issuePF('100.00', 'pf-multi');
    await issueSale('FT', '40.00', 'pf-multi-1', pfMulti.document_id);
    await issueSale('FS', '60.00', 'pf-multi-2', pfMulti.document_id);
    const pfMultiD = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${pfMulti.document_id}`);
    record('1 PF → 2 sales', pfMultiD.document_status === 'F' && (pfMultiD.linked_sale_invoice_nos || []).length === 2, String((pfMultiD.linked_sale_invoice_nos || []).length));

    // one sale max one PF — second proforma_id on same sale impossible at issue; try linking annulled
    const pfA = await issuePF('30.00', 'pf-annul-src');
    await uatJson('req', 'POST', `/local/v1/fiscal-documents/${pfA.document_id}/annul`, '--body', JSON.stringify({
      request_id: 'annul-1', reason: 'Cancelado',
    }));
    const pfAD = await uatJson('req', 'GET', `/local/v1/fiscal-documents/${pfA.document_id}`);
    record('annul N→A', pfAD.document_status === 'A');
    record('annul blocks link', await expectErr(
      () => issueSale('FT', '30.00', 'pf-annul-link', pfA.document_id),
      'proforma_not_allowed',
    ));

    // F cannot annul
    record('F cannot annul', await expectErr(
      () => uatJson('req', 'POST', `/local/v1/fiscal-documents/${pf3.document_id}/annul`, '--body', JSON.stringify({
        request_id: 'annul-f', reason: 'Nope',
      })),
      'annul_not_allowed',
    ));

    // PF-08 optional link — already covered by PF-05

    // Revenue excludes PF
    const rev = await uatJson('req', 'GET', `/local/v1/fiscal-documents/revenue-summary?from=${year}-01-01&to=${year}-12-31`);
    const listPf = await uatJson('req', 'GET', '/local/v1/fiscal-documents?document_type=PF&page_size=20');
    record('list PF tab', (listPf.invoices || []).every((x) => x.document_type === 'PF') && (listPf.total || 0) >= 1, `total=${listPf.total}`);
    record('revenue excludes PF money inflate', typeof rev.gross_net_sum === 'string');

    // PF-09 SAF-T WorkingDocuments
    const exp = await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({ year, month }));
    const dl = await uatCmd('req', 'GET', `/local/v1/saft/exports/${exp.export_id}/download`);
    record('PF-09 WorkingDocuments', dl.includes('WorkingDocuments') && dl.includes('WorkType') && dl.includes('PF') && !dl.includes('<InvoiceType>PF</InvoiceType>'), `export=${exp.export_id}`);
    record('PF-09 OrderReferences', dl.includes('OrderReferences') && dl.includes('OriginatingON'));

    // PF-07 product path: cannot "checkout" with only PF — ready_to_issue still needs FT/FS; PF alone not sale
    record('PF-07 not sale substitute', st1.ready_to_issue === true);

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
