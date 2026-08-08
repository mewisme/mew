# 0053 — Runtime MVP 4 — Module Resolution, Path Aliases, and Custom Loaders

## Document Control

| Item | Detail |
|---|---|
| Phase | Runtime / MVP 4 |
| Primary objective | Match Node resolution while adding TypeScript path aliases, extension mapping, custom loader chaining, and package-manager layout awareness. |
| Required predecessors | 0019, 0025, 0052 |
| Primary binaries | `m`, `mx` where applicable |
| Implementation language | Go for control plane and package-manager internals; embedded JavaScript only where Node extension APIs require it |
| Status at plan creation | Planned |

## Objective

Match Node resolution while adding TypeScript path aliases, extension mapping, custom loader chaining, and package-manager layout awareness.

This MVP must be independently reviewable, testable, and releasable behind an explicit experimental gate when it changes public behavior. It must leave stable interfaces for later MVPs and avoid shortcuts that make lockfile compatibility, transactional installation, or cross-platform support impossible.

## Sequence and Dependencies

- Complete and merge 0019 before starting this MVP.
- Complete and merge 0025 before starting this MVP.
- Complete and merge 0052 before starting this MVP.

Work inside this MVP should normally follow this order:

1. Freeze behavior and data contracts with fixtures and interface tests.
2. Implement the smallest vertical path that reaches a user-visible result.
3. Add failure injection and cross-platform coverage before broadening features.
4. Measure performance and resource use before declaring the MVP complete.
5. Record intentional differences from Nub and from incumbent package managers.

## Nub Reference Mapping

- Nub module.register hooks, tsconfig paths, extension mapping, and custom loader support
- Nub PnP runtime helpers

The Nub implementation is a behavioral reference, not a source-level porting target. Mew must preserve observable semantics where parity is intentional while selecting Go-native architecture, concurrency, storage, and error-handling patterns.

## User-Visible Surface

```bash
m app.ts --loader ./loader.mjs
```
```bash
m resolve-module @app/core
```

## In Scope

- Node CJS and ESM resolution preservation.
- tsconfig paths/baseUrl behavior.
- `.js` to `.ts` development mapping policy.
- Custom ESM loader and preload chaining.
- Isolated node_modules and Yarn PnP awareness.
- Package imports/exports, conditions, self-reference, and URL modules only where explicitly supported.

## Explicit Non-Goals

- Do not implement features assigned to later indexed MVPs.
- Do not silently diverge from a documented compatibility contract.
- Do not optimize by weakening integrity, determinism, or crash safety.

## Architecture and Interfaces

- Do not replace Node resolution wholesale when a hook can make a narrow augmentation.
- Custom loaders run in documented order and receive original user arguments.
- Resolver errors preserve Node-compatible context plus Mew explanation.

Every new package must expose narrow interfaces, accept `context.Context` for cancellable work, avoid global mutable state, and return typed errors carrying stable Mew error codes. Persistent formats must be versioned and deterministic.

## Detailed Implementation Checklist

### Contracts & types

- [x] Plan resolver augmentation without replacing Node resolution wholesale
- [x] Implement ESM custom loader and preload chaining
- [x] Support package imports/exports and conditions where explicitly adopted
- [x] Test CJS/ESM interop and monorepo path alias fixtures

### Core logic

- [x] Preserve Node CJS and ESM resolution semantics baseline
- [x] Document and enforce custom loader execution order
- [x] Implement self-reference and URL module policy boundaries
- [x] Test custom loader composition scenarios

### CLI / UX

- [x] Implement tsconfig baseUrl and paths matcher
- [x] Pass original user loader arguments through chain
- [x] Preserve Node-compatible error context plus Mew explanations
- [x] Benchmark resolution hot path with cache

### Tests & fixtures

- [x] Implement .js to .ts development extension mapping policy
- [x] Implement isolated node_modules layout awareness
- [x] Add module trace diagnostics command

### Docs & observability

- [x] Implement CJS require registration hooks where needed
- [x] Implement Yarn PnP runtime resolution adapter
- [x] Test Node package exports/imports corpus

## Test Plan

<!-- ENRICHMENT-TESTS -->
- [x] Acceptance: tsconfig paths resolve consistently in monorepos
- [x] Acceptance: Custom loaders run in documented order with user args preserved
- [x] Acceptance: PnP projects resolve modules through adapter
- [x] Acceptance: Resolution errors include Node context and Mew guidance
- [x] Acceptance: Plain Node opt-out bypasses Mew resolution hooks
- [x] Fixture ready: `fixtures/resolution/exports — package exports corpus`
- [x] Fixture ready: `fixtures/resolution/cjs-esm — interop matrix`
- [x] Fixture ready: `fixtures/resolution/paths — tsconfig path aliases`
- [x] Fixture ready: `fixtures/resolution/pnp — Yarn PnP projects`
- [x] Fixture ready: `fixtures/resolution/custom-loader — loader chaining`


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
| Module resolution hooks | Nub module.register | Narrow Node augmentation | 0053 |
| tsconfig paths | Nub | baseUrl and paths matcher | 0053 |
| Custom loaders | Nub | ESM loader/preload chaining | 0053 |
| PnP runtime adapter | Nub PnP helpers | Layout-aware resolution | 0053 |

## Go Package Map

**Packages / paths:**

- `internal/runtime`
- `runtime/`
- `internal/compat`
- `cmd/m`

**Forbidden import edges:**

- internal/resolver (install-time)
- internal/linker (mutations)

## Data Flow

```mermaid
flowchart LR
  flowchart LR
  req[ImportRequest] --> hook[ResolverHook]
  hook --> paths[TsconfigPaths]
  paths --> layout[LayoutAdapter]
  layout --> load[LoaderChain]
  load --> node[NodeResolution]
```

## Commands and Flags

| Command | Flags | Notes |
|---|---|---|
| `m app.ts --loader ./loader.mjs` | `--loader` | Custom ESM loader chain |
| `m resolve-module @app/core` | debug subcommand | Resolution trace |

Preserve Node CJS/ESM semantics; augment via hooks only.

## Persistent Artifacts

| Artifact | Purpose |
|---|---|
| Module trace diagnostic format | Structured resolution logs |
| Loader ordering manifest | Documented hook chain |

## Concrete Test Fixtures

- `fixtures/resolution/exports — package exports corpus`
- `fixtures/resolution/cjs-esm — interop matrix`
- `fixtures/resolution/paths — tsconfig path aliases`
- `fixtures/resolution/pnp — Yarn PnP projects`
- `fixtures/resolution/custom-loader — loader chaining`

## Acceptance Scenarios

1. tsconfig paths resolve consistently in monorepos
2. Custom loaders run in documented order with user args preserved
3. PnP projects resolve modules through adapter
4. Resolution errors include Node context and Mew guidance
5. Plain Node opt-out bypasses Mew resolution hooks

## Nub Conformance Targets

- Nub module.register and tsconfig paths | parity
- Nub custom loader support | parity
- Nub PnP runtime helpers | parity

## Open Decisions

- .js to .ts mapping default in development vs production
- URL import support scope for v1

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
