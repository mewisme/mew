# 0050–0051 Runtime foundation certification

Gate C certification evidence for the 0050 (Node launch) and 0051 (Go
transform service) runtime foundation.

## Certification summary

| Field | Value |
|---|---|
| Date | 2026-08-06 |
| Branch | `worktree-order-implement` (historical; commit no longer reachable from any branch) |
| Certified commit SHA | `6831061ab58f7d387c71d0fd9619489e6eff17f7` (⚠️ no longer present in repository) |
| Go version | 1.26.5 |
| esbuild version | 0.28.1 |
| Status | **GREEN** |

## CI evidence

| Run | ID | URL | Result |
|---|---|---|---|
| Normal CI (PR #24) | 31098685304 | https://github.com/mewisme/mew/actions/runs/31098685304 | success |
| Full certification | 31098844245 | https://github.com/mewisme/mew/actions/runs/31098844245 | success |

Both runs verified against the exact same commit SHA (`6831061`).

## Platforms tested (12 combinations)

| Platform | Node version | Result |
|---|---|---|
| Linux (amd64) | 18.x (maintenance) | Pass |
| Linux (amd64) | 20.x (LTS) | Pass |
| Linux (amd64) | 22.x (current) | Pass |
| Linux (amd64) | 24.x (next) | Pass |
| macOS (arm64) | 18.x (maintenance) | Pass |
| macOS (arm64) | 20.x (LTS) | Pass |
| macOS (arm64) | 22.x (current) | Pass |
| macOS (arm64) | 24.x (next) | Pass |
| Windows (amd64) | 18.x (maintenance) | Pass |
| Windows (amd64) | 20.x (LTS) | Pass |
| Windows (amd64) | 22.x (current) | Pass |
| Windows (amd64) | 24.x (next) | Pass |

All 12 combinations exercised by the full workflow runtime matrix
(`full.yml`, `runtime-node-matrix` job). Each entry produced a
machine-readable certification report uploaded as an artifact.

## Certification artifacts

Individual artifacts: `runtime-certification-{os}-node-{version}`
(12 artifacts, one per matrix entry).

Aggregate artifact: `runtime-certification-aggregate.ndjson`
(produced by the `runtime certification aggregate` job, validates exactly
12 reports with no missing, duplicate, malformed, failed, or stale entries).

Final gate: `full certification gate` depends on the aggregate job and
the rest of the full workflow. All gates passed.

## Runtime test commands (as executed by the workflow)

```bash
# Unit tests (runtime, transform, node, process, runner, workspace,
# lifecycle, CLI)
go test ./internal/runtime/... ./internal/transform/... ./internal/node/... \
  ./internal/process/... ./internal/runner/... ./internal/workspace/... \
  ./internal/lifecycle/... ./internal/cli/... -count=1 -timeout 15m

# E2E integration tests
go test ./tests/integration/... -count=1 \
  -run 'RuntimeE2E|NodeVersion|ScriptWinsOverFile|WorkingDir|CWD|Workspace.*Run|Lifecycle' \
  -v -timeout 20m
```

## E2E scenarios covered (26+ tests)

| Scenario | Tests |
|---|---|
| JS entrypoints (.js/.mjs/.cjs) | 3 |
| TS entrypoints (.ts/.mts/.cts) | 3 |
| ESM import resolution | 1 |
| Entrypoint argument forwarding | 1 |
| Node/V8 flag forwarding | 1 |
| Syntax error (JS) | 1 |
| Syntax error (TS) | 1 |
| Non-zero exit code propagation | 1 |
| Zero augmentation (--node) | 1 |
| Script wins over file-run | 1 |
| .jsx deferred to 0052 | 1 |
| .tsx deferred to 0052 | 1 |
| Cold cache → warm cache | 1 |
| Corrupt cache recovery | 1 |
| Source maps | 1 |
| Spaces in path | 1 |
| Unicode in path | 1 |
| node-args invocation style | 1 |
| mx rejects file-run | 1 |
| Cancellation/signal | 1 |
| Gate off | 1 |
| Transform credential isolation | 1 |
| Script CWD is project root | 1 |
| Script CWD with --cwd | 1 |
| Script CWD from subdirectory | 1 |
| Script CWD lifecycle (pre/build/post) | 1 |
| Script CWD failed script context | 1 |

All 26+ scenarios verified on all 12 OS/Node combinations.

## Benchmark results (Linux amd64, Node v22.x)

```bash
go test -bench=. ./internal/runtime/... ./internal/transform/... -benchtime=5x -count=1
```

| Benchmark | ns/op |
|---|---|
| AssetExtractionCold | ~814k |
| AssetVerifyWarm | ~252k |
| PlanJS | ~5.2M |
| BuildArgv | ~6k |
| EngineTransform | ~1.4M |
| CacheWriteRead | ~237k |
| CacheKeyDeterminism | ~6k |

Note: benchmark results are from local Linux amd64 (not CI). Automated
regression comparison deferred (see limitations).

## Known limitations

1. **Worker thread credential isolation**: Node worker threads created by user
   code inherit `process.env` after credential stripping (credentials already
   stripped by `preload.mjs`). Explicit worker `transferList` configuration
   deferred.
2. **Package-style tsconfig extends**: Returns an actionable error; resolution
   deferred to future plan.
3. **Performance regression gate**: Runtime benchmarks recorded but no automated
   regression comparison yet. Add when baseline stabilizes.
4. **cmd.exe inline quoting**: Windows package scripts that use inline `node -e`
   with double-quoted arguments are mangled by `cmd.exe /d /s /c` shell
   dispatch. Use standalone `.js` files for cross-platform scripts. A
   production fix for smarter Windows shell quoting is deferred.

## Bugs fixed during certification

- **Credential isolation order**: `preload.cjs` was deleting `MEW_TRANSFORM_*`
  env vars before `loader-register.mjs` could create the loader thread, causing
  the loader thread's `process.env` copy to lack credentials. Fixed by moving
  credential stripping to `preload.mjs` only (runs after loader thread creation).
  See `internal/runtime/assets/preload.cjs`, `preload.mjs`.
- **IsRuntimeFile disregarding deferred extensions**: `.tsx`/`.jsx` were not
  recognized as runtime files, so the dispatcher fell through to "unknown
  command" instead of returning the plan-0052 deferral message. Fixed by
  checking `nextPlanExts` in `IsRuntimeFile`.
  See `internal/runtime/entrypoint.go`.
- **node-args missing TS transform contribution**: The `node-args` subcommand
  did not build a transform contribution for TS entrypoints, causing "no
  endpoint or token" errors. Fixed by adding `buildTransformContribution` call
  in `nodeargs_cmd.go`.
- **Windows script CWD mismatch**: `app.Run` passed `ac.CWD` as
  `runner.RunOptions.ProjectRoot` instead of `proj.Root`, causing
  `INIT_CWD` and subprocess CWD to derive from different canonical paths
  on Windows (separate `filepath.Abs` calls could produce inconsistently
  normalized paths). Fixed by using `proj.Root` in `internal/app/run.go`.
- **macOS symlink path comparison**: `samePath` in E2E tests used
  `filepath.Clean` without resolving symlinks, so `/var` (symlink to
  `/private/var`) did not compare equal to `/private/var` on macOS.
  Fixed by adding `filepath.EvalSymlinks`.
- **Windows cache permission tests**: `os.Chmod 0o555` does not prevent
  file creation inside the directory on Windows; three tests in
  `internal/transform` relied on Unix permission semantics. Skipped on
  Windows with explicit rationale.
- **Windows E2E inline Node quoting**: Six E2E tests used `node -e "…"`
  inline commands that were mangled by `cmd.exe` shell dispatch on Windows.
  Rewrote tests to use standalone `.js` files (consistent with existing
  spaces/Unicode tests that already passed cross-platform).
