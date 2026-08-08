// Task for spawn/execFile children: imports a TypeScript module and
// writes the result to stdout. Used by child-spawn-ts.mjs to verify
// TypeScript transform works via spawn(process.execPath, ...).
// Note: spawn children don't have IPC (process.send), so we use stdout.
import { libValue } from './lib.ts';
process.stdout.write('libValue=' + libValue + '\n');
