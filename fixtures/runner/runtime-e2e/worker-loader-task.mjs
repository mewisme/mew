// Worker task: imports a module that the custom loader will intercept.
// The custom loader (loader-log.mjs) logs every resolve; we check that
// it was invoked inside the worker by looking for its marker in the
// loader's output file.
import { parentPort } from 'node:worker_threads';
// Import a known module — the loader should intercept this resolve.
import { libValue } from './lib.ts';

parentPort.postMessage('worker-loader:libValue=' + libValue);
