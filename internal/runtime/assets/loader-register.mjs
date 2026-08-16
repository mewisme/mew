// Mew runtime — loader registration shim for --node mode.
// Injected as --import when --node --loader <path> is used.
// Reads MEW_USER_LOADERS (newline-separated file:// URLs) from the
// environment, deletes it, and registers each user loader via
// module.register(). No credential handling, no ts-loader —
// just the user's loaders on stock Node.
//
// Hook chain (LIFO: last-registered fires first):
//   register loader-2  →  register loader-1
//   Result: loader-1 fires first, then loader-2, then Node default.

import { register } from 'node:module';

const raw = process.env.MEW_USER_LOADERS || '';
delete process.env.MEW_USER_LOADERS;

if (raw) {
  const loaders = raw.split('\n').filter((u) => u.length > 0);
  // Reverse: last-registered = first-called (outermost).
  for (let i = loaders.length - 1; i >= 0; i--) {
    try {
      register(loaders[i], import.meta.url, {
        parentURL: import.meta.url,
        data: {},
        transferList: [],
      });
    } catch (_) {
      // User loader registration failed. Let Node surface the error
      // if the loader is required for resolution.
    }
  }
}
