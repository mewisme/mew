// Error-throwing loader: throws immediately in resolve.
// Proves user loader errors propagate without being converted to Mew errors.
export async function resolve(specifier, context, nextResolve) {
  throw new Error('ERR_CUSTOM_LOADER: intentional test error');
}
