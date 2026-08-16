// Worker task: count credential-grabber occurrences in execArgv.
import { parentPort } from 'node:worker_threads';

var count = 0;
for (var i = 0; i < process.execArgv.length; i++) {
  if (process.execArgv[i].indexOf('credential-grabber') !== -1) count++;
}
parentPort.postMessage('cred-grabber-count=' + count);
