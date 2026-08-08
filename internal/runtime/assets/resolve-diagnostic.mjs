// Mew resolve-module diagnostic helper.
// Invoked by `m resolve-module` to perform real module resolution through
// the same algorithm used by the runtime loader (resolve-utils.mjs).
//
// Input (environment variables):
//   MEW_DIAG_SPECIFIER  — module specifier to resolve (required)
//   MEW_DIAG_IMPORTER   — absolute path of importing file (optional; cwd used if absent)
//   MEW_DIAG_OPTIONS    — JSON blob with {baseUrl, paths, pathMappings, configDir}
//
// Output (stdout): single JSON line — see ResolveDiagnosticResult schema.
// Exit code: 0 on success (resolved or confidently unresolved), 1 on fatal error.

import { pathToFileURL, fileURLToPath } from 'node:url';
import { resolve as pathResolve, join as pathJoin, dirname } from 'node:path';
import { createRequire } from 'node:module';

import {
  initResolutionState,
  matchPathPattern,
  tryResolveFile,
  resolveViaPaths,
  probeTypeScriptExtension,
  formatFromResolvedPath,
  findProjectRoot,
  ensurePnp,
  fileExists,
  isBuiltin,
  TS_EXT_PROBE,
} from './resolve-utils.mjs';

// ── Schema version ────────────────────────────────────────────────────

const SCHEMA_VERSION = 1;

// ── Helpers ───────────────────────────────────────────────────────────

function makeTraceStep(stage, outcome, extra) {
  return { stage, outcome, ...extra };
}

// safeImportResolve calls Node's native module resolution.
// Returns the resolved URL string or throws.
function safeImportResolve(specifier, parentURL) {
  // For relative/absolute specifiers, resolve syntactically via URL constructor.
  // Node's import.meta.resolve ignores the parent argument and resolves relative
  // to cwd, so we handle relative resolution ourselves then verify with Node.
  const isRelative = specifier.startsWith('.') || specifier.startsWith('/');

  if (isRelative && parentURL) {
    // Syntactic resolution: ./foo relative to parent URL.
    const resolved = new URL(specifier, parentURL).href;
    // Verify the resolved path exists via Node's import.meta.resolve (bare call).
    // This catches cases like package.json exports that redirect paths.
    if (typeof import.meta.resolve === 'function') {
      try {
        return import.meta.resolve(resolved);
      } catch (_) {
        // Node can't resolve it — try direct file access.
        return resolved;
      }
    }
    return resolved;
  }

  // For bare specifiers: use Node's native resolution.
  if (typeof import.meta.resolve === 'function') {
    return import.meta.resolve(specifier);
  }

  // Fallback for older Node: use createRequire + require.resolve (CJS semantics).
  const parentPath = parentURL ? fileURLToPath(parentURL) : process.cwd();
  const req = createRequire(pathToFileURL(pathJoin(dirname(parentPath), '_fallback_.cjs')));
  try {
    return pathToFileURL(req.resolve(specifier)).href;
  } catch (err) {
    const e = new Error(`Cannot find module '${specifier}'`);
    e.code = 'ERR_MODULE_NOT_FOUND';
    throw e;
  }
}

// ── Main resolution ──────────────────────────────────────────────────

async function resolveModule(specifier, importerPath, opts) {
  const trace = [];

  // Initialize resolution state from diagnostic options.
  if (opts) {
    initResolutionState(opts);
  }

  // Determine the effective importer (file:// URL for Node APIs).
  let importerURL;
  try {
    importerURL = importerPath ? pathToFileURL(importerPath).href : pathToFileURL(pathResolve('.') + '/_diagnostic_.mjs').href;
  } catch (_) {
    importerURL = pathToFileURL(pathResolve('.') + '/_diagnostic_.mjs').href;
  }

  // Stage 1: builtins
  if (isBuiltin(specifier) || specifier.startsWith('node:')) {
    trace.push(makeTraceStep('builtins', 'resolved', {
      specifier,
      url: specifier.startsWith('node:') ? specifier : `node:${specifier}`,
    }));
    return {
      schemaVersion: SCHEMA_VERSION,
      specifier,
      importer: importerPath || process.cwd(),
      resolved: true,
      target: {
        url: specifier.startsWith('node:') ? specifier : `node:${specifier}`,
        format: 'builtin',
      },
      trace,
    };
  }
  trace.push(makeTraceStep('builtins', 'skipped'));

  // Stage 2: URL and data: specifiers — pass through (Node handles them).
  if (specifier.startsWith('data:') || specifier.startsWith('file:') || specifier.startsWith('http:') || specifier.startsWith('https:')) {
    trace.push(makeTraceStep('url', 'resolved', { specifier, url: specifier }));
    return {
      schemaVersion: SCHEMA_VERSION,
      specifier,
      importer: importerPath || process.cwd(),
      resolved: true,
      target: { url: specifier, format: undefined },
      trace,
    };
  }

  // Stage 3: Node native resolution.
  let nodeResolved = null;
  let nodeError = null;
  try {
    nodeResolved = safeImportResolve(specifier, importerURL);
  } catch (err) {
    nodeError = err;
  }

  if (nodeResolved) {
    // Node resolved successfully. Check for TS extension substitution.
    const url = new URL(nodeResolved);
    if (url.protocol === 'file:') {
      const absPath = fileURLToPath(url);
      // TypeScript file: mark format.
      if (TS_EXT_PROBE.some(ext => absPath.endsWith(ext))) {
        trace.push(makeTraceStep('node-native', 'resolved', {
          resolved: absPath,
          format: formatFromResolvedPath(absPath),
          note: 'typescript file',
        }));
        return {
          schemaVersion: SCHEMA_VERSION,
          specifier,
          importer: importerPath || process.cwd(),
          resolved: true,
          target: {
            url: nodeResolved,
            path: absPath,
            format: formatFromResolvedPath(absPath),
          },
          trace,
        };
      }
      // JS file: probe for .ts extension substitution.
      if (!fileExists(absPath)) {
        const tsPath = probeTypeScriptExtension(absPath);
        if (tsPath) {
          trace.push(makeTraceStep('node-native', 'resolved', {
            resolved: absPath,
            substituted: tsPath,
            format: formatFromResolvedPath(tsPath),
            note: '.js→.ts extension substitution',
          }));
          return {
            schemaVersion: SCHEMA_VERSION,
            specifier,
            importer: importerPath || process.cwd(),
            resolved: true,
            target: {
              url: pathToFileURL(tsPath).href,
              path: tsPath,
              format: formatFromResolvedPath(tsPath),
            },
            trace,
          };
        }
        // Resolved .js file doesn't exist and no .ts variant.
        // Node resolved it (e.g. through exports map) but the file is missing.
        // This is a valid resolution that will fail at load time.
        trace.push(makeTraceStep('node-native', 'resolved', {
          resolved: absPath,
          format: formatFromResolvedPath(absPath),
          note: 'resolved file not on disk (may be virtual or deferred)',
        }));
        return {
          schemaVersion: SCHEMA_VERSION,
          specifier,
          importer: importerPath || process.cwd(),
          resolved: true,
          target: {
            url: nodeResolved,
            path: absPath,
            format: formatFromResolvedPath(absPath),
          },
          trace,
        };
      }
      // File exists — use Node's result.
      trace.push(makeTraceStep('node-native', 'resolved', {
        resolved: absPath,
        format: formatFromResolvedPath(absPath),
      }));
      return {
        schemaVersion: SCHEMA_VERSION,
        specifier,
        importer: importerPath || process.cwd(),
        resolved: true,
        target: {
          url: nodeResolved,
          path: absPath,
          format: formatFromResolvedPath(absPath),
        },
        trace,
      };
    }
    // Non-file URL (e.g. data:, https:) — pass through.
    trace.push(makeTraceStep('node-native', 'resolved', { url: nodeResolved }));
    return {
      schemaVersion: SCHEMA_VERSION,
      specifier,
      importer: importerPath || process.cwd(),
      resolved: true,
      target: { url: nodeResolved },
      trace,
    };
  }

  // Node native failed.
  const nodeErrCode = nodeError?.code || 'ERR_MODULE_NOT_FOUND';
  trace.push(makeTraceStep('node-native', 'miss', { error: nodeErrCode }));

  // Stage 4: Local extension probing for relative/absolute/plain specifiers.
  if (specifier.startsWith('.') || specifier.startsWith('/') || (!specifier.includes(':') && !specifier.startsWith('@'))) {
    const parentPath = importerPath || pathResolve('.');
    const parentDir = dirname(parentPath);
    const candidatePath = specifier.startsWith('.')
      ? pathResolve(parentDir, specifier)
      : (specifier.startsWith('/') ? specifier : null);
    if (candidatePath) {
      const found = tryResolveFile(candidatePath);
      if (found) {
        trace.push(makeTraceStep('extension-probe', 'resolved', {
          candidate: candidatePath,
          resolved: found,
          format: formatFromResolvedPath(found),
        }));
        return {
          schemaVersion: SCHEMA_VERSION,
          specifier,
          importer: importerPath || process.cwd(),
          resolved: true,
          target: {
            url: pathToFileURL(found).href,
            path: found,
            format: formatFromResolvedPath(found),
          },
          trace,
        };
      }
    }
    trace.push(makeTraceStep('extension-probe', 'miss', { candidate: candidatePath }));
  } else {
    trace.push(makeTraceStep('extension-probe', 'skipped', { reason: 'not a relative or absolute specifier' }));
  }

  // Stage 5: PnP resolution for bare specifiers.
  if (!specifier.startsWith('.') && !specifier.startsWith('/') && !specifier.includes(':')) {
    const issuerPath = importerPath || null;
    const pnpApi = ensurePnp(issuerPath);
    if (pnpApi && typeof pnpApi.resolveRequest === 'function') {
      const issuer = issuerPath || '/';
      try {
        const pnpResult = pnpApi.resolveRequest(specifier, issuer);
        if (pnpResult) {
          trace.push(makeTraceStep('pnp', 'resolved', {
            resolved: pnpResult,
            format: formatFromResolvedPath(pnpResult),
            pnpRoot: findProjectRoot(issuer),
          }));
          return {
            schemaVersion: SCHEMA_VERSION,
            specifier,
            importer: importerPath || process.cwd(),
            resolved: true,
            target: {
              url: pathToFileURL(pnpResult).href,
              path: pnpResult,
              format: formatFromResolvedPath(pnpResult),
            },
            pnp: { root: findProjectRoot(issuer) },
            trace,
          };
        }
        trace.push(makeTraceStep('pnp', 'miss', { reason: 'not in PnP map' }));
      } catch (err) {
        // PnP boundary violation — this IS the final error.
        trace.push(makeTraceStep('pnp', 'error', {
          error: err.message,
          code: err.code || 'ERR_PNP_UNDECLARED_DEPENDENCY',
        }));
        return {
          schemaVersion: SCHEMA_VERSION,
          specifier,
          importer: importerPath || process.cwd(),
          resolved: false,
          target: null,
          error: {
            code: err.code || 'ERR_PNP_UNDECLARED_DEPENDENCY',
            message: err.message,
            stage: 'pnp',
          },
          pnp: { root: findProjectRoot(issuer) },
          trace,
        };
      }
    } else {
      trace.push(makeTraceStep('pnp', 'skipped', { reason: 'no .pnp.cjs found' }));
    }
  } else {
    trace.push(makeTraceStep('pnp', 'skipped', { reason: 'not a bare specifier' }));
  }

  // Stage 6: tsconfig paths.
  const pathsResolved = resolveViaPaths(specifier, importerURL);
  if (pathsResolved) {
    // Determine which pattern matched.
    let matchedPattern = null;
    try {
      const opts = JSON.parse(process.env.MEW_DIAG_OPTIONS || '{}');
      if (opts.pathMappings) {
        for (const { pattern, targets } of opts.pathMappings) {
          if (matchPathPattern(specifier, pattern)) {
            matchedPattern = { pattern, targets };
            break;
          }
        }
      }
    } catch (_) {}

    trace.push(makeTraceStep('tsconfig-paths', 'resolved', {
      resolved: pathsResolved,
      format: formatFromResolvedPath(pathsResolved),
      pattern: matchedPattern?.pattern,
      targets: matchedPattern?.targets,
    }));
    return {
      schemaVersion: SCHEMA_VERSION,
      specifier,
      importer: importerPath || process.cwd(),
      resolved: true,
      target: {
        url: pathToFileURL(pathsResolved).href,
        path: pathsResolved,
        format: formatFromResolvedPath(pathsResolved),
      },
      trace,
    };
  }
  trace.push(makeTraceStep('tsconfig-paths', 'miss'));

  // All stages failed — resolution unsuccessful.
  return {
    schemaVersion: SCHEMA_VERSION,
    specifier,
    importer: importerPath || process.cwd(),
    resolved: false,
    target: null,
    error: {
      code: nodeErrCode,
      message: nodeError?.message || `Cannot find module '${specifier}'`,
      stage: 'node-native',
    },
    trace,
  };
}

// ── Entry point ──────────────────────────────────────────────────────

const specifier = process.env.MEW_DIAG_SPECIFIER;
if (!specifier) {
  console.error('MEW_DIAG_SPECIFIER is required');
  process.exit(1);
}

const importer = process.env.MEW_DIAG_IMPORTER || '';
let opts = {};
try {
  opts = JSON.parse(process.env.MEW_DIAG_OPTIONS || '{}');
} catch (_) {}

resolveModule(specifier, importer, opts).then(result => {
  process.stdout.write(JSON.stringify(result) + '\n');
  process.exit(result.resolved ? 0 : 1);
}).catch(err => {
  const result = {
    schemaVersion: SCHEMA_VERSION,
    specifier,
    importer: importer || process.cwd(),
    resolved: false,
    target: null,
    error: {
      code: err.code || 'ERR_M_INTERNAL',
      message: err.message || 'diagnostic resolution failed',
      stage: 'diagnostic',
    },
    trace: [makeTraceStep('diagnostic', 'error', { error: err.message, code: err.code })],
  };
  process.stdout.write(JSON.stringify(result) + '\n');
  process.exit(1);
});
