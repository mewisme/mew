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
//   Main thread monkey-patches the Worker constructor to inject
//   credentials into workerData via a Symbol key. When this module
//   re-evaluates in a worker (isMainThread=false, parentPort exists),
//   it extracts credentials from workerData and registers ts-loader
//   for the worker. The loader-thread branch (no parentPort) is
//   unchanged.
//
// Issue 39 — Child process propagation:
//   Main thread also monkey-patches child_process (fork, spawn,
//   execFile) to inject credentials into child Node processes. The
//   same env-based transport is used: MEW_TRANSFORM_* vars are placed
//   in the child's environment and credential-grabber is injected as
//   the first --require, so it captures+strips them before user code.
//   Non-Node children are not augmented. exec (shell-mediated) is
//   not augmented.
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

  // ── Child process augmentation (Issue 39) ────────────────────────
  // Inject Mew runtime support into child Node processes so they can
  // import/execute TypeScript through the same transform session.
  //
  // Same security model as the parent process: credentials are placed
  // in the child's environment, and the child's credential-grabber
  // (injected as the first --require) captures and strips them before
  // any user code executes.
  //
  // Supported child APIs:
  //   fork          — always a Node child; augment execArgv + env.
  //   spawn/execFile — augment when command is the current Node exe.
  //   exec          — not augmented (shell-mediated, unreliable).
  //
  // Non-Node children are not augmented. Duplicate injection is
  // prevented by checking whether cred-grabber is already present
  // in the child's execArgv/args. Credentials are always injected
  // into the env (they are absent from the parent's stripped env).
  if (_mewCredentials) {
    try {
      var cp = require('node:child_process');
      var nodePath = require('node:path');
      var nodeURL = require('node:url');

      var credGrabberPath = __filename;
      var parentHasSourceMaps = process.execArgv.indexOf('--enable-source-maps') !== -1;

      // _mewIsCurrentNode reports whether cmd resolves to process.execPath.
      function _mewIsCurrentNode(cmd) {
        if (!cmd) return false;
        if (cmd === process.execPath) return true;
        try { return nodePath.resolve(cmd) === nodePath.resolve(process.execPath); } catch (_) { return false; }
      }

      // _mewHasCredGrabber reports whether cred-grabber is already in args.
      function _mewHasCredGrabber(args) {
        if (!args) return false;
        for (var i = 0; i < args.length; i++) {
          if ((args[i] === '--require' || args[i] === '-r') && args[i + 1] === credGrabberPath) return true;
        }
        return false;
      }

      // _mewStripUnsafeNodeOptions removes flags from NODE_OPTIONS that
      // would execute before credential isolation (Issue 33 policy).
      var _mewUnsafeFlags = { '--require': true, '--import': true, '--loader': true, '--experimental-loader': true };
      var _mewUnsafeFlagCount = Object.keys(_mewUnsafeFlags).length;
      function _mewStripUnsafeNodeOptions(val) {
        if (!val || typeof val !== 'string') return '';
        // Tokenize on whitespace, respecting single and double quotes.
        var toks = val.match(/"[^"]*"|'[^']*'|\S+/g);
        if (!toks) return '';
        var out = [];
        var skipNext = false;
        for (var i = 0; i < toks.length; i++) {
          var t = toks[i];
          if (skipNext) { skipNext = false; continue; }
          // Equals form: --require=foo
          if (/^--(require|import|loader|experimental-loader)=/.test(t)) continue;
          // Flags that consume the next token as a value.
          if (_mewUnsafeFlags[t]) { skipNext = true; continue; }
          // -r and -rFOO forms (alias for --require).
          if (/^-r/.test(t)) continue;
          out.push(t);
        }
        return out.join(' ');
      }

      // _mewAugmentEnv builds a child env object from the user's env
      // (or process.env as default). MEW_TRANSFORM_* credentials from
      // the closure are injected. User-supplied MEW_TRANSFORM_* keys
      // are removed (case-insensitively) to prevent injection attacks.
      // Unsafe NODE_OPTIONS flags are stripped.
      function _mewAugmentEnv(userEnv) {
        var src = userEnv || process.env;
        var keys = Object.keys(src);
        var e = {};
        for (var i = 0; i < keys.length; i++) {
          var k = keys[i];
          // Skip user-supplied MEW_TRANSFORM_* (case-insensitive).
          // The real values from _mewCredentials replace them below.
          if (k.length >= 15) {
            var uk = k.toUpperCase();
            if (uk.indexOf('MEW_TRANSFORM_') === 0) continue;
          }
          if (k === 'NODE_OPTIONS' || (k.length === 12 && k.toUpperCase() === 'NODE_OPTIONS')) {
            var stripped = _mewStripUnsafeNodeOptions(src[k]);
            if (stripped) e[k] = stripped;
            continue;
          }
          e[k] = src[k];
        }
        e.MEW_TRANSFORM_ENDPOINT = _mewCredentials.endpoint;
        e.MEW_TRANSFORM_TOKEN = _mewCredentials.token;
        e.MEW_TRANSFORM_OPTIONS = _mewCredentials.options;
        e.MEW_TRANSFORM_OPTS_DIGEST = _mewCredentials.optsDigest;
        e.MEW_TRANSFORM_CONFIG_DIR = _mewCredentials.configDir;
        if (_mewCredentials.depTraceFile) {
          e.MEW_TRANSFORM_DEP_TRACE_FILE = _mewCredentials.depTraceFile;
        }
        if (_mewCredentials.depTraceRoot) {
          e.MEW_TRANSFORM_DEP_TRACE_ROOT = _mewCredentials.depTraceRoot;
        }
        return e;
      }

      // ── fork ──────────────────────────────────────────────────────
      // fork always launches the current Node executable. Augment
      // execArgv to inject credential-grabber before user flags,
      // and inject credentials into the child environment.
      var OrigFork = cp.fork;
      if (!OrigFork.__mewPatched) {
        cp.fork = function MewFork(modulePath, args, options) {
          // fork(modulePath, options) — args is optional.
          if (args && !Array.isArray(args)) {
            options = args;
            args = [];
          }
          if (!options) options = {};
          if (!Array.isArray(args)) args = [];

          var execArgv = options.execArgv || process.execArgv;
          if (!_mewHasCredGrabber(execArgv)) {
            var augmented = [];
            if (parentHasSourceMaps && execArgv.indexOf('--enable-source-maps') === -1) {
              augmented.push('--enable-source-maps');
            }
            augmented.push('--require', credGrabberPath);
            for (var i = 0; i < execArgv.length; i++) augmented.push(execArgv[i]);
            execArgv = augmented;
          }
          options.execArgv = execArgv;
          options.env = _mewAugmentEnv(options.env);
          return OrigFork.call(this, modulePath, args, options);
        };
        cp.fork.prototype = OrigFork.prototype;
        cp.fork.__mewPatched = true;
        OrigFork.__mewPatched = true;
      }

      // ── spawn ─────────────────────────────────────────────────────
      var OrigSpawn = cp.spawn;
      if (!OrigSpawn.__mewPatched) {
        cp.spawn = function MewSpawn(cmd, args, options) {
          if (!args) args = [];
          if (!options) options = {};
          var isNode = _mewIsCurrentNode(cmd);
          if (isNode && !_mewHasCredGrabber(args)) {
            var augmented = [];
            if (parentHasSourceMaps && args.indexOf('--enable-source-maps') === -1) {
              augmented.push('--enable-source-maps');
            }
            augmented.push('--require', credGrabberPath);
            for (var i = 0; i < args.length; i++) augmented.push(args[i]);
            args = augmented;
          }
          if (isNode) {
            options.env = _mewAugmentEnv(options.env);
          } else if (options.env) {
            // Non-Node child with explicit env: strip any MEW_TRANSFORM_*
            // the user may have injected (defense in depth).
            var cleanEnv = {};
            var cleanKeys = Object.keys(options.env);
            for (var ci = 0; ci < cleanKeys.length; ci++) {
              var ck = cleanKeys[ci];
              if (ck.length >= 15) {
                var cuk = ck.toUpperCase();
                if (cuk.indexOf('MEW_TRANSFORM_') === 0) continue;
              }
              cleanEnv[ck] = options.env[ck];
            }
            options.env = cleanEnv;
          }
          return OrigSpawn.call(this, cmd, args, options);
        };
        cp.spawn.prototype = OrigSpawn.prototype;
        cp.spawn.__mewPatched = true;
        OrigSpawn.__mewPatched = true;
      }

      // ── execFile ──────────────────────────────────────────────────
      var OrigExecFile = cp.execFile;
      if (!OrigExecFile.__mewPatched) {
        cp.execFile = function MewExecFile(file, args, options, callback) {
          if (!args) args = [];
          if (!options) options = {};
          var isNode = _mewIsCurrentNode(file);
          if (isNode && !_mewHasCredGrabber(args)) {
            var augmented = [];
            if (parentHasSourceMaps && args.indexOf('--enable-source-maps') === -1) {
              augmented.push('--enable-source-maps');
            }
            augmented.push('--require', credGrabberPath);
            for (var i = 0; i < args.length; i++) augmented.push(args[i]);
            args = augmented;
          }
          if (isNode) {
            options.env = _mewAugmentEnv(options.env);
          } else if (options.env) {
            var cleanEnv = {};
            var cleanKeys = Object.keys(options.env);
            for (var ci = 0; ci < cleanKeys.length; ci++) {
              var ck = cleanKeys[ci];
              if (ck.length >= 15) {
                var cuk = ck.toUpperCase();
                if (cuk.indexOf('MEW_TRANSFORM_') === 0) continue;
              }
              cleanEnv[ck] = options.env[ck];
            }
            options.env = cleanEnv;
          }
          if (callback !== undefined) {
            return OrigExecFile.call(this, file, args, options, callback);
          }
          return OrigExecFile.call(this, file, args, options);
        };
        cp.execFile.prototype = OrigExecFile.prototype;
        cp.execFile.__mewPatched = true;
        OrigExecFile.__mewPatched = true;
      }
    } catch (_) {
      // child_process may be unavailable in some contexts.
      // Child propagation is best-effort; child import of .ts
      // files will fail with a clear Node error instead of
      // silently bypassing the transform.
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
