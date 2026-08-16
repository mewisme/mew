// Ordering test loader A. Writes "A" then delegates.
import { appendFileSync } from 'node:fs';
const LOG = new URL('./output.txt', import.meta.url).pathname;

export async function resolve(specifier, context, nextResolve) {
  appendFileSync(LOG, 'A\n');
  return nextResolve(specifier, context);
}
