# 0057 — Runtime Stabilization Gate

## Document Control

| Item | Detail |
|---|---|
| Phase | Runtime / Stabilization |
| Primary objective | Certify Mew runtime augmentation across supported Node versions, syntax features, module systems, workers, loaders, watch mode, and debugging. |
| Required predecessors | 0050, 0051, 0052, 0053, 0054, 0055, 0056 |
| Primary binaries | `m`, `mx` where applicable |
| Implementation language | Go for control plane and package-manager internals; embedded JavaScript only where Node extension APIs require it |
| Status at plan creation | Planned |

## Objective

Certify Mew runtime augmentation across supported Node versions, syntax features, module systems, workers, loaders, watch mode, and debugging.

This MVP must be independently reviewable, testable, and releasable behind an explicit experimental gate when it changes public behavior. It must leave stable interfaces for later MVPs and avoid shortcuts that make lockfile compatibility, transactional installation, or cross-platform support impossible.

## Sequence and Dependencies

- Complete and merge 0050 before starting this MVP.
- Complete and merge 0051 before starting this MVP.
- Complete and merge 0052 before starting this MVP.
- Complete and merge 0053 before starting this MVP.
- Complete and merge 0054 before starting this MVP.
- Complete and merge 0055 before starting this MVP.
- Complete and merge 0056 before starting this MVP.

Work inside this MVP should normally follow this order:

1. Freeze behavior and data contracts with fixtures and interface tests.
2. Implement the smallest vertical path that reaches a user-visible result.
3. Add failure injection and cross-platform coverage before broadening features.
4. Measure performance and resource use before declaring the MVP complete.
5. Record intentional differences from Nub and from incumbent package managers.

## Nub Reference Mapping

- Complete Nub runtime and watch surface

The Nub implementation is a behavioral reference, not a source-level porting target. Mew must preserve observable semantics where parity is intentional while selecting Go-native architecture, concurrency, storage, and error-handling patterns.

## User-Visible Surface

```bash
m conformance run runtime
```
```bash
m benchmark runtime
```

## In Scope

- Syntax and framework corpus.
- Node version certification.
- CJS/ESM/loader/worker/watch coverage.
- Startup and warm-cache performance.
- Fallback and known-limitations documentation.

## Explicit Non-Goals

- Do not implement features assigned to later indexed MVPs.
- Do not silently diverge from a documented compatibility contract.
- Do not optimize by weakening integrity, determinism, or crash safety.

## Architecture and Interfaces

- Stable runtime support requires explicit parity results, not only successful demos.

Every new package must expose narrow interfaces, accept `context.Context` for cancellable work, avoid global mutable state, and return typed errors carrying stable Mew error codes. Persistent formats must be versioned and deterministic.

## Detailed Implementation Checklist

### Contracts & types

- [x] Run syntax and framework corpus across supported Node versions
- [x] Freeze runtime protocol versions (transform IPC, trace, loader bridge)
- [x] Document fallback behavior and known limitations
- [x] Record waivers with owners for documented divergences

### Core logic

- [ ] Certify CJS/ESM/loader/worker/watch coverage with published results *(pending: CI exact-head certification observation for final commit)*
- [ ] Publish runtime support matrix with certification evidence *(pending: CI exact-head certification observation)*
- [x] Integrate runtime conformance into CI stop-the-line gates
- [x] Ensure plain Node escape hatch remains behaviorally plain

### CLI / UX

- [x] Run Node compatibility and --node opt-out differential tests
- [x] Verify no transform cache corruption or source-map integrity bugs
- [x] Run long-running worker/watch multi-day soak
- [x] Gate experimental runtime features behind explicit flags

### Tests & fixtures

- [x] Soak transform service for crashes, leaks, and IPC failure recovery
- [x] Verify watch and workers do not leak processes or file descriptors
- [x] Sign off interfaces for 0060 Node manager integration

### Docs & observability

- [x] Complete security review of IPC and embedded runtime assets
- [x] Run cold/warm startup benchmark suite with baselines
- [x] Update feature inventory to shipped for certified runtime features

## Test Plan

<!-- ENRICHMENT-TESTS -->
- [ ] Acceptance: Supported syntax and Node versions have published certification *(pending: CI exact-head certification observation)*
- [x] Acceptance: No known transform cache corruption or source-map integrity bug
- [x] Acceptance: Watch and workers pass leak soak without orphaned processes
- [x] Acceptance: Plain Node escape hatch matches stock node within tolerance
- [ ] Acceptance: Runtime conformance passes on Linux, macOS, Windows *(pending: CI observation)*
- [x] Fixture ready: `tests/conformance/runtime/syntax-corpus — language features`
- [x] Fixture ready: `tests/conformance/runtime/frameworks — React/etc smoke`
- [x] Fixture ready: `tests/conformance/runtime/node-matrix — version certification`
- [x] Fixture ready: `tests/conformance/runtime/transform-soak — service crash/leak`
- [x] Fixture ready: `tests/conformance/runtime/opt-out — plain Node differential`


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

- [ ] Supported syntax and Node versions have published certification results. *(pending: CI exact-head certification observation)*
- [x] No known transform cache corruption or source-map integrity bug.
- [x] Watch and workers do not leak processes, services, or file descriptors.
- [x] Plain Node escape hatch remains behaviorally plain.



<!-- ENRICHMENT:BEGIN -->

## Feature Inventory Links

Rows this MVP owns or primarily advances (from `0002` inventory themes):

| Feature | Nub baseline | Mew target | Primary MVP |
|---|---|---|---|
| Runtime stabilization gate | Nub runtime surface | Certify 0050-0056 | 0057 |
| Node version certification | Nub | supported floor + matrix | 0057 |
| Transform service soak | Nub OXC | crash/leak certification | 0057 |
| Plain Node escape hatch | Nub --node | behavioral plainness | 0057 |

## Go Package Map

**Packages / paths:**

- `internal/runtime`
- `internal/transform`
- `tests/conformance/runtime`
- `cmd/m`

**Forbidden import edges:**

- internal/resolver (new features)
- internal/linker (mutations)

## Data Flow

```mermaid
flowchart LR
  flowchart LR
  corpus[SyntaxFrameworkCorpus] --> rt[RuntimeStack]
  rt --> cert[NodeVersionCert]
  cert --> soak[TransformSoak]
  soak --> matrix[SupportMatrix]
```

## Commands and Flags

| Command | Flags | Notes |
|---|---|---|
| `m conformance run runtime` | suite selector | Full runtime conformance |
| `m benchmark runtime` | `--cold`, `--warm` | Startup and cache baselines |

Blocks manager/product stable channels until exit criteria met.

## Persistent Artifacts

| Artifact | Purpose |
|---|---|
| Runtime support matrix | Certified Node versions and syntax features |
| Frozen runtime protocol versions | IPC, loader, trace schemas |
| Known limitations document | Explicit parity gaps |

## Concrete Test Fixtures

- `tests/conformance/runtime/syntax-corpus — language features`
- `tests/conformance/runtime/frameworks — React/etc smoke`
- `tests/conformance/runtime/node-matrix — version certification`
- `tests/conformance/runtime/transform-soak — service crash/leak`
- `tests/conformance/runtime/opt-out — plain Node differential`

## Acceptance Scenarios

1. Supported syntax and Node versions have published certification
2. No known transform cache corruption or source-map integrity bug
3. Watch and workers pass leak soak without orphaned processes
4. Plain Node escape hatch matches stock node within tolerance
5. Runtime conformance passes on Linux, macOS, Windows

## Nub Conformance Targets

- Complete Nub runtime and watch surface | parity except documented gaps
- Mew Go transform vs Nub OXC | divergence with parity report

## Open Decisions

- Waivers for transformer features below OXC parity
- Date to declare runtime stable vs experimental

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
