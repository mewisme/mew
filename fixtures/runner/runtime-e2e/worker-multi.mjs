// Verify multiple concurrent workers each get isolated runtime capabilities.
// Each worker imports lib.ts independently; results must not interfere.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const NUM_WORKERS = 3;

if (isMainThread) {
  let done = 0;
  let errors = [];
  let results = [];

  for (let i = 0; i < NUM_WORKERS; i++) {
    const worker = new Worker(join(__dirname, 'worker-ts-task.mjs'));
    worker.on('message', (msg) => {
      results.push(String(msg));
      done++;
      if (done === NUM_WORKERS) {
        // All workers done — verify results.
        const ok = results.every(r => r === 'libValue=resolved-lib-ts');
        writeFileSync('output.txt', ok ? 'all-workers-ok' : 'worker-mismatch:' + JSON.stringify(results));
      }
    });
    worker.on('error', (err) => {
      errors.push(err.message);
      done++;
      if (done === NUM_WORKERS) {
        writeFileSync('output.txt', 'worker-errors:' + errors.join(','));
      }
    });
  }
}
