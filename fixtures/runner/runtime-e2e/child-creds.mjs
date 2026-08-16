// Security test: verify that forked child processes cannot observe raw
// transform credentials (Issue 39). Credentials are placed in the child
// env and stripped by credential-grabber before user code runs.
// Probes env, process.argv, process.execArgv, and module exports.
import { fork } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const child = fork(join(__dirname, 'child-creds-task.mjs'), [], {
  stdio: ['inherit', 'inherit', 'inherit', 'ipc'],
});

child.on('message', (msg) => {
  writeFileSync('output.txt', String(msg));
});

child.on('error', (err) => {
  writeFileSync('output.txt', 'child-error:' + err.message);
});
