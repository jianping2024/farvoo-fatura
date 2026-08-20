#!/usr/bin/env node
/**
 * M2: main Agent binary (-fiscal-standalone) + fake TCP printer smoke.
 * Scenarios (no skip): health on embedded fiscal → setup → issue FT →
 * fake-printer sees ATCUD → bad printer leaves invoice SIGNED.
 */
import { spawn } from 'node:child_process';
import { mkdirSync, rmSync, existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const agent = join(root, 'apps', 'fiscal-agent');
const base = 'http://127.0.0.1:17882';
const bind = '127.0.0.1:17882';
const uat = join(root, 'scripts', 'fiscal-local-uat.mjs');
const pemPath = join(agent, 'internal', 'fiscal', 'testdata', 'dev_signing_key.pem');
const fakeLog = join(agent, 'data', 'fake-printer-m2.log');
const year = new Date().getFullYear();
const dataRoot = join(agent, 'data', 'm2-agent');

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
    await run('pkill', ['-f', 'fiscal-standalone']);
  } catch {
    /* */
  }
  try {
    await run('pkill', ['-f', 'fake-printer']);
  } catch {
    /* */
  }
  await new Promise((r) => setTimeout(r, 400));

  mkdirSync(join(agent, 'data'), { recursive: true });
  if (existsSync(dataRoot)) {
    try {
      await run('chmod', ['-R', 'u+w', dataRoot]);
    } catch {
      /* */
    }
    rmSync(dataRoot, { recursive: true, force: true });
  }
  mkdirSync(dataRoot, { recursive: true });
  if (existsSync(fakeLog)) rmSync(fakeLog);

  const dbPath = join(dataRoot, 'fiscal.db');
  const secureDir = join(dataRoot, 'fiscal-secure');

  const fake = spawn('go', ['run', '.'], {
    cwd: join(agent, 'dev', 'fake-printer'),
    env: {
      ...process.env,
      PATH: `/opt/homebrew/bin:${process.env.PATH}`,
      PRINTER_PORT: '9100',
      PRINTER_NAME: 'm2fake',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: true,
  });
  let fakeOut = '';
  fake.stdout.on('data', (d) => {
    fakeOut += d;
    writeFileSync(fakeLog, fakeOut);
  });
  fake.stderr.on('data', (d) => {
    fakeOut += d;
    writeFileSync(fakeLog, fakeOut);
  });
  await new Promise((r) => setTimeout(r, 1500));

  const child = spawn('go', ['run', '.', '-fiscal-standalone'], {
    cwd: agent,
    env: {
      ...process.env,
      PATH: `/opt/homebrew/bin:${process.env.PATH}`,
      FISCAL_DB: dbPath,
      FISCAL_DATA_DIR: secureDir,
      FISCAL_BIND: bind,
      FISCAL_STORE_ID: 'store-demo-001',
      FISCAL_AT_ENV: 'mock',
      FISCAL_ALLOW_LOCAL_PROVISION: '1',
      FISCAL_PRINTER_TCP: '127.0.0.1:9100',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: true,
  });
  let boot = '';
  child.stdout.on('data', (d) => (boot += d));
  child.stderr.on('data', (d) => (boot += d));

  let healthy = false;
  for (let i = 0; i < 80; i++) {
    if (child.exitCode != null && child.exitCode !== 0) break;
    try {
      await uatCmd('stack-health');
      healthy = true;
      break;
    } catch {
      await new Promise((r) => setTimeout(r, 250));
    }
  }
  record('agent-embedded-health', healthy, healthy ? base : boot.slice(-500));
  if (!healthy) {
    try {
      process.kill(-child.pid, 'SIGTERM');
    } catch {
      child.kill('SIGTERM');
    }
    try {
      process.kill(-fake.pid, 'SIGTERM');
    } catch {
      fake.kill('SIGTERM');
    }
    process.exit(1);
  }

  const pem = readFileSync(pemPath, 'utf8');
  const y = year;
  try {
    await uatCmd(
      'req',
      'PUT',
      '/local/v1/setup/taxpayer',
      '--body',
      JSON.stringify({
        tax_registration_number: '517535009',
        legal_name: 'Farvoo Demo Lda',
        address_detail: 'Rua 1',
        city: 'Lisboa',
        postal_code: '1000-001',
        country: 'PT',
        timezone: 'Europe/Lisbon',
        software_certificate_number: '0',
      }),
    );
    await uatCmd(
      'req',
      'PUT',
      '/local/v1/setup/at-credentials',
      '--body',
      JSON.stringify({ username: '517535009/37', password: 'x' }),
    );
    await uatCmd(
      'req',
      'POST',
      '/local/v1/setup/series/register',
      '--body',
      JSON.stringify({ series_code: `FT${y}DEMO01`, document_type: 'FT', fiscal_year: y }),
    );
    await uatCmd('req', 'POST', '/local/v1/setup/activate', '--body', JSON.stringify({ product_private_key_pem: pem }));
    await uatCmd(
      'req',
      'PUT',
      '/local/v1/setup/operator',
      '--body',
      JSON.stringify({ id: 'op-demo-cashier', role: 'cashier', display_name: 'C' }),
    );
    record('m2-setup', true);
  } catch (e) {
    record('m2-setup', false, String(e));
    try {
      process.kill(-child.pid, 'SIGTERM');
    } catch {
      child.kill('SIGTERM');
    }
    try {
      process.kill(-fake.pid, 'SIGTERM');
    } catch {
      fake.kill('SIGTERM');
    }
    process.exit(1);
  }

  const requestId = `m2-${Date.now()}`;
  let issue;
  try {
    const raw = await uatCmd(
      'req',
      'POST',
      '/local/v1/fiscal-documents',
      '--body',
      JSON.stringify({
        request_id: requestId,
        operator_id: 'op-demo-cashier',
        document_type: 'FT',
        snapshot: {
          source_system: 'farvoo',
          source_sale_id: `sale-${requestId}`,
          scope_type: 'session',
          scope_id: 's',
          fiscal_purpose: 'sale',
          lines: [
            {
              product_code: 'D',
              display_name: 'Prato',
              saft_name: 'Prato',
              quantity: '1',
              unit_price_gross: '12.50',
              vat_rate: '0.23',
            },
          ],
          customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
          payments: [{ method: 'CASH', amount: '12.50' }],
        },
      }),
    );
    issue = JSON.parse(raw);
    record('issue-ft', issue.document_status === 'SIGNED', issue.invoice_no);
  } catch (e) {
    record('issue-ft', false, String(e));
    try {
      process.kill(-child.pid, 'SIGTERM');
    } catch {
      child.kill('SIGTERM');
    }
    try {
      process.kill(-fake.pid, 'SIGTERM');
    } catch {
      fake.kill('SIGTERM');
    }
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
      '8000',
    );
    record('print-job-printed', true);
  } catch (e) {
    record('print-job-printed', false, String(e));
  }

  // fake printer should have received bytes with ATCUD
  await new Promise((r) => setTimeout(r, 500));
  const logTxt = existsSync(fakeLog) ? readFileSync(fakeLog, 'utf8') : fakeOut;
  record('fake-printer-atcud', logTxt.includes('ATCUD') || logTxt.includes(issue.atcud), logTxt.slice(-200));

  // print failure path: stop fake (process group), issue another — job fails, invoice remains SIGNED
  try {
    process.kill(-fake.pid, 'SIGTERM');
  } catch {
    try {
      fake.kill('SIGTERM');
    } catch {
      /* */
    }
  }
  await new Promise((r) => setTimeout(r, 800));
  // ensure :9100 is gone (orphan go-run child)
  try {
    await run('pkill', ['-f', 'fake-printer']);
  } catch {
    /* */
  }
  await new Promise((r) => setTimeout(r, 400));

  const requestId2 = `m2-fail-${Date.now()}`;
  try {
    const raw = await uatCmd(
      'req',
      'POST',
      '/local/v1/fiscal-documents',
      '--body',
      JSON.stringify({
        request_id: requestId2,
        operator_id: 'op-demo-cashier',
        document_type: 'FT',
        snapshot: {
          source_system: 'farvoo',
          source_sale_id: `sale-${requestId2}`,
          scope_type: 'session',
          scope_id: 's2',
          fiscal_purpose: 'sale',
          lines: [
            {
              product_code: 'D',
              display_name: 'Prato',
              saft_name: 'Prato',
              quantity: '1',
              unit_price_gross: '5.00',
              vat_rate: '0.23',
            },
          ],
          customer: { tax_id: '999999990', company_name: 'Consumidor Final', country: 'PT' },
          payments: [{ method: 'CASH', amount: '5.00' }],
        },
      }),
    );
    const iss2 = JSON.parse(raw);
    let failed = false;
    let lastJob = '';
    for (let i = 0; i < 80; i++) {
      const jobRaw = await uatCmd('req', 'GET', `/local/v1/print-jobs/${iss2.print_job_id}`);
      const job = JSON.parse(jobRaw);
      lastJob = job.job_status;
      if (job.job_status === 'FAILED') {
        failed = true;
        break;
      }
      await new Promise((r) => setTimeout(r, 250));
    }
    const docRaw = await uatCmd('req', 'GET', `/local/v1/fiscal-documents/by-request/${requestId2}`);
    const doc = JSON.parse(docRaw);
    record(
      'print-fail-keeps-signed',
      failed && doc.document_status === 'SIGNED',
      `jobFailed=${failed} last=${lastJob} status=${doc.document_status}`,
    );
  } catch (e) {
    record('print-fail-keeps-signed', false, String(e));
  }

  try {
    process.kill(-child.pid, 'SIGTERM');
  } catch {
    try {
      child.kill('SIGTERM');
    } catch {
      /* */
    }
  }
  try {
    await run('pkill', ['-f', 'fiscal-standalone']);
  } catch {
    /* */
  }
  const failed = results.filter((r) => r.status === 'fail');
  console.log('\n=== M2 SUMMARY ===');
  for (const r of results) console.log(`${r.status}\t${r.name}\t${r.note}`);
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
