// Verify workers cannot observe raw transform credentials (Issue 19 security).
// Credentials travel via module.register() data, never through env/argv/workerData.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const worker = new Worker(join(__dirname, 'worker-creds-task.mjs'));
  worker.on('message', (msg) => {
    writeFileSync('output.txt', String(msg));
  });
  worker.on('error', (err) => {
    writeFileSync('output.txt', 'worker-error:' + err.message);
  });
}
