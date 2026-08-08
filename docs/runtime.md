# Node runtime (MVP 0050–0051)

Mew can run `.js`, `.mjs`, `.cjs`, `.ts`, `.mts`, and `.cts` files directly
through the `m` CLI. 0050 provides Node launch, augmentation, and preload
injection. 0051 adds the Go transform service (esbuild), TypeScript execution,
and a content-addressed transpile cache.

## Quick start

```text
MEW_EXPERIMENTAL_RUNTIME=1 m app.js
MEW_EXPERIMENTAL_RUNTIME=1 m app.ts
MEW_EXPERIMENTAL_RUNTIME=1 m --node app.js          # zero-augmentation
MEW_EXPERIMENTAL_RUNTIME=1 m node-args -- --trace-warnings app.js
```

## Gate

`MEW_EXPERIMENTAL_RUNTIME=1` enables all invocation styles:

| Style | Example | Augmentation |
|---|---|---|
| File-run (default) | `m app.js` | Full (preloads injected) |
| File-run (TS) | `m app.ts` | Full + transform service |
| File-run (zero-aug) | `m --node app.js` | None (stock Node) |
| Node-args | `m node-args -- <v8-flags> <entrypoint>` | Full (preloads injected) |

## Dispatch precedence

File-run dispatch sits between exact `package.json` script matching and bin
dispatch:

1. Built-in command (`install`, `run`, …)
2. Built-in alias
3. Exact `package.json` script
4. **JS/TS entrypoint** (`.js`, `.mjs`, `.cjs`, `.ts`, `.mts`, `.cts`) when `MEW_EXPERIMENTAL_RUNTIME=1`
5. Verified local bin
6. Typed suggestions

Built-in commands always win over same-named JS files. Use `m run <script>` to
force a script when a name collides.

Deferred extensions (`.tsx`, `.jsx`) return an actionable plan-0052 deferral
message instead of "unknown command".

## Node discovery

Mew discovers Node from the system `PATH` on every invocation. There is no
persistent version mapping, no download, and no network access (deferred to
plan 0060).

- `m app.js` → finds `node` on `PATH`
- `m app.ts` → finds `node` on `PATH`, starts transform service on localhost

After discovery, the Node version is parsed and capability flags are assigned:

| Capability | Minimum Node | Required for |
|---|---|---|
| `require-preload` | ≥ 12 | All entrypoints |
| `import-preload` | ≥ 16 | All entrypoints |
| `module-register` | ≥ 18.19 | TypeScript entrypoints |
| `source-maps` | ≥ 20.6 | `--enable-source-maps` (auto-injected) |

### Supported Node versions

| Version | Status | Notes |
|---|---|---|
| Node 24 | Supported | Full capabilities |
| Node 22 | Supported (LTS) | Full 0050/0051 capabilities |
| Node 20 | Supported (LTS) | Full capabilities |
| Node 18 | Supported (maintenance) | Requires ≥ 18.19 for TS |
| Node < 18 | Unsupported | Missing `module-register` for TS; JS-only may work |

## TypeScript transform

When a `.ts`, `.mts`, or `.cts` entrypoint is detected, Mew starts a local
Go transform service (esbuild) on a random TCP port. The service communicates
with the Node loader bridge over a length-prefixed JSON frame protocol.

```
m app.ts
  → Go starts esbuild transform service on 127.0.0.1:<random>
  → Node launches with credential-grabber preload (--require credential-grabber.cjs)
  → credential-grabber.cjs strips credentials from env, registers ts-loader.mjs
    via module.register() with credential data, registers user --loader modules
  → Node loads app.ts → loader hook fires → source sent to transform service
  → Transformed JS returned → Node executes
```

Transform credentials (`MEW_TRANSFORM_ENDPOINT`, `MEW_TRANSFORM_TOKEN`) are
captured by the credential grabber at its invocation time (first `--require`
preload). The grabber strips them from the environment before any user module
executes and passes them to `ts-loader.mjs` via `module.register()`'s `data`
option — no filesystem artifact, no env leak.

**Protocol**: JSON length-prefixed frames (u32le header + JSON body) over TCP.
Max frame size 48 MiB. Protocol version 2. Auth via bearer token.

**Cache**: Content-addressed SHA-256(`code || map`). Atomic temp+rename
publication. Metadata is commit record. Missing code/map with committed
metadata → corruption → cleaned up for re-transform.

### Module format determination

The loader determines the Node module format (`"module"` for ESM, `"commonjs"`
for CJS) based on file extension and the nearest `package.json` `"type"` field,
matching Node.js semantics:

| Extension | Package `"type"` | Format | Rationale |
|---|---|---|---|
| `.mts` | any | `module` (ESM) | `.mts` is always ESM |
| `.cts` | any | `commonjs` (CJS) | `.cts` is always CommonJS |
| `.ts`, `.tsx` | `"module"` | `module` (ESM) | Package type overrides |
| `.ts`, `.tsx` | `"commonjs"` or absent | `commonjs` (CJS) | Node default is CJS |
| `.ts`, `.tsx` | no `package.json` found | `commonjs` (CJS) | Node default is CJS |

The same rules apply after extension substitution (Issue 12): if a `.js`
specifier resolves to a `.ts` file, the format of the resolved `.ts` file
governs.

Format is included in the transform cache key. The same TypeScript source
transformed with different module formats produces separate cache entries
and cannot collide.

**Boundary lookup**: The loader walks up the directory tree from the resolved
file to find the nearest `package.json`. A nested `package.json` (e.g. in a
subdirectory with a different `"type"`) overrides the parent. Results are
cached per `package.json` directory for the lifetime of the Node process.

**Invalid `"type"` values**: Treated as `"commonjs"` (Node default). Malformed
or unreadable `package.json` files default to `"commonjs"`.

**Limitations**:
- Format determination applies to files resolved through the ESM loader hooks.
  CJS `require()` calls inside transformed modules bypass the loader hooks and
  use Node's native CJS resolution (no TypeScript extension mapping).
- Full Node16/NodeNext resolver parity (package `exports`/`imports`) belongs to
  Issue 16.

### Custom loaders (`--loader`)

Mew supports user-supplied ESM loader hooks via the `--loader` flag.
Multiple loaders compose into a single hook chain with Mew's TypeScript
transform loader.

**Syntax**:

```text
m --loader ./my-loader.mjs app.ts
m --loader ./a.mjs --loader ./b.mjs app.ts      # multiple loaders, ordered
m --node --loader ./my-loader.mjs app.js         # --node mode
```

**Accepted forms**: absolute paths, relative paths (resolved against working
directory), and `file://` URLs. Loader paths must exist at launch time —
missing paths produce a deterministic bootstrap error before Node starts.

**Hook chain** (LIFO: last-registered fires first):

```
User loader 1  (--loader a.mjs)     ← outermost, fires first
    ↓ nextResolve / nextLoad
User loader 2  (--loader b.mjs)
    ↓ nextResolve / nextLoad
Mew ts-loader  (TypeScript transform) ← innermost, fills gaps
    ↓ nextResolve / nextLoad
Node default loader
```

A loader that calls `nextResolve`/`nextLoad` delegates to the next hook.
A loader that returns without calling `next*` short-circuits the chain.
Errors thrown by user loaders propagate as Node module resolution errors
without converting them into Mew transform errors.

**Multiple `--loader` order**: first flag = outermost hook. `--loader a.mjs
--loader b.mjs` → `a.mjs` fires first.

**`--node` behavior**: user loaders are registered via a minimal shim
(`loader-register.mjs`). No credential handling, no ts-loader, no Mew
preloads — just the user's loaders on stock Node.

**Loader contract**: a custom loader module must export `resolve` and/or
`load` hooks following the Node.js loader API. It does not need to
self-register — Mew calls `module.register()` on its behalf. Loader modules
must not import Mew internals.

### Worker threads

Workers created from a Mew-augmented process automatically inherit runtime
capabilities. The credential grabber transports transform credentials into
workers via a non-enumerable `Symbol` key on `workerData`. Each worker
registers its own `ts-loader` hooks through `module.register()`, preserving
isolate-level isolation.

- Worker entrypoints using `.ts`, `.tsx`, `.mts`, `.cts` are supported.
- Workers preserve the effective environment from dotenv/`--mode`/host env.
- User `workerData`, `env`, and `execArgv` options are preserved.
- Workers that override `execArgv` opt out of Mew augmentation.
- Each worker registers its own loader hooks; one worker's failure does not
  affect other workers or the parent session.
- Transform options (source maps, decorator mode, JSX) are inherited from the
  parent runtime configuration.
- Workers cannot observe the parent's raw transform credentials through
  `process.env` or enumerable `workerData` properties.

**Limitations**: custom loaders registered on the parent thread via
`--loader` are not propagated to workers. Each worker that needs custom
loaders must register them explicitly. Worker-specific transform
configuration (separate tsconfig per worker) is not yet supported.

### Child processes

Mew does not automatically inject augmentation into child processes created
via `child_process.spawn()`, `exec()`, or `execFile()`. Unrelated child
processes receive a clean environment (credentials are already stripped by
the credential grabber before user code executes).

- `child_process.fork()` inherits `process.execArgv` from the parent,
  including Mew preloads. The child process re-runs preload scripts (Web
  Storage polyfill) but does **not** receive transform credentials —
  TypeScript support is not active in forked children.
- For an intentionally augmented Node child, invoke `m` from the child:
  `spawn('m', ['child.ts'])`. This creates a fresh transform session with
  its own scoped credentials.
- Non-Node executables spawned via `spawn()`/`exec()` receive the parent's
  environment (minus Mew-private bootstrap variables) and no Mew
  augmentation.

### Web Storage

Mew provides `localStorage` and `sessionStorage` globals via preload.
Both implement the standard `Storage` API:

- `getItem(key)` → `string | null`
- `setItem(key, value)`
- `removeItem(key)`
- `clear()`
- `key(index)` → `string | null`
- `length` → `number`

Keys and values are coerced to `String`. Missing keys return `null`.
Keys are enumerated in insertion order. Property-style access
(`storage.foo`) and `Object.keys(storage)` are not supported — use the
methods.

#### localStorage

Persisted per-project. The namespace is the first 16 hex characters of
`SHA-256(resolved project root)`. The data file lives at
`<cache>/webstorage/v1/<namespace>.json`.

| Property | Behavior |
|---|---|
| Persistence | Survives process restart |
| Isolation | Per project root (different directories = different namespaces) |
| Symlinks | Resolved before hashing (stable identity) |
| Format | Schema-versioned JSON (v1) |
| Writes | Atomic (temp file + fsync + rename) |
| Corruption | Malformed files are reset to empty with a console warning |
| Quota | 5 MiB default; override with `MEW_STORAGE_QUOTA_BYTES` env var |
| Quota exceeded | Throws `QuotaExceededError` (or `DOMException` on Node 17+) |
| No project | Standalone scripts without a `package.json` ancestor get in-memory-only localStorage |

#### sessionStorage

In-memory `Map`-backed storage, scoped to one JavaScript realm.

| Property | Behavior |
|---|---|
| Persistence | None (dies with the process) |
| Workers | Each worker thread gets an independent sessionStorage |
| Parent/worker | No shared state between parent and worker |
| Child processes | Not inherited (unrelated children get clean env) |

#### Worker and child process visibility

- **Workers** (`worker_threads`): Inherit preloads via `execArgv`. Each
  worker gets its own `localStorage` (same persistent file, same
  namespace) and its own `sessionStorage` (independent `Map`).
  Concurrent writes to `localStorage` from multiple workers use
  last-writer-wins atomic rename — no corruption, but no merge either.
- **Forked children** (`child_process.fork()`): Inherit preloads, get
  their own `localStorage` (same file) and independent
  `sessionStorage`.
- **Unrelated children** (`spawn`/`exec`): No storage globals (clean
  environment). No localStorage data leakage.

#### Limitations

- No `StorageEvent` or `storage` event listener.
- No property-style access (`storage.foo`), no `Object.keys(storage)`.
- No origin-based isolation (browser concept); Mew uses project-root
  namespace instead.
- No cross-project data sharing.
- Quota is per-storage-object, not per-project aggregate.
- Moving or renaming a project directory changes the namespace (old
  data file is left in place but orphaned).

### Yarn Plug'n'Play (PnP) resolution

When a project contains `.pnp.cjs` at its root, Mew's ts-loader detects it
and delegates bare package resolution to the Yarn PnP API.

**Discovery**: The loader walks up from the importing file's directory
looking for `.pnp.cjs`. Discovery is scoped to the project root — results
are cached per root for the lifetime of the Node process. Each `m run`
invocation starts a fresh Node process, so there is no cross-project
contamination at the Go process level.

**API**: `.pnp.cjs` is loaded via `createRequire()` and must export
`resolveRequest(specifier, issuer, opts)` following the standard Yarn PnP
contract. The `issuer` argument is a native filesystem path (converted from
the `context.parentURL` file:// URL by the loader).

**Resolver ordering**:

```
Builtins (node:fs, etc.) → skip PnP
Relative / absolute paths    → skip PnP
Bare specifiers              → PnP resolveRequest → tsconfig paths → Node fallback
```

**Error handling**: When PnP owns a dependency and rejects it (undeclared
dependency, boundary violation), the error propagates as a Node module
resolution error. When PnP returns null (package not in map), resolution
falls through to tsconfig paths and Node.

**`.pnp.data.json`**: Not sufficient for PnP resolution. Only `.pnp.cjs`
provides a usable resolver API. Projects with only `.pnp.data.json` are
treated as non-PnP projects.

**Multi-project isolation**: PnP state is keyed by project root in a
per-process cache. Each Node process resolves against its own project
context. Worker threads receive their own loader module instances.

**Unsupported**: PnP `resolveVirtual` (virtual paths), `getAllLocators`,
and full Yarn PnP API surface beyond `resolveRequest`.

### Module resolution diagnostics (`m resolve-module`)

`m resolve-module <specifier>` runs the same resolution algorithm used by
the runtime loader and reports the actual result with a structured trace.

```text
m resolve-module @app/core
m resolve-module --from ./src ./helpers
m resolve-module --json lodash
```

**Flags**:

| Flag | Description |
|---|---|
| `--from <dir>` | Resolve from `<dir>` (default: cwd) |
| `--json` | Emit structured JSON (schema version 1) |

**Resolution stages** (in order):

1. **builtins** — Node built-in modules (`node:fs`, etc.)
2. **node-native** — Node's native ESM resolution (node_modules, exports, imports)
3. **extension-probe** — TypeScript extension substitution (`.js` → `.ts`, `.mjs` → `.mts`, `.cjs` → `.cts`)
4. **pnp** — Yarn PnP resolution via `.pnp.cjs`
5. **tsconfig-paths** — tsconfig `baseUrl` and `paths` pattern matching

Each stage records its outcome: `resolved`, `miss`, `skipped`, or `error`.

**Output**:

- **Human** (`--json` absent): trace table with stage, outcome, resolved path, format, and pattern matches.
- **JSON** (`--json`): structured output with `schemaVersion`, `specifier`, `importer`, `resolved`, `target` (url, path, format), ordered `trace` steps, and typed `error` data when unresolved.

**Exit codes**: 0 on resolution success, 1 on resolution failure or diagnostic error.

**Limitations**:
- Diagnostics run the resolution algorithm via a Node subprocess — they require Node on `PATH`.
- When Node is unavailable, the command falls back to static tsconfig path analysis.
- Custom loaders registered at runtime are not invoked during diagnostics.
- The diagnostic reflects the current resolver behavior; it does not claim full Node/TypeScript resolution parity.

## Environment file loading

Mew automatically discovers `.env` files in the working directory and supports
explicit env-file paths. All loaded variables are merged into the child process
environment before user application code starts.

### Syntax

Env files support the following syntax:

```
# Comments start with # (blank lines are ignored).
KEY=value
export KEY=value

# Quoting
UNQUOTED=hello world
DOUBLE="line1\nline2\t${VAR}"
SINGLE='literal $NOEXPAND'

# Variable expansion
URL=http://${HOST}:${PORT}
SIMPLE=$HOST
DEFAULTED=${MISSING:-fallback}
```

- Keys must start with a letter or underscore (`[A-Za-z_]`) and contain only
  letters, digits, and underscores (`[A-Za-z0-9_]+`). Invalid or empty keys
  produce a parse error.
- Lines without `=` in the trimmed content are rejected as malformed.
- Single-quoted values are literal (no expansion, no escapes).
- Double-quoted values support `\n`, `\r`, `\t`, `\\`, `\"`, `\$` escapes and
  `${VAR}`/`$VAR` expansion.
- Unquoted values support expansion and `\\`, `\$`, `\#`, `\ `, `\t`, `\"`,
  `\'` escapes.
- `\n` in double-quoted values produces a literal newline; `\t` produces a tab.
- Unknown escape sequences in double-quoted values keep the backslash verbatim;
  unknown escapes in unquoted values keep the backslash verbatim.

### Variable expansion

Three expansion forms are supported:

| Form | Example | Behavior |
|---|---|---|
| `$VAR` | `$HOME` | Expands to value of `VAR` |
| `${VAR}` | `${HOME}` | Same, with explicit boundary |
| `${VAR:-default}` | `${PORT:-3000}` | Uses default if `VAR` is unset or empty |

Expansion rules:

- Variables are looked up in the accumulated environment from all files loaded
  so far, plus the host (shell) environment.
- Earlier files' values are visible to later files (cross-file expansion).
- Within a single file, each line sees values from earlier lines in the same
  file (sequential evaluation).
- Forward references (a variable defined later in the same file) resolve to
  empty (or to the host environment value if present).
- Expansion is single-pass; expanded values are not re-expanded.
- Self-references (`X=$X` before `X` is defined) resolve to the host
  environment value (or empty if not in host env).

### Precedence

The effective environment for the child process is built in this order (highest
number wins):

1. Host (shell) environment — `os.Environ()` at process start. These values
   are **never** overridden by dotenv files or `--mode`.
2. Dotenv file values (auto-discovered or explicit). Loaded in file order;
   later files override earlier files for the same key. Dotenv values apply
   **only** for keys not already present in the host environment.
3. `--mode` sets `NODE_ENV` only when `NODE_ENV` is absent from the host
   environment. If `NODE_ENV` is set in the host environment (e.g., via a
   parent shell), that value is preserved and `--mode` does not override it.
4. Internal runtime variables (loaders, transform service) are always applied
   and override host values when necessary for correctness.

### Auto-discovery (optional)

When no `--env-file` is given, Mew searches the working directory for env files
following mode-aware precedence:

```
.env.<mode>.local > .env.<mode> > .env.local > .env
```

- Files are loaded in order (lowest precedence first); later files override earlier.
- Missing files are silently skipped — auto-discovery is always optional.
- `--mode <mode>` enables mode-specific discovery. It also sets `NODE_ENV=<mode>`
  when `NODE_ENV` is not already present in the host environment.

### Explicit files (`--env-file`, repeatable)

Each path supplied to `--env-file` is **required**. Mew fails before user code
runs when an explicit file:

- Does not exist (`ERR_M_ENV_FILE_NOT_FOUND`)
- Is not readable (`ERR_M_ENV_FILE_READ`)
- Contains a parse error (`ERR_M_ENV_FILE_PARSE`)

If any required file fails, Mew does **not** start the application with a
partial environment — all explicit files must load and parse successfully.

- Multiple `--env-file` flags are processed in declaration order.
- Later files override earlier files for the same key.
- Relative paths are resolved against the working directory.
- Paths containing spaces are supported.

### `--no-env-file`

Disables auto-discovery. When combined with `--env-file`, auto-discovery is
skipped but the explicitly listed files are still loaded (and are still
required).

| Flags | Behavior |
|---|---|
| *(none)* | Auto-discover `.env*` files in cwd (optional) |
| `--env-file a.env` | Load `a.env` (required), skip auto-discovery |
| `--env-file a.env --env-file b.env` | Load both (required), `b.env` overrides `a.env` |
| `--no-env-file` | Skip all env files, `--mode` still sets `NODE_ENV` when absent from host |
| `--no-env-file --env-file a.env` | Skip auto-discovery, load `a.env` (required) |

### Watch mode

`m watch` applies the same env-file semantics: auto-discovery is optional,
explicit files are required, and a failing explicit file prevents the
supervisor from starting the child process. Direct execution and watch mode
use the same environment construction pipeline.

## Augmentation

### Default (full)

When augmentation is enabled, Mew injects preload assets into the Node argv:

```
node --require <cache>/credential-grabber.cjs --require <cache>/preload.cjs --import <cache>/preload.mjs <entrypoint> [app-args]
```

The credential grabber and preload files provide bootstrap boundaries. For
TypeScript entrypoints, `credential-grabber.cjs` registers the TS loader hooks
via `module.register()` and also registers any user `--loader` modules.

### Zero-augmentation (`--node`)

`m --node app.js` bypasses all injection and runs stock Node:

```
node <entrypoint> [app-args]
```

## Runtime assets

Embedded assets live in `internal/runtime/assets/`:

| Asset | Module type | Role | Purpose |
|---|---|---|---|
| `preload.cjs` | CommonJS | preload-cjs | CJS bootstrap boundary |
| `preload.mjs` | ESM | preload-esm | ESM bootstrap + credential stripping |
| `credential-grabber.cjs` | CommonJS | credential-grabber | Captures env, registers TS + user loaders |
| `loader-register.mjs` | ESM | loader-registration | Registration shim for `--node --loader` |
| `ts-loader.mjs` | ESM | loader-support | TS transform hook implementation |
| `manifest.json` | — | — | Content-addressed asset catalog |

Assets are extracted to `<cache-root>/runtime/<bundle-version>/` on first use
with SHA-256 verification and atomic writes.

## Runtime diagnostics

### Trace (`m runtime trace`)

`m runtime trace <entrypoint>` executes the application with structured runtime
event collection. Events use schema version 1 and cover lifecycle, transform,
cache, env, resolution, worker, and watch categories.

```text
m runtime trace app.ts                   # human summary (event counts per category)
m runtime trace --json app.ts            # NDJSON event stream on stdout
```

Human mode prints a session line plus per-category event counts after execution.
`--json` mode streams one JSON event per line via a bounded ring buffer.

Events are redacted: bearer tokens, query tokens, env-secret keys, and URL
userinfo are stripped. EnvData never includes values (only key names).

### Support bundle (`m runtime support-bundle`)

Collects a redacted diagnostic archive for bug reports:

```text
m runtime support-bundle                  # writes mew-support-bundle.tgz
m runtime support-bundle --output out.tgz # custom path
m runtime support-bundle --json          # print manifest to stdout
```

Contents (schema version 1): version info, OS/arch, Node version and
capabilities, feature inventory summary, doctor report, config key names.
Excluded: source code, env values, store contents, credentials, private paths.
`--force` required to overwrite an existing output file.

### Cache explain (`m cache explain`)

Introspects the transform cache:

```text
m cache explain                  # summary: entry count, bytes, orphans
m cache explain --key <key>      # detail for a specific cache entry
m cache explain --json           # machine-readable output
```

Read-only. Reports per-entry disposition (`hit`, `miss`, `corrupt`,
`schema-stale`, `orphan`, `unreadable`) with reason codes covering key
mismatch, schema mismatch, source/options/config/format/map-mode mismatches,
malformed metadata, missing code/map, digest mismatch, and I/O errors.

### Doctor runtime (`m doctor runtime`)

Health checks for the runtime subsystem:

```text
m doctor runtime        # human report
m doctor runtime --json # machine-readable
```

Checks: `node-capabilities`, `transform-handshake`, `transform-roundtrip`,
`source-map`, `tsconfig`, `runtime-cache`, `loader-bridge`, `watch-backend`,
`inspector`, `worker`. Each check reports `ok`, `warn`, `fail`, or `skipped`.
`--strict` treats warnings as failures.

### Watch mode (`m watch`)

```text
m watch app.ts                            # file-run watch
m watch --clear-screen=false app.ts       # preserve terminal output
m watch --debounce 500 app.ts             # custom debounce (default 200ms)
m watch --debounce 500 -- app.ts --flag   # app arguments after --
```

Long-lived supervisor with short-lived application child. Restarts on relevant
source, config, or dependency changes. Uses fsnotify (native) with 500ms
polling fallback. Dependency graph collected from transform and module
resolution hooks. Env files and tsconfig changes trigger restart.

## Error codes

See [`errors.md`](errors.md#runtime-mvp-0050) for the full error table.

## Architecture

| Package | Purpose |
|---|---|
| `internal/runtime/` | LaunchRequest → LaunchPlan → ProcessSupervisor |
| `internal/runtime/assets/` | Embedded JS and manifest |
| `internal/transform/` | Go transform service (esbuild), IPC protocol, cache |
| `internal/node/` | PATH discovery and version parsing |

Runtime and transform packages must not import `resolver`, `linker`, `store`, or
`fetch`. See [`forbidden-imports.md`](architecture/forbidden-imports.md).

## Benchmarks

```bash
go test -bench=. ./internal/runtime/... ./internal/transform/... -benchtime=5x -count=1
```

See [`evidence/runtime/0050-0051-certification.md`](evidence/runtime/0050-0051-certification.md)
for recorded results.

## Related

- [`cli.md`](cli.md) — `--node` flag, `node-args` subcommand, command precedence
- [`runner.md`](runner.md) — script runner and exec dispatch
- [`errors.md`](errors.md) — error code catalog
- [`architecture/package-map.md`](architecture/package-map.md) — package boundaries
