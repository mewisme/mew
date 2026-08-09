// Verify workerData=null remains null in the worker.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const worker = new Worker(join(__dirname, 'worker-data-task.mjs'), {
    workerData: null,
  });
  worker.on('message', (msg) => writeFileSync('output.txt', msg));
  worker.on('error', (err) => writeFileSync('output.txt', 'error:' + err.message));
}
