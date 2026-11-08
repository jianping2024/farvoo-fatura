#!/usr/bin/env node
/**
 * Clean full-document-type SAF-T UAT on a fresh mock DB (no pkill, no AT burn).
 * Issues FT/FS/FR/NC/ND/PF/RG/GR/GT → export SAF-T → assert VALID + XSD.
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  ensureOwnerSession, setFiscalProfileViaDb, envWithCookie, fiscalAgentTestEnv,
} from './fiscal-session-helper.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17884';
const base = `http://${bind}`;
const stamp = Date.now();
const dbPath = join(agent, 'data', `fiscal-saft-clean-${stamp}.db`);
const dataDir = join(agent, 'data', `fiscal-saft-clean-${stamp}-secure`);
const outXml = join(agent, 'data', `saft-clean-all-types-${stamp}.xml`);
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const year = new Date().getFullYear();
const month = new Date().getMonth() + 1;

let uatEnv = { ...process.env, FISCAL_UAT_BASE: base };

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
  return (await run(process.execPath, [uat, ...args], { env: uatEnv })).trim();
}

async function uatJson(...args) {
  return JSON.parse(await uatCmd(...args));
}

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

function pad(n) { return String(n).padStart(2, '0'); }
function movementStart() {
  const d = new Date(Date.now() + 60 * 60 * 1000);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}:00`;
}

function line(name, gross = '12.50', rate = '23.00') {
  return {
    display_name: name, saft_name: name, unit_price_gross: gross,
    vat_rate_percent: rate, quantity: '1', product_type: 'P', unit_of_measure: 'UN',
  };
}

async function setup() {
  const sess = ensureOwnerSession(base);
  uatEnv = envWithCookie(base, sess.cookie);
  const pem = readFileSync(pemPath, 'utf8');
  await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
    tax_registration_number: '517535009', legal_name: 'Farvoo Demo Lda',
    address_detail: 'Rua Demo 1', city: 'Lisboa', postal_code: '1000-001',
    country: 'PT', timezone: 'Europe/Lisbon', software_certificate_number: '0',
  }));
  await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
    username: '517535009/37', password: 'demo-secret',
  }));
  for (const [docType, suffix] of [
    ['FT', 'FT01'], ['FS', 'FS01'], ['FR', 'FR01'], ['NC', 'NC01'], ['ND', 'ND01'],
    ['PF', 'PF01'], ['RG', 'RG01'], ['GR', 'GR01'], ['GT', 'GT01'],
  ]) {
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `${docType}${year}CLN${suffix}`, document_type: docType, fiscal_year: year,
    }));
  }
  setFiscalProfileViaDb(dbPath, 'restaurant', 3);
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({
    product_private_key_pem: pem,
  }));
}

async function issueManual(body) {
  return uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify(body));
}

async function main() {
  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true, force: true });

  const childEnv = fiscalAgentTestEnv({
    PATH: `/opt/homebrew/bin:/usr/bin:/bin:${process.env.PATH || ''}`,
    HOME: process.env.HOME,
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: dataDir,
    FISCAL_BIND: bind,
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_AT_ENV: 'mock',
    FISCAL_ALLOW_LOCAL_PROVISION: '1',
  });
  // Prefer a prebuilt binary (sandbox Go module cache is often broken).
  const bin = process.env.FISCAL_LOCAL_BIN || '/tmp/fiscal-local-saft-clean';
  const child = existsSync(bin)
    ? spawn(bin, [], { cwd: agent, env: childEnv, stdio: ['ignore', 'pipe', 'pipe'] })
    : spawn('/opt/homebrew/bin/go', ['run', './cmd/fiscal-local'], {
      cwd: agent,
      env: {
        ...childEnv,
        GOPATH: `${process.env.HOME}/go`,
        GOMODCACHE: `${process.env.HOME}/go/pkg/mod`,
        GOCACHE: `${process.env.HOME}/Library/Caches/go-build`,
        GOTOOLCHAIN: 'local',
      },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
  let boot = '';
  child.stdout.on('data', (d) => (boot += d));
  child.stderr.on('data', (d) => (boot += d));

  let healthy = false;
  for (let i = 0; i < 150; i++) {
    if (child.exitCode != null && child.exitCode !== 0) break;
    try { await uatCmd('stack-health'); healthy = true; break; }
    catch { await new Promise((r) => setTimeout(r, 250)); }
  }
  record('stack-health', healthy, healthy ? base : boot.slice(-800));
  if (!healthy) {
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    await setup();
    record('setup', true);
  } catch (e) {
    record('setup', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  const issued = {};
  try {
    issued.ft = await issueManual({
      request_id: `cln-ft-${stamp}`, document_type: 'FT',
      customer_nif: '509442013', customer_name: 'Cliente Conta',
      payment_method: 'CARD',
      lines: [
        line('Cafe', '0.85', '13.00'),
        line('Coca-Cola', '2.20', '23.00'),
        line('Pao', '1.15', '6.00'),
      ],
    });
    record('issue-FT', issued.ft.document_type === 'FT', issued.ft.invoice_no);
  } catch (e) { record('issue-FT', false, String(e)); }

  try {
    issued.fs = await issueManual({
      request_id: `cln-fs-${stamp}`, document_type: 'FS',
      customer_nif: '999999990', customer_name: 'Consumidor Final',
      payment_method: 'CASH', lines: [line('Menu FS', '8.00')],
    });
    record('issue-FS', issued.fs.document_type === 'FS', issued.fs.invoice_no);
  } catch (e) { record('issue-FS', false, String(e)); }

  try {
    issued.fr = await issueManual({
      request_id: `cln-fr-${stamp}`, document_type: 'FR',
      customer_nif: '509442013', customer_name: 'Cliente Conta',
      payment_method: 'CASH', lines: [line('Menu FR', '15.00')],
    });
    record('issue-FR', issued.fr.document_type === 'FR', issued.fr.invoice_no);
  } catch (e) { record('issue-FR', false, String(e)); }

  try {
    issued.nc = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${issued.ft.document_id}/credit-notes`, '--body', JSON.stringify({
      request_id: `cln-nc-${stamp}`, reason: 'Devolucao teste', credit_full: true,
    }));
    record('issue-NC', issued.nc.document_type === 'NC', issued.nc.invoice_no);
  } catch (e) { record('issue-NC', false, String(e)); }

  try {
    issued.nd = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${issued.fs.document_id}/debit-notes`, '--body', JSON.stringify({
      request_id: `cln-nd-${stamp}`, reason: 'Ajuste teste', debit_full: true,
    }));
    record('issue-ND', issued.nd.document_type === 'ND', issued.nd.invoice_no);
  } catch (e) { record('issue-ND', false, String(e)); }

  try {
    issued.pf = await issueManual({
      request_id: `cln-pf-${stamp}`, document_type: 'PF',
      customer_nif: '509442013', customer_name: 'Cliente Conta',
      lines: [line('Banquete', '50.00')],
    });
    record('issue-PF', issued.pf.document_type === 'PF', issued.pf.invoice_no);
  } catch (e) { record('issue-PF', false, String(e)); }

  try {
    // hang-account FT then RG receipt
    issued.ftHang = await issueManual({
      request_id: `cln-fthang-${stamp}`, document_type: 'FT',
      customer_nif: '509442013', customer_name: 'Cliente Conta',
      payment_method: 'ACCOUNT',
      lines: [line('Conta', '40.00')],
    });
    issued.rg = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${issued.ftHang.document_id}/receipts`, '--body', JSON.stringify({
      request_id: `cln-rg-${stamp}`,
      receive_full: true,
      payment_method: 'CASH',
    }));
    record('issue-RG', issued.rg.document_type === 'RG', issued.rg.invoice_no);
  } catch (e) { record('issue-RG', false, String(e)); }

  try {
    issued.gr = await issueManual({
      request_id: `cln-gr-${stamp}`, document_type: 'GR',
      customer_nif: '509442013', customer_name: 'Cliente GR',
      customer_address: 'Rua Entrega 10', customer_city: 'Lisboa', customer_postal_code: '1000-001',
      payment_method: 'CASH', lines: [line('Mercadoria', '25.00')],
      ship_from: { address_detail: 'Armazem Rua Demo 1', postal_code: '1000-001', city: 'Lisboa' },
      ship_to: { address_detail: 'Rua Entrega 10', postal_code: '1000-002', city: 'Lisboa' },
      movement_start_at: movementStart(), recipient_is_vat_payer: true,
    });
    record('issue-GR', issued.gr.document_type === 'GR', issued.gr.invoice_no);
  } catch (e) { record('issue-GR', false, String(e)); }

  try {
    issued.gt = await issueManual({
      request_id: `cln-gt-${stamp}`, document_type: 'GT',
      movement_purpose: 'transfer',
      customer_nif: '517535009', customer_name: 'Farvoo Demo Lda',
      customer_address: 'Rua Destino 5', customer_city: 'Porto', customer_postal_code: '4000-001',
      payment_method: 'CASH', lines: [line('Transferencia', '30.00')],
      ship_from: { address_detail: 'Armazem Rua Demo 1', postal_code: '1000-001', city: 'Lisboa' },
      ship_to: { address_detail: 'Rua Destino 5', postal_code: '4000-001', city: 'Porto' },
      movement_start_at: movementStart(), recipient_is_vat_payer: true,
    });
    record('issue-GT', issued.gt.document_type === 'GT', issued.gt.invoice_no);
  } catch (e) { record('issue-GT', false, String(e)); }

  let exp;
  try {
    exp = await uatJson('req', 'POST', '/local/v1/saft/exports', '--body', JSON.stringify({
      year, month, operator_id: 'op-demo-cashier',
    }));
    record('export-saft-VALID', exp.validation_status === 'VALID',
      `status=${exp.validation_status} count=${exp.invoice_count} errors=${JSON.stringify(exp.validation_errors || [])}`);
    const xml = await uatCmd('req', 'GET', `/local/v1/saft/exports/${exp.export_id}/download`, '--raw');
    writeFileSync(outXml, xml);
    record('export-file', existsSync(outXml), outXml);

    const markers = {
      FT: '<InvoiceType>FT</InvoiceType>',
      FS: '<InvoiceType>FS</InvoiceType>',
      FR: '<InvoiceType>FR</InvoiceType>',
      NC: '<InvoiceType>NC</InvoiceType>',
      ND: '<InvoiceType>ND</InvoiceType>',
      PF: '<WorkType>PF</WorkType>',
      GR: '<MovementType>GR</MovementType>',
      GT: '<MovementType>GT</MovementType>',
      RG: '<PaymentType>RG</PaymentType>',
    };
    for (const [k, m] of Object.entries(markers)) {
      record(`xml-has-${k}`, xml.includes(m));
    }
    // amount invariants spot-check: no SettlementAmount; GrossTotal 2dp present
    record('no-SettlementAmount', !xml.includes('<SettlementAmount>'));
    record('has-NetTotal', xml.includes('<NetTotal>'));
  } catch (e) {
    record('export-saft-VALID', false, String(e));
  }

  try {
    if (existsSync(outXml)) {
      await run('bash', [join(root, 'scripts', 'fiscal-saft-xsd-check.sh')], {
        env: { ...process.env, SAFT_XSD_CHECK_FILES: outXml },
      });
      record('xsd-check', true, outXml);
    } else {
      record('xsd-check', false, 'no xml');
    }
  } catch (e) {
    record('xsd-check', false, String(e).slice(0, 500));
  }

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n--- summary ---');
  console.log(JSON.stringify({
    pass: results.length - failed.length,
    fail: failed.length,
    db: dbPath,
    xml: outXml,
    export: exp || null,
    results,
  }, null, 2));
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(1); });
