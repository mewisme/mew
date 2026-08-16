# 0056 — Runtime MVP 7 — Debugging, Inspection, and Runtime Diagnostics

## Document Control

| Item | Detail |
|---|---|
| Phase | Runtime / MVP 7 |
| Primary objective | Integrate Node inspector, source maps, transform traces, module traces, and support bundles for production-quality debugging. |
| Required predecessors | 0052, 0053, 0055 |
| Primary binaries | `m`, `mx` where applicable |
| Implementation language | Go for control plane and package-manager internals; embedded JavaScript only where Node extension APIs require it |
| Status at plan creation | Planned |

## Objective

Integrate Node inspector, source maps, transform traces, module traces, and support bundles for production-quality debugging.

This MVP must be independently reviewable, testable, and releasable behind an explicit experimental gate when it changes public behavior. It must leave stable interfaces for later MVPs and avoid shortcuts that make lockfile compatibility, transactional installation, or cross-platform support impossible.

## Sequence and Dependencies

- Complete and merge 0052 before starting this MVP.
- Complete and merge 0053 before starting this MVP.
- Complete and merge 0055 before starting this MVP.

Work inside this MVP should normally follow this order:

1. Freeze behavior and data contracts with fixtures and interface tests.
2. Implement the smallest vertical path that reaches a user-visible result.
3. Add failure injection and cross-platform coverage before broadening features.
4. Measure performance and resource use before declaring the MVP complete.
5. Record intentional differences from Nub and from incumbent package managers.

## Nub Reference Mapping

- Nub Node/V8 flag injection and diagnostics

The Nub implementation is a behavioral reference, not a source-level porting target. Mew must preserve observable semantics where parity is intentional while selecting Go-native architecture, concurrency, storage, and error-handling patterns.

## User-Visible Surface

```bash
m --inspect app.ts
```
```bash
m --inspect-brk app.ts
```
```bash
m runtime trace app.ts
```
```bash
m doctor runtime
```

## In Scope

- Node inspector flags and port handling.
- Source-map support across transforms and loaders.
- Transform, cache, env, module, worker, and watch traces.
- Safe support bundles.
- Compatibility opt-out comparison.

## Explicit Non-Goals

- Do not implement features assigned to later indexed MVPs.
- Do not silently diverge from a documented compatibility contract.
- Do not optimize by weakening integrity, determinism, or crash safety.

## Architecture and Interfaces

- Pass Node inspector semantics through rather than emulating them.
- Trace output must be structured and redact secrets/source content according to policy.
- Diagnostic features must not materially change runtime ordering.

Every new package must expose narrow interfaces, accept `context.Context` for cancellable work, avoid global mutable state, and return typed errors carrying stable Mew error codes. Persistent formats must be versioned and deterministic.

## Detailed Implementation Checklist

### Contracts & types

- [x] Route --inspect and --inspect-brk flags to stock Node unchanged
- [x] Implement module and transform timing diagnostic views
- [x] Add inspector startup and break-on-start tests
- [x] Publish safe defaults for CI (no inspect bind to 0.0.0.0)

### Core logic

- [x] Handle inspector port allocation and collision diagnostics
- [x] Implement cache explain command for transpile cache
- [x] Test mapped breakpoints and stack traces in TS/TSX
- [x] Freeze trace schema before 0057 stabilization

### CLI / UX

- [x] Integrate source-map support across transforms and loaders
- [x] Implement support bundle collection with redaction policy
- [x] Benchmark trace overhead when diagnostics enabled
- [x] Add doctor runtime checks for common misconfigurations

### Tests & fixtures

- [x] Define runtime trace event schema with versioning
- [x] Ensure traces do not materially change runtime ordering
- [x] Document debugger configuration for common editors

### Docs & observability

- [x] Emit transform, cache, env, module, worker, and watch trace events
- [x] Redact secrets and sensitive source content per policy
- [x] Compare behavior against m --node opt-out baseline

## Test Plan

<!-- ENRICHMENT-TESTS -->
- [x] Acceptance: m --inspect-brk app.ts breaks on first line with mapped sources
- [x] Acceptance: Stack traces map through transforms to original TypeScript
- [x] Acceptance: Support bundles contain no secrets or full source by default
- [x] Acceptance: Trace output validates against published schema
- [x] Acceptance: Diagnostics do not change execution order materially
- [x] Fixture ready: `fixtures/debug/inspect — inspector startup/break`
- [x] Fixture ready: `fixtures/debug/breakpoints — mapped TS/TSX stacks`
- [x] Fixture ready: `fixtures/debug/trace-redaction — secrets stripped`
- [x] Fixture ready: `fixtures/debug/support-bundle — archive format golden`
- [x] Fixture ready: `fixtures/debug/overhead — trace cost benchmarks`


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
| Node inspector passthrough | Nub V8 flags | --inspect/--inspect-brk | 0056 |
| Source-map debugging | Nub | mapped breakpoints/stacks | 0056 |
| Runtime traces | Nub diagnostics | transform/module/cache traces | 0056 |
| Support bundles | Nub | redacted diagnostic archives | 0056 |

## Go Package Map

**Packages / paths:**

- `internal/runtime`
- `internal/transform`
- `cmd/m (doctor, trace)`

**Forbidden import edges:**

- internal/resolver
- internal/linker

## Data Flow

```mermaid
flowchart LR
  flowchart LR
  flags[InspectorFlags] --> node[StockNodeInspector]
  traces[TraceEvents] --> schema[RuntimeTraceSchema]
  maps[SourceMaps] --> dbg[DebuggerMapping]
  bundle[SupportBundle] --> redact[RedactionPolicy]
```

## Commands and Flags

| Command | Flags | Notes |
|---|---|---|
| `m --inspect app.ts` | `--inspect`, `--inspect-brk` | Pass through to Node |
| `m runtime trace app.ts` | trace subcommand | Structured runtime events |
| `m doctor runtime` | doctor | Health and compatibility report |
| `m cache explain` | explain | Transpile cache introspection |

Pass Node inspector semantics; do not emulate debugger protocol.

## Persistent Artifacts

| Artifact | Purpose |
|---|---|
| Runtime trace event schema | Versioned structured diagnostics |
| Support bundle format | Redacted support archive spec |

## Concrete Test Fixtures

- `fixtures/debug/inspect — inspector startup/break`
- `fixtures/debug/breakpoints — mapped TS/TSX stacks`
- `fixtures/debug/trace-redaction — secrets stripped`
- `fixtures/debug/support-bundle — archive format golden`
- `fixtures/debug/overhead — trace cost benchmarks`

## Acceptance Scenarios

1. m --inspect-brk app.ts breaks on first line with mapped sources
2. Stack traces map through transforms to original TypeScript
3. Support bundles contain no secrets or full source by default
4. Trace output validates against published schema
5. Diagnostics do not change execution order materially

## Nub Conformance Targets

- Nub Node/V8 flag injection | parity
- Nub diagnostics and traces | parity
- Nub source-map debugging | parity

## Open Decisions

- Default support bundle contents vs opt-in verbosity
- Whether runtime trace ships enabled in CI problem matchers

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
