// Worker task that imports a TypeScript module and posts the result back.
// Used by worker-ts.mjs to verify TypeScript transform works inside workers.
import { parentPort } from 'node:worker_threads';
import { libValue } from './lib.ts';

parentPort.postMessage('libValue=' + libValue);
