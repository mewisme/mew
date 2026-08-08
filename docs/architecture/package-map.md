# Package map

Authoritative listing of repository paths and one-line purposes.
Every path named in [`AGENTS.md`](../../AGENTS.md) repository shape must appear here.

**Path state** — `absent` (no directory), `reserved` (documented placeholder), `exists` (on disk today).

**Capability state** — `implemented`, `partial`, `scaffolded`, `planned`, or `reserved`.

- `implemented` — production code with tests; ships to users.
- `partial` — core path works; non-trivial surface still gated or missing.
- `scaffolded` — directory and types exist; no production path yet.
- `planned` — scheduled for a specific MVP but no code.
- `reserved` — namespace held; no user-facing capability.

Do not use "certified" unless a documented certification suite exists and is passing. Do not use "experimental" or "shipped" — those describe gating/release, not implementation state.

## Entry and presentation

| Path | Purpose | Path state | Capability state |
|---|---|---|---|
| `cmd/m/` | Primary CLI entrypoint binary | exists | implemented |
| `cmd/mx/` | Package executor entrypoint binary | exists | implemented |
| `internal/app/` | Process-level orchestration across domains | exists | implemented |
| `internal/cli/` | Parsing, dispatch, help, completions | exists | implemented |
| `internal/config/` | Layered configuration loader | exists | implemented |
| `internal/diagnostics/` | Errors, progress, redaction, reporters | exists | implemented |
| `internal/presentation/` | Output modes, capabilities, themes, static renderers | exists | implemented |
| `internal/presentation/help/` | Plain + internal ASCII-only rich Markdown renderers for topic help (no Glamour) | exists | implemented |
| `internal/presentation/pager/` | Safe optional pager resolve/exec for topic help | exists | implemented |
| `internal/help/` | Embedded terminal-help topic registry (no Charm) | exists | implemented |
| `docs/terminal-help/` | Curated embedded topic Markdown + embed FS | exists | implemented |
| `internal/prompt/` | Interactive policy and Prompter contract (stdlib only) | exists | implemented |
| `internal/presentation/prompt/` | Huh rich + accessible numbered prompt adapters | exists | implemented |
| `internal/apperr/` | Typed ERR_M_* errors and exit mapping | exists | implemented |
| `internal/trace/` | Lightweight in-process spans (no OTel) | exists | implemented |
| `internal/charter/` | Charter consistency tests (docs gate) | exists | implemented |
| `internal/archcheck/` | Import-graph and package-map acceptance tests | exists | implemented |
| `internal/bootstrap/` | Clean-clone style repository gate tests | exists | implemented |

## Package-manager domain

| Path | Purpose | Path state | Capability state |
|---|---|---|---|
| `internal/manifest/` | package.json read/normalize/edit | exists | implemented |
| `internal/project/` | Project root discovery and identity | exists | implemented |
| `internal/workspace/` | Workspace graph, filters, catalogs | exists | implemented |
| `internal/registry/` | Registry clients, auth, metadata cache | exists | implemented |
| `internal/resolver/` | Semver + graph resolution + traces | exists | implemented |
| `internal/semver/` | npm-compatible range satisfaction (Masterminds/v3) | exists | implemented |
| `internal/lockfile/` | Canonical graph + format adapters | exists | implemented |
| `internal/lockfile/mlock/` | Native m.lock codec | exists | implemented |
| `internal/fetch/` | Concurrent tarball download | exists | implemented |
| `internal/archive/` | Safe extraction and path validation | exists | implemented |
| `internal/store/` | Content-addressed global store | exists | implemented |
| `internal/linker/` | Hoisted/isolated layouts + bins | exists | partial |
| `internal/linker/planner/` | hardlink/reflink/copy/symlink/junction | exists | implemented |
| `internal/transaction/` | Stage, journal, commit, rollback | exists | implemented |
| `internal/lifecycle/` | Dependency lifecycle scripts | exists | implemented |
| `internal/policy/` | Trust and sandbox policy | exists | implemented |
| `internal/graph/` | Shared canonical graph helpers | exists | implemented |
| `internal/plan/` | Mutation plan types | exists | implemented |
| `internal/snapshot/` | Install history snapshots | exists | implemented |
| `internal/binmeta/` | Verified bin metadata (bins.v1.json) | exists | implemented |
| `internal/binresolve/` | Bin resolution and path lookup | exists | implemented |
| `internal/fsx/` | Extended filesystem helpers (locks, guards, atomic writes) | exists | implemented |
| `internal/contentid/` | Content identity (SRI parsing, key validation) | exists | implemented |
| `internal/jsonfile/` | JSON/JSONC file read/write helpers | exists | implemented |
| `internal/pack/` | Tarball pack with sandbox | exists | implemented |
| `internal/darkmode/` | OS dark-mode detection | exists | implemented |
| `internal/journal/` | Crash-recovery journals | reserved | planned |

## Runner and runtime

| Path | Purpose | Path state | Capability state |
|---|---|---|---|
| `internal/runner/` | Scripts, exec, dlx environment builder | exists | implemented |
| `internal/process/` | Signals, shells, child execution | exists | implemented |
| `internal/runtime/` | Node launch orchestration | exists | implemented |
| `internal/runtime/assets/` | Embedded loader/preload JS | exists | implemented |
| `internal/transform/` | Go transform service + IPC | exists | partial |
| `internal/node/` | Node discovery and provisioning | exists | implemented |
| `internal/pmmanager/` | External PM detect/pin/invoke | exists | implemented |
| `internal/shim/` | Cross-platform shims | reserved | planned |
| `runtime/` | Source for go:embed runtime assets | exists | partial |

## Compatibility, security, distribution

| Path | Purpose | Path state | Capability state |
|---|---|---|---|
| `internal/compat/` | Nub/npm/pnpm/Yarn/Bun adapters | exists | implemented |
| `internal/advisory/` | OSV advisory DB, audit scanning, fix suggestions | exists | implemented |
| `internal/sbom/` | CycloneDX/SPDX export | exists | implemented |
| `internal/provenance/` | Signature/provenance verify/emit | exists | implemented |
| `internal/capsule/` | Portable dependency capsules (descriptors) | exists | implemented |
| `internal/plugin/` | External m-\<verb\> discovery (no in-process load) | reserved | planned |
| `internal/analysis/phantom/` | Optional phantom-dependency analysis | reserved | planned |

SBOM evidence: [`docs/evidence/core/pass32-ci.md`](../evidence/core/pass32-ci.md).

## Support and fixtures

| Path | Purpose | Path state | Capability state |
|---|---|---|---|
| `internal/testkit/` | Fixtures, clean-home, local registry | exists | implemented |
| `internal/features/` | Feature inventory schema/runtime | exists | implemented |
| `internal/releasetrain/` | MVP dependency graph validation | exists | implemented |
| `fixtures/registry/` | Local packuments and tarballs | exists | implemented |
| `fixtures/projects/` | Project corpora | exists | implemented |
| `tests/` | Conformance, integration, soak, and benchmark suites | exists | implemented |
| `tests/conformance/` | Differential conformance suites | exists | implemented |
| `tests/integration/` | End-to-end integration suites | exists | implemented |
| `benchmarks/` | Perf baselines and waivers | exists | partial |

## Release and docs

| Path | Purpose | Path state | Capability state |
|---|---|---|---|
| `release/` | Release metadata and notes | reserved | planned |
| `install/` | install.sh / install.ps1 sources | reserved | planned |
| `.github/actions/` | GitHub Action sources | reserved | planned |
| `docker/` | Container images and Dockerfiles | reserved | planned |
| `docs/` | User and architecture docs | exists | implemented |
| `docs/adr/` | Architecture decision records | exists | implemented |
| `docs/architecture/` | This package map and boundary docs | exists | implemented |
| `plans/` | Implementation archive | exists | implemented |

## Deferred package decisions

- **No `internal/pm` umbrella.** Flat packages under `internal/` own package-manager
  domains. An umbrella package may be reconsidered only after two concrete callers
  need shared orchestration beyond `internal/app`.
- **`assets/runtime/`** in the 0003 plan listing is synonymous with top-level
  `runtime/` for embed sources; use `runtime/` and `internal/runtime/assets/`.
