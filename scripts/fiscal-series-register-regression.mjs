#!/usr/bin/env node
/**
 * Series register: first OK, same-code idempotent, different-code rejected, status shows codes.
 *   node scripts/fiscal-series-register-regression.mjs
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const bind = '127.0.0.1:17889';
const base = `http://${bind}`;
const dbPath = join(agent, 'data', 'fiscal-series-reg.db');
const dataDir = join(agent, 'data', 'fiscal-series-reg-secure');
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
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
    await uatCmd('req', 'PUT', '/local/v1/setup/taxpayer', '--body', JSON.stringify({
      tax_registration_number: '517535009', legal_name: 'Farvoo Demo Lda',
      address_detail: 'Rua Demo 1', city: 'Lisboa', postal_code: '1000-001',
      country: 'PT', timezone: 'Europe/Lisbon', software_certificate_number: '0',
    }));
    await uatCmd('req', 'PUT', '/local/v1/setup/at-credentials', '--body', JSON.stringify({
      username: '517535009/37', password: 'demo-secret',
    }));
    record('setup-taxpayer-at', true);
  } catch (e) {
    record('setup-taxpayer-at', false, String(e));
    child.kill('SIGTERM');
    process.exit(1);
  }

  const fsCode = `FS${year}REG01`;
  try {
    const first = await uatJson('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: fsCode, document_type: 'FS', fiscal_year: year,
    }));
    record('register-fs-first', first.fs_series_ok === true && first.idempotent_hit !== true && first.series_code === fsCode,
      `code=${first.series_code} idem=${first.idempotent_hit}`);
  } catch (e) {
    record('register-fs-first', false, String(e).slice(0, 120));
  }

  try {
    const again = await uatJson('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: fsCode, document_type: 'FS', fiscal_year: year,
    }));
    record('register-fs-idempotent', again.idempotent_hit === true && again.fs_series_ok === true, `idem=${again.idempotent_hit}`);
  } catch (e) {
    record('register-fs-idempotent', false, String(e).slice(0, 120));
  }

  try {
    await uatJson('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `FS${year}REG02`, document_type: 'FS', fiscal_year: year,
    }));
    record('register-fs-second-rejected', false, 'expected series_already_active');
  } catch (e) {
    const msg = String(e);
    record('register-fs-second-rejected', msg.includes('series_already_active'), msg.slice(0, 100));
  }

  try {
    const ft = await uatJson('req', 'POST', '/local/v1/setup/series/register', '--body', JSON.stringify({
      series_code: `FT${year}REG01`, document_type: 'FT', fiscal_year: year,
    }));
    const st = await uatJson('req', 'GET', '/local/v1/setup/status');
    record('register-ft-and-status-codes',
      ft.series_ok === true && st.series_code === `FT${year}REG01` && st.fs_series_code === fsCode,
      `ft=${st.series_code} fs=${st.fs_series_code}`);
  } catch (e) {
    record('register-ft-and-status-codes', false, String(e).slice(0, 120));
  }

  try {
    const html = readFileSync(join(agent, 'internal', 'fiscal', 'bootstrap', 'admin', 'index.html'), 'utf8');
    const n = (html.match(/POST', '\/local\/v1\/setup\/series\/register'/g) || []).length;
    record('admin-single-register-callsite', n === 1, `call_sites=${n}`);
  } catch (e) {
    record('admin-single-register-callsite', false, String(e));
  }

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n--- summary ---');
  console.log(JSON.stringify({ pass: results.length - failed.length, fail: failed.length, results }, null, 2));
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => { console.error(e); process.exit(1); });
