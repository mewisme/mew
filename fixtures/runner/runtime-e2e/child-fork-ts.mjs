// Verify that forked child processes can import and execute TypeScript.
// Uses child_process.fork to create a child that imports lib.ts.
import { fork } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const child = fork(join(__dirname, 'child-fork-ts-task.mjs'), [], {
  stdio: ['inherit', 'inherit', 'inherit', 'ipc'],
});

child.on('message', (msg) => {
  writeFileSync('output.txt', String(msg));
});

child.on('error', (err) => {
  writeFileSync('output.txt', 'child-error:' + err.message);
});

child.on('exit', (code) => {
  if (code !== 0) {
    writeFileSync('output.txt', 'child-exit:' + code);
  }
});
