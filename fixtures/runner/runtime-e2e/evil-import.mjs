// Attacker-style probe: malicious --import preload that attempts to
// recover transform credentials through every available channel.
// Writes probe results to output.txt as key=value lines.
// Expected result: all probes report "absent" or "none".
import { writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { accessSync, constants } from 'node:fs';
import { isMainThread } from 'node:worker_threads';
import { createRequire } from 'node:module';

const results = [];

// Probe 1: env vars (must be absent)
const envVars = [
  'MEW_TRANSFORM_ENDPOINT',
  'MEW_TRANSFORM_TOKEN',
  'MEW_TRANSFORM_OPTIONS',
  'MEW_TRANSFORM_OPTS_DIGEST',
  'MEW_TRANSFORM_CONFIG_DIR',
];
for (const v of envVars) {
  results.push('import-env-' + v + '=' + (process.env[v] || 'absent'));
}

// Probe 2: old PID-based temp file
const oldCredsFile = join(tmpdir(), '.mew-creds-' + process.pid + '.json');
try {
  accessSync(oldCredsFile, constants.R_OK);
  results.push('import-old-pid-creds-file=present');
} catch (e) {
  results.push('import-old-pid-creds-file=absent');
}

// Probe 3: process.argv secrets
let hasEndpoint = false;
for (const arg of process.argv) {
  if (arg.includes('127.0.0.1:') && arg.includes('MEW_TRANSFORM')) {
    hasEndpoint = true;
  }
}
results.push('import-argv-has-endpoint=' + (hasEndpoint ? 'present' : 'absent'));

// Probe 4: execArgv secrets
let hasEndpointExec = false;
for (const arg of process.execArgv) {
  if (arg.includes('127.0.0.1:')) {
    hasEndpointExec = true;
  }
}
results.push('import-execArgv-has-endpoint=' + (hasEndpointExec ? 'present' : 'absent'));

// Probe 5: try to access credential-grabber via createRequire
try {
  const require = createRequire(import.meta.url);
  // Find grabber path from execArgv
  let grabberPath = null;
  for (const arg of process.execArgv) {
    if (arg.endsWith && arg.endsWith('credential-grabber.cjs')) {
      grabberPath = arg;
      break;
    }
  }
  if (grabberPath) {
    const creds = require(grabberPath);
    if (creds.endpoint) {
      results.push('import-grabber-endpoint=' + String(creds.endpoint).substring(0, 50));
    } else {
      results.push('import-grabber-endpoint=absent');
    }
    if (creds.token) {
      results.push('import-grabber-token=present');
    } else {
      results.push('import-grabber-token=absent');
    }
  } else {
    results.push('import-grabber-probe=not-found');
  }
} catch (e) {
  results.push('import-grabber-probe=error:' + e.message);
}

// Probe 6: globalThis suspicious symbols
try {
  const syms = Object.getOwnPropertySymbols(globalThis);
  let suspiciousCount = 0;
  for (const sym of syms) {
    const desc = sym.toString();
    if (desc.includes('mew') || desc.includes('creds') || desc.includes('token')) {
      suspiciousCount++;
    }
  }
  results.push('import-globalThis-suspicious=' + suspiciousCount);
} catch (e) {
  results.push('import-globalThis-suspicious=error:' + e.message);
}

results.push('import-is-main-thread=' + String(isMainThread));

writeFileSync('output.txt', results.join('\n'));
