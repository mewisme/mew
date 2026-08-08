# 0055 — Runtime MVP 6 — Dependency-Aware Watch Mode

## Document Control

| Item | Detail |
|---|---|
| Phase | Runtime / MVP 6 |
| Primary objective | Restart applications safely when relevant source, configuration, environment, or package dependencies change while preserving terminal and signal behavior. |
| Required predecessors | 0040, 0053, 0054 |
| Primary binaries | `m`, `mx` where applicable |
| Implementation language | Go for control plane and package-manager internals; embedded JavaScript only where Node extension APIs require it |
| Status at plan creation | Planned |

## Objective

Restart applications safely when relevant source, configuration, environment, or package dependencies change while preserving terminal and signal behavior.

This MVP must be independently reviewable, testable, and releasable behind an explicit experimental gate when it changes public behavior. It must leave stable interfaces for later MVPs and avoid shortcuts that make lockfile compatibility, transactional installation, or cross-platform support impossible.

## Sequence and Dependencies

- Complete and merge 0040 before starting this MVP.
- Complete and merge 0053 before starting this MVP.
- Complete and merge 0054 before starting this MVP.

Work inside this MVP should normally follow this order:

1. Freeze behavior and data contracts with fixtures and interface tests.
2. Implement the smallest vertical path that reaches a user-visible result.
3. Add failure injection and cross-platform coverage before broadening features.
4. Measure performance and resource use before declaring the MVP complete.
5. Record intentional differences from Nub and from incumbent package managers.

## Nub Reference Mapping

- Nub watch supervisor, dependency graph watch, env-file forwarding, and Windows path normalization

The Nub implementation is a behavioral reference, not a source-level porting target. Mew must preserve observable semantics where parity is intentional while selecting Go-native architecture, concurrency, storage, and error-handling patterns.

## User-Visible Surface

```bash
m watch src/index.ts
```
```bash
m watch --clear-screen=false app.ts
```

## In Scope

- Long-lived supervisor and short-lived application child.
- Dependency graph discovery from transform and module resolution.
- Watch tsconfig extends chains, package.json, explicit and auto-discovered env files, and configured globs.
- Debounce, restart coalescing, clear-screen policy, and restart-on-demand.
- Native watcher fallback and polling mode.

## Explicit Non-Goals

- Do not implement features assigned to later indexed MVPs.
- Do not silently diverge from a documented compatibility contract.
- Do not optimize by weakening integrity, determinism, or crash safety.

## Architecture and Interfaces

- Supervisor does not execute user application code.
- Every restart rebuilds child environment and runtime state.
- Normalize short/long paths, case, and symlinks for watcher identity.

Every new package must expose narrow interfaces, accept `context.Context` for cancellable work, avoid global mutable state, and return typed errors carrying stable Mew error codes. Persistent formats must be versioned and deterministic.

## Detailed Implementation Checklist

### Contracts & types

- [x] Implement watcher abstraction with native and polling backends
- [x] Implement clear-screen policy flag
- [x] Handle atomic save, rename, delete/recreate edge cases
- [x] Run resource leak soak on watch sessions

### Core logic

- [x] Implement long-lived supervisor and short-lived application child
- [x] Implement restart-on-demand interactive key
- [x] Reload env and tsconfig changes without supervisor crash
- [x] Benchmark watcher CPU use on large trees

### CLI / UX

- [x] Collect dependency files from transform and module resolution hooks
- [x] Rebuild child environment and runtime state on every restart
- [x] Never execute user application code in supervisor process
- [x] Document platform watcher limitations

### Tests & fixtures

- [x] Watch tsconfig extends chains, package.json, env files, and globs
- [x] Implement restart state machine with signal escalation
- [x] Add rapid change and restart storm tests

### Docs & observability

- [x] Implement debounce and restart coalescing policy
- [x] Normalize short/long paths, case, and symlinks for watcher identity
- [x] Test child ignoring termination and forced kill paths

## Test Plan

<!-- ENRICHMENT-TESTS -->
- [x] Acceptance: m watch restarts app when relevant source or config changes
- [x] Acceptance: Supervisor survives env/tsconfig reloads
- [x] Acceptance: Debouncing prevents restart storms on rapid saves
- [x] Acceptance: No process or file descriptor leaks in soak tests
- [x] Acceptance: TTY and signal behavior preserved across restarts
- [x] Fixture ready: `fixtures/watch/atomic-save — editor save patterns`
- [x] Fixture ready: `fixtures/watch/symlink-case — path identity normalization`
- [x] Fixture ready: `fixtures/watch/restart-storm — debounce coalescing`
- [x] Fixture ready: `fixtures/watch/stubborn-child — signal escalation`
- [x] Fixture ready: `fixtures/watch/deps-graph — hook-collected dependencies`


Required test layers:

- Unit tests for parsing, normalization, deterministic ordering, and error classification.
- Golden tests for manifests, lockfiles, command output, and migration reports.
- Integration tests against local fixture registries and isolated temporary homes.
- Failure-injection tests for network interruption, disk exhaustion, permission errors, process termination, and corrupted cache entries.
- Cross-platform tests for Linux, macOS, and Windows, including path length, case sensitivity, junctions, symlinks, and executable shims.
- Conformance tests comparing intentional compatibility surfaces with the corresponding Nub or package-manager behavior.

## Performance Requirements

- Add benchmarks for every newly introduced hot path.
- Avoid unbounded goroutines, file descriptors, memory growth, or registry requests.
- Publish baseline measurements in repository benchmark artifacts.

All performance claims must be backed by reproducible benchmark commands, machine metadata, cold/warm cache separation, and multiple samples. Performance regressions on critical paths require an explicit waiver.

## Security and Trust Requirements

- Validate all external input and fail closed on malformed or ambiguous data.
- Use least-privilege filesystem access and redact credentials in diagnostics.
- Maintain integrity verification before extraction or execution.

Secrets must never be written to logs, lockfiles, snapshots, telemetry, crash reports, or plan files. Archive extraction, script execution, registry authentication, and path construction must be treated as hostile-input boundaries.

## Risks and Mitigations

- Compatibility drift: mitigate with fixture-based conformance tests.
- Cross-platform divergence: mitigate with platform-specific CI and filesystem probes.
- Premature abstraction: require at least two concrete callers before generalizing an interface.

## Deliverables

- [x] Production implementation and public interfaces.
- [x] Unit, integration, conformance, and failure-injection tests.
- [x] User documentation and migration notes where behavior is public.
- [x] Benchmark baseline and diagnostic instrumentation.

## Exit Criteria

- [x] All required tests pass on supported operating systems.
- [x] No unresolved correctness, integrity, or data-loss issue remains.
- [x] Public behavior and intentional deviations are documented.
- [x] The next dependent MVP can consume stable interfaces without reaching into internals.



<!-- ENRICHMENT:BEGIN -->

## Feature Inventory Links

Rows this MVP owns or primarily advances (from `0002` inventory themes):

| Feature | Nub baseline | Mew target | Primary MVP |
|---|---|---|---|
| m watch | Nub watch supervisor | dependency-aware restarts | 0055 |
| Dependency graph watch | Nub | from transform + resolution | 0055 |
| Debounce/restart policy | Nub | coalescing and clear-screen | 0055 |
| Watcher backends | Nub | native + polling fallback | 0055 |

## Go Package Map

**Packages / paths:**

- `internal/runtime`
- `internal/process`
- `cmd/m (watch)`

**Forbidden import edges:**

- internal/resolver
- internal/linker

## Data Flow

```mermaid
flowchart LR
  flowchart LR
  sup[WatchSupervisor] --> watch[FileWatcher]
  watch --> deps[DependencyCollect]
  deps --> restart[RestartStateMachine]
  restart --> child[AppChildProcess]
```

## Commands and Flags

| Command | Flags | Notes |
|---|---|---|
| `m watch src/index.ts` | entry file | Long-lived supervisor |
| `m watch --clear-screen=false app.ts` | `--clear-screen` | Terminal UX policy |

Supervisor never executes user application code directly.

## Persistent Artifacts

| Artifact | Purpose |
|---|---|
| Watch event diagnostic schema | Restart reason tracing |
| Dependency file index | Collected from runtime hooks |

## Concrete Test Fixtures

- `fixtures/watch/atomic-save — editor save patterns`
- `fixtures/watch/symlink-case — path identity normalization`
- `fixtures/watch/restart-storm — debounce coalescing`
- `fixtures/watch/stubborn-child — signal escalation`
- `fixtures/watch/deps-graph — hook-collected dependencies`

## Acceptance Scenarios

1. m watch restarts app when relevant source or config changes
2. Supervisor survives env/tsconfig reloads
3. Debouncing prevents restart storms on rapid saves
4. No process or file descriptor leaks in soak tests
5. TTY and signal behavior preserved across restarts

## Nub Conformance Targets

- Nub watch supervisor | parity
- Nub dependency graph watch | parity
- Nub Windows path normalization in watch | parity

## Open Decisions

- Default debounce interval
- Polling fallback trigger conditions on each OS

<!-- ENRICHMENT:END -->

## AI-Agent Handoff Contract

- Read 0000, 0003, 0005, 0007, 0008, and the immediate predecessor before changing code.
- Prefer small vertical pull requests over broad mechanical ports.
- Never copy Rust architecture blindly; preserve behavior and invariants using idiomatic Go.
- Update the feature matrix and conformance inventory when behavior changes.

Before submitting work, an agent must provide:

1. A concise behavior summary and the exact compatibility target.
2. A list of files and public interfaces changed.
3. Commands used for tests, benchmarks, and static analysis.
4. Known gaps, deferred cases, and platform limitations.
5. Evidence that generated files and fixtures are deterministic.
6. A rollback note for any persistent-format or migration change.
