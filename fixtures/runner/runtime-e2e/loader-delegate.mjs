// Delegating loader: calls nextResolve first, then augments the result.
// Proves hook chaining works with Mew's ts-loader.
import { writeFileSync } from 'node:fs';

const LOG = new URL('./output.txt', import.meta.url).pathname;

export async function resolve(specifier, context, nextResolve) {
  const result = await nextResolve(specifier, context);
  writeFileSync(LOG, 'loader-delegate:resolve:' + specifier + ':url=' + result.url + '\n', { flag: 'a' });
  // Augment: add a marker to the format if applicable.
  return result;
}

export async function load(url, context, nextLoad) {
  writeFileSync(LOG, 'loader-delegate:load:' + url + '\n', { flag: 'a' });
  return nextLoad(url, context);
}
