// Verify custom loaders propagate to workers (Issue 8).
// Parent runs with --loader ./loader-log.mjs; worker must also
// see the custom loader invoked for its imports.
// The loader appends to output.txt; this entrypoint overwrites
// output.txt with the worker result. Between the two, both the
// loader marker and worker result should be visible: the loader
// fires during worker module resolution before the worker task
// posts its message back, so loader output appears first.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync, readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const worker = new Worker(join(__dirname, 'worker-loader-task.mjs'));
  worker.on('message', (msg) => {
    // Read any loader output that was appended, then write final result.
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
