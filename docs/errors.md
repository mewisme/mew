# Error codes

Stable machine-readable codes for Mew CLI failures. Pattern: `ERR_M_<DOMAIN>_<DETAIL>`
(see [`naming.md`](naming.md)). Nub `ERR_NUB_*` codes are behavioral references only.

Human modes render typed failures through presentation `ErrorView` (title, message,
context, code, hints) on stderr. Structured `json` / `ndjson` error documents are
unchanged. See [`architecture/cli-presentation.md`](architecture/cli-presentation.md)
and [`reporters.md`](reporters.md).

## Registry (MVP 0005)

| Code | Exit | Meaning |
|---|---|---|
| `ERR_M_OK` | 0 | Sentinel; not used for failures |
| `ERR_M_USAGE` | 2 | Invalid arguments or flag misuse |
| `ERR_M_CANCELLED` | 130 | Context canceled / interrupt |
| `ERR_M_INTERNAL` | 1 | Unexpected failure |
| `ERR_M_INTERNAL_PANIC` | 1 | Panic recovered at command boundary |
| `ERR_M_IO` | 1 | Filesystem I/O |
| `ERR_M_CONFIG` | 1 | Configuration (seed for 0006) |
| `ERR_M_NETWORK` | 1 | Network / registry (seed) |
| `ERR_M_INTEGRITY` | 1 | Checksum / integrity (seed) |
| `ERR_M_LOCKFILE` | 1 | Lockfile parse, checksum, graph, or frozen manifest drift (MVP 0015) |
| `ERR_M_UNIMPLEMENTED` | 1 | Reserved command stub not yet implemented (MVP 0010) |
| `ERR_M_UNSUPPORTED` | 1 | Operation not supported on this identity or format (npm incumbent lock mutation, publish provenance without provider) |
| `ERR_M_MANIFEST` | 1 | package.json parse / validate (MVP 0011) |
| `ERR_M_NOT_FOUND` | 1 | Project root, package.json, or package script missing (MVP 0011, 0040) |
| `ERR_M_RESOLVE` | 1 | Dependency resolution failure: unsatisfiable range, cycle, missing packument, or limit exceeded (MVP 0013) |
| `ERR_M_TRANSACTION` | 1 | Transaction journal, commit, rollback, recovery, or project lock failure (MVP 0017) |
| `ERR_M_STORE` | 1 | Global content store import, verify, or prune failure (MVP 0018) |
| `ERR_M_POLICY` | 1 | Lifecycle script trust block (0021) or org supply-chain policy violation (0030) |
| `ERR_M_PNP_UNSUPPORTED` | 1 | Yarn Berry PnP install blocked (MVP 0025; see [`yarn-lockfile.md`](yarn-lockfile.md)) |
| `ERR_M_EXEC` | 1 | Local binary launch failure: invalid shebang, missing ComSpec, malformed shim, or launch construction failure (MVP 0043) |
| `ERR_M_TIMEOUT` | 1 | Bounded helper or child timeout (e.g. PnP probe; MVP 0043) |
| `ERR_M_INTEGRITY` | 1 | Ambiguous incomplete transaction state, tree manifest collision, or verification failure |

### Transaction detail (0017 journal v3)

| Situation | Code | Notes |
|---|---|---|
| Concurrent install (`lock` held) | `ERR_M_TRANSACTION` | Another process holds `.mew/txn/lock` |
| Lock wait cancelled | `ERR_M_CANCELLED` | Context cancelled during `AcquireProjectLock` |
| Multiple incomplete `committing` journals | `ERR_M_INTEGRITY` | Directory scan found ambiguous state |
| Incomplete txn after preflight recovery | `ERR_M_INTEGRITY` | `BeginMutation` refused to start |
| Lock release without ownership | `ERR_M_TRANSACTION` | `ReleaseDirLock` returned `ReleaseNotOwner` / `ReleaseMissingOwner` |
| Commit / publish failure | `ERR_M_TRANSACTION` | Roll back via `m recover` when incomplete |
| Recovery failure | `ERR_M_TRANSACTION` | Partial `node_modules` rename may need manual cleanup |
| Symlink/junction in guarded path | `ERR_M_TRANSACTION` | Ancestor guard on `.mew` / `node_modules` / snapshots |
| Post-commit prune failure | `ERR_M_IO` | Install already committed; retry prune or `m snapshot list` |
| Post-commit cleanup incomplete | `ERR_M_TRANSACTION` | Lock released or `current` clear failed after commit; run `m recover` |
| Store import lock release failure | (warning only) | `ImportResult.CleanupWarnings`; published tree remains valid |
| StoreID collision during isolated layout | `ERR_M_INTEGRITY` | Collision-resistant digest still collided (extremely rare) |
| Windows directory sync denied | (none — no-op) | `fsx.SyncDir` ignores access-denied on directory handles; file sync still runs |

Unknown codes map to exit **1**.

## Supply-chain security (MVP 0030)

| Situation | Code | Notes |
|---|---|---|
| Advisory cache missing with `--offline` | `ERR_M_NETWORK` | `app.audit`; seed `<cache>/advisory/osv.json` |
| Advisory cache missing (online) | `ERR_M_NOT_FOUND` | Same path; copy or refresh advisory DB |
| Org policy violation on `m policy check` | `ERR_M_POLICY` | `policy.check`; use `--json` for violations |
| Org policy violation on install validate | `ERR_M_POLICY` | `app.policy` / `install`; transaction rolls back |
| Invalid `mew.policy.json` | `ERR_M_CONFIG` | `policy.load` / `policy.normalize` |
| Provenance attestation mismatch | `ERR_M_INTEGRITY` | `verify.provenance` / `app.provenance` |
| Unknown SBOM format | `ERR_M_USAGE` | `app.sbom`; use `cyclonedx` or `spdx` |

See [`audit.md`](audit.md), [`sbom.md`](sbom.md), [`policy.md`](policy.md).

## Script runner (MVP 0040)

| Situation | Code | Exit | Notes |
|---|---|---|---|
| Missing `package.json` script | `ERR_M_NOT_FOUND` | 1 | `runner.lookup`; lists available scripts when present |
| Missing project root | `ERR_M_NOT_FOUND` | 1 | `project.open` before run |
| Invalid regex selector | `ERR_M_USAGE` | 2 | `/pattern/` parse or compile failure |
| Child script failure | (none) | child code | `ExitHint` on `apperr.Error`; not `ERR_M_*` |
| Parent interrupt / cancel | `ERR_M_CANCELLED` | 130 | After best-effort child signal |

`m run --if-present` exits **0** when the script is missing (no error code).

See [`runner.md`](runner.md).

## Workspace script runner (MVP 0041)

| Situation | Code | Exit | Notes |
|---|---|---|---|
| Workspace flags without `-r` or `--filter` | `ERR_M_USAGE` | 2 | `app.run`; e.g. `--workspace-concurrency` alone |
| Workspaces gate disabled | `ERR_M_USAGE` | 2 | `app.run`; set `MEW_EXPERIMENTAL_WORKSPACES=1` |
| Not a workspace project | `ERR_M_MANIFEST` | 1 | `runner.workspace` |
| No packages match filter / selection | `ERR_M_NOT_FOUND` | 1 | `runner.workspace` / `runner.schedule` |
| Cyclic workspace dependency in selection | `ERR_M_RESOLVE` | 1 | `workspace.graph`; no deadlock |
| Invalid `--workspace-order` / `--workspace-output` | `ERR_M_USAGE` | 2 | `runner.workspace` |
| Negative `--workspace-concurrency` | `ERR_M_USAGE` | 2 | `runner.workspace` |
| Member `package.json` parse failure | `ERR_M_MANIFEST` | 1 | `runner.workspace`; per-member path in subject |
| Missing script (no `--if-present`) | `ERR_M_NOT_FOUND` | 1 | Fails that member; bail stops the run |
| Child script failure | (none) | child code | Bail: first failure exit; continue: earliest failed index |
| Parent interrupt / cancel | `ERR_M_CANCELLED` | 130 | In-flight children cancelled |

`m run --if-present` in workspace mode marks a member with no script as `skip`
(terminal success) and still releases dependents.

See [`runner.md`](runner.md).

## Direct script dispatch (MVP 0042)

| Situation | Code | Exit | Notes |
|---|---|---|---|
| Bare `m` | `ERR_M_USAGE` | 2 | Lists up to 10 scripts when manifest valid; never executes |
| Unknown command (with suggestions) | `ERR_M_USAGE` | 2 | Max 3 typed suggestions; never fuzzy-executes |
| Gate off + exact script exists | `ERR_M_USAGE` | 2 | Suggests `m run <script>` only |
| Workspace direct shortcut, workspaces gate off | `ERR_M_USAGE` | 2 | Requires both direct-script and workspace gates |
| Explicit missing project via `--cwd` | `ERR_M_NOT_FOUND` | 1 | Script dispatch only |
| Malformed manifest during script lookup | `ERR_M_MANIFEST` | 1 | |
| Child script non-zero | (none) | child code | Same as `m run` |

## `mx` DLX (MVP 0044)

| Situation | Code | Exit | Notes |
|---|---|---|---|
| Invalid package spec / unknown mx flag | `ERR_M_USAGE` | 2 | `mx.parse` / `dlx.spec` |
| Missing selector or Mode B command | `ERR_M_USAGE` | 2 | `mx.parse` |
| Non-TTY remote fetch without `--yes` | `ERR_M_USAGE` | 2 | `dlx.consent`; metadata may have occurred; zero artifacts |
| User denies consent | `ERR_M_POLICY` | 1 | `dlx.consent` |
| Unsupported package protocol | `ERR_M_UNSUPPORTED` | 1 | `dlx.spec` |
| Package has no bin / command not in requested packages | `ERR_M_NOT_FOUND` | 1 | `dlx.bininfer` |
| Ambiguous Mode A bins / multiple `-p` owners | `ERR_M_USAGE` | 2 | `dlx.bininfer` |
| Offline request mapping missing | `ERR_M_NOT_FOUND` | 1 | `dlx.cache` / request index |
| Offline warm environment corrupt | `ERR_M_INTEGRITY` | 1 | `dlx.cache` |
| Consent / request / environment lock timeout | `ERR_M_TIMEOUT` | 1 | `dlx.lock` |
| Execution lease failure / cache publication | `ERR_M_IO` | 1 | `dlx.lease` / publish |
| Lifecycle build blocked with no usable bin | `ERR_M_POLICY` | 1 | ephemeral lifecycle policy |
| Artifact integrity mismatch | `ERR_M_INTEGRITY` | 1 | fetch / store verify |
| Registry / resolver failures | (preserved) | 1 | not flattened to `ERR_M_RESOLVE` |
| Child non-zero | (none) | child code | `ExitHint` via ProcessSupervisor |
| Parent cancellation | `ERR_M_CANCELLED` | 130 | existing supervisor mapping |

See [`runner.md`](runner.md).

## Unified execution (MVP 0045)

| Situation | Code | Exit | Notes |
|---|---|---|---|
| Missing `exec` command selector | `ERR_M_USAGE` | 2 | `exec.parse` / `envexec.validate` |
| `--snapshot` without id / `--capsule` without path | `ERR_M_USAGE` | 2 | selector-boundary parse in `exec.parse` |
| Snapshot and capsule flags combined | `ERR_M_USAGE` | 2 | `exec.parse` |
| Source flags after command selector | `ERR_M_USAGE` | 2 | child args pass through unchanged |
| Missing snapshot id / corrupt snapshot | `ERR_M_NOT_FOUND` / `ERR_M_INTEGRITY` | 1 | `snapshot.load` / validate |
| Missing capsule / corrupt archive | `ERR_M_NOT_FOUND` / `ERR_M_INTEGRITY` | 1 | `capsule.open` |
| Snapshot or capsule registry fetch | (never) | — | network forbidden by locked policy |
| `m env inspect` missing source subcommand | `ERR_M_USAGE` | 2 | Cobra grammar |
| Inspect with invalid source fields | `ERR_M_USAGE` | 2 | `envexec.validate` |
| Inspect project, no `package.json` | `ERR_M_NOT_FOUND` | 1 | `project.find` |
| Warm shared-cache identity mismatch | `ERR_M_INTEGRITY` | 1 | `envexec.cache` verify |
| Child non-zero | (none) | child code | shared ProcessSupervisor |

See [`runner.md`](runner.md) and [`cli.md`](cli.md).

## Runtime (MVP 0050)

| Situation | Code | Exit | Notes |
|---|---|---|---|
| Node binary not found on PATH | `ERR_M_RUNTIME_NODE_NOT_FOUND` | 1 | `node.discover` |
| Node version unparseable | `ERR_M_RUNTIME_NODE_VERSION` | 1 | `node.discover` / `node.query-version` |
| Node version unsupported | `ERR_M_RUNTIME_NODE_UNSUPPORTED` | 1 | capability check |
| Invalid or missing entrypoint | `ERR_M_RUNTIME_ENTRYPOINT` | 1 | `runtime.entrypoint` / `runtime.node-args` |
| Node process invocation failure | `ERR_M_RUNTIME_INVOCATION` | 1 | `runtime.launch` (non-zero child exit propagates via ExitHint) |
| Corrupt asset manifest | `ERR_M_RUNTIME_ASSET_MANIFEST` | 1 | `assets.manifest` |
| Asset digest mismatch | `ERR_M_RUNTIME_ASSET_DIGEST` | 1 | `assets.verify` / `runtime.cache` |
| Asset extraction failure | `ERR_M_RUNTIME_ASSET_EXTRACTION` | 1 | `runtime.cache` |
| Asset cache read failure | `ERR_M_RUNTIME_ASSET_CACHE` | 1 | `assets.read` / `runtime.verify` |
| Node process failed to start | `ERR_M_RUNTIME_NODE_START` | 1 | `runtime.launch` |
| Inspector non-loopback bind rejected | `ERR_M_INSPECTOR_BIND` | 1 | `runtime.inspect` |
| Inspector port out of range | `ERR_M_INSPECTOR_PORT` | 1 | `runtime.inspect` |
| Inspector host malformed | `ERR_M_INSPECTOR_HOST` | 1 | `runtime.inspect` |
| Conflicting inspector flag values | `ERR_M_INSPECTOR_DUPLICATE` | 1 | `runtime.inspect` |

See [`runtime.md`](runtime.md).

## Go API

Package [`internal/apperr`](../internal/apperr): `New`, `Wrap`, `CodeOf`, `ExitCode`.

Every public failure path should return an `*apperr.Error` (or wrap into one at the CLI boundary).

## Debug bundles

Future `m doctor report` archives must require explicit consent and apply the same
redaction rules as reporters. Spec pointer: [`reporters.md`](reporters.md).
Not implemented in 0005.
