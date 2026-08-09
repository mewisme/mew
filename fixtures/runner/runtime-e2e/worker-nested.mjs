// Verify nested workers receive TypeScript augmentation.
// The outer worker creates an inner worker that imports TypeScript.
import { Worker, isMainThread } from 'node:worker_threads';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (isMainThread) {
  const worker = new Worker(join(__dirname, 'worker-nested-outer.mjs'));
  worker.on('message', (msg) => writeFileSync('output.txt', msg));
  worker.on('error', (err) => writeFileSync('output.txt', 'error:' + err.message));
}
