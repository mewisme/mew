# Runtime Known Limitations

Documented as of 0052-0057 implementation. Each entry includes the limitation, impact, and planned resolution path.

## Transformer

### OXC divergence

**Limitation**: Mew uses esbuild for TypeScript/JSX/TSX transforms, not OXC (the Nub reference). Esbuild covers the same syntax surface but has different diagnostic formatting, different JSX dev-mode output, and different decorator handling for legacy TypeScript decorators.

**Impact**: Identical TypeScript input may produce semantically equivalent but byte-different output compared to Nub. This is intentional divergence — Mew targets behavioral parity, not bit-for-bit output parity.

**Resolution**: No planned migration to OXC. Divergence documented as permanent.

### Decorator metadata emission

**Limitation**: `emitDecoratorMetadata` tsconfig flag is explicitly rejected during normalization with an `ERR_M_TRANSFORM_UNSUPPORTED` diagnostic. Mew is a transpiler, not a type checker, and lacks the type information required to emit `design:type`, `design:paramtypes`, and `design:returntype` metadata.

**Impact**: TypeScript projects using `emitDecoratorMetadata` with reflection metadata (`reflect-metadata` package) must remove the flag from tsconfig. The transform will not proceed with this option set.

**Resolution**: No current plan to support. Metadata emission requires compiler type information unavailable to a transpiler-only architecture.

### TC39 decorators (stage 3)

**Limitation**: Standard TC39 decorators are transformed via esbuild (default mode when `experimentalDecorators` is not set). This works for TypeScript sources (`.ts`/`.tsx`). Standard decorators in `.js`/`.mjs` files rely on extension substitution (`.js` → `.ts`) and the JSX loader (`.jsx` → `.tsx`). Direct `.jsx` entrypoints are not supported because the transform engine only accepts `ts|tsx|mts|cts` loader modes.

**Impact**: Projects using `@decorator` syntax in `.ts`/`.tsx` files work correctly. JS/JSX decorator support pending JS loader.

**Resolution**: Not yet available. Requires a JSX loader mode in the transform service.

## Loader Bridge

### TypeScript type checking

**Limitation**: Mew is a transpiler, not a type checker. No semantic diagnostics are performed — type errors are silently accepted.

**Impact**: Invalid TypeScript (type mismatches, missing properties) will execute without errors. Developers must run `tsc --noEmit` separately for type checking.

**Resolution**: By design. Type checking is out of scope for MewJS. Documented as permanent divergence.

### PnP unplugged mode

**Limitation**: PnP resolution requires `.pnp.cjs` at the project root. Only `.pnp.cjs` provides a usable resolver API. `.pnp.data.json` without `.pnp.cjs` (Yarn PnP "unplugged" mode) is not detected as a PnP project — resolution falls through to tsconfig paths and stock Node resolution.

**Impact**: Projects using Yarn PnP in unplugged mode will not get PnP-aware resolution.

**Resolution**: Planned for 0060+ when PnP integration is revisited.

### Custom loader ordering

**Behavior**: Custom loaders specified via `--loader` are registered via `module.register()` by Mew's credential-grabber (augmented mode) or loader-register shim (`--node` mode). They sit *outside* ts-loader in the hook chain: user loader 1 (outermost) → user loader 2 → ... → ts-loader → Node default. A user loader that short-circuits (returns without calling `nextResolve`/`nextLoad`) skips ts-loader entirely.

**Impact**: A custom loader that claims all `.ts` resolutions will prevent Mew's ts-loader from running, breaking path alias resolution and extension mapping. This is by design: user loaders are authoritative.

**Resolution**: Documented behavior. Use `m resolve-module` for debugging loader chains and resolution traces (0053).

## Runtime

### Worker threads

**Limitation**: Worker threads inherit the preload chain and transform
capabilities from the main thread (Issue 19). Worker-specific transform
configuration (separate tsconfig) is not supported. Custom loaders
registered via `--loader` on the parent are not propagated to workers.

**Impact**: Workers use the same transform options as the main thread
entrypoint. Multi-package monorepos where workers need different tsconfig
settings are not supported. Workers requiring custom ESM loader hooks must
register them explicitly.

**Resolution**: Per-worker transform configuration planned for 0060+.
Custom loader propagation to workers may be revisited with Node's evolving
loader API.

### Web Storage

**Limitation**: `localStorage` persists per-project (namespace = SHA-256 of project root) under the Mew cache directory. `sessionStorage` is per-realm, in-memory only, and does not survive process exit. Cross-project data sharing, origin-based isolation, and the full browser `StorageEvent` API are not supported. Property-style access (`storage.foo`) and `Object.keys(storage)` are deliberately unsupported — use `getItem`/`setItem`.

**Lock protocol**: Cross-process localStorage mutations are serialized via a directory-based lock (mkdir is atomic on all supported platforms). Each lock acquisition writes an `owner.json` containing `lockId` (ABA guard), `pid`, `processStart`, and `heartbeat`. Staleness is determined by:

1. Live process (`kill(pid,0)` succeeds) + recent heartbeat → never stale (lock age alone is never sufficient to steal a lock from a provably live owner)
2. Live process + stale heartbeat (no refresh for >60s) → stale (PID reuse guard: the original owner stopped renewing, a new process got the same PID)
3. Live process + legacy owner (pre-heartbeat) → not stale (conservative; PID reuse for legacy locks is a documented limitation)
4. Dead process (ESRCH) → stale after 5s grace period
5. Malformed/missing owner → stale after 5s grace period

Stale takeover is ABA-safe: the lock directory is atomically renamed to a tombstone before a replacement lock is created. On release, the owner verifies `lockId` before deleting the canonical lock directory — an old owner whose lock was taken over cannot delete the successor's lock.

**Windows process liveness**: `process.kill(pid, 0)` works on all platforms including Windows. The previous StaleLockMaxAge age-based fallback (which could steal a live owner's lock after 60s) has been replaced with the heartbeat protocol described above. PID reuse is mitigated by the heartbeat mechanism.

**Impact**: Packages using `getItem`/`setItem`/`removeItem`/`clear`/`key`/`length` work. Packages relying on `StorageEvent`, origin-based access control, or the `storage` event listener will not find those features. Moving a project directory changes its namespace and "loses" prior localStorage data (the old file remains but is no longer associated).

**Resolution**: Storage API surface is stable. Property-style access and `StorageEvent` are not planned. Quota (5 MiB default, `MEW_STORAGE_QUOTA_BYTES` override) and atomic writes are implemented.

### Watch mode

**Limitation**: Watch mode uses fsnotify with a polling fallback on platforms/filesystems where inotify/FSEvents/ReadDirectoryChangesW are unavailable. The polling interval is 500ms.

**Impact**: On network filesystems (NFS, CIFS) or inside some Docker configurations, watch mode falls back to polling and may have up to 500ms of latency.

**Resolution**: Configurable polling interval planned for 0060+.

### Source map runtime consumption

**Limitation**: Mew automatically passes `--enable-source-maps` to Node (>= 20.6) so that error stack traces map back to original TypeScript source. However, the runtime loader always requests inline source maps; external source maps (`.map` files) are only written to the transform cache and never to the user project directory.

**Impact**: Stack traces from transformed TypeScript modules show original source file names and line numbers when Node >= 20.6. Debugger breakpoint mapping requires inspector protocol integration (see below).

**Resolution**: External `.map` file writing to user project directories is not planned. Mew's transform cache retains maps for all generated code; inline maps are embedded directly in the emitted JavaScript.

### Inspector passthrough

**Limitation**: `--inspect` and `--inspect-brk` flags are parsed, validated, and normalized by Mew (loopback-only bind policy by default, remote binding requires `MEW_EXPERIMENTAL_REMOTE_INSPECTOR=1`). The flags are then passed through to Node's V8 inspector. Mew adds `--enable-source-maps` so that inline source maps are available to the inspector for stack traces. Mew does not intercept or rewrite the inspector protocol; breakpoint resolution relies on the debugger client consuming the source maps normally.

**Impact**: Stack traces in the debugger console show mapped source locations (via `--enable-source-maps`). Breakpoints in TypeScript source depend on the debugger client's source-map support. Debugger-visible scripts include sourceMapURL fields pointing to inline source maps.

**Resolution**: Inspector flags are passed through by design. Source-map availability is tested via inspector e2e tests (0056).

## Transform Service

### Service lifecycle

**Limitation**: The transform service is started on-demand per `m` invocation and shut down when the parent process exits. There is no persistent daemon mode.

**Impact**: Cold starts pay the full service startup cost (~50-100ms). Repeated `m` invocations (e.g., in watch mode restarts) each pay the startup cost.

**Resolution**: Persistent daemon mode planned for 0060+ (Node manager integration).

### Concurrent transform limits

**Limitation**: The transform service uses a worker pool (default 4 goroutines) over a single TCP connection. Throughput is bounded by the worker pool size and single-process esbuild performance.

**Impact**: Transform throughput is bounded by worker pool size and single-process esbuild performance. Each transform request runs concurrently within the pool.

**Resolution**: Configurable worker pool size or multiple service instances planned for post-0060.

## Node Version Support

| Node version | Status |
|---|---|
| >=18.19.0 | Supported (maintenance) |
| >=20.6.0 | Supported |
| >=22.0.0 | Supported (primary) |
| >=24.0.0 | Supported |

Node 16.x and earlier are unsupported. The minimum supported Node version is
18.19.0 — the first 18.x release to include `module.register()` (experimental
in 18.19, stable from 20.6). Runtime augmentation requires `module.register()`
for loader registration and tsconfig path resolution. Node 18.0–18.18 will be
rejected with `ERR_M_RUNTIME_NODE_UNSUPPORTED` before any launch attempt.

## Gated Features

Features behind experimental flags (current as of 0057 development):

| Feature | Gate | Status |
|---|---|---|
| Runtime augmentation | `MEW_EXPERIMENTAL_RUNTIME=1` | Experimental |
| Direct dispatch bins | `MEW_EXPERIMENTAL_EXEC_DIRECT_DISPATCH=1` | Experimental |
| Watch mode | `MEW_EXPERIMENTAL_RUNTIME=1` | Experimental |
| Web Storage | `MEW_EXPERIMENTAL_RUNTIME=1` | Supported |
| Debug inspection | `--inspect`/`--inspect-brk` | Passthrough only |
| Source maps | `--enable-source-maps` | Auto-injected (Node >= 20.6) |
