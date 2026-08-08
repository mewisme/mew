# Testing strategy

Hermetic fixtures, clean-home isolation, local registry, fuzz smoke, and
conformance stubs (MVP 0008). Normal CI must never depend on the public npm
registry. Large ecosystem corpora belong in scheduled jobs (MVP 0080).

Source: [`plans/0008-testing-strategy.md`](../plans/0008-testing-strategy.md).

## Layout

```text
fixtures/
  registry/v1/
    manifest.json           # SHA-256 index (fail closed on mismatch)
    packuments/             # npm-shaped packument JSON
    tarballs/               # synthetic *.tgz (not downloaded from npm)
  projects/
    basic-cjs/ basic-esm/ typescript-app/ workspace-simple/
  identity/                 # lockfile identity cases (0006)
  security/evil-archives/   # known-bad member names; never extract in prod
  lifecycle/                # lifecycle script fixtures + registry (0021)
tests/
  integration/              # clean-home + local registry smoke
  conformance/              # differential harness + inventory stub
internal/testkit/           # TempHome, LoadRegistry, DiffReport, faults, FS probe
```

```mermaid
flowchart LR
  fix[Fixtures] --> reg[LocalRegistry]
  reg --> mew[MewUnderTest]
  mew --> cmp[Compare]
  cmp --> ref[ReferencePM]
```

## Adaptive test execution

`tools/testexec` is the canonical Go test orchestrator. It provides adaptive
parallelism for local development and CI:

- **Automatic worker count** adapts to available CPUs and workload type
- **Process-level sharding** for heavy packages (integration, app, transaction)
- **Test binary reuse** via `go test -c` + `-test.run` for sharded packages
- **Deterministic round-robin** assignment; LPT scheduling when timing data available
- **Isolated environments** per worker (HOME, XDG, MEW_* vars in temp dirs)

### Worker count

| Mode | Behavior |
|---|---|
| `auto` (default) | unit=NCPU, integration≤4, crash≤3, race≤2 |
| `1` | Serial execution |
| `N` | Explicit worker count |

Override via Make: `make test TESTEXEC_WORKERS=2` or env: `TESTEXEC_WORKERS=2`.

Direct use: `go run ./tools/testexec [-workers N] [-short] [-race] [-tags TAGS] [packages...]`.

### Per-worker CPU budget

Each worker gets `GOMAXPROCS = floor(logicalCPUs / workers)`, minimum 1.
This prevents N child processes each scheduling as though they own all CPUs.

### Heavy package sharding

Packages classified as heavy (`tests/integration`, `tests/conformance`,
`internal/app`, `internal/transaction`) use process-level sharding:
compile test binary once with `go test -c`, split discovered test functions
across workers, run each subset with `-test.run` in an isolated process.

Light packages run normally via `go test -p N`.

### Serial mode

`TESTEXEC_WORKERS=1` runs all packages serially (one `go test` process).
Useful for debugging and low-resource machines. Same test coverage as auto mode.

## Clean-home contract

`testkit.CleanEnv` / `TempHome` set:

- `HOME`, `USERPROFILE`
- `XDG_CACHE_HOME`, `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`
- `MEW_HOME`, `MEW_CACHE_DIR`, `MEW_STORE_DIR`, `MEW_CONFIG_DIR`

All under a `t.TempDir()`. Tests must copy fixtures into the temp tree
(`CopyFixture`) before mutating. Do not write into the developer’s real home.

## Local fixture registry

1. Edit blobs under `fixtures/registry/v1/`.
2. Update `manifest.json` SHA-256 for every blob path.
3. `LoadRegistry` verifies checksums on load; mismatch fails the test.
4. `Start` serves `GET /{name}` (packument) and `GET /{name}/-/{file}.tgz`.

Synthetic packages only. Generator bytes are checked in; do not fetch from
`registry.npmjs.org` in tests or CI.

Smoke today: HTTP fetch packument + tarball + integrity check. Full `m install`
against the fixture registry waits for later install MVPs.

## How to add a fixture

1. Create a directory under `fixtures/<area>/`.
2. Prefer hand-authored text. For binaries, record SHA-256 in a nearby
   `manifest.json` (registry) or digests file.
3. Document any generator command in a fixture `README.md`.
4. Cover with a test that uses `CopyFixture` or `LoadRegistry`.
5. Never mutate checked-in fixtures from tests.

## Differential comparison

Schema: `testkit.DiffReport` (`schemaVersion`, `skipped`, `skipReason`, `mew`,
`reference`, `diffs[]`).

- Normalize with `NormalizeOutput` (absolute paths, CRLF, ISO timestamps).
- If `npm` / `nub` is absent, conformance smoke writes a skipped report and still
  validates the schema.
- Documented reference pins: **npm 10**, **pnpm 9**; Nub pin field in
  [`tests/conformance/inventory.json`](../tests/conformance/inventory.json).

```powershell
make conformance
# or
go test ./tests/conformance/... -count=1
```

## Lifecycle tests (0021)

Integration tests in `tests/integration/lifecycle*_test.go` use
`fixtures/lifecycle/registry` and `testkit.EnableLifecycle` (`MEW_EXPERIMENTAL_LIFECYCLE=1`).

```powershell
go test ./tests/integration/... -run Lifecycle -count=1
```

Node.js is required for script fixtures that invoke `node`; trust/policy tests run without Node.

See [`docs/lifecycle.md`](lifecycle.md).

Unit coverage for restricted execution and explicit-empty env:

```powershell
go test ./internal/process/... ./internal/lifecycle/... -count=1
```

## Workspace stabilization (0022 pass 12)

```powershell
go test ./internal/app/... -run "Untouched|MergeFiltered" -count=1
go test ./tests/integration/... -run Workspace -count=1
```

See [`docs/workspaces.md`](workspaces.md).

## Fuzz smoke

| Target | Package | Notes |
|---|---|---|
| `FuzzParseJSON` | `internal/manifest` | malformed package.json |
| `FuzzDecodeGraph` | `internal/graph` | truncated/garbage graphs |
| `FuzzDecodePnpmLock` | `internal/compat/pnpm` | hostile YAML; corpus `testdata/lockfile/fuzz/` |
| `FuzzLoadConfig` | `internal/config` | hostile JSONC |

Deferred until packages exist: archive path fuzz (`internal/archive`), semver
ranges (`internal/semver`).

```powershell
make fuzz-smoke
```

## Stabilization pass 14 suites (module rename / workspace / snapshot / lifecycle)

Go module path is `github.com/mewisme/mew` (renamed from `github.com/mewisme/m`).

| Area | Tests | What it proves |
|------|-------|----------------|
| Untouched subgraph edges | `internal/app/workspace_merge_test.go`, `tests/integration/workspaces_test.go` | Package-to-package edges preserved for filtered install |
| Transactional member restore | `tests/integration/snapshot_workspace_test.go`, `tests/integration/snapshot_crash_test.go` | No live member writes before commit; workspace restore crash matrix |
| Member manifest paths | `internal/snapshot/member_path_test.go` | Strict `ParseMemberManifestPath` contract |
| Restore consistency | `internal/snapshot/validate.go` | v2 member/lock/importer consistency before restore |
| Lifecycle timeout typing | `internal/lifecycle/run_test.go` | `DeadlineExceeded` / `Canceled` preserved |

## Stabilization pass 17 suites (lock bridge / MVP 0023)

| Area | Tests | What it proves |
|------|-------|----------------|
| Snapshot instances | `internal/compat/pnpm/graph_peer_test.go` | Peer-context package keys in canonical graph |
| Strict ref resolution | `internal/compat/pnpm/refresolve_test.go` | No importer fallback; dangling refs abort |
| Fixture verify | `tools/conformance/verify-fixtures` | SHA-256 + pin metadata for all generated families |
| pnpm mutation conformance | `tests/conformance/lock_bridge_pnpm_test.go` | 7 families × 3 majors: frozen after add/update/remove, node_modules import, txn restore |
| Nub families | `tests/conformance/lock_bridge_pnpm_test.go` | Six derived fixtures (all parse + validate; workspace included) |
| Txn failure injection | `internal/app/lock_txn_test.go` | Backup/publish/staging/encode failures preserve incumbent |
| No-CGO gate | `internal/archcheck/nocgo_test.go`, CI `no-cgo-gate` | Production builds with `CGO_ENABLED=0` |
| Fuzz / limits | `internal/compat/pnpm/fuzz_test.go`, `limits_test.go` | Hostile YAML, package keys, index caps |

CI jobs: `no-cgo-gate`, `fixture-verify`, `conformance-pnpm-{9,10,11}` (parse + `MutationSuite`),
`conformance-pnpm-unsupported`, `conformance-nub-fixtures`.

```powershell
$env:CGO_ENABLED = "0"
go run ./tools/conformance/verify-fixtures
go test ./tests/conformance/... -count=1 -run TestLockBridge
go test ./internal/compat/pnpm/... ./internal/app/... -count=1
```

Non-race CI jobs set `CGO_ENABLED=0`; race jobs remain the only CGO exception.

## Stabilization pass 15 suites (lock bridge / MVP 0023)

| Area | Tests | What it proves |
|------|-------|----------------|
| Txn-only incumbent writes | `internal/app/lock_txn_test.go` | No live `nub.lock`/`pnpm-lock.yaml` writes outside install txn |
| Migration fail-closed | `internal/app/lock_txn_test.go`, `tests/integration/lock_bridge_test.go` | Loss report + no incumbent overwrite |
| pnpm conformance | `tests/conformance/lock_bridge_pnpm_test.go` | Parse, Mew validate, byte no-op, pnpm frozen against local registry |
| Nub fixtures | `tests/conformance/lock_bridge_pnpm_test.go` | Deterministic parse/validate |
| Input limits / fuzz | `internal/compat/pnpm/limits_test.go`, `fuzz_test.go` | Size/depth/duplicate-key rejection |

CI jobs: `conformance-pnpm-9`, `conformance-pnpm-10`, `conformance-pnpm-11`, `conformance-nub-fixtures`.

### Runner certification (MVP 0046)

Harness package: [`tests/conformance/runner/`](../tests/conformance/runner/).

Manifest: [`tests/conformance/runner-matrix/manifest.json`](../tests/conformance/runner-matrix/manifest.json).

```powershell
$env:CGO_ENABLED = "0"
go test ./internal/conformance/... -count=1
go test ./tests/conformance/runner/... -count=1
go run ./cmd/m conformance run runner --json
go run ./cmd/m benchmark runner --json --profile smoke
```

CI jobs: `conformance-runner-linux`, `conformance-runner-windows`,
`conformance-runner-macos`, `conformance-runner-aggregate`.

Docs: [`runner-compatibility.md`](../docs/runner-compatibility.md),
[`runner-waivers.md`](../docs/runner-waivers.md).

### CLI UX certification (UX-0008)

Manifest: [`tests/conformance/cli-ux/manifest.json`](../tests/conformance/cli-ux/manifest.json).

```powershell
$env:CGO_ENABLED = "0"
go run ./cmd/m conformance run cli-ux --json
```

Evidence and performance: [`docs/evidence/cli-ux/`](evidence/cli-ux/),
[`plans/ux/performance-baseline.md`](../plans/ux/performance-baseline.md).
Architecture: [`architecture/cli-presentation.md`](architecture/cli-presentation.md).

```powershell
go test ./internal/app/... -run Merge -count=1
go test ./internal/snapshot/... -count=1
go test ./internal/lifecycle/... -count=1
go test ./tests/integration/... -run "WorkspaceFilter|SnapshotWorkspace" -count=1
go test -tags crash ./tests/integration/... -run WorkspaceSnapshotRestoreCrash -count=1 -timeout 30m
```

## Stabilization pass 18 suites (Node launch / runtime)

| Area | Tests | What it proves |
|------|-------|----------------|
| Node discovery and version | `internal/node/discover_test.go` | Stock Node detection, PATH fallback, version parsing, capabilities |
| Node argument parsing | `internal/runtime/nodeargs_test.go` | V8 flag partitioning, value-taking flags, entrypoint detection, app-arg separation |
| Entrypoint resolution | `internal/runtime/entrypoint_test.go` | JS/CJS/ESM detection, missing file, directory, unsupported extension |
| Augmentation argv building | `internal/runtime/nodeargs_test.go` | Preload injection order, zero-augmentation bypass |
| CLI dispatch integration | `internal/cli/dispatch_test.go` | File-run dispatch, `--node` flag, gate on/off, builtin precedence |

```powershell
go test ./internal/runtime/... -count=1
go test ./internal/node/... -count=1
go test ./internal/cli/... -run "FileRun|NodeFlag|RuntimeGate" -count=1
```

## Stabilization pass 13 suites (workspace / lifecycle / snapshot)

| Area | Tests | What it proves |
|------|-------|----------------|
| Directed closure merge | `internal/app/workspace_merge_test.go` | Filtered install keeps untouched importer packages; deterministic merge |
| Filtered remove | `internal/app/workspace_remove_test.go`, `tests/integration/workspaces_test.go` | Transactional member-only remove; `ERR_M_NOT_FOUND` |
| Update filter rejection | `internal/app/update_test.go`, `tests/integration/workspaces_test.go` | `update --filter` → `ERR_M_USAGE` |
| Lifecycle timeout source | `internal/lifecycle/timeout_test.go` | Config-only timeout; no ambient `os.Getenv` |
| Snapshot v2 members | `internal/snapshot/store_test.go`, `tests/integration/snapshot_workspace_test.go` | `manifests/` capture, v1 compat, workspace member restore |

## Stabilization pass 11 suites (invocation snapshot completeness)

| Area | Package / path | What it proves |
|---|---|---|
| EnvSnapshot semantics | `internal/config/env_snapshot_test.go`, `internal/config/load_spec_test.go` | `Initialized()` vs zero-value; initialized-empty never reads ambient |
| Empty env isolation | `internal/app/context_test.go`, `internal/app/mutation_session_test.go` | `Options.Env: []string{}` blocks host `MEW_*` and tokens after reload |
| Registry auth snapshot | `internal/config/auth_token_test.go`, `internal/app/auth_snapshot_test.go` | Packument + tarball `Authorization` from invocation token; Windows casing |
| Store prune scan roots | `internal/app/store_prune_test.go`, `tests/integration/store_prune_snapshot_test.go` | `MEW_HOME` from snapshot, not ambient, for prune manifest scan |
| Install warning sections | `internal/app/finish_cleanup_test.go` | Critical + non-critical + store sections emit independently |

## Stabilization pass 10 suites (config path resolution + CLI output)

| Area | Package / path | What it proves |
|---|---|---|
| Config path resolution | `internal/config/paths_test.go`, `internal/config/global_path_test.go` | `ResolveConfigPath` against invocation CWD; `IsPathWithin` project-root classification; frozen `GlobalConfigPathFromEnv` |
| App config classification | `internal/app/context_test.go`, `internal/app/mutation_session_test.go` | Monorepo `--config` vs project root; env snapshot global path; reload preserves frozen paths after `chdir` |
| CLI JSON output | `internal/cli/install_cmd_test.go` | Single JSON document (no post-encode prose); warning-only vs critical field presence |
| Human cleanup output | `internal/app/finish_cleanup_test.go` | Warning-only vs critical `FormatInstallSummary` messages |
| Abort cleanup severity | `internal/app/abort_test.go` | `populateAbortCleanup` critical vs warning code handling |

## Stabilization pass 9 suites (config load spec + cleanup severity)

| Area | Package / path | What it proves |
|---|---|---|
| Config load spec | `internal/config/load_spec_test.go`, `internal/app/mutation_session_test.go` | `ConfigLoadSpec` clone/immutability; explicit `--config` path/env/CLI preservation; missing/malformed explicit config on reload |
| Config reload during lock wait | `tests/integration/mutation_config_wait_test.go` | `app.New` before config rewrite; custom `custom.jsonc` (not default `m.jsonc`); malformed config during lock wait |
| Cleanup severity | `internal/transaction/finish_result_test.go`, `internal/app/finish_cleanup_test.go` | `CriticalCleanupError` vs `WarningErrors`; warning-only committed finish; critical recovery semantics |
| CLI cleanup output | `internal/cli/install_cmd_test.go` | Warning-only vs critical JSON fields |

## Stabilization pass 8 suites (config ordering + cleanup chain)

| Area | Package / path | What it proves |
|---|---|---|
| Config reload during lock wait | `tests/integration/mutation_config_wait_test.go` | Cross-process lock-wait ordering (superseded by pass 9 custom-config sync) |
| AppContext ordering API | `internal/app/mutation_session_test.go` | `AppContext` errors before `ReopenProject`; linker/registry reload on shared-context isolation |
| Cleanup error chain | `internal/transaction/finish_result_test.go`, `internal/app/abort_test.go`, `internal/cli/install_cmd_test.go` | Critical cleanup in error chain; CLI JSON cleanup fields |

## Stabilization pass 6 suites (hard-fix durability)

| Area | Package / path | What it proves |
|---|---|---|
| Mutation session ordering | `internal/app/mutation_session_test.go`, `mutation_prepare_test.go` | Lock before live reads; abort/finish release ownership |
| Verified current cleanup | `internal/transaction/current_cleanup_test.go`, `recovery_ownership_test.go` | `current` head + generation files cleared with verification |
| Windows reparse backup | `internal/transaction/reparse_backup.go`, `internal/fsx/reparse_windows_test.go` | Junction backup sidecar round-trip |
| Store lock cleanup | `internal/store/lock_cleanup_test.go` | Import lock release warnings; not-owner paths |
| Snapshot restore under lock | `tests/integration/snapshot_restore_test.go` | `m rollback` / restore via mutation session |
| Publish file durability | `internal/transaction/publish_file_test.go` | Atomic file publish under transaction |

## Stabilization pass 5 suites (hard-fix integration)

| Area | Package / path | What it proves |
|---|---|---|
| Mutation ordering | `tests/integration/mutation_ordering_test.go` | Concurrent `m add`; incomplete-txn recovery before resolve; lock-wait cancellation |
| Crash matrix (ordering hooks) | `tests/integration/txn_crash_test.go`, `snapshot_crash_test.go`, `update_crash_test.go` | `MEW_TXN_CRASH_AT` through backup/publish/commit/finish/recovery/rollback boundaries |
| npm SRI / content keys | `internal/contentid/contentid_test.go`, `internal/store/import_helper_test.go`, `internal/store/helpers_test.go` | npm `dist.integrity` base64 and hex → normalized `algo/hex` store keys |
| Isolated layout + rollback | `tests/integration/isolated_test.go`, `install_test.go` (`TestInstallFailurePreservesOldTree`) | Virtual store topology; failed install leaves prior tree intact |
| ABA proc takeover | `internal/fsx/lockdir_aba_proc_test.go` | Cross-process stale lock takeover blocked when live lock reappears (project/import/index) |

Run integration suites without `-short` (proc tests skip under `-short`). Crash-matrix files use the `crash` build tag and are excluded from default `go test ./...`:

```powershell
go test ./tests/integration/... -count=1
go test -tags crash ./tests/integration/... -count=1 -run Crash -timeout 30m
```

## Stabilization pass 3 suites (0016–0020 hard correctness)

| Area | Package / path | What it proves |
|---|---|---|
| Mutation preflight | `internal/transaction/preflight_test.go`, `preflight_proc_test.go` | Scan incomplete txns, recover before begin, stale lock + concurrent install |
| ABA-safe lock takeover | `internal/fsx/lockdir_aba_proc_test.go` | Tombstone rename; live lock survives stale cleanup race |
| Owner-safe release | `internal/fsx/lockdir_release_test.go` | Failed release leaves lock dir intact |
| Process crash matrix | `tests/integration/txn_crash_test.go`, `snapshot_crash_test.go`, `update_crash_test.go` | `MEW_TXN_CRASH_AT` at resolve/fetch/link/lockfile/validate/backup/publish/commit/finish boundaries |
| Atomic snapshot restore | `tests/integration/snapshot_restore_test.go` | Single-txn restore; live tree unchanged on failure |
| Policy parity | `internal/resolver/policy_parity_test.go`, `policy_drift_test.go` | Install/update same graph; fingerprint drift disables reuse |
| Index + collisions | `internal/store/index_proc_test.go`, `treemanifest_security_test.go` | Cross-process index lock; portable path collision reject |

## Stabilization pass 2 suites (0017–0020)

| Area | Package / path | What it proves |
|---|---|---|
| Project lock contention | `internal/transaction/lock_proc_test.go` | 20-process exclusive lock, stale recovery, ctx cancel |
| Crash recovery | `internal/transaction/inject_test.go`, `tests/integration/txn_inject_test.go`, `tests/integration/txn_crash_test.go`, `tests/integration/snapshot_crash_test.go`, `tests/integration/update_crash_test.go` | Kill-boundary recovery, idempotent `m recover`, auto-recover on retry |
| Store import locks | `internal/store/import_proc_test.go` | External `.locks/<algo>/<hex>.lock`, GC safety |
| Tree manifest security | `internal/store/treemanifest_security_test.go` | Bidirectional verify, hostile manifests, legacy re-import |
| Graph aliases | `internal/graph/alias_test.go`, `fixtures/resolver/aliases/` | `Edge.Name` round-trip |
| Peer nearest + instances | `internal/resolver/peers_nearest_test.go`, `peers_instances_test.go` | Nearest provider, dual peer-context nodes |
| Incremental update | `internal/resolver/incremental_diff_test.go` | Edge-keyed closure, fingerprint drift |
| Path guards | `internal/fsx`, `internal/transaction/paths_test.go` | Ancestor symlink/junction rejection |
| Registry cancel | `internal/registry/bounded_test.go` | `Packument` / `Packuments` ctx cancellation |
| Isolated linker | `internal/linker/isolated/fixture_test.go`, `tests/integration/isolated_test.go` | StoreID, phantom `require()` |

Cross-process tests use file-based coordination and bounded timeouts; they may be
slower on shared CI runners.

Race checks (when CGO enabled):

```powershell
$env:CGO_ENABLED = "1"
go test -race ./internal/transaction/... ./internal/store/... ./internal/resolver/... -count=1
```

On Windows without CGO, race builds are skipped; concurrency is covered by
cross-process lock/import tests instead.

## Failure injection

- `FaultyRoundTripper` — network cut after N requests
- `LimitedWriter` — `ENOSPC` after N bytes

## Filesystem probes

`ProbeFS` reports symlink support, junction (Windows stub), and case sensitivity.

## Required metadata for differential runs

Record in reports or job logs: OS, Go version (`go env GOVERSION`), and
reference tool versions (`npm --version`, `pnpm --version`, `nub --version`
when present).

## CI policy

| Suite | Normal PR CI | Scheduled / 0080 |
|---|---|---|
| `go test ./...` | yes (hermetic; linux/macOS/windows matrix) | yes |
| `go test -race ./...` | yes (ubuntu) | yes |
| `race-windows` job | `transaction`, `store`, `fsx` on windows-latest | yes |
| `crash-integration` job | `go test -tags crash ./tests/integration/... -run Crash -count=1 -timeout 30m` on ubuntu + windows (no `-short`) | yes |
| `platform-lock` job | cross-process lock + ABA proc tests on all OS (darwin uses `x/sys/unix` flock) | yes |
| `golangci-lint run` | yes (Ubuntu) | yes |
| Fixture registry | local only | local only |
| Public npm | never | never |
| Large ecosystem corpus | no | yes |
| Reference PM differential | skip if absent | pin tools in image |
