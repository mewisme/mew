# Compatibility Axes

Mew evaluates compatibility on five **independent axes**. A feature may be at parity on one axis and deferred on another. States: **parity**, **intentional divergence**, **extension**, **deferred**.

## Axis summary

| Axis | Scope | Primary MVPs |
|---|---|---|
| **CLI** | Commands, flags, help, dispatch, exit codes | 0010, 0026, 0040–0046 |
| **Lockfile** | Read/write/validate/migrate per format | 0015, 0023–0025 |
| **Config** | Project/global config, identity detection, env | 0006, 0061 |
| **Runtime** | Node launch, TS/JSX, loaders, watch, debug | 0050–0057 |
| **Layout** | `node_modules` structure, store, linking strategy | 0018, 0019 |

## Cross-axis feature matrix (selected)

| Feature | CLI | Lockfile | Config | Runtime | Layout | State |
|---|---|---|---|---|---|---|
| `m install` / `m add` / `m remove` | deferred | deferred | deferred | n/a | deferred | deferred |
| `m.lock` native format | n/a | deferred | deferred | n/a | deferred | deferred |
| `nub.lock` preserve on install | n/a | **parity** | deferred | n/a | deferred | **parity** |
| `package-lock.json` preserve | n/a | deferred | deferred | n/a | deferred | deferred |
| `pnpm-lock.yaml` preserve | n/a | **parity** | deferred | n/a | deferred | **parity** (pass 15: pnpm 9/10/11 frozen CI) |
| Identity detection order | deferred | deferred | deferred | n/a | n/a | deferred |
| Transactional install + rollback | deferred | n/a | n/a | n/a | deferred | deferred |
| Isolated linker (pnpm/Nub style) | n/a | n/a | deferred | n/a | deferred | deferred |
| `m run` script runner | **certified** | n/a | n/a | deferred | n/a | **certified** (0046) |
| Workspace script orchestration (`-r run`, filters) | **parity** | n/a | n/a | deferred | n/a | **parity** |
| Direct `m <script>` shortcuts | extension | n/a | n/a | n/a | n/a | **extension** |
| `mx` local/remote exec | deferred | n/a | n/a | deferred | n/a | deferred |
| TypeScript execution | n/a | n/a | deferred | **certified** | n/a | **certified** (0057) |
| Node version manager | deferred | n/a | deferred | deferred | n/a | deferred |
| External PM meta-manager | deferred | n/a | deferred | n/a | n/a | deferred |
| Nub CLI grammar (core PM) | deferred | n/a | deferred | n/a | n/a | parity (intent) |
| Nub runtime augmentation | n/a | n/a | n/a | **certified** | n/a | **certified** (0057: 3 OS × 4 Node, 9 suites) |

## Intentional extensions (Mew-only)

| Extension | Description | Owning MVP |
|---|---|---|
| Direct script shortcuts | `m dev`, `m start`, etc. without `run` | 0042 |
| Explainable plans | Resolver/install decision traces, semantic diffs | 0028 |
| Dependency time travel | Historical snapshot restore | 0028 |
| Verified capsules | Portable offline dependency bundles | 0029 |
| Org policy file | `.github.com/mewisme/mew/policy.toml` enforcement | 0030 |

## Nub conformance targets

| Nub surface | Mew target | State |
|---|---|---|
| Product positioning (`nub` / `nubx`) | `m` / `mx` identity | parity (intent) |
| Stock Node augmentation | No libnode fork | parity (intent) |
| `nub.lock` round-trip | Preserve when Nub identity | **parity** (0023) |
| Direct `m <script>` | Not in Nub | **extension** |
| MIT / repo conventions | Process alignment | deferred |

## INDEX MVP → charter objective map

Every indexed MVP maps to at least one charter objective.

| MVP | Title | Charter objective(s) |
|---|---|---|
| 0001 | Program Charter | Product identity, compatibility policy, naming |
| 0002 | Feature Inventory | Traceability, parity matrix |
| 0003 | Target Architecture | Go control plane, module boundaries |
| 0004 | Repository Bootstrap | Engineering standards, reproducible builds |
| 0005 | Error Model | Stable diagnostics, `ERR_M_*` codes |
| 0006 | Configuration Identity | Identity detection, layered config |
| 0007 | Data Model | Canonical graph, shared interfaces |
| 0008 | Testing Strategy | Conformance, fixtures, hermetic CI |
| 0009 | Release Train | Delivery ordering, dependency graph |
| 0010 | CLI Foundation | `m` / `mx` command shell |
| 0011 | Manifest Discovery | `package.json`, workspace discovery |
| 0012 | Registry Cache | npm metadata, offline cache |
| 0013 | Basic Resolver | Semver, deterministic graph |
| 0014 | Fetch & Integrity | Tarball verify, safe extraction |
| 0015 | `m.lock` Format | Native lockfile |
| 0016 | Basic Installer | First `m install` / add / remove |
| 0017 | Transaction Rollback | Atomic mutation, rollback |
| 0018 | Global Store & Linker | Content store, smart linking |
| 0019 | Isolated Linker | Virtual store layout |
| 0020 | Advanced Resolver | Peers, overrides, workspaces |
| 0021 | Lifecycle Sandbox | Script trust, sandbox policy |
| 0022 | Workspaces | Monorepo, catalogs, filters |
| 0023 | Nub/pnpm Lock Bridge | `nub.lock`, pnpm preserve |
| 0024 | npm Locks | `package-lock.json` compatibility |
| 0025 | Bun/Yarn Locks | Bun and Yarn adapters |
| 0026 | PM Command Surface | Complete PM grammar |
| 0027 | Advanced Sources | Patches, pack, publish |
| 0028 | Explain & History | Plans, diffs, time travel |
| 0029 | Performance & Offline | Capsules, offline-first |
| 0030 | Security & SBOM | Audit, provenance, policy |
| 0031 | Core Stabilization | PM core certification |
| 0040 | Script Runner | `m run` |
| 0041 | Workspace Runner | Multi-package scripts |
| 0042 | Direct Script Shortcuts | **`m <script>` extension** |
| 0043 | Local Exec | `m exec`, `.bin` discovery |
| 0044 | `mx` DLX | Remote fetch and execute |
| 0045 | Unified Execution | Shared environment builder |
| 0046 | Runner Stabilization | Runner certification |
| 0050 | Node Launch | Stock Node, preload injection |
| 0051 | Transform Service | TypeScript via Go pipeline |
| 0052 | JSX & Decorators | Full transform parity |
| 0053 | Module Resolution | Loaders, path aliases |
| 0054 | Env & Modern APIs | `.env`, selected polyfills |
| 0055 | Watch Mode | Dependency-aware restart |
| 0056 | Debugging | Inspector, source maps |
| 0057 | Runtime Stabilization | Runtime certification |
| 0060 | Node Manager | Node install/select |
| 0061 | PM Meta-Manager | External PM provisioning |
| 0062 | Shims | Cross-platform version shims |
| 0070 | Project Init | TypeScript-first scaffold |
| 0071 | Plugins | `m-<verb>` convention |
| 0072 | Installers & Releases | Distribution channels |
| 0073 | GitHub Action | CI integration |
| 0074 | Docker Images | Container distribution |
| 0080 | Conformance Program | Continuous parity testing |
| 0081 | Performance Program | Benchmark governance |
| 0082 | Threat Model | Security review gates |
| 0083 | Rust→Go Migration Map | Reference component mapping |
| 0084 | Versioning Policy | Semver, format stability |
| 0085 | Dependency Roadmap | Go dependency selection |
| 0086 | AI Agent Protocol | Agent implementation workflow |
| 0087 | Definition of Done | Program completion standard |
| 0088 | Reference Index | Source bibliography |
| 0089 | Research Spikes | Pre-implementation decisions |
| 0090 | Future Backlog | Post-parity ideas |

## Review process

When an MVP changes public behavior:

1. Update the relevant row in this matrix.
2. Update the feature inventory (0002).
3. Add or reference conformance fixtures.
4. Record intentional divergence in an ADR when irreversible.
