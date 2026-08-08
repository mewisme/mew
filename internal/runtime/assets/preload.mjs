// Mew runtime preload (ESM).
// Runs before every user module (--import).
//
// Transform credentials (MEW_TRANSFORM_*) are already stripped by
// credential-grabber.cjs, which runs as the first --require.
// The deletions below are defense-in-depth: in the unlikely event
// credential-grabber did not run, user code still sees clean env.
delete process.env.MEW_TRANSFORM_ENDPOINT;
delete process.env.MEW_TRANSFORM_TOKEN;
delete process.env.MEW_TRANSFORM_OPTIONS;
delete process.env.MEW_TRANSFORM_OPTS_DIGEST;
delete process.env.MEW_TRANSFORM_CONFIG_DIR;
delete process.env.MEW_TRANSFORM_DEP_TRACE_FILE;
delete process.env.MEW_TRANSFORM_DEP_TRACE_ROOT;

// Web Storage (localStorage, sessionStorage).
// Canonical implementation in web-storage.cjs.
// localStorage: persisted per-project when MEW_LOCAL_STORAGE_PATH is set,
//   in-memory-only otherwise.  sessionStorage: in-memory, per-realm.
import { createRequire } from 'node:module';
const _require = createRequire(import.meta.url);
const { createLocalStorage, createSessionStorage } = _require('./web-storage.cjs');

// Only install if Node does not already provide a native implementation.
if (typeof globalThis.localStorage === 'undefined') {
  globalThis.localStorage = createLocalStorage({
    filePath: process.env.MEW_LOCAL_STORAGE_PATH || null,
  });
}
if (typeof globalThis.sessionStorage === 'undefined') {
  globalThis.sessionStorage = createSessionStorage();
}
