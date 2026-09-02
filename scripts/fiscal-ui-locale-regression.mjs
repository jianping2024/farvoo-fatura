#!/usr/bin/env node
/**
 * UI locale → invoice locale (scheme A) regression — no skip.
 */
import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ensureOwnerSession, envWithCookie } from './fiscal-session-helper.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = process.env.FISCAL_UAT_BASE || 'http://127.0.0.1:17881';
const dbPath = join(agent, 'data', 'fiscal-locale-regression.db');
const dataDir = join(agent, 'data', 'fiscal-locale-secure');
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

let uatEnv = {};
async function uatCmd(...args) {
  return (await run(process.execPath, [uat, ...args], {
    env: { ...process.env, FISCAL_UAT_BASE: base, ...uatEnv },
  })).trim();
}

function payloadLocale(invoiceId) {
  const sql = `SELECT json_extract(payload_json,'$.locale') FROM local_print_jobs WHERE invoice_id='${invoiceId}' LIMIT 1;`;
  const cli = spawnSync('sqlite3', [dbPath, sql], { encoding: 'utf8' });
  if (cli.status !== 0) throw new Error(cli.stderr || cli.stdout);
  return (cli.stdout || '').trim();
}

const results = [];
function record(name, ok, note) {
  results.push({ name, status: ok ? 'pass' : 'fail', note: note || '' });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${note ? ' — ' + note : ''}`);
}

async function main() {
  try { await run('pkill', ['-f', 'fiscal-locale-bin']); } catch { /* none */ }
  await new Promise((r) => setTimeout(r, 300));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dbPath)) rmSync(dbPath);
  if (existsSync(dataDir)) rmSync(dataDir, { recursive: true, force: true });

  const childEnv = { ...process.env, PATH: `/opt/homebrew/bin:${process.env.PATH}` };
  for (const k of Object.keys(childEnv)) {
    if (k.startsWith('FISCAL_')) delete childEnv[k];
  }
  const station = '2951b0b2-aaaa-bbbb-cccc-ddddeeeeffff';
  Object.assign(childEnv, {
    FISCAL_DB: dbPath,
    FISCAL_DATA_DIR: dataDir,
    FISCAL_BIND: '127.0.0.1:17881',
    FISCAL_STORE_ID: 'store-demo-001',
    FISCAL_SEED: '1',
    FISCAL_ALLOW_DEV_KEY: '1',
    FISCAL_AT_ENV: 'mock',
    FISCAL_STATION_PRINTERS_JSON: JSON.stringify({ [station]: 'tcp:127.0.0.1:9100' }),
  });

  const bin = join(agent, 'data', 'fiscal-locale-bin');
  await run('go', ['build', '-o', bin, './cmd/fiscal-local'], { cwd: agent, env: childEnv });
  const child = spawn(bin, [], { cwd: agent, env: childEnv, stdio: ['ignore', 'pipe', 'pipe'] });
  let boot = '';
  child.stdout.on('data', (d) => (boot += d));
  child.stderr.on('data', (d) => (boot += d));

  let healthy = false;
  for (let i = 0; i < 60; i++) {
    if (child.exitCode != null) break;
    try {
      await uatCmd('stack-health');
      healthy = true;
      break;
    } catch {
      await new Promise((r) => setTimeout(r, 200));
    }
  }
  if (!healthy) {
    child.kill();
    throw new Error('stack-health failed\n' + boot);
  }
  record('stack-health', true);

  await uatCmd('exec-db', '--db', dbPath, '--sql', 'DELETE FROM operators;');
  const { cookie } = ensureOwnerSession(base, 'Locale Owner', '123456');
  uatEnv = envWithCookie(base, cookie);
  record('owner-session', !!cookie);

  const get0 = JSON.parse(await uatCmd('req', 'GET', '/local/v1/setup/ui-locale'));
  record('GET default zh→pt', get0.ui_locale === 'zh' && get0.invoice_locale === 'pt', JSON.stringify(get0));

  const putEn = JSON.parse(await uatCmd('req', 'PUT', '/local/v1/setup/ui-locale', '--body', JSON.stringify({ ui_locale: 'en' })));
  record('PUT en→en', putEn.ui_locale === 'en' && putEn.invoice_locale === 'en', JSON.stringify(putEn));

  const mkIssue = (tag) => JSON.stringify({
    request_id: `locale-${tag}-${Date.now()}`,
    document_type: 'FT',
    station_id: station,
    snapshot: {
      source_system: 'LOCAL',
      source_sale_id: `locale-${tag}-sale`,
      lines: [{
        product_code: 'P1', product_description: 'Item', quantity: '1',
        unit_price_gross: '10.00', vat_rate: '0.23', product_type: 'P', unit_of_measure: 'UN',
      }],
      customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
      payments: [{ method: 'CASH', amount: '10.00' }],
    },
  });

  const issuedEn = JSON.parse(await uatCmd('req', 'POST', '/local/v1/fiscal-documents', '--body', mkIssue('en')));
  record('issue FT (en UI)', !!issuedEn.document_id, issuedEn.invoice_no || '');
  await new Promise((r) => setTimeout(r, 600));
  const locEn = payloadLocale(issuedEn.document_id);
  record('payload.locale=en', locEn === 'en', locEn);

  const putZh = JSON.parse(await uatCmd('req', 'PUT', '/local/v1/setup/ui-locale', '--body', JSON.stringify({ ui_locale: 'zh' })));
  record('PUT zh→pt', putZh.ui_locale === 'zh' && putZh.invoice_locale === 'pt', JSON.stringify(putZh));

  const issuedZh = JSON.parse(await uatCmd('req', 'POST', '/local/v1/fiscal-documents', '--body', mkIssue('zh')));
  record('issue FT (zh UI)', !!issuedZh.document_id, issuedZh.invoice_no || '');
  await new Promise((r) => setTimeout(r, 600));
  const locZh = payloadLocale(issuedZh.document_id);
  record('payload.locale=pt', locZh === 'pt', locZh);

  const prefs = JSON.parse(readFileSync(join(dataDir, 'ui_locale.json'), 'utf8'));
  record('prefs ui_locale=zh', prefs.ui_locale === 'zh', JSON.stringify(prefs));

  child.kill('SIGTERM');
  const failed = results.filter((r) => r.status === 'fail');
  console.log(JSON.stringify({ summary: { pass: results.length - failed.length, fail: failed.length }, results }, null, 2));
  if (failed.length) process.exit(1);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
