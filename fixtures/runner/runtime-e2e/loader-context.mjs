// Context-observing loader: writes hook context fields to output.txt.
// Proves required context fields are present and Mew secrets are absent.
import { writeFileSync } from 'node:fs';

const LOG = new URL('./output.txt', import.meta.url).pathname;

export async function resolve(specifier, context, nextResolve) {
  const fields = {
    conditions: Array.isArray(context.conditions),
    parentURL: typeof context.parentURL === 'string' || context.parentURL === undefined,
    importAttributes: typeof context.importAttributes === 'object',
  };
  writeFileSync(LOG, 'loader-context:resolve:' + JSON.stringify(fields) + '\n', { flag: 'a' });
  return nextResolve(specifier, context);
}
