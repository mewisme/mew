// Outer worker task: creates an inner worker that imports TypeScript.
// Verifies nested worker propagation.
import { Worker, parentPort, isMainThread } from 'node:worker_threads';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const inner = new Worker(join(__dirname, 'worker-ts-task.mjs'));
inner.on('message', (msg) => {
  parentPort.postMessage('nested:' + msg);
});
inner.on('error', (err) => {
  parentPort.postMessage('nested-error:' + err.message);
});
