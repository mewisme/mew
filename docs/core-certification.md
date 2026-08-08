# Package-manager core certification

MVP **0031**. Evidence index for the PM core shipped in MVPs **0010–0030**.
Runner work (**0040+**) may depend on the install interfaces and schemas
frozen here; see [`schema-freeze.md`](schema-freeze.md).

Related: [`security-pm-core.md`](security-pm-core.md),
[`../testdata/certification/sign-off-checklist.md`](../testdata/certification/sign-off-checklist.md),
[`pm-commands.md`](pm-commands.md).

## CI tiers

Two workflows, two jobs to do.

| Tier | Workflow | Trigger | Contains |
|---|---|---|---|
| Default gate | [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) | every pull request and push to `main` | gofmt, `go mod tidy` diff, `go vet`, lint, architecture, license and dependency allowlist, `go test ./... -short` on Linux, `m`/`mx` build, one limited Windows smoke job (config, identity, CLI, paths) |
| Full certification | [`.github/workflows/full.yml`](../.github/workflows/full.yml) | nightly schedule, `workflow_dispatch`, `v*` tags | full Linux/macOS/Windows matrix, race detector, benchmarks, all-target cross compilation, pnpm/npm/Yarn/Bun lock conformance, crash integration, soak, core certification, CLI UX and runner certification, `govulncheck` |

The default gate is the blocking check. It is deliberately narrow: it must catch
compilation failures, unit regressions, configuration, identity, and
install-preflight regressions, plus Windows path behavior — and nothing slower
than that. The heavy suites are not deleted, only moved off the blocking path;
every one remains runnable on demand and still uploads the same reports and
artifacts. `-short` is what separates the tiers: soak and wall-clock comparisons
skip in the default gate and execute in full.

## Certification entry points

Machine-readable step list: [`tools/certification/core-manifest.json`](../tools/certification/core-manifest.json).

| Command | Purpose |
|---|---|
| `make core-cert` | Full local gate (manifest `core-cert` target) |
| `make core-cert-fast` | Fixture verify + crash-shard verify + conformance list |
| `make core-cert-security` | Audit/SBOM/provenance focused tests |
| `make core-cert-crash` | Crash-shard assignment verify (full `-tags crash` suite is separate) |
| `make core-cert-performance` | Bench correctness + advisory regression |
| `python tools/certification/run_core_cert.py <name>` | Direct runner (same manifest as Make) |
| `go run ./cmd/m conformance run core [--json]` | Execute the core-matrix test suites |
| `go run ./cmd/m conformance list` | List suite ids from `tests/conformance/core-matrix/manifest.json` |
| `go run ./cmd/m doctor [--json] [--strict]` | Project and PM health checks |
| `go run ./cmd/m bench install [--warm\|--cold] [--json]` | Install benchmark (see [`performance.md`](performance.md)) |
| `python tools/soak/install_loop.py --count <n> --mode warm` | Repeated install soak (CI: `--count 10`; manual: `--count 100`) |
| `python tools/bench/check_correctness.py --mode warm` | Install bench correctness (≥7 samples, digest) |
| `python tools/bench/check_regression.py --mode warm` | Install performance regression (platform-matched; advisory on CI until Ubuntu baseline) |

`m benchmark` is the compatibility alias for `m bench` (0031 plan surface).

## Certification matrix

| Gate | Local reproduction | CI job |
|---|---|---|
| Unit + integration tests | `CGO_ENABLED=0 go test ./... -count=1` | `test` (ubuntu, macos, windows) |
| Race detector | `go test -race ./... -count=1` | `race`, `race-macos`, `race-windows` |
| No-CGO production build | `CGO_ENABLED=0 go build ./cmd/m ./cmd/mx` | `no-cgo-gate`, `cross` |
| Vet + lint + vuln + allowlist | `go vet ./...`; `golangci-lint run ./...`; `govulncheck ./...` | `lint`, `vuln`, `allowlist` |
| Platform lockfiles | — | `platform-lock` |
| Fixture provenance | `go run ./tools/conformance/verify-fixtures` | `fixture-verify` |
| Crash-shard assignment | `go run ./tools/ci/verify-crash-shards` | `crash-shard-verify` |
| Transaction / snapshot crash recovery | `go test -tags crash ./tests/integration/... -run Crash -timeout 30m` | `crash-integration` (+ Windows shards + `crash-integration-report`) |
| pnpm 9 lock bridge + mutation | `go test ./tests/conformance/... -run 'Pnpm9' -count=1` | `conformance-pnpm-9` |
| pnpm 10 lock bridge + mutation | `go test ./tests/conformance/... -run 'Pnpm10' -count=1` | `conformance-pnpm-10` |
| pnpm 11 lock bridge + mutation | `go test ./tests/conformance/... -run 'Pnpm11' -count=1` | `conformance-pnpm-11` |
| Unsupported pnpm rejection | `go test ./tests/conformance/... -run UnsupportedLegacy -count=1` | `conformance-pnpm-unsupported` |
| npm lock bridge (read-only) | `go test ./tests/conformance/... -run LockBridgeNpm -count=1` | `conformance-npm` |
| bun lock bridge | `go test ./tests/conformance/... -run LockBridgeBun -count=1` | `conformance-bun` |
| Yarn Classic + Berry | `go test ./tests/conformance/... -run LockBridgeYarn -count=1` | `conformance-yarn` |
| Nub derived fixtures | `go test ./tests/conformance/... -run LockBridgeNub -count=1` | `conformance-nub-fixtures` |
| Core conformance aggregate | `go run ./cmd/m conformance run core --json` | `core-stabilization` (0031; `MEW_CONFORMANCE_REQUIRE_TOOLS=1`) |
| Conformance negative probes | `go run ./cmd/m conformance run core --filter integration.cert-negative-*` (must fail) | `cert-negative-probes` |
| PM health | `go run ./cmd/m doctor --json` | `core-stabilization` |
| Soak (short) | `python tools/soak/install_loop.py --count 10 --mode warm` | `core-stabilization` |
| Install bench correctness | `python tools/bench/check_correctness.py --mode warm` | `bench-correctness` (blocking) |
| Install bench regression | `python tools/bench/check_regression.py --mode warm` | `bench-regression` (advisory; structured waiver) |
| License + dependency allowlist | `go run ./tools/check-license`; `go run ./tools/check-deps` | `gate-probe` |

Pinned pnpm producer versions: `tools/conformance/pnpm-versions.env` (9.15.9 /
10.34.5 / 11.17.0). Inventory: [`tests/conformance/inventory.json`](../tests/conformance/inventory.json).

## Core conformance report (schema v2)

`m conformance run core --json` emits `schemaVersion: 2` with:

- `commitSHA`, `goVersion`, `startedAt`, `finishedAt`, `passed`
- `tools[]` — resolved external binaries (`node`, `pnpm`, …) with versions
- Per-suite `testsMatched`, `passed`, `failed`, `skipped`, `exitCode`

**Fail-closed rules** (Pass 32): certification **passes only when every required
suite has matched tests, zero failures, and zero skips** when
`MEW_CONFORMANCE_REQUIRE_TOOLS=1` (CI `core-stabilization`). Suites with
`requireTools: true` treat missing Node/pnpm as fatal, not skip. Negative probe
suites (`integration.cert-negative-*`) must fail in `cert-negative-probes` CI.

## Lock-bridge certification scope

Certified (fixture parse, graph conversion, byte-preserving no-op or mutation
where applicable):

- **npm** — `package-lock.json` v2/v3 corpus under `fixtures/locks/npm/` (read-only; semantic mutation rejected with `ERR_M_UNSUPPORTED`)
- **pnpm 9 / 10 / 11** — generated fixtures under `fixtures/locks/generated/pnpm-{9,10,11}/` including mutation families: basic, transitive, optional, peer-context, alias-peer, workspace, alias, patch
- **Yarn Classic** — `fixtures/locks/yarn/classic/`
- **Yarn Berry (node_modules)** — `fixtures/locks/yarn/berry-nm/`
- **Yarn Berry (PnP read-only)** — parse + identity; install rejected with typed error
- **bun** — `fixtures/locks/bun/`
- **Nub** — derived-format fixtures under `fixtures/locks/generated/nub-*` (not live Nub binary runs)

## Crash and transaction evidence

Crash integration covers install interruption, update interruption, snapshot
restore, and workspace snapshot paths. Windows runs in dedicated shards
(snapshot, install/txn, update) per
[`.github/workflows/full.yml`](../.github/workflows/full.yml).

See [`transaction.md`](transaction.md) and
[`docs/evidence/core/pass20-security-controls.md`](evidence/core/pass20-security-controls.md)
for patch-sandbox, provenance, store-integrity, and capsule evidence.

## Pass 32 hardening evidence (core subset)

| Area | Shipped control | Evidence |
|---|---|---|
| Blob store integrity | `PutVerified` / `GetVerified` / `ExistsVerified`; corrupt quarantine | `internal/store/verified.go`, `verified_test.go` |
| Core certification | `go test -json` runner; skip/zero-match fail; `MEW_CONFORMANCE_REQUIRE_TOOLS` | `internal/conformance/testjson.go`, `cert-negative-probes` CI |
| npm lock bridge | Read-only incumbent; semantic mutation `ERR_M_UNSUPPORTED` | `tests/conformance/lock_bridge_npm_test.go` |
| `m pack` sandbox | Root containment; symlink/reparse rejection; size limits | `internal/pack/sandbox.go` |
| OSV advisory | Multi-interval range state machine; `m audit --fail-on` | `internal/advisory/range.go`, `audit_cmd.go` |
| Provenance | Explicit `TrustConfiguredKey` in production; exact package binding | `internal/provenance/trust.go`, `app/provenance.go` |
| `m publish --provenance` | Fail closed before upload without provider | `internal/app/publish.go` |
| Capsule | Atomic verified create; quarantined restore | `internal/capsule/archive.go` |
| SBOM | Graph `dependencies` / `bom-ref` / SPDX `DEPENDS_ON` | `internal/sbom/sbom.go`, golden fixtures |
| Bench regression | Multi-sample warmup + median/p95 metadata | `internal/app/bench.go`, `benchmarks/install-baseline.json` |

## Known limitations (pass 32 residual risks)

1. **Nub executable conformance** — derived-format fixture validation and
   Mew-native graph tests only; no frozen Nub binary differential matrix.
2. **Yarn Berry partial** — PnP install mode is read-only / rejected; full PnP
   linker parity is deferred post-0031.
3. **pnpm 11 patch config** — `patchedDependencies` may live in
   `pnpm-workspace.yaml`; fixtures retain dual `package.json#pnpm` fields for
   cross-major parity.
4. **Differential npm/pnpm CI** — `m conformance run runtime` is implemented (0057)
   with 12-cell Node×OS matrix in full CI; full 0080 conformance program scope
   remains deferred.
5. **Live Sigstore** — provenance verification uses fixture DSSE bundles in
   tests with `TrustFixtureKey`; production `m verify provenance` requires
   `TrustConfiguredKey` (configured public key). Live Fulcio/Sigstore roots
   (`TrustSigstoreRoots`) return unsupported.
6. **Advisory feed signing** — `m audit` uses cached OSV bytes with digest
   only; cryptographic feed signature verification deferred.
7. **npm incumbent mutation** — parse, validate, frozen install, and
   byte-preserving no-op only; dependency changes require npm or `m.lock`
   migration.

## Schema freeze (0031)

The following are frozen for runner MVPs (**0040+**). Breaking changes require
an ADR and explicit migration:

- `m.lock` document shape (`lockfileVersion: 3`)
- Shipped PM CLI grammar documented in [`pm-commands.md`](pm-commands.md)
- Machine-readable report schemas listed in [`schema-freeze.md`](schema-freeze.md)

Install orchestration interfaces in `internal/app` (`Install`, `InstallOptions`,
`InstallResult`, transaction journal) are stable contracts for 0040; new fields
may be added in a backward-compatible way only.

## Open decisions

| Decision | Status |
|---|---|
| Core v1 beta channel promotion date | TBD — track in release train |
| Yarn Berry features deferred post-0031 | PnP install/link; Plug'n'Play runtime |
| `m conformance run runtime` scope | Implemented (0057, `tests/conformance/runtime-matrix/`) |

## Sign-off

Human checklist (0087-aligned, PM-core subset):
[`testdata/certification/sign-off-checklist.md`](../testdata/certification/sign-off-checklist.md).

### Pass 32 CI evidence (2026-07-30)

| Field | Value |
|---|---|
| Final `origin/main` SHA | `f0ce96df82b262819334a121c584b93b1aeaa309` |
| Workflow run ID | [`30487309379`](https://github.com/mewisme/mew/actions/runs/30487309379) |
| `head_sha` | `f0ce96df82b262819334a121c584b93b1aeaa309` |
| Code SHA (Pass 32 fixes) | `f19f3f73bd7dc4169a8a95c598a645b2077b9539` (run [`30486713425`](https://github.com/mewisme/mew/actions/runs/30486713425)) |
| Matrix | 38 jobs success (ubuntu, macOS, Windows test/race/cross/platform-lock/crash/conformance/core-stabilization) |
| Core certification artifact | `core-certification-report` from `core-stabilization` job |
| Scorecard | [`docs/evidence/core/pass32-ci.md`](evidence/core/pass32-ci.md) |

Note: Windows `platform-lock` required one failed-job rerun on the same SHA (transient dual-winner flake). `bench-regression` and core-stabilization bench steps remain advisory (`continue-on-error: true`).
