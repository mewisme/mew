// Verify effective environment propagation into workers (Issue 19).
// Workers should receive the parent's environment (minus stripped creds)
// and should observe user-specified env overrides.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  // Pass an explicit env to the worker via worker option.
  const worker = new Worker(join(__dirname, 'worker-env-task.mjs'), {
    env: { ...process.env, TEST_PROPAGATED: 'from-parent-env' },
  });
  worker.on('message', (msg) => {
    writeFileSync('output.txt', String(msg));
  });
  worker.on('error', (err) => {
    writeFileSync('output.txt', 'worker-error:' + err.message);
  });
}
