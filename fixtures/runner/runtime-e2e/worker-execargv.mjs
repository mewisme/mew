// Verify explicit custom execArgv is preserved in the worker.
// The worker should see the user's custom flags plus Mew bootstrap.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const worker = new Worker(join(__dirname, 'worker-execargv-task.mjs'), {
    execArgv: ['--experimental-vm-modules', '--no-warnings'],
  });
  worker.on('message', (msg) => writeFileSync('output.txt', msg));
  worker.on('error', (err) => writeFileSync('output.txt', 'error:' + err.message));
}
