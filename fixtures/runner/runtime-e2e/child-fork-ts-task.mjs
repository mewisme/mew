// Task for forked child: imports a TypeScript module and sends the result back.
// Used by child-fork-ts.mjs to verify TypeScript transform works in child processes.
import { libValue } from './lib.ts';

if (process.send) {
  process.send('libValue=' + libValue);
}
