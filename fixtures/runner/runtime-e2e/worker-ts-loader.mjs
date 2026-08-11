// Verify custom loader + TypeScript both work in workers (Issue 8).
// Preserves loader log output by reading before writing the worker result.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const worker = new Worker(join(__dirname, 'worker-ts-task.mjs'));
  worker.on('message', (msg) => {
    var existing = '';
    try { existing = readFileSync('output.txt', 'utf-8'); } catch (_) {}
    writeFileSync('output.txt', existing + String(msg));
  });
  worker.on('error', (err) => {
    writeFileSync('output.txt', 'worker-error:' + err.message);
  });
  worker.on('exit', (code) => {
    if (code !== 0) {
      var existing = '';
      try { existing = readFileSync('output.txt', 'utf-8'); } catch (_) {}
      writeFileSync('output.txt', existing + 'worker-exit:' + code);
    }
  });
}
