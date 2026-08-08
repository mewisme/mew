// Ordinary custom loader: exports resolve hook, does NOT self-register.
// Proves Mew's module.register() call registers hooks correctly.
import { writeFileSync } from 'node:fs';

const LOG = new URL('./output.txt', import.meta.url).pathname;

export async function resolve(specifier, context, nextResolve) {
  writeFileSync(LOG, 'loader-log:resolve:' + specifier + '\n', { flag: 'a' });
  return nextResolve(specifier, context);
}

export async function load(url, context, nextLoad) {
  writeFileSync(LOG, 'loader-log:load:' + url + '\n', { flag: 'a' });
  return nextLoad(url, context);
}
