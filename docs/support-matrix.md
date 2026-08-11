# Runtime Support Matrix

Certified capability surface for MewJS runtime augmentation (0050–0057).
Last updated: 2026-08-12 (0057 stabilization gate).

## Module Systems

| Feature | Status | Evidence | Notes |
|---|---|---|---|
| CommonJS (`.cjs`, `"type": "commonjs"`) | Certified | `runtime-syntax`, `runtime-resolution` | Preload injection via `--require` |
| ES modules (`.mjs`, `"type": "module"`) | Certified | `runtime-syntax`, `runtime-resolution` | Preload injection via `--import` |
| Mixed CJS/ESM projects | Certified | `runtime-syntax` | `package.json` `"type"` field honored |
| Dynamic `import()` | Certified | `runtime-syntax` | Works in both CJS and ESM contexts |

## TypeScript & JSX

| Feature | Status | Evidence | Notes |
|---|---|---|---|
| TypeScript (`.ts`) | Certified | `runtime-syntax`, `runtime-cache` | esbuild transform; ESM/CJS output |
| TSX (`.tsx`) | Certified | `runtime-syntax` | JSX transform; automatic runtime |
| MTS (`.mts`) | Certified | `runtime-resolution` | Forced ESM output |
| CTS (`.cts`) | Certified | `runtime-resolution` | Forced CJS output |
| JSX (`.jsx`) | Partial | `runtime-syntax` | Via extension substitution only (`.jsx` → `.tsx` loader) |
| Decorators (legacy + TC39) | Certified | `runtime-syntax` | Legacy via `experimentalDecorators`; TC39 stage 3 via esbuild |
| `emitDecoratorMetadata` | Unsupported | `docs/runtime/known-limitations.md` | Rejected with `ERR_M_TRANSFORM_UNSUPPORTED` |
| Type checking | Out of scope | `docs/runtime/known-limitations.md` | Transpile-only; use `tsc --noEmit` |

## Module Resolution

| Feature | Status | Evidence | Notes |
|---|---|---|---|
| Standard Node resolution | Certified | `runtime-resolution` | `node_modules` traversal |
| tsconfig paths | Certified | `runtime-resolution` | `compilerOptions.paths` + `baseUrl` |
| Custom ESM loaders (`--loader`) | Certified | `runtime-resolution` | Via `module.register()` |
| PnP resolution (`.pnp.cjs`) | Certified | `runtime-resolution` | Yarn PnP API integration |
| PnP unplugged mode | Unsupported | `docs/runtime/known-limitations.md` | Requires `.pnp.cjs`, not `.pnp.data.json` alone |
| Module format detection (NodeNext/Node16) | Certified | `runtime-resolution` | Per-file format from `package.json` `"type"` |

## Workers & Child Processes

| Feature | Status | Evidence | Notes |
|---|---|---|---|
| Worker threads | Certified | `runtime-workers` | Preload chain inheritance |
| `worker_threads` isolation | Certified | `runtime-workers` | Credential boundaries enforced |
| `child_process.fork()` | Certified | `runtime-workers` | Transform capabilities inherited |
| `child_process.spawn()` | Certified | `runtime-workers` | Plain Node escape hatch available |
| Worker-specific tsconfig | Deferred | `docs/runtime/known-limitations.md` | Planned for 0060+ |
| Custom loader propagation to workers | Deferred | `docs/runtime/known-limitations.md` | Workers must register loaders explicitly |

## Watch Mode

| Feature | Status | Evidence | Notes |
|---|---|---|---|
| File-watch restart | Certified | `runtime-watch` | fsnotify + 500ms polling fallback |
| Dependency-aware restart | Certified | `runtime-watch` | Watches `node_modules` and tsconfig |
| Graceful shutdown | Certified | `runtime-watch` | SIGTERM → child cleanup → restart |
| Process leak prevention | Certified | `runtime-watch`, `runtime-workers` | No orphaned processes after restart cycles |
| NFS/Docker polling fallback | Documented | `docs/runtime/known-limitations.md` | 500ms polling latency |

## Web Storage

| Feature | Status | Evidence | Notes |
|---|---|---|---|
| `localStorage` (persistent) | Certified | `runtime-storage` | Per-project namespace, file-backed |
| `sessionStorage` (per-realm) | Certified | `runtime-storage` | In-memory, non-persistent |
| Cross-process locking | Certified | `runtime-storage` | Directory-based lock with heartbeat |
| Quota enforcement | Certified | `runtime-storage` | 5 MiB default; `MEW_STORAGE_QUOTA_BYTES` |
| `StorageEvent` API | Unsupported | `docs/runtime/known-limitations.md` | Not planned |
| Property-style access | Unsupported | `docs/runtime/known-limitations.md` | Use `getItem`/`setItem` only |

## Diagnostics & Source Maps

| Feature | Status | Evidence | Notes |
|---|---|---|---|
| Inline source maps | Certified | `runtime-cache`, `runtime-diagnostics` | Embedded in transformed output |
| External source maps | Certified | `runtime-cache` | Written to transform cache only |
| `--enable-source-maps` | Certified | `runtime-diagnostics` | Auto-injected for Node >= 20.6 |
| `--inspect` / `--inspect-brk` | Certified | `runtime-diagnostics` | Passthrough to V8 inspector |
| Trace events | Certified | `runtime-diagnostics` | Schema v1; resolve/transform spans |
| Doctor diagnostics | Certified | `runtime-diagnostics` | `m doctor` with runtime probes |
| Cache explain | Certified | `runtime-diagnostics` | `m cache explain` for transform cache |
| Support bundle | Certified | `runtime-diagnostics` | Aggregated diagnostics export |

## Node Version Support

| Node Version | Status | Evidence | Capability Notes |
|---|---|---|---|
| 24.x | Certified | CI: 3 OS × Node 24 | Full capabilities |
| 22.x (LTS) | Certified | CI: 3 OS × Node 22 | Full capabilities; primary target |
| 20.x (LTS) | Certified | CI: 3 OS × Node 20 | Full capabilities |
| 18.x (maintenance) | Certified | CI: 3 OS × Node 18 | Requires >= 18.19 for `module.register()` |
| < 18.x | Unsupported | — | Minimum supported Node is 18.x |

## Operating System Support

| OS | Status | Evidence |
|---|---|---|
| Linux (amd64) | Certified | CI: ubuntu-latest × 4 Node versions |
| macOS (amd64/arm64) | Certified | CI: macos-latest × 4 Node versions |
| Windows (amd64) | Certified | CI: windows-latest × 4 Node versions |

## Certification Matrix

Full certification: 3 OS × 4 Node versions = 12 cells, exact-head SHA binding, fail-closed aggregation.
See `.github/workflows/runtime-cert.yml` for the canonical certification contract.

Runtime conformance suites (9 required, all multi-platform):

| Suite ID | Coverage |
|---|---|
| `runtime-syntax` | TS, TSX, MTS, CTS, JS, MJS, CJS, imports |
| `runtime-resolution` | Loaders, PnP, tsconfig paths, module format |
| `runtime-env` | Environment loading, precedence, expansion |
| `runtime-workers` | Worker isolation, credentials, child processes |
| `runtime-storage` | localStorage, sessionStorage |
| `runtime-watch` | Watch start, restart, shutdown |
| `runtime-diagnostics` | Trace, doctor, cache explain, support, inspector, opt-out |
| `runtime-cache` | Transform cache round-trip, key stability, source maps |
| `runtime-failure` | Syntax errors, missing modules, exit codes |

## Gated Features

Features behind experimental flags as of 0057:

| Feature | Gate |
|---|---|
| Runtime augmentation | `MEW_EXPERIMENTAL_RUNTIME=1` |
| Direct dispatch bins | `MEW_EXPERIMENTAL_EXEC_DIRECT_DISPATCH=1` |
| Watch mode | `MEW_EXPERIMENTAL_RUNTIME=1` |
| Web Storage | `MEW_EXPERIMENTAL_RUNTIME=1` |
| Debug inspection | `--inspect`/`--inspect-brk` (passthrough) |
| Remote inspector | `MEW_EXPERIMENTAL_REMOTE_INSPECTOR=1` |

## Evidence References

- Certification workflow: `.github/workflows/runtime-cert.yml`
- Conformance manifest: `tests/conformance/runtime-matrix/manifest.json`
- Conformance tests: `tests/conformance/runtime/*_test.go`
- Known limitations: `docs/runtime/known-limitations.md`
- Protocol versions: `docs/runtime/protocol-versions.md`
- Schema freeze: `docs/schema-freeze.md`
- Compatibility axes: `docs/compatibility-axes.md`
- Feature inventory: `features/inventory.json`
