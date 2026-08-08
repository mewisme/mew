# Nub crate to Mew package map

Nub is a behavioral reference, not a source-level port. Preserve observable
semantics where parity is intentional; use idiomatic Go architecture elsewhere.

| Nub component | Mew target | Responsibility |
|---|---|---|
| `crates/nub-cli` | `cmd/m`, `cmd/mx`, `internal/app`, `internal/cli` | Argument parsing, dispatch, presentation, initialization, consent, and command orchestration. |
| `crates/nub-core` | `internal/project`, `internal/workspace`, `internal/process`, `internal/node`, `internal/runtime` | Project discovery, scripts, process control, Node provisioning, runtime asset management. |
| `vendor/aube/crates/aube-manifest` | `internal/manifest` | Manifest parsing, normalization, and non-destructive edits. |
| `vendor/aube/crates/aube-registry` | `internal/registry` | Registry configuration, authentication, metadata, and tarball access. |
| `vendor/aube/crates/aube-resolver` | `internal/resolver` | Semver, graph expansion, peers, optional dependencies, overrides, and decisions. |
| `vendor/aube/crates/aube-lockfile` | `internal/lockfile` plus adapters | Canonical graph conversion and lockfile compatibility. |
| `vendor/aube/crates/aube-store` | `internal/store` | Immutable content store, cache, leases, and garbage collection. |
| `vendor/aube/crates/aube-linker` | `internal/linker` | Hoisted and isolated layouts, bins, links, and filesystem planning. |
| `vendor/aube/crates/aube-scripts` | `internal/lifecycle` | Lifecycle discovery, trust, sandboxing, and build outputs. |
| `crates/nub-native` | `internal/transform` plus embedded loader bridge | Replace native OXC addon with evaluated Go transform pipeline and versioned IPC/loader protocol. |
| `runtime/*.mjs` and `runtime/*.cjs` | `internal/runtime/assets` embedded with go:embed | Rewrite and minimize Node-side hooks, preloads, PnP helpers, workers, and storage assets. |
| `crates/nub-phantom-*` | `internal/analysis/phantom` | Optional parser-backed phantom-dependency analysis after core layout stabilizes. |
| `install.sh`, `install.ps1`, npm packages, Docker, Actions | `release/`, `install/`, `.github/actions/`, `docker/` | Reproducible signed distribution and CI integration. |

## Intentional omissions / divergence

| Topic | Classification | Notes |
|---|---|---|
| Stock-Node augmentation | parity | Same product model as Nub |
| OXC native addon | divergence | Go transform + optional IPC instead of N-API OXC |
| Direct `m <script>` shortcuts | extension | Mew product differentiator (charter) |
| JSX transforms | parity (0052) | classic + automatic runtimes, jsxFactory/jsxFragmentFactory/jsxImportSource |
| Decorators | parity (0052) | legacy TS decorators via esbuild; emitDecoratorMetadata carried for cache keys; standard TC39 decorators deferred to esbuild upstream |
| Source maps | parity (0052) | inline/external via esbuild; sourceMap/inlineSourceMap/inlineSources tsconfig flags; sourceRoot passthrough; mapRoot cached; --enable-source-maps auto-injected (Node >= 20.6) |
| TypeScript type-checking | divergence | Mew is a transpiler not a type checker; no semantic diagnostics |
| Decorator metadata emission | divergence | emitDecoratorMetadata explicitly rejected during tsconfig normalization (ERR_M_TRANSFORM_UNSUPPORTED diagnostic) |
| tsconfig paths/baseUrl resolution | parity (0052/0053) | Deterministic specificity-ordered path pattern matching in Node loader resolve hook; exact before wildcard, longest prefix wins; Node fallback errors preserved; remaining 0053 work: directory/index resolution |
| .js → .ts extension mapping | parity (0053) | Implemented: .js→.ts/.tsx, .jsx→.tsx, .mjs→.mts, .cjs→.cts with deterministic candidate ordering; existing JS files take precedence; loader active for all entrypoints |
| Custom ESM loader chaining | parity (0053) | `--loader` flag registers via `module.register()`; user hooks outermost, ts-loader innermost |
| PnP runtime adapter | parity (0053) | .pnp.cjs detection + resolveRequest integration in ts-loader resolve hook |
| resolve-module command | extension (0053) | Go-side diagnostic showing tsconfig paths, baseUrl, pattern matches |
| .env auto-discovery | parity (0054) | Mode-aware .env* loading with precedence: .env.[mode].local > .env.[mode] > .env.local > .env |
| .env variable expansion | parity (0054) | ${VAR}, $VAR, ${VAR:-default} expansion in .env files; double/single-quoted values |
| --env-file / --no-env-file | parity (0054) | Explicit env file loading and auto-discovery kill switch |
| --mode flag | parity (0054) | Mode-aware .env selection; sets NODE_ENV only when absent from host environment |
| Web Storage polyfill | parity (0054) | localStorage (persisted, per-project) and sessionStorage (in-memory, per-realm) globals via preload |
| Watch mode | parity (0055) | Long-lived supervisor with fsnotify watcher; debounce, clear-screen, and graceful restart; polling fallback |
| Debugging and inspection | parity (0056) | Node inspector passthrough (--inspect/--inspect-brk); doctor runtime checks; cache explain; runtime diagnostics |
| Runtime stabilization | parity (0057) | Conformance matrix; benchmark runtime; frozen protocol versions; known limitations; support matrix |
