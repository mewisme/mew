# Mew Implementation Master Checklist

## Program status

- Current MVP: **0052** — Runtime MVP 3 — JSX, Decorators, and Source-Map Parity
- Last updated: 2026-08-07
- Source of truth: per-MVP files under `plans/00xx-*.md`
- Regenerate: `python3 plans/scripts/enrich_and_generate.py`
- Runtime certification: pending (0057 stabilization gate not yet reached)

## Do now

**Next:** [0052 - Runtime MVP 3 — JSX, Decorators, and Source-Map Parity](0052-jsx-decorators-sourcemaps.md)

<!-- CHECKLIST:NARRATIVE:BEGIN -->
**Stabilization pass 17 (2026-08-02):** Runner/runtime stabilization — P0 defects fixed across 0040-0051: ExitStatus type for child exits (no ERR_M_INTERNAL), graceful cancellation with 10s grace period, workspace cancellation error, conditional TS loader injection, runtime asset pre-execution verification, transform cache wired into execution, concurrent loader request map, Phase A boolean parity, UsedFallback telemetry, corrupt environment recovery, 13 ERR_M_TRANSFORM_* codes, tsconfig wiring, manifest validation, file URL compliance, staging cleanup, timeout result discarding. 13 commits, 1576 tests pass. Evidence: [`docs/evidence/core/pass32-ci.md`](../docs/evidence/core/pass32-ci.md).

MVP 0021 lifecycle scripts shipped on `main`. Stabilization pass 11 complete on `67a0ed7` — CI https://github.com/mewisme/mew/actions/runs/30310739645 (21/21 green).

**Stabilization pass 12 (2026-07-28):** explicit-empty lifecycle env, honest restricted-execution contract, prepare cache disabled, transactional `add --filter`, filtered-install closure merge, install-family `--filter` audit. Evidence: [`docs/evidence/core/pass32-ci.md`](../docs/evidence/core/pass32-ci.md).

**Stabilization pass 13 (2026-07-28):** directed workspace closure merge, transactional `remove --filter`, `update --filter` rejection, config-only lifecycle timeout, snapshot schema v2 member manifests. Evidence: [`docs/evidence/core/pass32-ci.md`](../docs/evidence/core/pass32-ci.md).

**Stabilization pass 14 (2026-07-28):** module rename to `github.com/mewisme/mew`, untouched workspace subgraph edge preservation, transactional snapshot member restore, typed lifecycle timeouts, strict member manifest paths, v2 restore consistency validation. Evidence: [`docs/evidence/core/pass32-ci.md`](../docs/evidence/core/pass32-ci.md).

**Stabilization pass 15 (2026-07-28):** MVP 0023 lock bridge hardening — txn-only incumbent writes, detection evidence order, generated conformance fixtures, pnpm 9/10/11 + nub CI jobs, input limits/fuzz, migration reports. Evidence: [`docs/evidence/core/pass32-ci.md`](../docs/evidence/core/pass32-ci.md).

**Stabilization pass 16 (2026-07-28):** MVP 0023 completion — legacy reject, identity/ref resolve, field loss audit, ProjectHints wiring, binary-generated fixture families, mutation conformance, txn failure injection, CI unsupported job. Evidence: [`docs/evidence/core/pass32-ci.md`](../docs/evidence/core/pass32-ci.md).

Stabilization pass 10 merged to `main` at `d980e12`.

Stabilization pass 9 (`stabilization-pass-9` from `fae9b48`): `ConfigLoadSpec` preserves load inputs across mutation reload; `CriticalCleanupError`/`WarningErrors` split; custom-config lock-wait proc test.

Stabilization pass 8 complete 2026-07-28: merged `fae9b48`.
<!-- CHECKLIST:NARRATIVE:END -->

## MVP completion (65)

| ID | MVP | Phase | Predecessors | Status | Plan |
|----|-----|-------|--------------|--------|------|
| 0001 | Program Charter and Product Contract | Foundation | None | [x] | [0001](0001-program-charter.md) |
| 0002 | Complete Feature Inventory and Parity Matrix | Foundation | 0001 | [x] | [0002](0002-feature-inventory.md) |
| 0003 | Target Architecture and Rust-to-Go Boundaries | Foundation | 0001, 0002 | [x] | [0003](0003-target-architecture.md) |
| 0004 | Repository Bootstrap, Tooling, and Engineering Standards | Foundation | 0003 | [x] | [0004](0004-repository-bootstrap.md) |
| 0005 | Stable Error Model, Diagnostics, and Observability | Foundation | 0004 | [x] | [0005](0005-error-observability.md) |
| 0006 | Configuration and Project Identity Model | Foundation | 0004, 0005 | [x] | [0006](0006-configuration-identity.md) |
| 0007 | Canonical Data Model and Core Interfaces | Foundation | 0003, 0006 | [x] | [0007](0007-data-model-interfaces.md) |
| 0008 | Testing, Fixtures, Fuzzing, and Conformance Strategy | Foundation | 0004, 0007 | [x] | [0008](0008-testing-strategy.md) |
| 0009 | Release Train and MVP Dependency Graph | Foundation | 0001, 0002, 0003, 0008 | [x] | [0009](0009-release-train-overview.md) |
| 0010 | Core MVP 1 — CLI Foundation and Command Dispatch | Core / MVP 1 | 0004, 0005, 0006, 0007 | [x] | [0010](0010-cli-foundation.md) |
| 0011 | Core MVP 2 — Manifest Parsing and Project Discovery | Core / MVP 2 | 0010 | [x] | [0011](0011-manifest-project-discovery.md) |
| 0012 | Core MVP 3 — Registry Client and Metadata Cache | Core / MVP 3 | 0011 | [x] | [0012](0012-registry-cache.md) |
| 0013 | Core MVP 4 — npm Semver and Basic Dependency Resolver | Core / MVP 4 | 0012 | [x] | [0013](0013-semver-basic-resolver.md) |
| 0014 | Core MVP 5 — Tarball Fetch, Integrity, and Safe Extraction | Core / MVP 5 | 0012, 0013 | [x] | [0014](0014-fetch-integrity-extraction.md) |
| 0015 | Core MVP 6 — Native `m.lock` Format | Core / MVP 6 | 0007, 0013 | [x] | [0015](0015-m-lock.md) |
| 0016 | Core MVP 7 — Basic End-to-End Installer | Core / MVP 7 | 0011, 0013, 0014, 0015 | [x] | [0016](0016-basic-installer.md) |
| 0017 | Core MVP 8 — Transactional Install and Instant Rollback | Core / MVP 8 | 0016 | [x] | [0017](0017-transaction-rollback.md) |
| 0018 | Core MVP 9 — Global Content Store and Smart Filesystem Pl... | Core / MVP 9 | 0014, 0017 | [x] | [0018](0018-global-store-smart-linker.md) |
| 0019 | Core MVP 10 — Isolated Virtual Store and Node Modules Layout | Core / MVP 10 | 0018 | [x] | [0019](0019-isolated-linker.md) |
| 0020 | Core MVP 11 — Full Dependency Resolver | Core / MVP 11 | 0019 | [x] | [0020](0020-advanced-resolver.md) |
| 0021 | Core MVP 12 — Lifecycle Scripts, Trust, and Sandbox Policy | Core / MVP 12 | 0018, 0020 | [x] | [0021](0021-lifecycle-sandbox.md) |
| 0022 | Core MVP 13 — Workspaces, Catalogs, and Filtering | Core / MVP 13 | 0011, 0020, 0021 | [x] | [0022](0022-workspaces-catalogs.md) |
| 0023 | Core MVP 14 — Nub and pnpm Lockfile Bridge | Core / MVP 14 | 0015, 0020, 0022 | [x] | [0023](0023-nub-pnpm-lock-bridge.md) |
| 0024 | Core MVP 15 — npm Lockfile and Shrinkwrap Compatibility | Core / MVP 15 | 0023 | [x] | [0024](0024-npm-locks.md) |
| 0025 | Core MVP 16 — Bun and Yarn Lockfile Compatibility | Core / MVP 16 | 0023, 0024 | [x] | [0025](0025-bun-yarn-locks.md) |
| 0026 | Core MVP 17 — Complete Package-Manager Command Surface | Core / MVP 17 | 0021, 0022, 0023, 0024, 0025 | [x] | [0026](0026-pm-command-surface.md) |
| 0027 | Core MVP 18 — Advanced Sources, Patches, Pack, and Publish | Core / MVP 18 | 0026 | [x] | [0027](0027-advanced-sources-publish.md) |
| 0028 | Core MVP 19 — Explainability, Plans, Semantic Diffs, and ... | Core / MVP 19 | 0017, 0020, 0026 | [x] | [0028](0028-explain-plan-history.md) |
| 0029 | Core MVP 20 — Performance, Offline Operation, and Portabl... | Core / MVP 20 | 0018, 0026, 0028 | [x] | [0029](0029-performance-offline-capsules.md) |
| 0030 | Core MVP 21 — Audit, SBOM, Provenance, and Supply-Chain P... | Core / MVP 21 | 0012, 0021, 0027, 0029 | [x] | [0030](0030-security-audit-sbom.md) |
| 0031 | Core MVP 22 — Package-Manager Core Stabilization Gate | Core / Stabilization | 0010, 0011, 0012, 0013, 0014, 0015, 0016, 0017, 0018, 0019, 0020, 0021, 0022, 0023, 0024, 0025, 0026, 0027, 0028, 0029, 0030 | [x] | [0031](0031-core-stabilization.md) |
| 0040 | Runner MVP 1 — Package Script Runner | Runner / MVP 1 | 0031 | [x] | [0040](0040-script-runner.md) |
| 0041 | Runner MVP 2 — Workspace Script Orchestration | Runner / MVP 2 | 0022, 0040 | [x] | [0041](0041-workspace-runner.md) |
| 0042 | Runner MVP 3 — Direct `m <script>` Shortcuts | Runner / Mew Extension | 0010, 0040 | [x] | [0042](0042-direct-script-shortcuts.md) |
| 0043 | Runner MVP 4 — Local Package Binary Execution | Runner / MVP 4 | 0019, 0040 | [x] | [0043](0043-local-exec.md) |
| 0044 | Runner MVP 5 — `mx` Remote Fetch and Execution | Runner / MVP 5 | 0021, 0029, 0043 | [x] | [0044](0044-mx-dlx.md) |
| 0045 | Runner MVP 6 — Unified Execution and Snapshot Environments | Runner / MVP 6 | 0028, 0029, 0043, 0044 | [x] | [0045](0045-unified-execution.md) |
| 0046 | Runner Stabilization Gate | Runner / Stabilization | 0040, 0041, 0042, 0043, 0044, 0045 | [x] | [0046](0046-runner-stabilization.md) |
| 0050 | Runtime MVP 1 — Node Launch and Compatibility Boundary | Runtime / MVP 1 | 0046 | [x] | [0050](0050-node-launch-compat.md) |
| 0051 | Runtime MVP 2 — Go Transform Service and TypeScript Execu... | Runtime / MVP 2 | 0050 | [x] | [0051](0051-go-transform-service.md) |
| 0052 | Runtime MVP 3 — JSX, Decorators, and Source-Map Parity | Runtime / MVP 3 | 0051 | [ ] | [0052](0052-jsx-decorators-sourcemaps.md) |
| 0053 | Runtime MVP 4 — Module Resolution, Path Aliases, and Cust... | Runtime / MVP 4 | 0019, 0025, 0052 | [ ] | [0053](0053-module-resolution-loaders.md) |
| 0054 | Runtime MVP 5 — Environment Loading, Workers, Storage, an... | Runtime / MVP 5 | 0050, 0053 | [ ] | [0054](0054-env-modern-apis.md) |
| 0055 | Runtime MVP 6 — Dependency-Aware Watch Mode | Runtime / MVP 6 | 0040, 0053, 0054 | [ ] | [0055](0055-watch-mode.md) |
| 0056 | Runtime MVP 7 — Debugging, Inspection, and Runtime Diagno... | Runtime / MVP 7 | 0052, 0053, 0055 | [ ] | [0056](0056-debugging-inspection.md) |
| 0057 | Runtime Stabilization Gate | Runtime / Stabilization | 0050, 0051, 0052, 0053, 0054, 0055, 0056 | [ ] | [0057](0057-runtime-stabilization.md) |
| 0060 | Manager MVP 1 — Node Version Manager | Managers / MVP 1 | 0031, 0050 | [ ] | [0060](0060-node-manager.md) |
| 0061 | Manager MVP 2 — Package-Manager Meta-Manager | Managers / MVP 2 | 0023, 0024, 0025, 0060 | [ ] | [0061](0061-pm-manager.md) |
| 0062 | Manager MVP 3 — Node, PM, and Self Shims | Managers / MVP 3 | 0010, 0060, 0061 | [ ] | [0062](0062-shims.md) |
| 0070 | Product MVP 1 — TypeScript-First Project Initialization | Product / MVP 1 | 0011, 0031, 0051 | [ ] | [0070](0070-project-init.md) |
| 0071 | Product MVP 2 — External Command Plugin Convention | Product / MVP 2 | 0010, 0043, 0062 | [ ] | [0071](0071-plugins.md) |
| 0072 | Distribution MVP 1 — Releases, Installers, and Package Ch... | Distribution / MVP 1 | 0031, 0046, 0057, 0062 | [ ] | [0072](0072-installers-releases.md) |
| 0073 | Distribution MVP 2 — GitHub Action and CI Integration | Distribution / MVP 2 | 0029, 0060, 0072 | [ ] | [0073](0073-github-action.md) |
| 0074 | Distribution MVP 3 — Docker Images and Hosted Builder Int... | Distribution / MVP 3 | 0029, 0060, 0072 | [ ] | [0074](0074-docker-builders.md) |
| 0080 | Cross-Cutting — Compatibility and Conformance Program | Cross-Cutting | 0002, 0008 | [ ] | [0080](0080-conformance-program.md) |
| 0081 | Cross-Cutting — Performance and Resource Program | Cross-Cutting | 0008, 0010 | [ ] | [0081](0081-performance-program.md) |
| 0082 | Cross-Cutting — Threat Model and Security Review Plan | Cross-Cutting | 0003, 0005 | [ ] | [0082](0082-threat-model.md) |
| 0083 | Cross-Cutting — Nub Rust to Mew Go Migration Map | Cross-Cutting | 0002, 0003 | [ ] | [0083](0083-rust-go-migration-map.md) |
| 0084 | Cross-Cutting — Versioning, Formats, and Support Policy | Cross-Cutting | 0009 | [ ] | [0084](0084-release-versioning-policy.md) |
| 0085 | Cross-Cutting — Go Dependency Selection Roadmap | Cross-Cutting | 0003, 0004 | [ ] | [0085](0085-dependency-roadmap.md) |
| 0086 | Cross-Cutting — AI Agent Implementation Protocol | Cross-Cutting | 0004, 0008, 0009 | [ ] | [0086](0086-ai-agent-protocol.md) |
| 0087 | Cross-Cutting — Global Definition of Done | Cross-Cutting | 0009, 0080, 0081, 0082, 0084 | [ ] | [0087](0087-definition-of-done.md) |
| 0088 | Reference Index and Research Sources | Cross-Cutting | 0002, 0083 | [ ] | [0088](0088-reference-index.md) |
| 0089 | Open Research Spikes and Decision Gates | Cross-Cutting | 0003, 0085 | [ ] | [0089](0089-research-spikes.md) |
| 0090 | Future Extensions Beyond Nub Parity | Future | 0087 | [ ] | [0090](0090-future-backlog.md) |

## Aggregated tasks by MVP

### 0001 - Program Charter and Product Contract

- status: done
- plan: [0001-program-charter.md](0001-program-charter.md)

- [x] Write product charter covering Mew, Mewx, m.lock, and Nub parity goal
- [x] Define compatibility axes: CLI grammar, lockfile, config, runtime, layout
- [x] Document supported OS/arch and Node floor
- [x] Freeze binary, config, cache, env, and error-code naming conventions
- [x] Document experimental-feature and versioning policy
- [x] Create compatibility-state vocabulary: parity, intentional divergence, extension, deferred
- [x] Document dispatch precedence reserved for 0042 script shortcuts
- [x] Document existing-lockfile preservation and new-project m.lock default
- [x] List signature Mew differentiators with owning MVP IDs
- [x] Draft user-facing identity strings for --version placeholders
- [x] Draft migration narrative outline for npm/pnpm/Yarn/Bun/Nub users
- [x] Review charter against representative npm, pnpm, Bun, Yarn, and Nub projects
- [x] Verify every later INDEX module maps to an explicit product objective
- [x] Add charter consistency checklist used by later MVP reviews
- [x] Publish charter in docs/ and link from README/AGENTS.md
- [x] Create ADR template for irreversible decisions
- [x] Record open human-owned decisions with owners
- [x] Acceptance: Charter names m, mx, m.lock, and Nub as behavioral reference without source-port language
- [x] Acceptance: Compatibility axes table covers CLI, lockfile, config, runtime, and layout
- [x] Acceptance: Every INDEX MVP maps to at least one charter objective
- [x] Acceptance: Direct script shortcuts listed as intentional Mew extension
- [x] Acceptance: ADR process documented before any persistent format is designed
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0002 - Complete Feature Inventory and Parity Matrix

- status: done
- plan: [0002-feature-inventory.md](0002-feature-inventory.md)

- [x] Define versioned feature-inventory JSON schema
- [x] Define statuses: planned, in-progress, shipped, intentional-omit, deferred
- [x] Define compatibility_class: parity, extension, divergence, deferred
- [x] Require fields: id, module, nub_status, mew_status, primary_mvp, tests[]
- [x] Extract all public Nub commands, flags, config keys, and documented behaviors
- [x] Add Mew-only features from charter
- [x] Assign every feature to exactly one primary MVP
- [x] Link conformance test IDs where known
- [x] Specify m features table and JSON output shapes
- [x] Hide internal source paths from user-facing output
- [x] Schema validation tests
- [x] Inventory-to-command-tree consistency test (after 0010)
- [x] Inventory-to-documentation consistency test
- [x] CI fails when shipped commands are absent from inventory
- [x] Generate human-readable tables from inventory
- [x] Document how agents update inventory on behavior changes
- [x] Ensure every INDEX MVP owns at least one inventory row
- [x] Acceptance: Schema rejects inventory rows missing primary_mvp
- [x] Acceptance: Every INDEX MVP owns at least one inventory row
- [x] Acceptance: Mew extensions marked compatibility_class=extension
- [x] Acceptance: m features --format json validates against schema
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0003 - Target Architecture and Rust-to-Go Boundaries

- status: done
- plan: [0003-target-architecture.md](0003-target-architecture.md)

- [x] Produce full package map with one-line purpose per directory
- [x] Define core interfaces: Registry, Resolver, Store, Linker, LockfileAdapter, Transaction, ScriptRunner, ProcessSupervisor
- [x] Decide immutability boundaries and copy-on-write points
- [x] Specify transform IPC framing, auth, cancellation sketch
- [x] Define extension points without public plugin ABI
- [x] Document stock-Node augmentation boundary (no libnode fork)
- [x] Document resolve-complete-before-mutate rule
- [x] Map every Nub crate to Mew package or intentional omission
- [x] List forbidden import edges
- [x] Keep cmd/m and cmd/mx as thin entrypoints in the diagram
- [x] Document presentation vs domain separation
- [x] Compile-time or test-time import graph checks
- [x] Interface fakes proving independent testability
- [x] IPC round-trip sketch tests when protocol exists
- [x] Expand proposed repository tree to full listing
- [x] Link architecture from AGENTS.md
- [x] Record decisions that block later MVPs
- [x] Document embedded runtime asset digest verification and cache versioning
- [x] Acceptance: Every AGENTS.md package appears in the map
- [x] Acceptance: No cyclic dependency in the documented graph
- [x] Acceptance: JS surface limited to Node extension APIs
- [x] Acceptance: Transaction boundary documented for all install-family mutations
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0004 - Repository Bootstrap, Tooling, and Engineering Standards

- status: done
- plan: [0004-repository-bootstrap.md](0004-repository-bootstrap.md)

- [x] Choose Go minimum version and document it
- [x] Initialize module path and license headers
- [x] Define directory skeleton matching 0003
- [x] Add Makefile/task targets: test, vet, lint, race, fuzz-smoke, vuln
- [x] Pin golangci-lint and govulncheck versions
- [x] Create internal/testkit with temp home and fixture registry helpers
- [x] Add license and dependency allowlist checks
- [x] Stub cmd/m and cmd/mx main packages compiling to --help placeholder
- [x] Document developer doctor command contract
- [x] Clean-clone bootstrap test
- [x] CI self-test that fails each quality gate intentionally in a job
- [x] Cross-platform compile matrix including Windows
- [x] Write AGENTS.md with ownership and reading order
- [x] Add CONTRIBUTING with exact commands
- [x] Document fixture checksum policy
- [x] Add GitHub Actions for Linux, macOS, Windows, amd64, arm64
- [x] Configure race tests and fuzz smoke targets
- [x] Acceptance: Fresh clone: go test ./... passes on Linux/macOS/Windows CI
- [x] Acceptance: Lint and vet wired in CI
- [x] Acceptance: AGENTS.md present and linked from README
- [x] Acceptance: cmd/m and cmd/mx build
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0005 - Stable Error Model, Diagnostics, and Observability

- status: done
- plan: [0005-error-observability.md](0005-error-observability.md)

- [x] Define typed error with stable code, operation, subject
- [x] Publish initial error code registry
- [x] Define progress event schema (phase, package, bytes, …)
- [x] Define redaction rules for URLs, tokens, headers
- [x] Implement error wrapping helpers
- [x] Implement human and NDJSON reporters
- [x] Implement cancellation mapping to exit codes
- [x] Add trace span hooks without mandatory OTel dependency
- [x] Map codes to exit statuses
- [x] Ensure secrets never print in default or debug modes without explicit unsafe flag
- [x] Table tests for code→exit mapping
- [x] Redaction golden tests
- [x] Progress event golden tests
- [x] Document codes for users and agents
- [x] Document reporter formats
- [x] Implement TTY detection, color policy, and width-safe rendering
- [x] Add panic recovery at command boundaries with crash IDs
- [x] Acceptance: Every public failure path yields a stable code
- [x] Acceptance: Tokens in registry URLs are redacted in logs
- [x] Acceptance: JSON reporter validates against schema
- [x] Acceptance: NDJSON progress events are line-atomic under concurrency
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0006 - Configuration and Project Identity Model

- status: done
- plan: [0006-configuration-identity.md](0006-configuration-identity.md)

- [x] Define config layer precedence: defaults, global, project, env, CLI
- [x] Define identity detection order matching AGENTS.md
- [x] List owned config keys vs pass-through npmrc keys
- [x] Forbid reading unrelated branded PM config as authority
- [x] Implement layered loader with deterministic merge
- [x] Implement identity detector for packageManager, devEngines, lockfile, Mew native
- [x] Implement offline/prefer-offline flags in config model
- [x] Validate unknown keys policy (warn vs fail)
- [x] Specify config command grammar
- [x] Effective-config debug output with redaction
- [x] Precedence table tests
- [x] Identity detection fixtures for each lockfile type
- [x] Malformed config fail-closed tests
- [x] Document every public config key
- [x] Document identity detection with examples
- [x] Preserve comments and ordering when modifying user-owned JSONC
- [x] Separate credential references from non-secret configuration
- [x] Add unsupported-config diagnostics that never silently ignore safety-critical options
- [x] Acceptance: Detection order matches AGENTS.md
- [x] Acceptance: Conflicting signals produce explicit errors, not silent picks
- [x] Acceptance: Env overrides project overrides user as documented
- [x] Acceptance: pnpm-specific files are not read for Mew-identity projects unless importing
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0007 - Canonical Data Model and Core Interfaces

- status: done
- plan: [0007-data-model-interfaces.md](0007-data-model-interfaces.md)

- [x] Freeze Manifest, Dependency, Importer, Package, Graph, Edge types
- [x] Freeze ResolutionDecision and PeerContext types
- [x] Freeze Policy, Plan, Snapshot, Capsule descriptors
- [x] Define deterministic sort keys for all collections
- [x] Define immutability rules for graph values
- [x] Define ID schemes for packages and importers
- [x] Define integrity and tarball URL fields
- [x] Define migration-friendly version fields
- [x] Keep source-format fields in adapter-owned extension maps
- [x] Use sorted slices or explicit deterministic encoders rather than map iteration
- [x] Specify explain/plan JSON shapes consumed by 0028
- [x] Round-trip golden encoding tests
- [x] Ordering stability tests
- [x] Invalid graph rejection tests
- [x] Publish data-model doc with diagrams
- [x] Link types to owning packages
- [x] Provide builders for tests while keeping production models immutable after validation
- [x] Version serialized internal caches independently from public lockfiles
- [x] Acceptance: All later core MVPs can depend on these types without reaching into adapters
- [x] Acceptance: Deterministic encoding byte-identical across platforms
- [x] Acceptance: Version field present on every persistent model
- [x] Acceptance: Peer-context identity collisions are detectable and rejected
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0008 - Testing, Fixtures, Fuzzing, and Conformance Strategy

- status: done
- plan: [0008-testing-strategy.md](0008-testing-strategy.md)

- [x] Define fixture manifest format and checksums
- [x] Define clean-home test contract
- [x] Define differential comparison report schema
- [x] Implement local fixture registry server helper
- [x] Implement isolated HOME/XDG/cache redirection
- [x] Implement reference PM invocation wrappers (optional when tools present)
- [x] Define fuzz targets list for parsers
- [x] Document how to add a fixture
- [x] Document required metadata: OS, tool versions
- [x] Smoke: install from fixture registry
- [x] Failure injection helpers: network cut, disk full simulation
- [x] Cross-platform path/symlink/junction probes
- [x] Testing strategy doc with layout diagram
- [x] Conformance inventory stub for 0080
- [x] Never make normal CI dependent on the public npm registry
- [x] Keep large ecosystem corpus tests in scheduled jobs
- [x] Normalize nondeterministic output before comparison
- [x] Add known-bad corpus verifying parsers fail safely
- [x] Acceptance: Tests never require public registry access
- [x] Acceptance: Clean-home tests do not touch developer global state
- [x] Acceptance: Fixture checksums verified on load
- [x] Acceptance: Differential harness smoke test passes on pinned Nub revision when available
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0009 - Release Train and MVP Dependency Graph

- status: done
- plan: [0009-release-train-overview.md](0009-release-train-overview.md)

- [x] Create milestone dependency graph with no cycles
- [x] Define alpha/beta/rc/stable criteria
- [x] Define which MVPs may ship experimentally
- [x] Define support windows for lock adapters and Node
- [x] Define stop-the-line criteria
- [x] Map every inventory feature to a milestone
- [x] Define backport and format-migration policy
- [x] Require readers before writers for public formats
- [x] Document feature-flag naming for experimental commands
- [x] Validate graph has no cycles
- [x] Dry-run release checklist on empty scaffold
- [x] Publish release-train doc
- [x] Keep INDEX.md synchronized
- [x] Every MVP must preserve rollback to the preceding stable release
- [x] Public formats require validation before migration
- [x] No calendar promises; sequencing is dependency-driven
- [x] Link stabilization gates 0031, 0046, 0057 to release channels
- [x] Define compatibility certification gates before GA
- [x] Acceptance: Every inventory feature has a milestone
- [x] Acceptance: Stabilization gates 0031/0046/0057 cannot start early
- [x] Acceptance: Stop-the-line criteria include corruption and integrity failures
- [x] Acceptance: Milestone graph has no cycles and matches INDEX.md ordering
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0010 - Core MVP 1 — CLI Foundation and Command Dispatch

- status: done
- plan: [0010-cli-foundation.md](0010-cli-foundation.md)

- [x] Implement cmd/m and cmd/mx main entrypoints with shared bootstrap
- [x] Create internal/cli root command with Cobra tree and persistent pre-run
- [x] Wire internal/app application context: cwd, config load, reporter init
- [x] Implement m version with semver build metadata from ldflags
- [x] Normalize global flags: --cwd, --offline, --debug, --color, --no-color
- [x] Implement exit-code mapping from internal/diagnostics error codes
- [x] Add context cancellation propagation from SIGINT/SIGTERM
- [x] Register built-in command stubs for later MVPs with stable not-implemented errors
- [x] Implement reserved-name list from feature inventory to block script collisions
- [x] Generate shell completion for bash, zsh, fish, and PowerShell
- [x] Add hidden m __dispatch diagnostic showing effective command resolution
- [x] Detect invoked binary name m vs mew and mx vs mewx for help text
- [x] Ensure global flags apply before any subcommand handler runs
- [x] Add golden tests for --help and version output formatting
- [x] Add table-driven tests for flag parsing edge cases
- [x] Document command precedence: built-in > alias > script (future 0042)
- [x] Keep handlers thin: parse flags, delegate to app services only
- [x] Avoid importing resolver, linker, fetch, or registry packages
- [x] Acceptance: m --help and mx --help render stable usage without panic
- [x] Acceptance: m version prints name, semver, commit, and build date
- [x] Acceptance: Global --cwd changes effective project root for downstream services
- [x] Acceptance: SIGINT returns non-zero exit and cancels in-flight context
- [x] Acceptance: Reserved built-in names cannot be shadowed by future script shortcuts
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0011 - Core MVP 2 — Manifest Parsing and Project Discovery

- status: done
- plan: [0011-manifest-project-discovery.md](0011-manifest-project-discovery.md)

- [x] Implement walk-up project root discovery from cwd with package.json boundary
- [x] Parse package.json without destructive reformatting or key reordering
- [x] Normalize dependencies, devDependencies, peerDependencies, optionalDependencies
- [x] Support workspaces field: array and {packages: [...]} forms
- [x] Expand workspace globs to concrete member paths deterministically
- [x] Detect packageManager and devEngines.packageManager for identity hints
- [x] Expose read-only manifest accessor on internal/app project handle
- [x] Implement safe manifest field updates preserving comments and formatting where possible
- [x] Validate package name, version, and bin field shapes with actionable errors
- [x] Handle missing package.json with typed not-found error code
- [x] Support package.json in subpath importers for future workspace installs
- [x] Add golden tests for manifest read/write on representative fixtures
- [x] Add tests for workspace glob edge cases: negation, braces, duplicates
- [x] Document manifest normalization contract for resolver consumers
- [x] Reject cyclic workspace definitions with clear diagnostics
- [x] Cache parsed manifest per project root with file watcher invalidation hook
- [x] Keep manifest package free of network or filesystem mutation beyond package.json
- [x] Acceptance: package.json round-trips without unintended whitespace or key loss
- [x] Acceptance: Workspace globs resolve to stable sorted member list
- [x] Acceptance: Project discovery stops at first valid root from cwd
- [x] Acceptance: Invalid manifest fields produce stable machine-readable error codes
- [x] Acceptance: Normalized dependency map matches npm semantics for scoped packages
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0012 - Core MVP 3 — Registry Client and Metadata Cache

- status: done
- plan: [0012-registry-cache.md](0012-registry-cache.md)

- [x] Implement npm registry packument fetch with semver version index
- [x] Support scoped registries via .npmrc and project config layering
- [x] Resolve auth tokens per registry URL with redaction in diagnostics
- [x] Implement metadata disk cache keyed by registry URL + package name
- [x] Honor ETag and If-None-Match for conditional requests
- [x] Implement bounded concurrent metadata fetch worker pool
- [x] Add exponential backoff retry for transient 5xx and network errors
- [x] Respect --offline: fail closed when cache miss
- [x] Support HTTP/SOCKS proxy from config and environment
- [x] Validate packument JSON schema and reject malformed responses
- [x] Normalize dist-tags, versions, and dist.integrity fields
- [x] Add integration tests against fixtures/registry local server
- [x] Add failure tests: 404, 401, timeout, truncated body
- [x] Document registry client interface for resolver package
- [x] Never log auth headers or token values
- [x] Implement cache corruption detection and safe eviction
- [x] Support custom registry URL per scope prefix
- [x] Acceptance: Packument fetch succeeds against local fixture registry
- [x] Acceptance: ETag cache returns 304 and avoids re-download
- [x] Acceptance: --offline fails with clear error when metadata absent
- [x] Acceptance: Auth token never appears in stderr or debug logs
- [x] Acceptance: Concurrent fetches respect worker pool limit
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0013 - Core MVP 4 — npm Semver and Basic Dependency Resolver

- status: done
- plan: [0013-semver-basic-resolver.md](0013-semver-basic-resolver.md)

- [x] Integrate npm-compatible semver range parsing and satisfaction
- [x] Resolve direct dependencies from normalized manifest declarations
- [x] Expand transitive dependencies from registry packuments recursively
- [x] Produce deterministic canonical graph with stable node ordering
- [x] Detect and report dependency cycles with full path
- [x] Handle scoped package names and registry URL routing
- [x] Select highest matching version for ^ ~ * and exact ranges
- [x] Emit structured decision trace for each version choice
- [x] Support resolution from empty graph (greenfield)
- [x] Support resolution from partial lock hints (prepare for 0015)
- [x] Bound recursion depth and fan-out with clear limits
- [x] Add unit tests for semver edge cases: prerelease, build metadata
- [x] Add integration tests resolving fixture registry graphs
- [x] Add golden tests for deterministic graph encoding
- [x] Document resolver input/output interfaces for lockfile adapter
- [x] Fail closed on missing packument or unsatisfiable range
- [x] Avoid any node_modules or lockfile mutation in this MVP
- [x] Acceptance: Simple ^ range resolves to expected highest compatible version
- [x] Acceptance: Transitive closure matches fixture registry graph
- [x] Acceptance: Identical inputs produce byte-identical canonical graph
- [x] Acceptance: Unsatisfiable range returns stable error with package name
- [x] Acceptance: Cycle detection reports full cycle path
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0014 - Core MVP 5 — Tarball Fetch, Integrity, and Safe Extraction

- status: done
- plan: [0014-fetch-integrity-extraction.md](0014-fetch-integrity-extraction.md)

- [x] Implement tarball download from registry dist.tarball URL
- [x] Verify dist.integrity sha512 before any extraction
- [x] Support concurrent downloads with bounded worker pool
- [x] Write downloads to temp files and atomic rename on success
- [x] Implement gzip tar extraction with path traversal prevention
- [x] Reject absolute paths, .. segments, and symlink escapes in archives
- [x] Normalize file modes and timestamps for reproducible extraction
- [x] Handle truncated downloads and checksum mismatch with retry
- [x] Integrate with registry auth for private package tarballs
- [x] Support --offline: read from local cache only
- [x] Store verified blobs in content-addressed staging area
- [x] Add failure injection tests: corrupt tarball, wrong hash, disk full
- [x] Add cross-platform extraction tests on Windows junctions
- [x] Document fetch/archive interfaces for installer MVP
- [x] Redact signed URLs from error messages and logs
- [x] Implement download resume only if spec requires (document deferral)
- [x] Clean up partial temp files on cancellation
- [x] Acceptance: Valid tarball extracts to expected file tree
- [x] Acceptance: Integrity mismatch aborts before extraction
- [x] Acceptance: Path traversal archive is rejected without writing files
- [x] Acceptance: Concurrent downloads respect worker limit
- [x] Acceptance: Cancelled download removes partial temp files
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0015 - Core MVP 6 — Native `m.lock` Format

- status: done
- plan: [0015-m-lock.md](0015-m-lock.md)

- [x] Define m.lock schema version and top-level document structure
- [x] Serialize canonical graph to importer-aware lock sections
- [x] Record package identity, resolution, integrity, and dependency edges
- [x] Include settings block for linker mode and resolver policy placeholders
- [x] Implement deterministic key ordering and stable encoding
- [x] Implement lockfile read parser with forward-compatible unknown field handling
- [x] Implement lockfile write from resolver output without data loss
- [x] Validate lockfile against manifest on --frozen-lockfile
- [x] Detect lockfile/manifest drift with actionable diff summary
- [x] Add golden tests for round-trip encode/decode
- [x] Add migration stub for future schema bumps
- [x] Document m.lock field reference for adapter MVPs
- [x] Support peer-context placeholders for 0020 without full peer resolution
- [x] Never embed secrets or auth tokens in lockfile
- [x] Reject ambiguous duplicate package entries
- [x] Integrate with internal/resolver graph model from 0007
- [x] Add fuzz smoke tests for parser robustness
- [x] Acceptance: Resolver graph round-trips through m.lock losslessly
- [x] Acceptance: Generated m.lock is byte-identical across platforms for same input
- [x] Acceptance: Frozen lockfile mode fails when manifest changes
- [x] Acceptance: Corrupt lockfile returns stable parse error code
- [x] Acceptance: Schema version field present on every document
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0016 - Core MVP 7 — Basic End-to-End Installer

- status: done
- plan: [0016-basic-installer.md](0016-basic-installer.md)

- [x] Implement install application service orchestrating resolve-fetch-link
- [x] Resolve from package.json or existing m.lock per policy
- [x] Fetch all packages before any node_modules mutation
- [x] Implement conservative hoisted linker placing deps at top level
- [x] Materialize node_modules in staging directory before publish
- [x] Create .bin directory with platform shims for declared bins
- [x] Implement m add with dev/prod dependency type selection
- [x] Implement m remove with manifest and lockfile update
- [x] Prune stale packages from hoisted tree after remove
- [x] Support --prod to omit devDependencies
- [x] Support --dry-run printing plan without disk mutation
- [x] Support --frozen-lockfile failing on drift
- [x] Emit install summary: added, removed, changed counts
- [x] Add integration tests: install from empty, from lock, add, remove
- [x] Compare require() behavior with npm on basic-cjs fixture
- [x] Failure tests: original node_modules remains usable on error
- [x] Document non-goals: no global store, no isolated layout yet
- [x] Acceptance: m install on greenfield project produces working node_modules
- [x] Acceptance: m add lodash updates m.lock and node_modules
- [x] Acceptance: m remove prunes unused packages from hoisted tree
- [x] Acceptance: Failed install does not leave corrupt partial node_modules
- [x] Acceptance: m install --frozen-lockfile fails when package.json changed
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0017 - Core MVP 8 — Transactional Install and Instant Rollback

- status: done
- plan: [0017-transaction-rollback.md](0017-transaction-rollback.md)

- [x] Define transaction phases: inspect, resolve, plan, fetch, stage, validate, commit
- [x] Journal every filesystem mutation with inverse operations
- [x] Keep original manifest, lockfile, and node_modules until commit succeeds
- [x] Implement rollback applying journal in reverse on any failure
- [x] Implement crash recovery: detect incomplete journal and offer recover
- [x] Create snapshot on successful commit with monotonic ID
- [x] Implement m snapshot list and m snapshot restore
- [x] Validate staged tree before commit: integrity, bins, expected packages
- [x] Integrate transaction boundary with install/add/remove from 0016
- [x] Ensure partial fetch does not mutate committed state
- [x] Add failure injection: kill process mid-commit, disk full, permission denied
- [x] Add tests proving old node_modules works after failed install
- [x] Document journal format and retention policy
- [x] Limit journal size with rotation policy
- [x] Never delete committed state without successful staging validation
- [x] Emit transaction progress events to diagnostics reporter
- [x] Support dry-run generating plan without journal writes
- [x] Acceptance: Failed install leaves prior node_modules intact and usable
- [x] Acceptance: Interrupted commit can be recovered or cleanly rolled back
- [x] Acceptance: Snapshot restore returns project to prior dependency state
- [x] Acceptance: Journal records sufficient ops for full rollback
- [x] Acceptance: Commit is atomic: no half-updated lockfile visible
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0018 - Core MVP 9 — Global Content Store and Smart Filesystem Planner

- status: done
- plan: [0018-global-store-smart-linker.md](0018-global-store-smart-linker.md)

- [x] Implement content-addressed global store keyed by integrity hash
- [x] Import verified tarballs into store without duplication
- [x] Probe filesystem for hardlink, reflink, symlink, junction support
- [x] Implement link planner choosing safest fastest strategy per path
- [x] Fall back to copy when hardlink/reflink unavailable or cross-device
- [x] Use Windows junctions/symlinks per platform policy
- [x] Integrate store with hoisted linker from 0016
- [x] Track store reference counts from project link manifests
- [x] Implement m store path and m store prune commands
- [x] Prune unreferenced blobs with dry-run preview
- [x] Add cross-platform tests: Linux, macOS, Windows linking
- [x] Add tests for cross-filesystem copy fallback
- [x] Document store layout and garbage collection rules
- [x] Never mutate store blobs in place after import
- [x] Verify integrity on store read before linking
- [x] Support MEW_STORE_DIR override with validation
- [x] Emit link strategy summary in install diagnostics
- [x] Acceptance: Identical package imported twice shares one store blob
- [x] Acceptance: Link planner selects copy on cross-device install
- [x] Acceptance: Store prune removes only unreferenced blobs
- [x] Acceptance: Corrupt store entry is detected and re-fetched
- [x] Acceptance: m store path reports configured location
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0019 - Core MVP 10 — Isolated Virtual Store and Node Modules Layout

- status: done
- plan: [0019-isolated-linker.md](0019-isolated-linker.md)

- [x] Implement isolated linker creating per-package node_modules trees
- [x] Layout packages under node_modules/.pnpm/<id>/node_modules/
- [x] Symlink or junction top-level aliases to isolated paths
- [x] Prevent access to undeclared dependencies (phantom dep test)
- [x] Support hoisted mode as compatibility fallback via config
- [x] Integrate with global store from 0018 for file linking
- [x] Update m.lock settings block with linker mode
- [x] Handle peer dependency symlink targets in isolated layout
- [x] Create .bin shims resolving through isolated paths
- [x] Add integration tests comparing pnpm fixture layouts
- [x] Add Windows junction tests for long paths
- [x] Document isolated vs hoisted trade-offs
- [x] Ensure transaction rollback works with isolated tree
- [x] Validate staged isolated tree before commit
- [x] Support scoped packages in virtual store paths
- [x] Deterministic ordering of virtual store directory names
- [x] Emit layout summary in install output
- [x] Acceptance: Isolated install blocks requiring undeclared dependencies
- [x] Acceptance: pnpm-simple fixture layout matches expected structure
- [x] Acceptance: Hoisted mode still works via --linker=hoisted
- [x] Acceptance: Isolated .bin shims execute correctly on Windows
- [x] Acceptance: Linker mode persists in m.lock settings
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0020 - Core MVP 11 — Full Dependency Resolver

- status: done
- plan: [0020-advanced-resolver.md](0020-advanced-resolver.md)

- [x] Implement peer dependency constraint collection per importer
- [x] Generate peer contexts as part of package identity
- [x] Implement auto-install peers policy with strict and loose modes
- [x] Prune optional dependencies failing os/cpu/libc filters
- [x] Apply overrides and resolutions rewriting dependency edges
- [x] Support npm: alias protocol and package aliases
- [x] Resolve workspace:* and workspace:^ protocol to local packages
- [x] Support file:, link:, and portal: source placeholders
- [x] Implement incremental lock reuse preserving unaffected subgraph
- [x] Minimize graph churn on targeted m update
- [x] Emit conflict explanation tree for unsatisfiable peers
- [x] Record resolver policy choices in m.lock settings
- [x] Add conformance fixtures for peer, optional, override cases
- [x] Add workspace protocol resolution tests
- [x] Document peer context ID format
- [x] Fail with actionable errors for missing workspace targets
- [x] Benchmark resolver on large monorepo fixture
- [x] Acceptance: Conflicting peer deps produce explanation tree not silent wrong version
- [x] Acceptance: workspace:* resolves to correct local package version
- [x] Acceptance: Optional dep skipped on unsupported platform
- [x] Acceptance: Override replaces transitive version deterministically
- [x] Acceptance: Targeted update preserves unrelated lock subgraph
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0021 - Core MVP 12 — Lifecycle Scripts, Trust, and Sandbox Policy

- status: done
- plan: [0021-lifecycle-sandbox.md](0021-lifecycle-sandbox.md)

- [x] Implement lifecycle script discovery from package.json scripts field
- [x] Run preinstall/install/postinstall/prepare in npm order
- [x] Enforce ignore-scripts flag and config
- [x] Implement trust policy: prompt or allowlist for unknown scripts
- [x] Execute scripts in sandbox with restricted env and filesystem
- [x] Propagate correct PATH and node_modules/.bin for script context
- [x] Cache reproducible build script outputs keyed by inputs
- [x] Write audit log entry for every script: package, script, exit code
- [x] Fail install on script failure with rollback via transaction
- [x] Support Windows cmd/sh and Unix sh shebang resolution
- [x] Redact secrets from script environment in logs
- [x] Add tests with benign fixture scripts
- [x] Add failure tests: script exit non-zero triggers rollback
- [x] Document lifecycle policy for CI (--ignore-scripts default?)
- [x] Integrate with isolated linker bin paths
- [x] Never execute lifecycle during --dry-run
- [x] Support m approve-builds to add package to trust list
- [x] Acceptance: postinstall script runs after package materialized
- [x] Acceptance: Failing lifecycle script triggers full install rollback
- [x] Acceptance: --ignore-scripts skips all lifecycle execution
- [x] Acceptance: Untrusted package prompts or blocks per policy
- [x] Acceptance: Audit log records script executions
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0022 - Core MVP 13 — Workspaces, Catalogs, and Filtering

- status: done
- plan: [0022-workspaces-catalogs.md](0022-workspaces-catalogs.md)

- [x] Parse pnpm catalog: and catalog:default in package.json
- [x] Resolve catalog references to concrete versions in manifests
- [x] Implement --filter pattern matching package names and paths
- [x] Support -r recursive install across workspace members
- [x] Resolve workspace dependency graph with topological ordering
- [x] Install all importers atomically in single transaction
- [x] Validate workspace: protocol targets exist in graph
- [x] Detect duplicate workspace package names across members
- [x] Support root package.json as workspace importer
- [x] Implement m ls -r workspace tree listing
- [x] Add integration tests on workspace-simple and nested fixtures
- [x] Ensure filter install does not break unrelated members
- [x] Record per-importer sections in m.lock for all members
- [x] Document filter grammar compatibility with pnpm
- [x] Fail on catalog reference to undefined catalog entry
- [x] Support negation patterns in filters if pnpm-compatible
- [x] Emit workspace install summary per importer
- [x] Acceptance: m install -r installs all workspace members atomically
- [x] Acceptance: catalog: deps resolve to catalog-defined versions
- [x] Acceptance: --filter installs only matching packages and deps
- [x] Acceptance: Broken workspace: reference fails with clear error
- [x] Acceptance: m.lock contains importer section per workspace package
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0023 - Core MVP 14 — Nub and pnpm Lockfile Bridge

- status: done
- plan: [0023-nub-pnpm-lock-bridge.md](0023-nub-pnpm-lock-bridge.md)

- [x] Detect nub.lock and pnpm-lock.yaml per identity rules from 0006
- [x] Implement nub.lock reader adapter to canonical graph
- [x] Implement pnpm-lock.yaml reader for supported major generations
- [x] Preserve incumbent lockfile on install without user migrate
- [x] Write round-trip safe nub.lock when project identity is Nub
- [x] Write round-trip safe pnpm-lock when identity is pnpm
- [x] Implement m migrate lock --to m.lock with dry-run report
- [x] Document lossy conversions explicitly in migration output
- [x] Validate adapter output against resolver for drift detection
- [x] Add golden tests per lockfile generation fixture
- [x] Add diff tool comparing canonical graph from two lock sources
- [x] Support peer/importer metadata required for isolated layout
- [x] Never overwrite incumbent lock without explicit migrate
- [x] Handle lockfile version unsupported with upgrade guidance
- [x] Integrate with transaction commit for lock writes
- [x] Fuzz parser smoke on lockfile corpora
- [x] Record adapter version in migration report
- [x] Acceptance: Install on nub.lock project preserves nub.lock format
- [x] Acceptance: pnpm-lock.yaml project installs without silent m.lock conversion
- [x] Acceptance: m migrate lock --dry-run lists lossy fields
- [x] Acceptance: Adapter round-trip nub.lock golden matches source
- [x] Acceptance: Unsupported lock version returns actionable error
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0024 - Core MVP 15 — npm Lockfile and Shrinkwrap Compatibility

- status: done
- plan: [0024-npm-locks.md](0024-npm-locks.md)

- [x] Implement package-lock.json v2 and v3 parsers
- [x] Map npm lock packages array to canonical graph nodes
- [x] Preserve npm project identity: write package-lock not m.lock
- [x] Support npm-shrinkwrap.json read and write
- [x] Handle lockfileVersion field and forward compatibility
- [x] Import integrity and resolved URL fields from npm lock
- [x] Support bundledDependencies and packages link fields
- [x] Install produces npm-compatible hoisted layout
- [x] Detect package-lock drift vs package.json on frozen install
- [x] Add golden tests for npm lock v2/v3 fixtures
- [x] Add differential tests vs npm install on fixture projects
- [x] Document npm-specific fields preserved in adapter
- [x] Implement migrate to m.lock with loss report
- [x] Never strip package-lock on npm-identity project install
- [x] Handle absent package-lock: generate on first install
- [x] Support workspaces in package-lock v3
- [x] Validate lockfilePackages ordering determinism on write
- [x] Acceptance: npm fixture install matches package-lock dependency tree
- [x] Acceptance: package-lock.json preserved after m install on npm project
- [x] Acceptance: Frozen install fails when package.json conflicts with lock
- [x] Acceptance: npm-shrinkwrap project installs correctly
- [x] Acceptance: Lock v2 and v3 fixtures parse without error
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0025 - Core MVP 16 — Bun and Yarn Lockfile Compatibility

- status: done
- plan: [0025-bun-yarn-locks.md](0025-bun-yarn-locks.md)

- [x] Implement bun.lock parser adapter to canonical graph
- [x] Implement yarn.lock classic parser
- [x] Implement Yarn Berry lockfile read for node-modules mode
- [x] Certify Berry PnP read path or document explicit deferral
- [x] Preserve bun.lock on Bun-identity projects
- [x] Preserve yarn.lock on Yarn classic identity projects
- [x] Detect yarn berry via .yarnrc.yml and lockfile format
- [x] Support migrate lock from bun/yarn to m.lock
- [x] Document unsupported Berry features with clear errors
- [x] Add golden fixtures per lock type
- [x] Add differential install tests where reference tool available
- [x] Handle yarn resolutions field mapping to overrides
- [x] Support zero-install cache metadata read-only if present
- [x] Never silently convert yarn/bun locks to m.lock
- [x] Integrate identity detection from 0006
- [x] Validate parser against fuzz corpora
- [x] Emit migration report for lossy bun/yarn conversions
- [x] Acceptance: bun.lock fixture imports to valid install graph
- [x] Acceptance: yarn.lock classic project installs with preserved lock
- [x] Acceptance: Berry node-modules fixture installs without PnP
- [x] Acceptance: Unsupported Berry feature fails with documented error
- [x] Acceptance: Identity detection selects correct lock adapter
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0026 - Core MVP 17 — Complete Package-Manager Command Surface

- status: done
- plan: [0026-pm-command-surface.md](0026-pm-command-surface.md)

- [x] Complete PM subcommand tree with consistent flag naming
- [x] Implement m ci: clean install from lock in CI mode
- [x] Implement m outdated with recursive workspace support
- [x] Implement m dedupe rewriting lock to minimal graph
- [x] Implement m prune removing extraneous node_modules packages
- [x] Implement m list (m ls) dependency tree display
- [x] Route all mutating commands through transaction journal
- [x] Unify --dry-run behavior across install family
- [x] Unify --frozen-lockfile across ci and install
- [x] Add pnpm-compatible flag aliases where documented
- [x] Generate comprehensive --help per subcommand
- [x] Add integration tests per command on fixture projects
- [x] Document Mew grammar divergences from pnpm/npm
- [x] Ensure mx does not expose PM commands
- [x] Stable JSON output for outdated --json
- [x] Exit codes consistent across PM commands
- [x] Deprecate stubs replaced by real implementations with warnings
- [x] Acceptance: m ci fails when lockfile out of sync with manifest
- [x] Acceptance: m outdated reports available updates as JSON
- [x] Acceptance: m dedupe reduces duplicate packages in lock
- [x] Acceptance: All mutating commands rollback on failure
- [x] Acceptance: Help text complete for every PM subcommand
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0027 - Core MVP 18 — Advanced Sources, Patches, Pack, and Publish

- status: done
- plan: [0027-advanced-sources-publish.md](0027-advanced-sources-publish.md)

- [x] Support git+https, git+ssh, and github: dependency sources
- [x] Support file: and tarball: local dependency paths
- [x] Fetch git sources at resolved commit/tag with submodule policy
- [x] Implement pnpm-style patch commit workflow (m patch)
- [x] Apply patches deterministically during install
- [x] Implement m pack producing npm-compatible tarball
- [x] Validate package files field and .npmignore on pack
- [x] Implement m publish with registry auth and OTP support
- [x] Record non-registry sources in m.lock with integrity
- [x] Validate git URL and ref before fetch
- [x] Sandbox git fetch network access per policy
- [x] Add provenance attestation hook points (optional)
- [x] Add tests for git dep, file dep, patch, pack fixtures
- [x] Redact credentials in publish error output
- [x] Support --dry-run on publish
- [x] Document supported source protocols matrix
- [x] Never execute arbitrary scripts from git deps without policy
- [x] Acceptance: Git dependency installs at pinned commit
- [x] Acceptance: Applied patch changes installed file content deterministically
- [x] Acceptance: m pack tarball matches npm pack file list
- [x] Acceptance: m publish --dry-run validates without uploading
- [x] Acceptance: file: dependency resolves relative to manifest
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0028 - Core MVP 19 — Explainability, Plans, Semantic Diffs, and Time Travel

- status: done
- plan: [0028-explain-plan-history.md](0028-explain-plan-history.md)

- [x] Implement m explain showing version selection reasoning
- [x] Implement m explain peer for peer dependency conflicts
- [x] Implement m plan previewing fetch/link/manifest changes
- [x] Implement semantic diff between two lock graphs
- [x] Compare m.lock revisions and incumbent lock formats
- [x] Integrate snapshot list/restore from 0017 with UX polish
- [x] Emit structured JSON for explain and plan for agents
- [x] Colorize human explain output via diagnostics reporter
- [x] Support diff against npm/pnpm locks via adapters
- [x] Add golden tests for explain output on fixture graphs
- [x] Add plan preview tests matching actual install delta
- [x] Document explain trace schema
- [x] Never mutate state in explain/plan/diff commands
- [x] Support piping plan to file for CI review
- [x] Link explain output to stable error codes
- [x] Performance: explain completes in <1s on large graph fixture
- [x] Add m history showing snapshot timeline
- [x] Acceptance: m explain prints version selection path for target package
- [x] Acceptance: m plan --json matches actual install file changes on dry-run
- [x] Acceptance: m diff lock detects semver bump between two locks
- [x] Acceptance: m snapshot restore returns project to recorded state
- [x] Acceptance: Explain/plan/diff never modify project files
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0029 - Core MVP 20 — Performance, Offline Operation, and Portable Capsules

- status: done
- plan: [0029-performance-offline-capsules.md](0029-performance-offline-capsules.md)

- [x] Profile install phases: resolve, fetch, extract, link, lifecycle
- [x] Optimize hot paths identified by profiling
- [x] Implement warm-cache fast path skipping redundant metadata fetches
- [x] Make --offline first-class: preflight cache completeness check
- [x] Implement m capsule create bundling store + lock + metadata
- [x] Implement m capsule restore for CI/container bootstrap
- [x] Add benchmark harness m bench install with cold/warm modes
- [x] Publish baseline benchmark artifacts in repo
- [x] Add CI regression gate on critical path benchmarks
- [x] Tune worker pool defaults per CPU count
- [x] Reduce allocator churn in resolver and linker
- [x] Implement metadata batch fetch where registry supports
- [x] Document offline workflow for air-gapped environments
- [x] Capsule integrity verification on restore
- [x] Never sacrifice integrity for performance
- [x] Add soak test script for repeated install cycles
- [x] Document performance tuning env vars
- [x] Acceptance: Warm install measurably faster than cold on benchmark fixture
- [x] Acceptance: Offline install succeeds when capsule/cache complete
- [x] Acceptance: Capsule round-trip produces identical node_modules hash
- [x] Acceptance: Benchmark CI gate fails on >10% regression without waiver
- [x] Acceptance: Phase timing diagnostics available via --debug
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0030 - Core MVP 21 — Audit, SBOM, Provenance, and Supply-Chain Policy

- status: done
- plan: [0030-security-audit-sbom.md](0030-security-audit-sbom.md)

- [x] Implement m audit against OSV/npm advisory data
- [x] Support offline audit from cached advisory DB
- [x] Implement m sbom CycloneDX and SPDX export
- [x] Include direct and transitive deps in SBOM
- [x] Verify package provenance attestations when present
- [x] Implement dependency age policy (minimum release age)
- [x] Implement org policy file for deny/warn on licenses and packages
- [x] Fail install when policy severity exceeds threshold
- [x] Redact internal package names in SBOM if configured
- [x] Add audit fixtures with known vulnerable versions
- [x] Add SBOM golden tests validating schema
- [x] Document trust model integration with 0021 lifecycle policy
- [x] Support m audit --fix suggesting safe bumps
- [x] Cache advisory DB with signature verification
- [x] Never phone home with project source code
- [x] Stable JSON schema for audit output
- [x] Integrate policy checks into transaction validate phase
- [x] Acceptance: m audit reports known CVE on fixture vulnerable package
- [x] Acceptance: m sbom output validates against CycloneDX schema
- [x] Acceptance: Policy deny blocks install of blocked package
- [x] Acceptance: Provenance verify passes on signed fixture package
- [x] Acceptance: Audit works offline with cached advisory DB
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0031 - Core MVP 22 — Package-Manager Core Stabilization Gate

- status: done
- plan: [0031-core-stabilization.md](0031-core-stabilization.md)

- [x] Run full conformance suite against Nub/npm/pnpm fixtures per 0080
- [x] Execute cross-platform integration matrix on Linux/macOS/Windows CI
- [x] Run soak tests: 100+ install cycles on representative projects
- [x] Fix all P0/P1 defects found during stabilization
- [x] Verify no critical data-loss or corruption paths remain
- [x] Implement m doctor health check for common misconfigurations
- [x] Publish core-certification.md with evidence links
- [x] Freeze public CLI and m.lock schema for runner MVPs
- [x] Review and close open decisions from 0010-0030
- [x] Benchmark baselines recorded and regression gates green
- [x] Security audit of threat model 0082 items for PM core
- [x] Documentation pass for all PM commands shipped
- [x] Verify transaction recovery on all supported platforms
- [x] Verify all lock adapters on certified fixture corpus
- [x] No new features: stabilization and fixes only
- [x] Sign-off checklist per 0087 definition of done
- [x] Unblock 0040 runner MVP with stable install interfaces
- [x] Acceptance: Full core conformance suite passes on all CI platforms
- [x] Acceptance: m doctor reports healthy state on clean fixture project
- [x] Acceptance: No open P0/P1 defects in PM core scope
- [x] Acceptance: core-certification.md published with test evidence
- [x] Acceptance: 0040 can depend on install/layout interfaces without breakage
- [x] Exit: Zero known data-loss or silent-integrity issue.
- [x] Exit: Certified read/write matrices are accurate and enforced by tests.
- [x] Exit: Transactional recovery succeeds for every injected commit interruption.
- [x] Exit: Core commands are documented and machine-readable output is versioned.
- [x] Exit: Performance and resource budgets are enforced in CI.

### 0040 - Runner MVP 1 — Package Script Runner

- status: done
- plan: [0040-script-runner.md](0040-script-runner.md)

- [x] Define ScriptRunner interface with context cancellation and stable error codes
- [x] Implement package.json script lookup with explicit missing-script diagnostics
- [x] Implement pre/post lifecycle hook expansion with ordering guarantees
- [x] Implement pure npm-compatible environment builder (INIT_CWD, npm_* vars, PATH)
- [x] Implement cross-platform shell selection and command quoting
- [x] Implement argument forwarding with `--` separator semantics
- [x] Implement reusable ProcessSupervisor with process groups
- [x] Implement signal forwarding, cancellation, and exit-code propagation
- [x] Implement stdin/TTY preservation and output interleaving policy
- [x] Implement human, silent, stream, aggregate, JSON, and NDJSON reporters
- [x] Implement per-package output prefix handling for future workspace use
- [x] Implement regex script selector parsing where adopted
- [x] Implement shell completion from manifest scripts
- [x] Add unit tests for env builder determinism and hook ordering
- [x] Add integration tests for signal, exit code, and quoting fixtures
- [x] Add conformance fixtures against Nub script runner behavior
- [x] Document m run as unambiguous escape hatch for built-in collisions
- [x] Benchmark script startup hot path without unbounded goroutines
- [x] Acceptance: m run dev executes script with npm-compatible environment on Linux/macOS/Windows
- [x] Acceptance: pre/post hooks run in documented order with correct failure propagation
- [x] Acceptance: Signals forwarded to child; exit code matches child process
- [x] Acceptance: m run remains explicit path when script name collides with built-in
- [x] Acceptance: Reporter modes produce deterministic structured output in CI
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0041 - Runner MVP 2 — Workspace Script Orchestration

- status: done
- plan: [0041-workspace-runner.md](0041-workspace-runner.md)

- [x] Integrate workspace filter from 0022 into script runner dispatch
- [x] Implement task graph generation from workspace dependency graph
- [x] Implement topological and reverse-topological scheduling modes
- [x] Implement parallel and sequential execution modes
- [x] Implement concurrency limits with resource-aware defaults
- [x] Implement bail, continue, resume, and changed-only failure policies
- [x] Detect workspace cycles and fail with explicit cycle diagnostics
- [x] Implement per-package output prefixes and summary aggregation
- [x] Preserve child cancellation and signal semantics from 0040
- [x] Implement readiness queue scheduler without deadlocks
- [x] Add machine-readable task events for CI consumption
- [x] Implement resume metadata for incremental workspace runs
- [x] Add synthetic DAG scheduling unit tests
- [x] Add cycle detection and failure propagation fixtures
- [x] Stress-test large workspace output and cancellation
- [x] Benchmark scheduler overhead on wide monorepos
- [x] Document workspace runner flags and failure policy semantics
- [x] Ensure deterministic task ordering for identical inputs
- [x] Acceptance: m -r run build executes packages in correct topological order
- [x] Acceptance: Concurrency limit respected; no unbounded goroutine fan-out
- [x] Acceptance: Workspace cycles diagnosed without deadlock
- [x] Acceptance: Per-package output remains attributable under parallel execution
- [x] Acceptance: Failure policies behave deterministically across platforms
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0042 - Runner MVP 3 — Direct `m <script>` Shortcuts

- status: done
- plan: [0042-direct-script-shortcuts.md](0042-direct-script-shortcuts.md)

- [x] Implement two-pass CLI dispatch after built-in and alias resolution
- [x] Wire exact package.json script fallback into main m dispatch
- [x] Preserve m run as unambiguous escape hatch for reserved names
- [x] Implement direct argument forwarding without requiring `--` when unambiguous
- [x] Implement reserved-name and built-in collision diagnostics
- [x] Implement Levenshtein or equivalent suggestion ranking (suggestions only)
- [x] Reject fuzzy execution; suggestions never auto-run scripts
- [x] Implement optional local executable lookup behind explicit policy flag
- [x] Add shell completion for dynamic manifest scripts
- [x] Document one-letter m shell alias conflicts
- [x] Build exhaustive collision matrix tests (built-in vs script names)
- [x] Test argument ambiguity and global-flag interaction
- [x] Test no-project and malformed-manifest behavior
- [x] Record intentional divergence from Nub in conformance inventory
- [x] Benchmark dispatch overhead on cold vs warm manifest reads
- [x] Gate behavior behind experimental flag until stabilization
- [x] Update feature inventory with extension compatibility_class
- [x] Ensure mx dispatch unaffected unless explicitly shared
- [x] Acceptance: m dev runs dev script when not a built-in command
- [x] Acceptance: m add runs built-in add; m run add runs add script if present
- [x] Acceptance: Misspelled commands show suggestions without executing
- [x] Acceptance: Dispatch precedence matches documented charter order
- [x] Acceptance: No fuzzy script execution occurs
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0043 - Runner MVP 4 — Local Package Binary Execution

- status: done
- plan: [0043-local-exec.md](0043-local-exec.md)

- [x] Implement bin index and lookup for current importer
- [x] Walk ancestor and workspace packages for .bin discovery
- [x] Implement explicit package-to-bin selection with ambiguity errors
- [x] Implement node_modules layout execution adapter
- [x] Implement Yarn PnP bin resolution adapter
- [x] Resolve executable identity before spawning (shebang, extensions)
- [x] Use direct process spawning; shell only when requested
- [x] Handle Windows cmd/PowerShell shims and Unix executable bits
- [x] Preserve PATH, cwd, TTY, signals, and exit codes
- [x] Fail closed with install suggestion on local miss (no registry fetch)
- [x] Integrate with ProcessSupervisor from 0040
- [x] Add bin collision and multiple-bin fixtures
- [x] Add Windows shim and PnP conformance tests
- [x] Document m exec vs mx remote execution boundary
- [x] Benchmark bin lookup on large node_modules trees
- [x] Redact credentials from exec diagnostics
- [x] Add stable error codes for missing/ambiguous bins
- [x] Ensure mx dlx does not weaken local-only exec contract
- [x] Acceptance: m exec eslint runs local bin without network access
- [x] Acceptance: Ambiguous package bins produce clear selection errors
- [x] Acceptance: PnP projects resolve bins through adapter
- [x] Acceptance: Windows shims execute with correct quoting
- [x] Acceptance: Local miss suggests install; never silently fetches
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0044 - Runner MVP 5 — `mx` Remote Fetch and Execution

- status: done
- plan: [0044-mx-dlx.md](0044-mx-dlx.md)

- [x] Implement mx argument parser and top-level dispatch
- [x] Implement local-first bin lookup reusing 0043 resolver
- [x] Implement package spec parsing and bin inference
- [x] Build ephemeral importer and minimal lock graph for remote packages
- [x] Reuse resolver, store, linker, lifecycle policy, and supervisor
- [x] Implement versioned execution-cache identity and atomic transaction
- [x] Implement TTY consent on first implicit fetch
- [x] Fail closed in non-TTY without explicit --yes
- [x] Support multiple packages and shell mode execution
- [x] Implement bin ambiguity errors with actionable diagnostics
- [x] Implement cache retention, cleanup, and prune commands
- [x] Add local-hit/no-network integration tests
- [x] Add consent and non-TTY matrix tests
- [x] Test concurrent same-spec cache construction
- [x] Add malicious lifecycle package fixtures with policy enforcement
- [x] Document mx vs m exec security boundary
- [x] Benchmark cold vs warm mx execution cache hits
- [x] Ensure execution environments isolated from project unless requested
- [x] Acceptance: mx vite@latest runs after explicit consent or --yes in CI
- [x] Acceptance: Local bin preferred without fetch when available
- [x] Acceptance: Non-TTY implicit fetch fails without --yes
- [x] Acceptance: Concurrent mx invocations share safe cache construction
- [x] Acceptance: Malicious lifecycle scripts blocked by policy
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0045 - Runner MVP 6 — Unified Execution and Snapshot Environments

- status: done
- plan: [0045-unified-execution.md](0045-unified-execution.md)

- [x] Define ExecutionRequest and PreparedEnvironment interfaces
- [x] Define environment provider contract for each source type
- [x] Refactor m exec local path onto shared environment builder
- [x] Refactor mx DLX path onto shared executable resolver
- [x] Add snapshot environment provider using lock adapters
- [x] Add capsule environment provider with integrity verification
- [x] Unify PATH, bin resolution, policy, reporter, and supervision
- [x] Expose environment identity in diagnostics and structured events
- [x] Implement explicit no-network and immutable execution modes
- [x] Add environment inspection command with provenance output
- [x] Implement cleanup leases for ephemeral execution roots
- [x] Never merge incompatible dependency graphs implicitly
- [x] Add behavior equivalence tests across all providers
- [x] Add leak and cleanup stress tests for ephemeral roots
- [x] Test concurrent execution isolation between environments
- [x] Document provider selection and failure semantics
- [x] Benchmark environment preparation across providers
- [x] Freeze public interfaces for 0046 stabilization
- [x] Acceptance: m exec and mx produce equivalent supervision through shared layer
- [x] Acceptance: Snapshot and capsule providers verify integrity before execution
- [x] Acceptance: Incompatible graphs never merge without explicit user action
- [x] Acceptance: Environment inspect shows identity, provenance, and cache state
- [x] Acceptance: Ephemeral roots cleaned up on success and failure
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0046 - Runner Stabilization Gate

- status: done
- plan: [0046-runner-stabilization.md](0046-runner-stabilization.md)

- [x] Assemble real-world script corpus across npm/pnpm/Yarn/Bun layouts
- [x] Run cross-shell quoting and process semantics corpus on all platforms
- [x] Soak long-lived processes and watch-like cancellation scenarios
- [x] Review executable trust UX for mx consent and policy surfaces
- [x] Freeze runner event schema with version field
- [x] Publish runner compatibility and divergence matrix
- [x] Verify no signal, exit-code, stdin, or output corruption bugs remain
- [x] Fully test direct script shortcut collision behavior
- [x] Certify mx never fetches implicitly in non-TTY without consent
- [x] Verify workspace scheduler determinism and resource bounds
- [x] Run multi-day process leak soak on supervisor
- [x] Run interactive TTY smoke on supported terminals
- [x] Run CI noninteractive behavior regression suite
- [x] Document known limitations and waivers with owners
- [x] Integrate runner conformance into CI stop-the-line gates
- [x] Benchmark runner hot paths against published baselines
- [x] Sign off interfaces consumed by runtime MVPs
- [x] Update feature inventory statuses to shipped where certified
- [x] Acceptance: No known signal, exit-code, stdin, or output corruption bug
- [x] Acceptance: Direct script shortcut collisions fully tested and documented
- [x] Acceptance: mx never fetches implicitly in non-TTY without explicit consent
- [x] Acceptance: Workspace scheduler deterministic and resource bounded
- [x] Acceptance: Runner conformance suite passes on Linux, macOS, Windows
- [x] Exit: No known signal, exit-code, stdin, or output corruption bug.
- [x] Exit: Direct script shortcut collision behavior is fully tested.
- [x] Exit: `mx` never fetches implicitly in non-TTY without explicit consent.
- [x] Exit: Workspace scheduler is deterministic and resource bounded.

### 0050 - Runtime MVP 1 — Node Launch and Compatibility Boundary

- status: done
- plan: [0050-node-launch-compat.md](0050-node-launch-compat.md)

- [x] Implement file-run dispatch without colliding with built-ins and scripts
- [x] Implement Node discovery interface for later 0060 integration
- [x] Implement Node argument and V8 flag classification/partitioning
- [x] Embed CommonJS and ESM preload assets via go:embed
- [x] Extract, hash-verify, and garbage-collect runtime assets on disk
- [x] Inject ESM/CJS preloads through supported Node extension surfaces
- [x] Implement --node and compatibility opt-out (zero augmentation)
- [x] Forward signals and exit codes through shared ProcessSupervisor
- [x] Detect JavaScript entrypoints (.js, .mjs, .cjs, later .ts via 0051)
- [x] Validate runtime asset digests before extraction or use
- [x] Add Node version matrix integration tests
- [x] Add CJS/ESM entrypoint and argument forwarding fixtures
- [x] Test runtime asset corruption and re-extraction recovery
- [x] Parity-test opt-out mode against plain node invocation
- [x] Document augmentation boundary vs stock Node
- [x] Benchmark cold asset extraction vs warm cache
- [x] Never mutate or patch the Node binary
- [x] Leave hooks for transform service without implementing TS yet
- [x] Acceptance: m app.js launches stock Node with embedded preloads when augmentation enabled
- [x] Acceptance: m --node app.js matches plain node behavior within documented tolerance
- [x] Acceptance: Corrupted runtime assets rejected and re-extracted safely
- [x] Acceptance: Signals and exit codes propagate correctly
- [x] Acceptance: No Node source patching or private libnode embedding
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0051 - Runtime MVP 2 — Go Transform Service and TypeScript Execution

- status: done
- plan: [0051-go-transform-service.md](0051-go-transform-service.md)

- [x] Benchmark candidate Go transformers (e.g. esbuild) against Nub/OXC corpus
- [x] Define transform request/response protocol with version, digest, options, errors, maps
- [x] Implement transform service startup, auth token, health check, cancellation
- [x] Implement idle shutdown and crash recovery for transform service
- [x] Implement Node loader bridge and format detection for .ts/.mts/.cts
- [x] Implement tsconfig parser, extends chain, and normalized compiler-option subset
- [x] Implement path mapping handoff to later resolver (0053)
- [x] Implement content-addressed transpile cache with atomic publication
- [x] Map diagnostics to original source locations
- [x] Implement fallback for unsupported syntax/options with clear errors
- [x] Add TS syntax corpus across supported Node versions
- [x] Test IPC corruption, timeout, service crash, and concurrent transforms
- [x] Add source-map stack trace integration tests
- [x] Benchmark warm-cache transform latency against budget
- [x] Document unsupported-feature report vs Nub/OXC
- [x] Keep transform output scoped to user source representation only
- [x] Spike synchronous loader IPC latency before freezing protocol
- [x] Use BLAKE3 or reviewed fast digest for cache keys
- [x] Acceptance: m app.ts executes TypeScript through stock Node without separate tsc step
- [x] Acceptance: Transform service recovers from crash without wedging Node loader
- [x] Acceptance: Transpile cache hits produce identical output bytes
- [x] Acceptance: Diagnostics reference original TypeScript sources
- [x] Acceptance: Unsupported options fail with actionable messages
- [x] Exit: All required tests pass on supported operating systems.
- [x] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [x] Exit: Public behavior and intentional deviations are documented.
- [x] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0052 - Runtime MVP 3 — JSX, Decorators, and Source-Map Parity

- status: in-progress
- plan: [0052-jsx-decorators-sourcemaps.md](0052-jsx-decorators-sourcemaps.md)

- [ ] Implement JSX option normalization (classic, automatic, importSource, dev)
- [ ] Support React, Preact, and custom JSX runtimes via tsconfig
- [ ] Implement or integrate standard decorator transforms
- [ ] Implement legacy TypeScript decorator compatibility path
- [ ] Research and choose decorator metadata emission strategy
- [ ] Implement inline and external source map generation
- [ ] Implement source-map chaining across loader stages
- [ ] Define source content inclusion policy for maps
- [ ] Implement diagnostic code frames pointing to original sources
- [ ] Add transform warnings and unsupported-option diagnostics
- [ ] Add transform parity report command for debugging
- [ ] Test React/Preact/custom JSX fixture projects
- [ ] Test decorator framework fixtures (legacy + standard)
- [ ] Verify stack traces through imports and async functions
- [ ] Include JSX/decorator options in transpile cache keys
- [ ] Document exact differences from TypeScript compiler
- [ ] Treat decorator metadata as separately certified capability
- [ ] Benchmark JSX/decorator transform hot paths
- [ ] Acceptance: m component.tsx runs with correct JSX runtime per tsconfig
- [ ] Acceptance: Legacy and standard decorators transpile for supported frameworks
- [ ] Acceptance: Stack traces map to original TSX/TS sources
- [ ] Acceptance: Transform parity report lists known divergences
- [ ] Acceptance: Cache keys change when relevant JSX/decorator options change
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0053 - Runtime MVP 4 — Module Resolution, Path Aliases, and Custom Loaders

- status: planned
- plan: [0053-module-resolution-loaders.md](0053-module-resolution-loaders.md)

- [ ] Plan resolver augmentation without replacing Node resolution wholesale
- [ ] Preserve Node CJS and ESM resolution semantics baseline
- [ ] Implement tsconfig baseUrl and paths matcher
- [ ] Implement .js to .ts development extension mapping policy
- [ ] Implement CJS require registration hooks where needed
- [ ] Implement ESM custom loader and preload chaining
- [ ] Document and enforce custom loader execution order
- [ ] Pass original user loader arguments through chain
- [ ] Implement isolated node_modules layout awareness
- [ ] Implement Yarn PnP runtime resolution adapter
- [ ] Support package imports/exports and conditions where explicitly adopted
- [ ] Implement self-reference and URL module policy boundaries
- [ ] Preserve Node-compatible error context plus Mew explanations
- [ ] Add module trace diagnostics command
- [ ] Test Node package exports/imports corpus
- [ ] Test CJS/ESM interop and monorepo path alias fixtures
- [ ] Test custom loader composition scenarios
- [ ] Benchmark resolution hot path with cache
- [ ] Acceptance: tsconfig paths resolve consistently in monorepos
- [ ] Acceptance: Custom loaders run in documented order with user args preserved
- [ ] Acceptance: PnP projects resolve modules through adapter
- [ ] Acceptance: Resolution errors include Node context and Mew guidance
- [ ] Acceptance: Plain Node opt-out bypasses Mew resolution hooks
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0054 - Runtime MVP 5 — Environment Loading, Workers, Storage, and Modern APIs

- status: planned
- plan: [0054-env-modern-apis.md](0054-env-modern-apis.md)

- [ ] Implement .env parser with variable expansion rules
- [ ] Implement mode-aware .env* discovery and precedence
- [ ] Support explicit --env-file and --no-env-file kill switch
- [ ] Define shell environment vs file vs flag precedence
- [ ] Construct per-child environment overlays explicitly in Go
- [ ] Never mutate global process environment from concurrent Go code
- [ ] Inject runtime state into worker threads and child Node processes
- [ ] Ensure worker augmentation avoids recursive unrelated services
- [ ] Implement selected Web Storage compatibility APIs
- [ ] Define storage persistence and isolation policy
- [ ] Wire NODE_ENV and --mode interaction documented
- [ ] Add environment trace diagnostics with redacted values by default
- [ ] Test precedence and expansion matrix exhaustively
- [ ] Prepare watch reload hooks for env/tsconfig changes (0055)
- [ ] Test worker and child-process propagation
- [ ] Test storage isolation and corruption recovery
- [ ] Document security implications of env expansion
- [ ] Benchmark env overlay construction per spawn
- [ ] Acceptance: --env-file overrides auto-discovery per documented policy
- [ ] Acceptance: Child processes receive explicit env overlays; parent env not raced
- [ ] Acceptance: Workers inherit transform/runtime hooks without recursive services
- [ ] Acceptance: Env trace redacts secrets by default
- [ ] Acceptance: Web Storage APIs behave per documented persistence policy
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0055 - Runtime MVP 6 — Dependency-Aware Watch Mode

- status: planned
- plan: [0055-watch-mode.md](0055-watch-mode.md)

- [ ] Implement watcher abstraction with native and polling backends
- [ ] Implement long-lived supervisor and short-lived application child
- [ ] Collect dependency files from transform and module resolution hooks
- [ ] Watch tsconfig extends chains, package.json, env files, and globs
- [ ] Implement debounce and restart coalescing policy
- [ ] Implement clear-screen policy flag
- [ ] Implement restart-on-demand interactive key
- [ ] Rebuild child environment and runtime state on every restart
- [ ] Implement restart state machine with signal escalation
- [ ] Normalize short/long paths, case, and symlinks for watcher identity
- [ ] Handle atomic save, rename, delete/recreate edge cases
- [ ] Reload env and tsconfig changes without supervisor crash
- [ ] Never execute user application code in supervisor process
- [ ] Add rapid change and restart storm tests
- [ ] Test child ignoring termination and forced kill paths
- [ ] Run resource leak soak on watch sessions
- [ ] Benchmark watcher CPU use on large trees
- [ ] Document platform watcher limitations
- [ ] Acceptance: m watch restarts app when relevant source or config changes
- [ ] Acceptance: Supervisor survives env/tsconfig reloads
- [ ] Acceptance: Debouncing prevents restart storms on rapid saves
- [ ] Acceptance: No process or file descriptor leaks in soak tests
- [ ] Acceptance: TTY and signal behavior preserved across restarts
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0056 - Runtime MVP 7 — Debugging, Inspection, and Runtime Diagnostics

- status: planned
- plan: [0056-debugging-inspection.md](0056-debugging-inspection.md)

- [ ] Route --inspect and --inspect-brk flags to stock Node unchanged
- [ ] Handle inspector port allocation and collision diagnostics
- [ ] Integrate source-map support across transforms and loaders
- [ ] Define runtime trace event schema with versioning
- [ ] Emit transform, cache, env, module, worker, and watch trace events
- [ ] Implement module and transform timing diagnostic views
- [ ] Implement cache explain command for transpile cache
- [ ] Implement support bundle collection with redaction policy
- [ ] Ensure traces do not materially change runtime ordering
- [ ] Redact secrets and sensitive source content per policy
- [ ] Add inspector startup and break-on-start tests
- [ ] Test mapped breakpoints and stack traces in TS/TSX
- [ ] Benchmark trace overhead when diagnostics enabled
- [ ] Document debugger configuration for common editors
- [ ] Compare behavior against m --node opt-out baseline
- [ ] Publish safe defaults for CI (no inspect bind to 0.0.0.0)
- [ ] Freeze trace schema before 0057 stabilization
- [ ] Add doctor runtime checks for common misconfigurations
- [ ] Acceptance: m --inspect-brk app.ts breaks on first line with mapped sources
- [ ] Acceptance: Stack traces map through transforms to original TypeScript
- [ ] Acceptance: Support bundles contain no secrets or full source by default
- [ ] Acceptance: Trace output validates against published schema
- [ ] Acceptance: Diagnostics do not change execution order materially
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0057 - Runtime Stabilization Gate

- status: planned
- plan: [0057-runtime-stabilization.md](0057-runtime-stabilization.md)

- [ ] Run syntax and framework corpus across supported Node versions
- [ ] Certify CJS/ESM/loader/worker/watch coverage with published results
- [ ] Run Node compatibility and --node opt-out differential tests
- [ ] Soak transform service for crashes, leaks, and IPC failure recovery
- [ ] Complete security review of IPC and embedded runtime assets
- [ ] Freeze runtime protocol versions (transform IPC, trace, loader bridge)
- [ ] Publish runtime support matrix with certification evidence
- [ ] Verify no transform cache corruption or source-map integrity bugs
- [ ] Verify watch and workers do not leak processes or file descriptors
- [ ] Run cold/warm startup benchmark suite with baselines
- [ ] Document fallback behavior and known limitations
- [ ] Integrate runtime conformance into CI stop-the-line gates
- [ ] Run long-running worker/watch multi-day soak
- [ ] Sign off interfaces for 0060 Node manager integration
- [ ] Update feature inventory to shipped for certified runtime features
- [ ] Record waivers with owners for documented divergences
- [ ] Ensure plain Node escape hatch remains behaviorally plain
- [ ] Gate experimental runtime features behind explicit flags
- [ ] Acceptance: Supported syntax and Node versions have published certification
- [ ] Acceptance: No known transform cache corruption or source-map integrity bug
- [ ] Acceptance: Watch and workers pass leak soak without orphaned processes
- [ ] Acceptance: Plain Node escape hatch matches stock node within tolerance
- [ ] Acceptance: Runtime conformance passes on Linux, macOS, Windows
- [ ] Exit: Supported syntax and Node versions have published certification results.
- [ ] Exit: No known transform cache corruption or source-map integrity bug.
- [ ] Exit: Watch and workers do not leak processes, services, or file descriptors.
- [ ] Exit: Plain Node escape hatch remains behaviorally plain.

### 0060 - Manager MVP 1 — Node Version Manager

- status: planned
- plan: [0060-node-manager.md](0060-node-manager.md)

- [ ] Implement Node release metadata client and local cache
- [ ] Implement version range, alias, and LTS resolution
- [ ] Select platform/architecture artifacts deterministically
- [ ] Download artifacts with retries and proxy/CA support
- [ ] Verify checksums before any extraction
- [ ] Extract and atomically publish immutable Node installations
- [ ] Discover pins from .nvmrc, .node-version, package.json engines/devEngines
- [ ] Define pin precedence across project and Mew config
- [ ] Implement per-project pin and automatic provisioning hooks
- [ ] Integrate resolved Node selection with 0050 runtime launch
- [ ] Implement m node command family with stable error codes
- [ ] Implement offline cache usage and installation GC/prune
- [ ] Add platform artifact fixture server tests
- [ ] Add checksum and extraction attack corpus tests
- [ ] Test version precedence and offline install scenarios
- [ ] Document separation of version resolution vs PATH shims (0062)
- [ ] Benchmark install and resolve hot paths
- [ ] Never execute unverified downloaded Node binary
- [ ] Acceptance: m node install 22 installs verified Node for current platform
- [ ] Acceptance: Tampered artifacts rejected before extraction
- [ ] Acceptance: Project pin resolves consistently across commands
- [ ] Acceptance: Runtime launch uses Node manager selection from 0050
- [ ] Acceptance: Offline install works from warm cache
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0061 - Manager MVP 2 — Package-Manager Meta-Manager

- status: planned
- plan: [0061-pm-manager.md](0061-pm-manager.md)

- [ ] Implement package-manager identity and version resolver
- [ ] Detect from packageManager, devEngines, lockfiles, executables, config
- [ ] Implement verified acquisition and content-addressed PM cache
- [ ] Run external managers in isolated verified environments
- [ ] Select Node version via 0060 for external PM invocation
- [ ] Implement m pm pin and update commands with per-major models
- [ ] Implement migration planner using lock adapters from core PM MVPs
- [ ] Execute migrations inside transaction with rollback snapshot
- [ ] Produce semantic loss report for every migration
- [ ] Implement cache inspection and prune commands
- [ ] Never silently rewrite incumbent lockfile identity
- [ ] Preserve m.lock as native; support npm/pnpm/Yarn/Bun round trips
- [ ] Add pinned manager install and invocation tests
- [ ] Add migration corpus with rollback verification
- [ ] Test ambiguous identity detection and diagnostics
- [ ] Document Corepack replacement positioning
- [ ] Benchmark PM invocation startup overhead
- [ ] Integrate trust policy for downloaded manager artifacts
- [ ] Acceptance: m pm which reports correct manager for incumbent lockfiles
- [ ] Acceptance: Pinned manager version used for invocation
- [ ] Acceptance: Migration produces loss report and rollback snapshot
- [ ] Acceptance: Failed migration restores prior manifest/lock state
- [ ] Acceptance: External PM runs under selected Node from 0060
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0062 - Manager MVP 3 — Node, PM, and Self Shims

- status: planned
- plan: [0062-shims.md](0062-shims.md)

- [ ] Design shim protocol and cross-platform installation paths
- [ ] Implement POSIX launcher scripts and Windows exe/cmd strategy
- [ ] Resolve pinned Node version for node shim without runtime augmentation
- [ ] Dispatch package-manager shims from project identity via 0061
- [ ] Support optional Mew self-version pinning in shims
- [ ] Implement recursion prevention and emergency bypass env vars
- [ ] Implement shim status, repair, backup, and uninstall flows
- [ ] Never replace unrelated executables without confirmation and backup
- [ ] Report resolved target in debug/diagnostic mode
- [ ] Integrate shell completion and PATH setup instructions
- [ ] Test PATH precedence matrix across shells
- [ ] Test recursion and broken-pin recovery scenarios
- [ ] Test Windows quoting and executable extension handling
- [ ] Document node vs m/mx augmentation boundary clearly
- [ ] Audit shim contents for minimal attack surface
- [ ] Benchmark shim dispatch overhead
- [ ] Coordinate with 0072 installers for default shim paths
- [ ] Ensure plain node calls through shim do not inject Mew preloads
- [ ] Acceptance: node shim runs pinned stock Node without Mew augmentation
- [ ] Acceptance: m/mx shims resolve project-pinned Mew version when configured
- [ ] Acceptance: Recursion guards prevent shim loops
- [ ] Acceptance: m shim remove restores prior PATH state from backup
- [ ] Acceptance: Windows shims handle extensions and quoting correctly
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0070 - Product MVP 1 — TypeScript-First Project Initialization

- status: planned
- plan: [0070-project-init.md](0070-project-init.md)

- [ ] Define scaffold templates as embedded versioned assets
- [ ] Implement interactive and noninteractive init prompts/flags
- [ ] Generate package.json with module type, scripts, and Mew identity defaults
- [ ] Generate tsconfig with sensible strict defaults
- [ ] Generate source layout, gitignore, and editor config stubs
- [ ] Implement dry-run file plan preview before write
- [ ] Integrate initial install via transaction boundary from core PM
- [ ] Rollback all writes on init failure or interruption
- [ ] Refuse overwrite of nonempty directories without explicit policy
- [ ] Delegate specialized framework templates to mx create-* hints
- [ ] Use m.lock as default for greenfield projects
- [ ] Preserve incumbent lockfile if detected in existing directory
- [ ] Add golden tests for generated project trees
- [ ] Smoke-test build/run on generated projects via 0051 runtime
- [ ] Test nonempty directory and interruption recovery
- [ ] Document every generated choice before commit
- [ ] Benchmark init time cold vs warm cache
- [ ] Publish migration notes from npm create equivalents
- [ ] Acceptance: m init creates deterministic TS project that runs with m dev
- [ ] Acceptance: Failed init leaves no partial manifest or half-written tree
- [ ] Acceptance: manifest-only mode writes only package.json
- [ ] Acceptance: Nonempty directory policy enforced with clear errors
- [ ] Acceptance: Framework templates directed to mx, not hidden in m init
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0071 - Product MVP 2 — External Command Plugin Convention

- status: planned
- plan: [0071-plugins.md](0071-plugins.md)

- [ ] Define m-<verb> executable naming and handshake protocol
- [ ] Discover plugins from PATH without loading untrusted code into m
- [ ] Enforce built-in command precedence; plugins never shadow built-ins
- [ ] Implement plugin metadata cache for completion and doctor
- [ ] Implement version compatibility checks in handshake
- [ ] Spawn plugins as subprocesses with minimized environment/credentials
- [ ] Implement structured plugin events and exit code propagation
- [ ] Implement m plugin list and doctor commands
- [ ] Display plugin trust and origin in diagnostics
- [ ] Support optional installation via mx or package-manager command
- [ ] Document SDK examples in JavaScript, Go, and shell
- [ ] Test shadowing and version mismatch failures
- [ ] Test malicious plugin output/protocol handling
- [ ] Test cross-platform executable discovery
- [ ] Integrate completion for discovered plugin verbs
- [ ] Audit plugin env surface against trust policy
- [ ] Benchmark plugin dispatch overhead
- [ ] Freeze handshake protocol before public SDK publish
- [ ] Acceptance: m hello runs m-hello on PATH when not a built-in
- [ ] Acceptance: Built-in commands always win over plugin names
- [ ] Acceptance: Plugin handshake rejects incompatible protocol versions
- [ ] Acceptance: m plugin doctor reports origin and trust metadata
- [ ] Acceptance: No untrusted code loaded into m process address space
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0072 - Distribution MVP 1 — Releases, Installers, and Package Channels

- status: planned
- plan: [0072-installers-releases.md](0072-installers-releases.md)

- [ ] Configure GoReleaser or equivalent with reproducible build flags
- [ ] Build release matrix for Linux, macOS, Windows supported architectures
- [ ] Embed runtime assets and license manifests in release artifacts
- [ ] Generate checksums, signatures, attestations, and SBOMs
- [ ] Implement POSIX and PowerShell installers with verification step
- [ ] Publish Homebrew, Scoop, Winget, and npm bootstrap package definitions
- [ ] Implement signed versioned channel manifests
- [ ] Implement m upgrade with staged replacement and rollback
- [ ] Retain recoverable previous version after update
- [ ] Reject tampered manifest or binary during install/update
- [ ] Add clean VM installation tests per platform
- [ ] Test interrupted replacement and locked-file Windows cases
- [ ] Document uninstall instructions per channel
- [ ] Coordinate default shim paths with 0062
- [ ] Require 0046 and 0057 stabilization before stable channel
- [ ] Publish benchmark baselines alongside release artifacts
- [ ] Never commit signing keys; use CI secret management
- [ ] Include m.lock compatibility note in release notes template
- [ ] Acceptance: Install script verifies checksum/signature before executing m
- [ ] Acceptance: m upgrade rolls back safely on failure
- [ ] Acceptance: Release artifacts include m and mx for all supported platforms
- [ ] Acceptance: Channel manifests are signed and versioned
- [ ] Acceptance: SBOM and provenance published per release
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0073 - Distribution MVP 2 — GitHub Action and CI Integration

- status: planned
- plan: [0073-github-action.md](0073-github-action.md)

- [ ] Implement action metadata and TypeScript/JavaScript bundle
- [ ] Download release artifacts from 0072 with checksum verification
- [ ] Implement version/channel input resolution
- [ ] Integrate Node provisioning via m node from 0060
- [ ] Implement cache key computation with format version salts
- [ ] Restore/save store, metadata, transform, and execution caches
- [ ] Implement frozen install and capsule restore helpers for CI
- [ ] Add GitHub job summaries and problem matchers
- [ ] Keep action outputs stable within major release
- [ ] Test GitHub-hosted Linux/macOS/Windows matrix
- [ ] Test fork PR credential safety (no secret leakage)
- [ ] Test cache poisoning and stale cache invalidation
- [ ] Document example workflows for monorepos and m.lock projects
- [ ] Never cache credentials or unsafe mutable project state
- [ ] Expose diagnostics for cache hit/miss reasons
- [ ] Pin action release to signed 0072 artifacts
- [ ] Benchmark CI install time cold vs warm cache
- [ ] Publish versioning policy for setup-m major bumps
- [ ] Acceptance: setup-m installs verified m on GitHub-hosted runners
- [ ] Acceptance: Cache restore speeds repeat CI runs without correctness loss
- [ ] Acceptance: Fork PRs do not expose repository secrets via action
- [ ] Acceptance: Node version inputs provision correct runtime
- [ ] Acceptance: Action outputs remain stable for v1 consumers
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0074 - Distribution MVP 3 — Docker Images and Hosted Builder Integration

- status: planned
- plan: [0074-docker-builders.md](0074-docker-builders.md)

- [ ] Create slim and full Dockerfiles from signed release artifacts
- [ ] Implement multi-architecture build and publish pipeline
- [ ] Run containers as non-root with documented writable cache paths
- [ ] Document BuildKit cache mount patterns for store and transform caches
- [ ] Provide capsule workflow examples for immutable CI dependencies
- [ ] Add hosted-builder install snippets (Railway, Fly, etc.) where applicable
- [ ] Detect lockfile type in container entry guidance
- [ ] Scan images for vulnerabilities in CI
- [ ] Smoke-test rootless and read-only filesystem recipes
- [ ] Test multi-arch execution on amd64 and arm64
- [ ] Document libc and native-addon implications for musl vs glibc
- [ ] Never embed registry credentials in image layers
- [ ] Pin image tags to reproducible release versions
- [ ] Coordinate Node provisioning inside images with 0060
- [ ] Publish read-only filesystem workaround patterns
- [ ] Benchmark image size and cold m install in container
- [ ] Add docker compose examples for monorepo CI
- [ ] Link docs from 0073 GitHub Action for hybrid workflows
- [ ] Acceptance: docker run mewjs/m:latest m --version succeeds on amd64 and arm64
- [ ] Acceptance: Images run non-root by default with working cache dirs
- [ ] Acceptance: Vulnerability scan gate passes on release images
- [ ] Acceptance: BuildKit examples demonstrate cache-efficient m install
- [ ] Acceptance: No credentials appear in image layers or history
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0080 - Cross-Cutting — Compatibility and Conformance Program

- status: planned
- plan: [0080-conformance-program.md](0080-conformance-program.md)

- [ ] Define certification axes: behavior, lockfile, layout, runtime
- [ ] Maintain target matrix per npm/pnpm/Yarn/Bun/Nub/m.lock majors
- [ ] Wire differential fixtures from 0008
- [ ] Record pinned tool versions per target
- [ ] Fail CI on certified regression without waiver
- [ ] Produce machine-readable conformance report
- [ ] Human summary for release notes
- [ ] Separate experimental vs certified suites
- [ ] Document how to add a new target
- [ ] Link inventory features to conformance IDs
- [ ] Windows/macOS/Linux required for layout tests
- [ ] Clean-home mandatory
- [ ] No public registry in certified jobs
- [ ] Quarantine flaky tests with expiry
- [ ] Integrate with 0031/0046/0057 gates
- [ ] Dashboard or artifact index
- [ ] Periodic re-baselines process
- [ ] Agent protocol for updating goldens
- [ ] Acceptance: Certified suite green on supported OS matrix
- [ ] Acceptance: Every compatibility target has pinned version + fixtures
- [ ] Acceptance: Regressions block release train gates
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0081 - Cross-Cutting — Performance and Resource Program

- status: planned
- plan: [0081-performance-program.md](0081-performance-program.md)

- [ ] List hot paths: startup, resolve, fetch, extract, link, scripts, transform
- [ ] Define cold vs warm cache benches
- [ ] Publish baseline artifacts with machine metadata
- [ ] CI regression detection with noise tolerance
- [ ] Resource ceilings: goroutines, FDs, memory
- [ ] Document waiver process with expiry
- [ ] Bench install on fixture graphs S/M/L
- [ ] Bench m --help startup
- [ ] Bench transform TS hello
- [ ] Track disk amplification
- [ ] Track network request counts
- [ ] Flamegraph/pprof guidance
- [ ] No unbounded worker pools
- [ ] Link budgets to MVP owners
- [ ] Windows bench runners where meaningful
- [ ] Separate micro vs end-to-end
- [ ] Prevent silent bench deletion
- [ ] Agent evidence requires bench commands
- [ ] Acceptance: Baselines recorded for critical paths
- [ ] Acceptance: CI fails on budget breach without waiver
- [ ] Acceptance: Cold/warm separated in reports
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0082 - Cross-Cutting — Threat Model and Security Review Plan

- status: planned
- plan: [0082-threat-model.md](0082-threat-model.md)

- [ ] Identify assets: credentials, lockfiles, store, scripts, Node bins
- [ ] Define trust boundaries: registry, git, filesystem, plugins, CI
- [ ] Enumerate adversaries and abuse cases
- [ ] Map controls to MVPs (integrity, sandbox, consent, redaction)
- [ ] Mandatory review triggers for security-boundary changes
- [ ] Secure coding checklist
- [ ] Fail-closed defaults policy
- [ ] Archive extraction threat cases
- [ ] Lifecycle script threat cases
- [ ] mx consent threat cases
- [ ] Shim PATH hijack cases
- [ ] Dependency confusion cases
- [ ] Signed release verification
- [ ] Incident response outline
- [ ] Link to 0021/0030/0044/0062
- [ ] Periodic threat-model refresh
- [ ] No secrets in plans/logs
- [ ] Agent escalation triggers
- [ ] Acceptance: Threat model covers download+execute surfaces
- [ ] Acceptance: Every abuse case maps to a control or accepted risk
- [ ] Acceptance: Security boundary PRs require checklist
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0083 - Cross-Cutting — Nub Rust to Mew Go Migration Map

- status: planned
- plan: [0083-rust-go-migration-map.md](0083-rust-go-migration-map.md)

- [ ] Inventory Nub crates and runtime assets
- [ ] Inventory Aube crates
- [ ] Map each to Go package or omission
- [ ] Attach owner MVP and test ID
- [ ] Mark JS assets that remain JS
- [ ] Mark behavioral replacements vs ports
- [ ] Document deprecated Nub behaviors Mew changes
- [ ] Keep map synchronized with 0003
- [ ] Reject line-by-line ports in review
- [ ] Cover CLI, core, native, runtime, installers
- [ ] Note data formats and error codes
- [ ] Note caches and protocols
- [ ] Update when upstream Nub commit changes
- [ ] Link sources/nub-reference-snapshot.md
- [ ] Agent reading order includes this map
- [ ] Track gaps as research spikes when needed
- [ ] No silent omissions
- [ ] Publish human-readable migration guide outline
- [ ] Acceptance: Every Nub crate has map/omit row
- [ ] Acceptance: Every mapped row has owner MVP
- [ ] Acceptance: Intentional omissions documented
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0084 - Cross-Cutting — Versioning, Formats, and Support Policy

- status: planned
- plan: [0084-release-versioning-policy.md](0084-release-versioning-policy.md)

- [ ] Define semver promises for CLI
- [ ] Define stability for Go public packages if any
- [ ] Lockfile version bump rules
- [ ] Cache version invalidation rules
- [ ] Plan/capsule/plugin compatibility
- [ ] Node version support windows
- [ ] Package-manager adapter support windows
- [ ] Experimental feature graduation rules
- [ ] Breaking change communication process
- [ ] Deprecation timeline minimums
- [ ] Windows/macOS/Linux support statement
- [ ] Document what is not covered
- [ ] Link to 0009 release channels
- [ ] Upgrade/rollback testing requirements
- [ ] Agent must note format version impacts
- [ ] Compatibility matrix published
- [ ] Align with 0087 DoD
- [ ] Review cadence
- [ ] Acceptance: Every persistent format has version policy
- [ ] Acceptance: Node floor documented
- [ ] Acceptance: Experimental graduation criteria clear
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0085 - Cross-Cutting — Go Dependency Selection Roadmap

- status: planned
- plan: [0085-dependency-roadmap.md](0085-dependency-roadmap.md)

- [ ] Prefer stdlib everywhere feasible
- [ ] Evaluate CLI library candidates
- [ ] Evaluate semver library candidates
- [ ] Evaluate archive/compress libs vs stdlib
- [ ] Evaluate HTTP hardening needs
- [ ] Evaluate FS notify for watch
- [ ] Evaluate transform parser deps
- [ ] Pin versions in go.mod
- [ ] License compatibility checks
- [ ] govulncheck in CI
- [ ] Periodic review calendar
- [ ] Document rejected alternatives
- [ ] No dependency for YAGNI features
- [ ] Size/attack-surface notes
- [ ] Windows support required for chosen deps
- [ ] Update 0004 bootstrap accordingly
- [ ] Agent must not add deps without ADR when required
- [ ] Publish dependency roadmap table
- [ ] Acceptance: Roadmap covers CLI, semver, transform, FS, net, archive, security, release
- [ ] Acceptance: Every non-stdlib dep has rationale
- [ ] Acceptance: Vulncheck wired
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0086 - Cross-Cutting — AI Agent Implementation Protocol

- status: planned
- plan: [0086-ai-agent-protocol.md](0086-ai-agent-protocol.md)

- [ ] Required reading order for agents
- [ ] Predecessor checks before coding
- [ ] Behavior-first research rules
- [ ] Small PR policy
- [ ] Test and benchmark evidence requirements
- [ ] Persistent-format safeguards
- [ ] Security escalation triggers
- [ ] Handoff status reporting
- [ ] Evidence template
- [ ] Review checklist
- [ ] Thread file schema status values
- [ ] Forbid secrets in threads
- [ ] Link to plan files and inventory updates
- [ ] No force-push/default-branch rules reminder
- [ ] Windows verification reminder
- [ ] Clean-home test reminder
- [ ] Align with AGENTS.md non-negotiables
- [ ] Document when to stop for human decisions
- [ ] Acceptance: Agents have deterministic workflow doc
- [ ] Acceptance: Evidence template lists 6 handoff items
- [ ] Acceptance: Human-owned decisions called out
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0087 - Cross-Cutting — Global Definition of Done

- status: planned
- plan: [0087-definition-of-done.md](0087-definition-of-done.md)

- [ ] Create review checklist covering behavior/interfaces/tests/docs/security/perf/compat/migration/recovery
- [ ] Create evidence index template
- [ ] Exception/waiver process with expiry
- [ ] Owner sign-off for persistent format changes
- [ ] Owner sign-off for security boundary changes
- [ ] Audit three completed MVPs against checklist (when available)
- [ ] CI fails expired waivers
- [ ] Final program exit criteria from 0087 plan
- [ ] Known-limitations disclosure required
- [ ] Feature inventory updated before Done
- [ ] Conformance links required
- [ ] No open critical integrity issues
- [ ] Release artifacts reproducible
- [ ] Align stabilization gates
- [ ] Agent cannot mark Done without evidence
- [ ] Document support expectations
- [ ] Link 0080/0081/0082/0084
- [ ] Publish DoD in docs/
- [ ] Acceptance: DoD checklist exists and is used
- [ ] Acceptance: Waivers expire automatically in policy
- [ ] Acceptance: Format/security changes need owner sign-off
- [ ] Exit: All planned feature inventory rows are shipped, intentionally omitted, or moved to an approved future backlog.
- [ ] Exit: All supported compatibility targets pass certification.
- [ ] Exit: All public formats have tested upgrade, recovery, and rollback paths.
- [ ] Exit: No open critical security or data-integrity issue.
- [ ] Exit: Release and installation channels are reproducible and verified.

### 0088 - Reference Index and Research Sources

- status: planned
- plan: [0088-reference-index.md](0088-reference-index.md)

- [ ] Index Nub behavior sources with commit pins
- [ ] Index npm/pnpm/Yarn/Bun lock format docs
- [ ] Index Node loader/preload/inspector docs
- [ ] Index registry protocol docs
- [ ] Index security standards (SRI, SBOM, provenance)
- [ ] Index Go implementation decision sources
- [ ] Keep plans/sources synchronized
- [ ] Record retrieval dates
- [ ] Mark stale sources
- [ ] Link MVPs to references
- [ ] No private URLs/secrets
- [ ] Process to refresh before major implementation waves
- [ ] Agent must cite sources for parity claims
- [ ] Separate normative vs informative
- [ ] Include license notes
- [ ] Cross-link 0083 migration map
- [ ] Publish docs/references/README
- [ ] Validate links periodically
- [ ] Acceptance: Authoritative source list exists
- [ ] Acceptance: Nub commit pin recorded
- [ ] Acceptance: Parity claims can cite a source entry
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0089 - Open Research Spikes and Decision Gates

- status: planned
- plan: [0089-research-spikes.md](0089-research-spikes.md)

- [ ] List spikes that can invalidate later contracts
- [ ] Timebox each spike
- [ ] Require written decision outcome
- [ ] Decorator metadata spike linked to 0052
- [ ] Transform IPC vs in-process spike linked to 0051
- [ ] Yarn PnP write certification spike linked to 0025
- [ ] Reflink reliability spike linked to 0018
- [ ] Sandbox capability matrix spike linked to 0021
- [ ] No production code from spike unless promoted
- [ ] Human-owned decisions surfaced
- [ ] Update Open Decisions in affected MVPs
- [ ] Archive spike evidence
- [ ] Prevent starting dependent MVP until gate cleared
- [ ] Agent protocol for spike threads
- [ ] Keep backlog of resolved spikes
- [ ] Link to 0009 stop-the-line when needed
- [ ] Do not expand critical path casually
- [ ] Publish spike index
- [ ] Acceptance: Open spikes listed with owners and due dates
- [ ] Acceptance: Blocking spikes prevent dependent MVP start
- [ ] Acceptance: Resolved spikes recorded with decisions
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.

### 0090 - Future Extensions Beyond Nub Parity

- status: planned
- plan: [0090-future-backlog.md](0090-future-backlog.md)

- [ ] OPTIONAL: Capture post-parity ideas without expanding critical path
- [ ] OPTIONAL: Tag each idea with value/risk/effort
- [ ] OPTIONAL: Require charter update before promotion
- [ ] OPTIONAL: Separate nice-to-have UX polish ideas
- [ ] OPTIONAL: Note ideas that conflict with stock-Node boundary
- [ ] OPTIONAL: Periodic backlog grooming
- [ ] OPTIONAL: Link rejected ideas with rationale
- [ ] OPTIONAL: Keep out of stabilization gates
- [ ] OPTIONAL: Do not assign primary_mvp that blocks 0031/0046/0057
- [ ] OPTIONAL: Agent must not implement backlog as if required
- [ ] OPTIONAL: Document promotion checklist
- [ ] OPTIONAL: Examples only — phantom dep analysis, extra templates
- [ ] OPTIONAL: Ensure INDEX marks 0090 as future
- [ ] OPTIONAL: No conformance dependency
- [ ] OPTIONAL: Review during release planning only
- [ ] Acceptance: Backlog explicitly non-blocking
- [ ] Acceptance: No critical-path MVP lists 0090 as required predecessor
- [ ] Acceptance: Promotion requires human decision
- [ ] Exit: All required tests pass on supported operating systems.
- [ ] Exit: No unresolved correctness, integrity, or data-loss issue remains.
- [ ] Exit: Public behavior and intentional deviations are documented.
- [ ] Exit: The next dependent MVP can consume stable interfaces without reaching into internals.
