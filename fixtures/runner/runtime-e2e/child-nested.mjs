// Verify nested child process TypeScript execution (child -> grandchild).
// The forked child creates another forked child that imports lib.ts.
// This proves propagation works across multiple generations without
// accumulating duplicate bootstrap flags.
import { fork } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

// Inline the grandchild task (uses process.send which works in forked children).
var grandchildCode = `
import { fork } from 'node:child_process';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
var __d = dirname(fileURLToPath(import.meta.url));
var gc = fork(join(__d, 'child-fork-ts-task.mjs'), [], {
  stdio: ['inherit', 'inherit', 'inherit', 'ipc'],
});
gc.on('message', function(m) {
  if (process.send) process.send('grandchild-' + m);
});
gc.on('error', function(e) {
  if (process.send) process.send('grandchild-error:' + e.message);
});
`;

writeFileSync(join(__dirname, '_nested-task.mjs'), grandchildCode);

var child = fork(join(__dirname, '_nested-task.mjs'), [], {
  stdio: ['inherit', 'inherit', 'inherit', 'ipc'],
});

child.on('message', function (msg) {
  writeFileSync('output.txt', String(msg));
});

child.on('error', function (err) {
  writeFileSync('output.txt', 'child-error:' + err.message);
});

child.on('exit', function (code) {
  if (code !== 0 && code !== null) {
    writeFileSync('output.txt', 'child-exit:' + code);
  }
});
