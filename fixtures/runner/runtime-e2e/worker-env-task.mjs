// Worker task that checks effective environment propagation (Issue 19).
// Reports whether MEW_TRANSFORM_* vars are present (should be absent — they
// are stripped by credential-grabber before user code executes).
import { parentPort, workerData } from 'node:worker_threads';

const results = [
  'MEW_TRANSFORM_ENDPOINT=' + (process.env.MEW_TRANSFORM_ENDPOINT || 'absent'),
  'MEW_TRANSFORM_TOKEN=' + (process.env.MEW_TRANSFORM_TOKEN || 'absent'),
  'MEW_USER_LOADERS=' + (process.env.MEW_USER_LOADERS || 'absent'),
  // Verify that dotenv/host env vars propagate correctly.
  'TEST_PROPAGATED=' + (process.env.TEST_PROPAGATED || 'absent'),
];
parentPort.postMessage(results.join('\n'));
