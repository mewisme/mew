# 0054 — Runtime MVP 5 — Environment Loading, Workers, Storage, and Modern APIs

## Document Control

| Item | Detail |
|---|---|
| Phase | Runtime / MVP 5 |
| Primary objective | Provide Nub-style environment-file loading and selected browser-compatible APIs without violating plain Node semantics or worker boundaries. |
| Required predecessors | 0050, 0053 |
| Primary binaries | `m`, `mx` where applicable |
| Implementation language | Go for control plane and package-manager internals; embedded JavaScript only where Node extension APIs require it |
| Status at plan creation | Planned |

## Objective

Provide Nub-style environment-file loading and selected browser-compatible APIs without violating plain Node semantics or worker boundaries.

This MVP must be independently reviewable, testable, and releasable behind an explicit experimental gate when it changes public behavior. It must leave stable interfaces for later MVPs and avoid shortcuts that make lockfile compatibility, transactional installation, or cross-platform support impossible.

## Sequence and Dependencies

- Complete and merge 0050 before starting this MVP.
- Complete and merge 0053 before starting this MVP.

Work inside this MVP should normally follow this order:

1. Freeze behavior and data contracts with fixtures and interface tests.
2. Implement the smallest vertical path that reaches a user-visible result.
3. Add failure injection and cross-platform coverage before broadening features.
4. Measure performance and resource use before declaring the MVP complete.
5. Record intentional differences from Nub and from incumbent package managers.

## Nub Reference Mapping

- Nub `.env` auto-discovery and explicit env-file precedence
- Nub worker propagation and Web Storage behavior

The Nub implementation is a behavioral reference, not a source-level porting target. Mew must preserve observable semantics where parity is intentional while selecting Go-native architecture, concurrency, storage, and error-handling patterns.

## User-Visible Surface

```bash
m --env-file .env.local app.ts
```
```bash
m --no-env-file app.ts
```
```bash
m --mode production app.ts
```

## In Scope

- Mode-aware `.env*` discovery, variable expansion, explicit env files, and kill switch.
- Shell environment precedence and per-child overlays.
- Worker inheritance of transform and runtime hooks.
- Web Storage compatibility where Nub provides it.
- Runtime cache and configuration propagation.

## Explicit Non-Goals

- Do not implement features assigned to later indexed MVPs.
- Do not silently diverge from a documented compatibility contract.
- Do not optimize by weakening integrity, determinism, or crash safety.

## Architecture and Interfaces

- Never mutate global process environment from concurrent Go code; construct child environments explicitly.
- Explicit env-file flags suppress auto-discovery according to documented policy.
- Worker augmentation must avoid recursively starting unrelated services.

Every new package must expose narrow interfaces, accept `context.Context` for cancellable work, avoid global mutable state, and return typed errors carrying stable Mew error codes. Persistent formats must be versioned and deterministic.

## Detailed Implementation Checklist

### Contracts & types

- [x] Implement .env parser with variable expansion rules
- [x] Never mutate global process environment from concurrent Go code
- [x] Wire NODE_ENV and --mode interaction documented
- [x] Test storage isolation and corruption recovery

### Core logic

- [x] Implement mode-aware .env* discovery and precedence
- [x] Inject runtime state into worker threads and child Node processes
- [x] Add environment trace diagnostics with redacted values by default
- [x] Document security implications of env expansion

### CLI / UX

- [x] Support explicit --env-file and --no-env-file kill switch
- [x] Ensure worker augmentation avoids recursive unrelated services
- [x] Test precedence and expansion matrix exhaustively
- [x] Benchmark env overlay construction per spawn

### Tests & fixtures

- [x] Define shell environment vs file vs flag precedence
- [x] Implement selected Web Storage compatibility APIs
- [x] Prepare watch reload hooks for env/tsconfig changes (0055)

### Docs & observability

- [x] Construct per-child environment overlays explicitly in Go
- [x] Define storage persistence and isolation policy
- [x] Test worker and child-process propagation

## Test Plan

<!-- ENRICHMENT-TESTS -->
- [x] Acceptance: --env-file overrides auto-discovery per documented policy
- [x] Acceptance: Child processes receive explicit env overlays; parent env not raced
- [x] Acceptance: Workers inherit transform/runtime hooks without recursive services
- [x] Acceptance: Env trace redacts secrets by default
- [x] Acceptance: Web Storage APIs behave per documented persistence policy
- [x] Fixture ready: `fixtures/runtime/env/precedence — shell/file/flag matrix`
- [x] Fixture ready: `fixtures/runtime/env/expansion — variable substitution`
- [x] Fixture ready: `fixtures/runtime/env/mode — production vs development files`
- [x] Fixture ready: `fixtures/runtime/workers — hook propagation`
- [x] Fixture ready: `fixtures/runtime/storage — Web Storage isolation`


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
| .env auto-discovery | Nub | mode-aware .env* loading | 0054 |
| Env precedence | Nub | shell vs file vs flags | 0054 |
| Worker propagation | Nub | inherit runtime hooks in workers | 0054 |
| Web Storage APIs | Nub | selected browser-compatible APIs | 0054 |

## Go Package Map

**Packages / paths:**

- `internal/runtime`
- `internal/config`
- `cmd/m`

**Forbidden import edges:**

- internal/resolver
- internal/linker

## Data Flow

```mermaid
flowchart LR
  flowchart LR
  discover[EnvDiscovery] --> parse[EnvParser]
  parse --> merge[PrecedenceMerge]
  merge --> child[ChildEnvOverlay]
  child --> worker[WorkerPropagation]
```

## Commands and Flags

| Command | Flags | Notes |
|---|---|---|
| `m --env-file .env.local app.ts` | `--env-file` | Explicit env file |
| `m --no-env-file app.ts` | `--no-env-file` | Kill auto-discovery |
| `m --mode production app.ts` | `--mode` | Mode-aware .env* selection |

Never mutate global process env from concurrent Go code.

## Persistent Artifacts

| Artifact | Purpose |
|---|---|
| Env trace diagnostic format | Redacted precedence chain |
| Storage persistence policy doc | Web Storage data locations |

## Concrete Test Fixtures

- `fixtures/runtime/env/precedence — shell/file/flag matrix`
- `fixtures/runtime/env/expansion — variable substitution`
- `fixtures/runtime/env/mode — production vs development files`
- `fixtures/runtime/workers — hook propagation`
- `fixtures/runtime/storage — Web Storage isolation`

## Acceptance Scenarios

1. --env-file overrides auto-discovery per documented policy
2. Child processes receive explicit env overlays; parent env not raced
3. Workers inherit transform/runtime hooks without recursive services
4. Env trace redacts secrets by default
5. Web Storage APIs behave per documented persistence policy

## Nub Conformance Targets

- Nub .env discovery and precedence | parity
- Nub worker propagation | parity
- Nub Web Storage behavior | parity where provided

## Open Decisions

- Default auto-discovery enablement in CI
- Which Web Storage APIs ship in v1 vs deferred

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
