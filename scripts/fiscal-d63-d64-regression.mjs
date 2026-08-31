#!/usr/bin/env node
/**
 * D6.3 / D6.4 regression: backup, integrity block/heal, prepare-swap.
 *   node scripts/fiscal-d63-d64-regression.mjs
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17888';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-d63.db');
const dataDir = join(agent, 'data', 'fiscal-d63-secure');
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
  for (const [docType, suffix] of [['FT', 'BFT'], ['FS', 'BFS'], ['NC', 'BNC'], ['ND', 'BND']]) {
    await uatCmd('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `${docType}${year}D63${suffix}`, document_type: docType, fiscal_year: year,
    }));
  }
  await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({ product_private_key_pem: pem }));
  await uatCmd('req', 'PUT', '/local/v1/setup/operator', '--body', JSON.stringify({
    id: 'op-demo-cashier', role: 'cashier', display_name: 'Demo', can_issue_nc: true,
  }));
  await uatCmd('req', 'POST', '/local/v1/products', '--body', JSON.stringify({
    product_code: 'DEMO1', display_name: 'Item', saft_name: 'Item',
    unit_price_gross: '10.00', vat_rate: '23.00',
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
  record('stack-health', healthy, healthy ? base : boot.slice(-400));
  if (!healthy) { child.kill('SIGTERM'); process.exit(1); }

  try {
    await setupFiscal();
    record('setup', true);
  } catch (e) {
    record('setup', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  try {
    await uatJson('req', 'POST', '/local/v1/fiscal-documents/manual', '--body', JSON.stringify({
      request_id: `d63-fs-${Date.now()}`, operator_id: 'op-demo-cashier', document_type: 'FS',
      customer_nif: '999999990', payment_method: 'CASH',
      lines: [{ product_code: 'DEMO1', quantity: '1' }],
    }));
    record('issue-fs', true);
  } catch (e) {
    record('issue-fs', false, String(e).slice(0, 120));
  }

  try {
    const bak = await uatJson('req', 'POST', '/local/v1/setup/backup', '--body', '{}');
    record('backup', !!bak.backup_path && bak.bytes > 0, bak.backup_path);
  } catch (e) {
    record('backup', false, String(e).slice(0, 120));
  }

  try {
    const ok = await uatJson('req', 'POST', '/local/v1/setup/integrity/verify', '--body', JSON.stringify({
      block_on_fail: true,
    }));
    record('integrity-ok', ok.ok === true, `checked=${ok.checked}`);
  } catch (e) {
    record('integrity-ok', false, String(e).slice(0, 120));
  }

  try {
    await run('sqlite3', [dbPath, `UPDATE series SET last_hash='BAD' WHERE document_type='FS' AND status='ACTIVE';`]);
    const bad = await uatJson('req', 'POST', '/local/v1/setup/integrity/verify', '--body', JSON.stringify({
      block_on_fail: true,
    }));
    const st = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('integrity-block', bad.ok === false && bad.blocked >= 1 && st.ready_to_issue === false,
      `blocked=${bad.blocked} ready=${st.ready_to_issue}`);
  } catch (e) {
    record('integrity-block', false, String(e).slice(0, 160));
  }

  try {
    await run('sqlite3', [dbPath, `
      UPDATE series SET last_hash=(
        SELECT i.hash FROM invoices i WHERE i.series_id=series.id ORDER BY i.sequence_number DESC LIMIT 1
      ) WHERE document_type='FS';
    `]);
    const healed = await uatJson('req', 'POST', '/local/v1/setup/integrity/verify', '--body', JSON.stringify({
      block_on_fail: false, heal_on_pass: true,
    }));
    const st = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('integrity-heal', healed.ok === true && healed.healed >= 1 && st.fs_series_ok === true,
      `healed=${healed.healed} fs_ok=${st.fs_series_ok}`);
  } catch (e) {
    record('integrity-heal', false, String(e).slice(0, 160));
  }

  try {
    const swap = await uatJson('req', 'POST', '/local/v1/setup/prepare-swap', '--body', JSON.stringify({
      operator_id: 'op-demo-cashier', backup: true,
    }));
    const st = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('prepare-swap', swap.ok === true && st.activated_ok === false && st.ready_to_issue === false,
      `backup=${!!swap.backup_path} activated=${st.activated_ok}`);
  } catch (e) {
    record('prepare-swap', false, String(e).slice(0, 160));
  }

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n--- summary ---');
  console.log(JSON.stringify({ pass: results.length - failed.length, fail: failed.length, results }, null, 2));
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(1); });
