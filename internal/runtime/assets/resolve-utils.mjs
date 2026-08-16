// Mew resolution utilities — shared between ts-loader.mjs and resolve-diagnostic.mjs.
// Pure resolution logic with no dependency on the transform service, connection
// management, or Node loader hooks. Both the production loader hook and the
// diagnostic script use these functions so there is exactly one resolution algorithm.

import { accessSync, readFileSync } from 'node:fs';
import { resolve as pathResolve, parse as pathParse, join as pathJoin, dirname } from 'node:path';
import { createRequire } from 'node:module';

// ── Extension probe tables ─────────────────────────────────────────────

export const TS_EXT_PROBE = ['.ts', '.tsx', '.mts', '.cts'];
export const JS_EXT_PROBE = ['.js', '.jsx', '.mjs', '.cjs'];
export const ALL_EXT_PROBE = [...TS_EXT_PROBE, ...JS_EXT_PROBE];

// ── Module-level resolution state ──────────────────────────────────────

let resolveBaseDir = '';
let resolvePaths = null;
let resolvePathMappings = null;
let pathsParsed = false;

// packageTypeCache maps a package.json directory to its "type" value.
const packageTypeCache = new Map();

// pnpCache maps a project root (native path) to its loaded PnP API.
//   - {PnpApi} — loaded and usable
//   - null      — negative cache: project has no usable PnP API
const pnpCache = new Map();

// ── State initialization ───────────────────────────────────────────────

// initResolutionState sets up path-resolution state from normalized tsconfig options.
// Call once before using resolveViaPaths, resolveBaseDir, or ensurePnp.
//   opts.baseUrl     — tsconfig baseUrl (string or empty)
//   opts.paths       — raw paths map (object, for backward compat)
//   opts.pathMappings — canonical ordered path mappings (array of {pattern, targets})
//   opts.configDir   — directory containing tsconfig.json
export function initResolutionState(opts) {
  if (opts.pathMappings && Array.isArray(opts.pathMappings) && opts.pathMappings.length > 0) {
    resolvePathMappings = opts.pathMappings;
  } else if (opts.paths && typeof opts.paths === 'object' && Object.keys(opts.paths).length > 0) {
    resolvePaths = opts.paths;
  }
  resolveBaseDir = opts.configDir || '';
  if (opts.baseUrl && resolveBaseDir) {
    resolveBaseDir = pathResolve(resolveBaseDir, opts.baseUrl);
  }
  pathsParsed = true;
}

// resetResolutionState clears all module-level caches. Used in tests and
// when a diagnostic script needs a fresh state.
export function resetResolutionState() {
  resolveBaseDir = '';
  resolvePaths = null;
  resolvePathMappings = null;
  pathsParsed = false;
  packageTypeCache.clear();
  pnpCache.clear();
}

// getResolveBaseDir returns the current resolution base directory.
export function getResolveBaseDir() {
  return resolveBaseDir;
}

// ── Path helpers ───────────────────────────────────────────────────────

export function loaderFromPath(p) {
  if (p.endsWith('.mts')) return 'mts';
  if (p.endsWith('.cts')) return 'cts';
  if (p.endsWith('.tsx')) return 'tsx';
  return 'ts';
}

// ── Package type detection ──────────────────────────────────────────────

// getPackageType returns "module" or "commonjs" for a file at the given path,
// determined by the "type" field of the nearest package.json. Defaults to
// "commonjs" (Node default) when no package.json exists, no "type" is set,
// or the value is invalid/unreadable.
export function getPackageType(filePath) {
  const dir = dirname(pathResolve(filePath));
  const { root } = pathParse(dir);
  let scan = dir;

  while (true) {
    if (packageTypeCache.has(scan)) {
      return packageTypeCache.get(scan);
    }

    const pkgPath = pathJoin(scan, 'package.json');
    try {
      accessSync(pkgPath);
      let pkgType = 'commonjs'; // Node default
      try {
        const raw = readFileSync(pkgPath, 'utf8');
        const pkg = JSON.parse(raw);
        if (pkg.type === 'module') {
          pkgType = 'module';
        }
      } catch (_) {
        // Unreadable/malformed package.json: default to commonjs.
      }
      packageTypeCache.set(scan, pkgType);
      return pkgType;
    } catch (_) {}

    if (scan === root) break;
    const parent = dirname(scan);
    if (parent === scan) break;
    scan = parent;
  }

  return 'commonjs';
}

// ── Format determination ────────────────────────────────────────────────

// formatFromPath returns the Node module format for a file.
// .mts → esm, .cts → cjs, .ts/.tsx → nearest package.json "type".
export function formatFromPath(p) {
  if (p.endsWith('.mts')) return 'esm';
  if (p.endsWith('.cts')) return 'cjs';
  if (p.endsWith('.ts') || p.endsWith('.tsx')) {
    const pkgType = getPackageType(p);
    return pkgType === 'module' ? 'esm' : 'cjs';
  }
  return 'esm';
}

// formatFromResolvedPath returns the Node module format string for a resolved path.
// Returns 'module', 'commonjs', or undefined (let Node decide).
export function formatFromResolvedPath(p) {
  if (p.endsWith('.mjs') || p.endsWith('.mts')) return 'module';
  if (p.endsWith('.cjs') || p.endsWith('.cts')) return 'commonjs';
  if (p.endsWith('.js') || p.endsWith('.ts') || p.endsWith('.tsx')) {
    const pkgType = getPackageType(p);
    return pkgType === 'module' ? 'module' : 'commonjs';
  }
  return undefined; // let Node decide
}

// ── PnP resolution adapter ─────────────────────────────────────────────

// findProjectRoot walks up from dir looking for .pnp.cjs.
// Returns the directory containing .pnp.cjs, or null.
export function findProjectRoot(dir) {
  let current = pathResolve(dir);
  const { root } = pathParse(current);
  while (current !== root) {
    try {
      accessSync(pathJoin(current, '.pnp.cjs'));
      return current;
    } catch (_) {}
    const parent = dirname(current);
    if (parent === current) break;
    current = parent;
  }
  return null;
}

// ensurePnp returns the PnP API for the project containing parentPath,
// or null when no PnP environment owns that path.
export function ensurePnp(parentPath) {
  let dir = parentPath ? dirname(parentPath) : (resolveBaseDir || '');
  if (!dir) {
    try {
      dir = pathResolve('.');
    } catch (_) { return null; }
  }
  const root = findProjectRoot(dir);
  if (!root) return null;
  if (pnpCache.has(root)) return pnpCache.get(root);

  const pnpPath = pathJoin(root, '.pnp.cjs');
  try {
    accessSync(pnpPath);
  } catch (_) {
    pnpCache.set(root, null);
    return null;
  }

  let api = null;
  try {
    api = createRequire(import.meta.url)(pnpPath);
  } catch (_) { /* PnP bootstrap threw — cache as unusable */ }
  pnpCache.set(root, api || null);
  return api;
}

// ── Path pattern matching ──────────────────────────────────────────────

// matchPathPattern returns captured wildcard values, or null on no match.
// TypeScript paths patterns: "@app/*" matches "@app/helpers" → ["helpers"].
export function matchPathPattern(specifier, pattern) {
  if (!pattern.includes('*')) {
    if (specifier === pattern) return [''];
    return null;
  }
  const parts = pattern.split('*');
  if (parts.length === 2) {
    const [prefix, suffix] = parts;
    if (specifier.startsWith(prefix) && specifier.endsWith(suffix) &&
        specifier.length >= prefix.length + (suffix ? suffix.length : 0)) {
      const captured = specifier.slice(prefix.length, suffix ? specifier.length - suffix.length : specifier.length);
      return [captured];
    }
    return null;
  }
  // Multiple wildcards: sequential match.
  let remaining = specifier;
  const captures = [];
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i];
    if (i === 0) {
      if (!remaining.startsWith(part)) return null;
      remaining = remaining.slice(part.length);
    } else if (i === parts.length - 1) {
      if (part === '') { captures.push(remaining); break; }
      if (!remaining.endsWith(part)) return null;
      captures.push(remaining.slice(0, remaining.length - part.length));
    } else {
      const idx = remaining.indexOf(part);
      if (idx === -1) return null;
      captures.push(remaining.slice(0, idx));
      remaining = remaining.slice(idx + part.length);
    }
  }
  return captures;
}

// ── File resolution ────────────────────────────────────────────────────

// fileExists checks whether an absolute file path exists on disk.
export function fileExists(absPath) {
  try { accessSync(absPath); return true; } catch (_) { return false; }
}

// probeTypeScriptExtension checks whether a .js/.mjs/.cjs resolved path
// has a corresponding .ts/.tsx/.mts/.cts file that should be used instead.
export function probeTypeScriptExtension(resolvedPath) {
  const parsed = pathParse(resolvedPath);
  const jsExts = ['.js', '.jsx', '.mjs', '.cjs'];
  if (!jsExts.includes(parsed.ext)) return null;
  const baseName = pathJoin(parsed.dir, parsed.name);
  const probeExts = parsed.ext === '.mjs' ? ['.mts'] :
                    parsed.ext === '.cjs' ? ['.cts'] :
                    parsed.ext === '.jsx' ? ['.tsx'] :
                    ['.ts', '.tsx'];
  for (const ext of probeExts) {
    const candidate = baseName + ext;
    try { accessSync(candidate); return candidate; } catch (_) {}
  }
  return null;
}

// tryResolveFile attempts to resolve a base path to an existing file.
// Probes TypeScript extensions, JavaScript counterparts, and extensionless paths.
export function tryResolveFile(basePath) {
  // Exact path exists.
  try { accessSync(basePath); return basePath; } catch (_) {}

  const parsed = pathParse(basePath);

  // TypeScript extension already — don't probe further.
  if (parsed.ext && TS_EXT_PROBE.includes(parsed.ext)) {
    return null;
  }

  // JavaScript extension: the exact file doesn't exist. Try the TypeScript
  // counterpart before giving up (e.g. ./foo.js → ./foo.ts, ./foo.mjs → ./foo.mts).
  if (parsed.ext && JS_EXT_PROBE.includes(parsed.ext)) {
    const tsPath = probeTypeScriptExtension(basePath);
    if (tsPath) return tsPath;
    return null;
  }

  // No extension: probe with TS then JS extensions.
  for (const ext of ALL_EXT_PROBE) {
    const candidate = basePath + ext;
    try { accessSync(candidate); return candidate; } catch (_) {}
  }

  return null;
}

// ── tsconfig paths resolution ──────────────────────────────────────────

// resolveViaPaths resolves a specifier against tsconfig paths/baseUrl.
// Returns the resolved absolute path, or null.
export function resolveViaPaths(specifier, parentURL) {
  // Use canonical ordered mappings when available (deterministic specificity order).
  if (resolvePathMappings) {
    for (const { pattern, targets } of resolvePathMappings) {
      const captures = matchPathPattern(specifier, pattern);
      if (!captures) continue;
      for (const replacement of targets) {
        // Substitute captured values into replacement.
        let resolved = replacement;
        for (let i = 0; i < captures.length; i++) {
          resolved = resolved.replace('*', captures[i]);
        }
        // Resolve relative to baseUrl.
        const fullPath = resolveBaseDir ? pathResolve(resolveBaseDir, resolved) : pathResolve(resolved);
        const found = tryResolveFile(fullPath);
        if (found) return found;
      }
    }
    return null;
  }
  // Fallback: iterate paths object.
  if (!resolvePaths) return null;
  for (const [pattern, replacements] of Object.entries(resolvePaths)) {
    const captures = matchPathPattern(specifier, pattern);
    if (!captures) continue;
    for (const replacement of replacements) {
      let resolved = replacement;
      for (let i = 0; i < captures.length; i++) {
        resolved = resolved.replace('*', captures[i]);
      }
      const fullPath = resolveBaseDir ? pathResolve(resolveBaseDir, resolved) : pathResolve(resolved);
      const found = tryResolveFile(fullPath);
      if (found) return found;
    }
  }
  return null;
}

// ── Builtin detection ──────────────────────────────────────────────────

// isBuiltin returns true for Node.js built-in module specifiers.
export function isBuiltin(specifier) {
  if (specifier.startsWith('node:')) return true;
  // Node builtins (partial list covering the common ones).
  const builtins = new Set([
    'assert', 'buffer', 'child_process', 'cluster', 'console', 'constants',
    'crypto', 'dgram', 'dns', 'domain', 'events', 'fs', 'http', 'http2',
    'https', 'inspector', 'module', 'net', 'os', 'path', 'perf_hooks',
    'process', 'punycode', 'querystring', 'readline', 'repl', 'stream',
    'string_decoder', 'timers', 'tls', 'trace_events', 'tty', 'url',
    'util', 'v8', 'vm', 'worker_threads', 'zlib',
  ]);
  return builtins.has(specifier);
}
