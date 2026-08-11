// Verify Mew hooks are injected exactly once in worker execArgv (Issue 8).
// The worker inspects process.execArgv and reports how many times
// credential-grabber appears.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const worker = new Worker(join(__dirname, 'worker-loader-once-task.mjs'));
  worker.on('message', (msg) => {
    writeFileSync('output.txt', String(msg));
  });
  worker.on('error', (err) => {
    writeFileSync('output.txt', 'worker-error:' + err.message);
  });
}
