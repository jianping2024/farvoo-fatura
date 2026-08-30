#!/usr/bin/env node
/**
 * Prep fiscal-local for M5 Admin §7 UAT: FT+NC in current month, server on 17880.
 *
 * Usage: node scripts/fiscal-m5-uat-prep.mjs
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17880';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-m5-uat.db');
const dataDir = join(agent, 'data', 'fiscal-m5-uat-secure');
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
  child.stdout.on('data', (d) => process.stdout.write(d));
  child.stderr.on('data', (d) => process.stderr.write(d));

  for (let i = 0; i < 120; i++) {
    try { await uatCmd('stack-health'); break; }
    catch { await new Promise((r) => setTimeout(r, 250)); }
  }

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
    series_code: `FT${year}M5UAT01`, document_type: 'FT', fiscal_year: year,
  }));
  await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
    series_code: `NC${year}M5UAT01`, document_type: 'NC', fiscal_year: year,
  }));
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({ product_private_key_pem: pem }));
  await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'op-demo-cashier', role: 'cashier', display_name: 'Demo', can_issue_nc: true,
  }));

  const ft = await uatJson('req', 'POST', '/local/v1/fiscal-documents', '--body', JSON.stringify({
    request_id: `m5-uat-ft-${Date.now()}`, operator_id: 'op-demo-cashier', document_type: 'FT',
    snapshot: {
      source_system: 'LOCAL', source_sale_id: 'sale-m5-uat', scope_type: 'session', scope_id: 'scope-m5-uat',
      fiscal_purpose: 'sale',
      lines: [{ product_code: 'DEMO1', display_name: 'Prato Demo', saft_name: 'Água 500ml',
        quantity: '1', unit_price_gross: '12.50', vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN' }],
      customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
      payments: [{ method: 'CASH', amount: '12.50' }],
    },
  }));
  const nc = await uatJson('req', 'POST', `/local/v1/fiscal-documents/${ft.document_id}/credit-notes`, '--body', JSON.stringify({
    request_id: `m5-uat-nc-${Date.now()}`, operator_id: 'op-demo-cashier', reason: 'Devolucao UAT M5', credit_full: true,
  }));

  console.log('\nM5 UAT ready:', base);
  console.log('Admin:', base + '/admin');
  console.log('Period:', year, month, '| FT:', ft.invoice_no, '| NC:', nc.invoice_no);
  console.log('Login: pick mode → any 4-digit PIN → 设置 → §7 SAF-T 月导');
  process.on('SIGINT', () => { child.kill('SIGTERM'); process.exit(0); });
}

main().catch((e) => { console.error(e); process.exit(1); });
