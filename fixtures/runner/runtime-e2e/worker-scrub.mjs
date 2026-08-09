// Verify Mew credentials are scrubbed before user worker code executes.
// The worker entrypoint itself is user code — by the time it runs,
// credential-grabber should have already stripped process.env.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const worker = new Worker(join(__dirname, 'worker-scrub-task.mjs'));
  worker.on('message', (msg) => writeFileSync('output.txt', msg));
  worker.on('error', (err) => writeFileSync('output.txt', 'error:' + err.message));
}
