// Verify that workers can import and execute TypeScript modules.
// Creates a Worker that imports lib.ts and reports the result.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const worker = new Worker(join(__dirname, 'worker-ts-task.mjs'));
  worker.on('message', (msg) => {
    writeFileSync('output.txt', String(msg));
  });
  worker.on('error', (err) => {
    writeFileSync('output.txt', 'worker-error:' + err.message);
  });
  worker.on('exit', (code) => {
    if (code !== 0) {
      writeFileSync('output.txt', 'worker-exit:' + code);
    }
  });
}
