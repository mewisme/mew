// Mew runtime preload (CommonJS).
// Runs before every user module (--require).
// Must NOT strip transform credentials here — this runs before
// loader-register.mjs creates the loader thread, and the loader
// thread needs MEW_TRANSFORM_* in its process.env copy to capture
// them. Credential stripping happens in preload.mjs (--import),
// which runs after loader thread creation.
'use strict';

// Web Storage (localStorage, sessionStorage).
// Canonical implementation in web-storage.cjs.
// localStorage: persisted per-project when MEW_LOCAL_STORAGE_PATH is set,
//   in-memory-only otherwise.  sessionStorage: in-memory, per-realm.
{
  const { createLocalStorage, createSessionStorage } = require('./web-storage.cjs');

  // Only install if Node does not already provide a native implementation.
  // Node 22+ may ship Web Storage globals under experimental flags; Mew
  // replaces the in-memory shim with its own persisted implementation
  // for localStorage, but defers to native sessionStorage when present.
  if (typeof globalThis.localStorage === 'undefined') {
    globalThis.localStorage = createLocalStorage({
      filePath: process.env.MEW_LOCAL_STORAGE_PATH || null,
    });
  }
  if (typeof globalThis.sessionStorage === 'undefined') {
    globalThis.sessionStorage = createSessionStorage();
  }
}
