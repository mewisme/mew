// Entrypoint for custom loader tests. Imports a module so loader resolve
// hooks fire, then writes a completion marker.
import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const LOG = resolve(__dirname, 'output.txt');

// Import a local module to trigger resolve hooks.
import('./lib-import.js').then(function (mod) {
  writeFileSync(LOG, 'entrypoint:done:' + mod.value + '\n', { flag: 'a' });
});
