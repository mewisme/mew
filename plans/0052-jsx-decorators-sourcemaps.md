# 0052 — Runtime MVP 3 — JSX, Decorators, and Source-Map Parity

## Document Control

| Item | Detail |
|---|---|
| Phase | Runtime / MVP 3 |
| Primary objective | Complete transform behavior for JSX/TSX, automatic runtimes, legacy and standard decorators, metadata emission, and production-quality diagnostics. |
| Required predecessors | 0051 |
| Primary binaries | `m`, `mx` where applicable |
| Implementation language | Go for control plane and package-manager internals; embedded JavaScript only where Node extension APIs require it |
| Status at plan creation | Planned |

## Objective

Complete transform behavior for JSX/TSX, automatic runtimes, legacy and standard decorators, metadata emission, and production-quality diagnostics.

This MVP must be independently reviewable, testable, and releasable behind an explicit experimental gate when it changes public behavior. It must leave stable interfaces for later MVPs and avoid shortcuts that make lockfile compatibility, transactional installation, or cross-platform support impossible.

## Sequence and Dependencies

- Complete and merge 0051 before starting this MVP.

Work inside this MVP should normally follow this order:

1. Freeze behavior and data contracts with fixtures and interface tests.
2. Implement the smallest vertical path that reaches a user-visible result.
3. Add failure injection and cross-platform coverage before broadening features.
4. Measure performance and resource use before declaring the MVP complete.
5. Record intentional differences from Nub and from incumbent package managers.

## Nub Reference Mapping

- Nub JSX options
- Nub decorators and `emitDecoratorMetadata` behavior
- Nub source-map integration

The Nub implementation is a behavioral reference, not a source-level porting target. Mew must preserve observable semantics where parity is intentional while selecting Go-native architecture, concurrency, storage, and error-handling patterns.

## User-Visible Surface

```bash
m component.tsx
```
```bash
m decorator-example.ts
```

## In Scope

- Classic and automatic JSX runtimes.
- jsxImportSource and development mode.
- Standard decorators and legacy TypeScript decorators.
- Decorator metadata strategy.
- Inline/external source maps and stack trace mapping.
- Transform warnings and unsupported-option diagnostics.

## Explicit Non-Goals

- Do not implement features assigned to later indexed MVPs.
- Do not silently diverge from a documented compatibility contract.
- Do not optimize by weakening integrity, determinism, or crash safety.

## Architecture and Interfaces

- Document exact TypeScript-compiler differences; Mew is a transpiler, not a type checker.
- Treat decorator metadata as a separately certified capability.
- Keep source maps stable and cache-keyed by relevant transform options.

Every new package must expose narrow interfaces, accept `context.Context` for cancellable work, avoid global mutable state, and return typed errors carrying stable Mew error codes. Persistent formats must be versioned and deterministic.

## Detailed Implementation Checklist

### Contracts & types

- [x] Implement JSX option normalization (classic, automatic, importSource, dev)
- [x] Implement inline and external source map generation
- [x] Add transform parity report command for debugging
- [x] Document exact differences from TypeScript compiler

### Core logic

- [x] Support React, Preact, and custom JSX runtimes via tsconfig
- [x] Implement source-map chaining across loader stages
- [x] Test React/Preact/custom JSX fixture projects
- [x] Treat decorator metadata as separately certified capability

### CLI / UX

- [x] Implement or integrate standard decorator transforms
- [x] Define source content inclusion policy for maps
- [x] Test decorator framework fixtures (legacy + standard)
- [x] Benchmark JSX/decorator transform hot paths

### Tests & fixtures

- [x] Implement legacy TypeScript decorator compatibility path
- [x] Implement diagnostic code frames pointing to original sources
- [x] Verify stack traces through imports and async functions

### Docs & observability

- [x] Research and choose decorator metadata emission strategy
- [x] Add transform warnings and unsupported-option diagnostics
- [x] Include JSX/decorator options in transpile cache keys

## Test Plan

<!-- ENRICHMENT-TESTS -->
- [x] Acceptance: m component.tsx runs with correct JSX runtime per tsconfig
- [x] Acceptance: Legacy and standard decorators transpile for supported frameworks
- [x] Acceptance: Stack traces map to original TSX/TS sources
- [x] Acceptance: Transform parity report lists known divergences
- [x] Acceptance: Cache keys change when relevant JSX/decorator options change
- [x] Fixture ready: `fixtures/transform/jsx-react — automatic runtime`
- [x] Fixture ready: `fixtures/transform/jsx-preact — importSource custom`
- [x] Fixture ready: `fixtures/transform/decorators-standard — TC39 decorators`
- [x] Fixture ready: `fixtures/transform/decorators-legacy — experimental legacy`
- [x] Fixture ready: `fixtures/transform/sourcemaps — stack trace mapping`


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
| JSX/TSX transforms | Nub JSX options | classic + automatic runtimes | 0052 |
| Decorators | Nub | standard + legacy TS decorators | 0052 |
| emitDecoratorMetadata | Nub | separately certified capability | 0052 |
| Source maps | Nub | inline/external + stack mapping | 0052 |

## Go Package Map

**Packages / paths:**

- `internal/transform`
- `internal/runtime`
- `cmd/m`

**Forbidden import edges:**

- internal/resolver
- internal/linker

## Data Flow

```mermaid
flowchart LR
  flowchart LR
  src[TSX/Decorators] --> norm[OptionNormalize]
  norm --> xform[TransformPipeline]
  xform --> maps[SourceMaps]
  maps --> diag[Diagnostics]
  diag --> node[StockNode]
```

## Commands and Flags

| Command | Flags | Notes |
|---|---|---|
| `m component.tsx` | jsx from tsconfig | React/Preact/custom runtime |
| `m decorator-example.ts` | decorator options | legacy + standard |
| `m transform report` | debug subcommand | parity report for transforms |

Mew is transpiler not type checker; document TS compiler differences.

## Persistent Artifacts

| Artifact | Purpose |
|---|---|
| Transform parity report output | Unsupported/divergent feature inventory |
| Source map chain metadata | Cache keys include transform options |

## Concrete Test Fixtures

- `fixtures/transform/jsx-react — automatic runtime`
- `fixtures/transform/jsx-preact — importSource custom`
- `fixtures/transform/decorators-standard — TC39 decorators`
- `fixtures/transform/decorators-legacy — experimental legacy`
- `fixtures/transform/sourcemaps — stack trace mapping`

## Acceptance Scenarios

1. m component.tsx runs with correct JSX runtime per tsconfig
2. Legacy and standard decorators transpile for supported frameworks
3. Stack traces map to original TSX/TS sources
4. Transform parity report lists known divergences
5. Cache keys change when relevant JSX/decorator options change

## Nub Conformance Targets

- Nub JSX options | parity
- Nub decorators and emitDecoratorMetadata | parity where certified
- Nub source-map integration | parity

## Open Decisions

- Decorator metadata: Go-native vs embedded JS vs native helper
- Default jsxImportSource when absent from tsconfig

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
