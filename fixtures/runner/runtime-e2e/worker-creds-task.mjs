// Security test: verify that worker cannot observe raw transform credentials.
// Credentials should be absent from process.env, workerData, and any
// enumerable properties. They are delivered to ts-loader via module.register()
// data and never exposed to user code.
import { parentPort, workerData } from 'node:worker_threads';

// Probe: check env (should be clean — creds stripped by parent's grabber).
const envProbes = [
  'MEW_TRANSFORM_ENDPOINT',
  'MEW_TRANSFORM_TOKEN',
  'MEW_TRANSFORM_OPTIONS',
  'MEW_TRANSFORM_OPTS_DIGEST',
  'MEW_TRANSFORM_CONFIG_DIR',
];
const results = envProbes.map(v => v + '=' + (process.env[v] || 'absent'));

// Probe: check that workerData has no enumerable cred keys (Symbol is non-enumerable).
if (workerData && typeof workerData === 'object') {
  const keys = Object.keys(workerData);
  results.push('workerData-keys=' + (keys.length === 0 ? 'none' : keys.join(',')));
  // Verify no credential-like keys in enumerable properties.
  const hasCredKeys = keys.some(k =>
    k.includes('MEW_') || k.includes('endpoint') || k.includes('token')
  );
  results.push('workerData-has-cred-keys=' + (hasCredKeys ? 'yes' : 'no'));
}

parentPort.postMessage(results.join('\n'));
