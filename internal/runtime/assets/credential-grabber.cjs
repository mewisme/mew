// Mew credential grabber — runs before any user preload.
// Node processes --require from left to right. Mew places this
// grabber first so it captures transform credentials from process.env,
// strips them before user code executes, and registers the TypeScript
// loader with credentials via module.register()'s data option.
//
// No credentials are written to the filesystem. The handoff uses
// Node's built-in loader registration API, passing data from the
// main thread directly to the loader thread's initialize hook.
// User --require preloads that follow see clean process.env and
// cannot recover credentials.
//
// This module runs twice in Node's two-phase startup:
//   1. Main thread (isMainThread=true): captures env, strips env,
//      registers loader with credentials via module.register().
//      No temp file, no module.exports exposure of real values.
//   2. Loader context (isMainThread=false): re-evaluated by Node's
//      loader thread; exports null values. The loader already
//      received credentials via the initialize hook.
//
// Issue 19 — Worker propagation:
//   Main thread also monkey-patches the Worker constructor to inject
//   credentials into workerData via a Symbol key. When this module
//   re-evaluates in a worker (isMainThread=false, parentPort exists),
//   it extracts credentials from workerData and registers ts-loader
//   for the worker. The loader-thread branch (no parentPort) is
//   unchanged.
'use strict';

const { isMainThread, parentPort, workerData } = require('node:worker_threads');

// ── Credential storage (main-thread closure) ──────────────────────
// Only populated on the main thread. Workers retrieve credentials
// via workerData instead.
let _mewCredentials = null;

// Symbol key for workerData injection. Symbol.for makes it
// accessible across module re-evaluation in workers.
const kMewCreds = Symbol.for('mew:transform-credentials');

if (isMainThread) {
  // ── Main thread ────────────────────────────────────────────────
  // Capture credentials from process.env and strip immediately.
  // credential-grabber runs FIRST (leftmost --require), so no user
  // code has executed yet.
  const endpoint = process.env.MEW_TRANSFORM_ENDPOINT || null;
  const token = process.env.MEW_TRANSFORM_TOKEN || null;
  const options = process.env.MEW_TRANSFORM_OPTIONS || '{}';
  const optsDigest = process.env.MEW_TRANSFORM_OPTS_DIGEST || '';
  const configDir = process.env.MEW_TRANSFORM_CONFIG_DIR || '';
  const depTraceFile = process.env.MEW_TRANSFORM_DEP_TRACE_FILE || '';
  const depTraceRoot = process.env.MEW_TRANSFORM_DEP_TRACE_ROOT || '';

  // Strip from process.env immediately — before any user --require.
  delete process.env.MEW_TRANSFORM_ENDPOINT;
  delete process.env.MEW_TRANSFORM_TOKEN;
  delete process.env.MEW_TRANSFORM_OPTIONS;
  delete process.env.MEW_TRANSFORM_OPTS_DIGEST;
  delete process.env.MEW_TRANSFORM_CONFIG_DIR;
  delete process.env.MEW_TRANSFORM_DEP_TRACE_FILE;
  delete process.env.MEW_TRANSFORM_DEP_TRACE_ROOT;

  // Store for worker propagation (Issue 19).
  if (endpoint && token) {
    _mewCredentials = { endpoint, token, options, optsDigest, configDir, depTraceFile, depTraceRoot };
  }

  // Register the TypeScript loader with credentials passed via
  // module.register()'s data option. This is the sole secure
  // handoff path: data travels from this closure directly to the
  // loader thread's initialize hook, never touching the filesystem
  // or module.exports.
  var register;
  try { register = require('node:module').register; } catch (_) {}
  if (register) {
    try {
      const { pathToFileURL } = require('node:url');
      const path = require('node:path');
      const tsLoader = pathToFileURL(path.join(__dirname, 'ts-loader.mjs')).href;
      const parentURL = pathToFileURL(__filename).href;

      if (endpoint && token) {
        // Register user loaders first (reverse order, outermost hooks win).
        const userLoadersRaw = process.env.MEW_USER_LOADERS || '';
        delete process.env.MEW_USER_LOADERS;
        if (userLoadersRaw) {
          const userLoaders = userLoadersRaw.split('\n').filter(function (u) { return u.length > 0; });
          // Reverse iteration: last-registered = first-called (LIFO chain).
          for (var i = userLoaders.length - 1; i >= 0; i--) {
            try {
              register(userLoaders[i], parentURL, { parentURL, data: {}, transferList: [] });
            } catch (_) { /* user loader registration failed; continue */ }
          }
        }
        // ts-loader registered last → innermost (fills gaps after user hooks).
        register(tsLoader, parentURL, {
          parentURL,
          data: { endpoint, token, options, optsDigest, configDir, depTraceFile, depTraceRoot },
          transferList: [],
        });
      } else {
        // No transform credentials: register only user loaders.
        const userLoadersRaw = process.env.MEW_USER_LOADERS || '';
        delete process.env.MEW_USER_LOADERS;
        if (userLoadersRaw) {
          const userLoaders = userLoadersRaw.split('\n').filter(function (u) { return u.length > 0; });
          for (var i = userLoaders.length - 1; i >= 0; i--) {
            try {
              register(userLoaders[i], parentURL, { parentURL, data: {}, transferList: [] });
            } catch (_) {}
          }
        }
      }
    } catch (_) {
      // require('node:module').register not available (Node < 20.6).
      // Fall back to dynamic import.
      import('node:module').then(function (mod) {
        try {
          const { pathToFileURL } = require('node:url');
          const path = require('node:path');
          const tsLoader = pathToFileURL(path.join(__dirname, 'ts-loader.mjs')).href;
          const parentURL = pathToFileURL(__filename).href;

          if (endpoint && token) {
            const userLoadersRaw = process.env.MEW_USER_LOADERS || '';
            delete process.env.MEW_USER_LOADERS;
            if (userLoadersRaw) {
              const userLoaders = userLoadersRaw.split('\n').filter(function (u) { return u.length > 0; });
              for (var i = userLoaders.length - 1; i >= 0; i--) {
                try {
                  mod.register(userLoaders[i], parentURL, { parentURL, data: {}, transferList: [] });
                } catch (_) {}
              }
            }
            mod.register(tsLoader, parentURL, {
              parentURL,
              data: { endpoint, token, options, optsDigest, configDir, depTraceFile, depTraceRoot },
              transferList: [],
            });
          } else {
            const userLoadersRaw = process.env.MEW_USER_LOADERS || '';
            delete process.env.MEW_USER_LOADERS;
            if (userLoadersRaw) {
              const userLoaders = userLoadersRaw.split('\n').filter(function (u) { return u.length > 0; });
              for (var i = userLoaders.length - 1; i >= 0; i--) {
                try {
                  mod.register(userLoaders[i], parentURL, { parentURL, data: {}, transferList: [] });
                } catch (_) {}
              }
            }
          }
        } catch (_) { /* registration unavailable */ }
      }).catch(function () { /* import failed — registration unavailable */ });
    }
  }

  // ── Worker constructor augmentation (Issue 19) ─────────────────
  // Inject Mew credentials into workerData so worker threads can
  // register ts-loader for themselves. Uses a Symbol key to avoid
  // colliding with user workerData and to prevent enumeration.
  //
  // Only active when transform credentials are present. If there
  // are no credentials (e.g. JS-only entrypoint), workers still
  // inherit preloads (Web Storage) but skip loader registration.
  if (_mewCredentials) {
    try {
      const workerThreads = require('node:worker_threads');
      const OriginalWorker = workerThreads.Worker;

      // Guard against double-patching (e.g. credential-grabber
      // somehow loaded twice in the same isolate).
      if (!OriginalWorker.__mewPatched) {
        workerThreads.Worker = function MewWorker(filename, options) {
          if (!options) options = {};

          // Preserve user workerData. Inject Mew credentials via
          // a non-enumerable Symbol key so user code cannot
          // trivially enumerate or read the raw credentials.
          var wd = options.workerData;
          if (wd !== undefined && typeof wd === 'object' && wd !== null) {
            // User provided an object — attach creds via Symbol key.
            wd[kMewCreds] = _mewCredentials;
          } else if (wd === undefined) {
            options.workerData = { [kMewCreds]: _mewCredentials };
          } else {
            // workerData is a primitive (unusual but valid Node API).
            // Wrap it so we can still inject credentials.
            options.workerData = {
              [kMewCreds]: _mewCredentials,
              // Hidden primitive wrapper; user code never sees this
              // because workerData is replaced with our object.
            };
            // Preserve the primitive value as a hidden property.
            Object.defineProperty(options.workerData, '_mew_userWorkerData', {
              value: wd,
              enumerable: false,
              writable: true,
            });
          }

          return new OriginalWorker(filename, options);
        };

        // Preserve prototype chain and static properties.
        workerThreads.Worker.prototype = OriginalWorker.prototype;
        workerThreads.Worker.__mewPatched = true;

        // Mark the original so we can detect re-patching.
        OriginalWorker.__mewPatched = true;
      }
    } catch (_) {
      // worker_threads may be unavailable in some contexts.
      // Worker propagation is best-effort; worker import of .ts
      // files will fail with a clear Node error instead of silently
      // bypassing the transform.
    }
  }

  // Export null values. Real credentials are never in module.exports.
  module.exports = { endpoint: null, token: null, options: '{}', optsDigest: '', configDir: '', depTraceFile: '', depTraceRoot: '' };
} else if (parentPort) {
  // ── Worker thread (Issue 19) ────────────────────────────────────
  // In a worker created from a Mew-augmented parent:
  //   1. Extract credentials from workerData (injected by parent's
  //      credential-grabber monkey-patch).
  //   2. Register ts-loader for this worker's isolate.
  //   3. Strip credentials from workerData.
  //   4. Export nulls (credentials are in the loader's initialize hook).
  //
  // If workerData lacks credentials (worker created without Mew
  // augmentation, or user overrode execArgv), skip registration.

  var creds = null;
  try {
    if (workerData && typeof workerData === 'object') {
      creds = workerData[kMewCreds] || null;
      // Strip credentials from workerData so user code cannot read them.
      delete workerData[kMewCreds];
    }
  } catch (_) {
    // workerData might not be available; skip.
  }

  if (creds && creds.endpoint && creds.token) {
    var register;
    try { register = require('node:module').register; } catch (_) {}
    if (register) {
      try {
        const { pathToFileURL } = require('node:url');
        const path = require('node:path');
        const tsLoader = pathToFileURL(path.join(__dirname, 'ts-loader.mjs')).href;
        const parentURL = pathToFileURL(__filename).href;

        // In workers, MEW_USER_LOADERS from the parent process
        // is already deleted. Workers don't inherit parent
        // loaders; user loaders must be explicitly set up per
        // worker via the worker's own env/options.
        delete process.env.MEW_USER_LOADERS;

        register(tsLoader, parentURL, {
          parentURL,
          data: {
            endpoint: creds.endpoint,
            token: creds.token,
            options: creds.options || '{}',
            optsDigest: creds.optsDigest || '',
            configDir: creds.configDir || '',
            depTraceFile: creds.depTraceFile || '',
            depTraceRoot: creds.depTraceRoot || '',
          },
          transferList: [],
        });
      } catch (_) {
        // Registration failed — worker will get Node-native errors
        // for .ts imports (ERR_UNKNOWN_FILE_EXTENSION).
      }
    }
  }

  // Export nulls — credentials delivered via module.register() data.
  module.exports = { endpoint: null, token: null, options: '{}', optsDigest: '', configDir: '', depTraceFile: '', depTraceRoot: '' };
} else {
  // ── Loader context ──────────────────────────────────────────────
  // The loader thread re-evaluates --require modules. Credentials
  // were already delivered via the initialize hook; export nulls.
  module.exports = { endpoint: null, token: null, options: '{}', optsDigest: '', configDir: '', depTraceFile: '', depTraceRoot: '' };
}
