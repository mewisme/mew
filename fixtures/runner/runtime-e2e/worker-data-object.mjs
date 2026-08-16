// Verify plain object workerData is preserved without Mew-injected keys.
// Also verify the parent-side object is not mutated after Worker construction.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const userData = { name: 'test', count: 99 };
  const keysBefore = Object.keys(userData).sort().join(',');
  const symbolsBefore = Object.getOwnPropertySymbols(userData).length;

  const worker = new Worker(join(__dirname, 'worker-data-task.mjs'), {
    workerData: userData,
  });

  // Parent-side object must be unchanged after Worker construction.
  const keysAfter = Object.keys(userData).sort().join(',');
  const symbolsAfter = Object.getOwnPropertySymbols(userData).length;

  const results = [];
  worker.on('message', (msg) => {
    results.push('worker:' + msg);
    // Report parent-side mutation check.
    if (keysBefore !== keysAfter || symbolsBefore !== symbolsAfter) {
      results.push('parent-mutated:keysBefore=' + keysBefore + ' keysAfter=' + keysAfter +
        ' symbolsBefore=' + symbolsBefore + ' symbolsAfter=' + symbolsAfter);
    } else {
      results.push('parent-unmutated');
    }
    writeFileSync('output.txt', results.join('\n'));
  });
  worker.on('error', (err) => writeFileSync('output.txt', 'error:' + err.message));
}
