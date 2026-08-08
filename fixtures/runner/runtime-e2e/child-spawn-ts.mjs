// Verify that spawn(process.execPath, ...) children can import TypeScript.
// Uses child_process.spawn to create a child that imports lib.ts.
import { spawn } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

var taskPath = join(__dirname, 'child-spawn-ts-task.mjs');

var child = spawn(process.execPath, [taskPath], {
  stdio: ['inherit', 'pipe', 'inherit'],
});

var stdout = '';
child.stdout.on('data', function (chunk) { stdout += chunk; });

child.on('error', function (err) {
  writeFileSync('output.txt', 'child-error:' + err.message);
});

child.on('exit', function (code) {
  writeFileSync('output.txt', stdout.trim() || 'child-exit:' + code);
});
