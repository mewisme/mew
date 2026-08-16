// Worker task: verify credential scrubbing before user code.
// By the time this module executes, credential-grabber should have
// already stripped MEW_TRANSFORM_* from process.env.
import { parentPort, workerData } from 'node:worker_threads';

const credKeys = [
  'MEW_TRANSFORM_ENDPOINT',
  'MEW_TRANSFORM_TOKEN',
  'MEW_TRANSFORM_OPTIONS',
  'MEW_TRANSFORM_OPTS_DIGEST',
  'MEW_TRANSFORM_CONFIG_DIR',
  'MEW_TRANSFORM_DEP_TRACE_FILE',
  'MEW_TRANSFORM_DEP_TRACE_ROOT',
];

const leaked = credKeys.filter(k => process.env[k] !== undefined);
const workerDataKeys = (workerData && typeof workerData === 'object')
  ? Object.keys(workerData).filter(k => k.includes('MEW_') || k.includes('_mew_'))
  : [];

const result = {
  envLeaked: leaked.length > 0 ? leaked.join(',') : 'none',
  workerDataLeaked: workerDataKeys.length > 0 ? workerDataKeys.join(',') : 'none',
};

parentPort.postMessage(JSON.stringify(result));
