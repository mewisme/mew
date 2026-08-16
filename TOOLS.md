# MewJS — Repository Tools Inventory

**Canonical tool inventory.** This file is the single source of truth for every repository-local developer tool. AI agents and contributors must consult it before creating or modifying tooling.

Complete inventory of every developer, build, test, CI, release, maintenance, generation, validation, migration, benchmark, and debugging tool present in the source tree.

## Quick command routing

Map common intents to canonical commands. Use these; do not compose ad hoc sequences when a documented command exists.

| Intent | Command | See |
|---|---|---|
| Build | `make build` | [build](#build-makefile-target) |
| Format | `make fmt` | Makefile |
| Format check | `make fmt-check` | Makefile |
| Unit tests | `make test-short` | [test](#test-makefile-target) |
| Integration tests | `make test-integration` | Makefile |
| Crash tests | `make test-crash` | [test-crash](#test-crash-makefile-target) |
| Runtime tests | `make test-runtime` | Makefile |
| Race tests | `make test-race` | [race](#race-makefile-target) |
| Adaptive parallel tests | `go run ./tools/testexec` | [testexec](#tools-testexec) |
| Lint | `make lint` | [lint](#lint-makefile-target) |
| Quality (all gates) | `make quality` | Makefile |
| Normal local CI | `make ci` | Makefile |
| Generated-file update | `make generate` | Makefile |
| Generated-file check | `make generate-check` | Makefile |
| Runtime asset manifest update | `make assets` | [update-runtime-assets.py](#tools-update-runtime-assets-py) |
| Runtime asset manifest check | `make assets-check` | [update-runtime-assets.py](#tools-update-runtime-assets-py) |
| Plan/checklist generation | `make plans` | [enrich_and_generate.py](#plans-scripts-enrich_and_generate-py) |
| Cleanup | `make clean` | Makefile |
| Release readiness | `make release-check` | Makefile |

## Agent synchronization rules

**Must read.** Before any of these actions, read this file in full:

- Creating a new script or tool
- Composing a multi-command maintenance workflow
- Changing build, test, generation, validation, certification, benchmark, install, uninstall, or release tooling
- Modifying the Makefile or CI command paths
- Claiming that no existing repository command supports a task

**Must update.** Any change that adds, removes, renames, merges, or changes the behavior of a repository tool must update this file in the same change. This applies to:

- Python, shell, PowerShell, batch, JavaScript, TypeScript, and Go tools
- Make targets
- `go:generate` commands
- Package scripts
- CI-only helper commands
- Report generators, validators, benchmark tools, install/uninstall scripts, release/certification tooling

Updates must cover changed paths, invocations, flags, inputs, outputs, dependencies, callers, platform support, status, and limitations. Removing a tool must remove or update its entry. Renaming or converting a tool must document the old-to-new relationship.

**Precedence.** This file documents commands and tooling behavior. It does not override repository architecture, security, contribution, or release policies. When this file and an instruction file disagree about a command, verify current source and update the stale documentation — do not silently pick one.

## Validation

Run `make docs-check` to verify that AI instruction files reference this file and that all quick-command targets exist. The check also runs as part of `make quality`.

## Scope

All executable or tool-like sources in this repository, including: Go `package main` programs; shell, Python, and PowerShell scripts; Makefile targets; CI workflow-embedded tooling; runtime assets; configuration and data files that drive tool behavior; wrapper scripts. Excludes: ordinary production packages, test suites (unless the test binary doubles as a gate tool), fixture data, vendored/third-party code, build artifacts, and generated output.

## Inventory methodology

1. Recursive `find` by extension (`.sh`, `.ps1`, `.psm1`, `.py`, `.js`, `.mjs`, `.cjs`), executable bit, and known shebangs, excluding `node_modules`, `vendor`, `.git`, `fixtures`, and `testdata`.
2. `grep "^package main"` across all Go files.
3. `grep "go:generate"` across all Go sources (zero results).
4. Parse Makefile for `.PHONY` targets and their recipes.
5. Parse `.github/workflows/*.yml` for embedded scripts and tool invocations.
6. Read each discovered tool's source in full.
7. Follow cross-references between scripts, Makefile, CI workflows, and documentation.

## Quick index

### Build tools
- [`build` (Makefile target)](#build-makefile-target)
- [Cross-compile loop (CI-embedded)](#cross-compile-loop-ci-embedded)
- [`m` binary](#m-binary)
- [`mx` binary](#mx-binary)

### Test tools
- [`test` (Makefile target)](#test-makefile-target)
- [`tools/testexec`](#tools-testexec) — canonical adaptive Go test orchestrator
- [Go benchmark driver (CI-embedded)](#go-benchmark-driver-ci-embedded)
- [`race` (Makefile target)](#race-makefile-target)
- [`test-crash` (Makefile target)](#test-crash-makefile-target)
- [`fuzz-smoke` (Makefile target / `fuzz_smoke.py`)](#fuzz_smoke-py)
- [`vuln` / govulncheck (Makefile target)](#vuln-govulncheck-makefile-target)

### Lint and quality tools
- [`vet` (Makefile target)](#vet-makefile-target)
- [`lint` (Makefile target)](#lint-makefile-target)
- [`gofmt check` (CI-embedded)](#gofmt-check-ci-embedded)
- [`go mod tidy` check (CI-embedded)](#go-mod-tidy-check-ci-embedded)
- [`allowlist` / `tools/check-deps`](#tools-check-deps)
- [`allowlist` / `tools/check-license`](#tools-check-license)
- [Production architecture check (CI-embedded)](#production-architecture-check-ci-embedded)

### CI gate tools
- [`tools/ci/verify-crash-shards`](#tools-ci-verify-crash-shards)
- [`tools/conformance/verify-fixtures`](#tools-conformance-verify-fixtures)
- [CI gate aggregator (CI-embedded)](#ci-gate-aggregator-ci-embedded)

### Certification tools
- [`tools/certification/run_core_cert.py`](#tools-certification-run_core_cert-py)
- [`tools/bench/check_correctness.py`](#tools-bench-check_correctness-py)
- [`tools/bench/check_regression.py`](#tools-bench-check_regression-py)
- [`tools/soak/install_loop.py`](#tools-soak-install_loop-py)
- [`tools/ci/verify_plan_generation.py`](#tools-ci-verify_plan_generation-py)

### Conformance tools
- [`tools/conformance/generate_lock_fixtures.py`](#tools-conformance-generate_lock_fixtures-py)
- [`tools/conformance/fixturemeta/cmd`](#tools-conformance-fixturemeta-cmd)

### Dev installation tools
- [`scripts/install-dev.sh`](#scripts-install-dev-sh)
- [`scripts/install-dev.ps1`](#scripts-install-dev-ps1)
- [`scripts/uninstall-dev.sh`](#scripts-uninstall-dev-sh)
- [`scripts/uninstall-dev.ps1`](#scripts-uninstall-dev-ps1)
- [`scripts/lib/devinstall.sh`](#scripts-lib-devinstall-sh)
- [`scripts/lib/DevInstall.psm1`](#scripts-lib-devinstall-psm1)
- [`scripts/lib/devinstall_test.sh`](#scripts-lib-devinstall_test-sh)
- [`scripts/lib/devinstall_test.ps1`](#scripts-lib-devinstall_test-ps1)
- [`tools/devinstall/smoke.py`](#tools-devinstall-smoke-py)

### Code generation and fixture tools
- [`tools/gen_0008_fixtures.go`](#tools-gen_0008_fixtures-go)
- [`tools/gen_archives.go`](#tools-gen_archives-go)
- [`tools/gen_fixture_tarballs.go`](#tools-gen_fixture_tarballs-go)
- [`tools/gen_help_golden.go`](#tools-gen_help_golden-go)
- [`tools/update-runtime-assets.py`](#tools-update-runtime-assets-py)

### Plans and documentation tools
- [`plans/scripts/enrich_and_generate.py`](#plans-scripts-enrich_and_generate-py)

### Validation tools
- [`tools/sbom/validate.py`](#tools-sbom-validate-py)

### Runtime assets
- [`credential-grabber.cjs`](#credential-grabber-cjs)
- [`loader-register.mjs`](#loader-register-mjs)
- [`preload.cjs`](#preload-cjs)
- [`preload.mjs`](#preload-mjs)
- [`ts-loader.mjs`](#ts-loader-mjs)

### Wrapper scripts
- Wrappers are documented alongside their canonical entrypoints; see [Tool dependency and wrapper relationships](#tool-dependency-and-wrapper-relationships) for a complete map.

---

## Detailed tool entries

### Build tools

#### `build` (Makefile target)
- **Path**: `Makefile` (lines 49-52)
- **Type**: Makefile target
- **Category**: build
- **Platforms**: Unix (Linux, macOS), Windows (with `EXE=.exe`)
- **Purpose**: Production build of `m` and `mx` binaries with link-time version stamping.
- **Invocation**: `make build [VERSION=dev] [COMMIT=...] [BUILD_DATE=...]`
- **Inputs**: `VERSION` (default `dev`), `COMMIT`, `BUILD_DATE`; always `CGO_ENABLED=0`. LDFLAGS set from these variables.
- **Outputs**: `bin/m` and `bin/mx` (plus `.exe` suffix on Windows).
- **Dependencies**: Go 1.26.5+.
- **Used by**: developer manual build; not used in CI (CI uses inline `go build -trimpath`).
- **Status**: active.

#### Cross-compile loop (CI-embedded)
- **Path**: `.github/workflows/full.yml` (lines 152-168, job `cross-compile`)
- **Type**: CI-embedded shell script
- **Category**: build
- **Platforms**: ubuntu-latest (cross-compiling for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64)
- **Purpose**: Verify all supported GOOS/GOARCH targets compile without error.
- **Invocation**: CI only; inline `for` loop over six targets, sets `GOOS`/`GOARCH`, appends `.exe` for Windows, builds both binaries to `$RUNNER_TEMP`.
- **Dependencies**: Go 1.26.5+.
- **Used by**: full.yml nightly and on `v*` tags.
- **Status**: active.

#### `m` binary
- **Path**: `cmd/m/main.go`
- **Type**: Go (`package main`)
- **Category**: build (primary product)
- **Platforms**: Linux, macOS, Windows
- **Purpose**: Primary CLI binary (`m`). Entrypoint delegates to `internal/cli.ExecuteM`. Subcommands include `install`, `update`, `remove`, `doctor`, `conformance run/verify`, `bench install`, plus completion, version, and help.
- **Invocation**: `go build -o bin/m ./cmd/m`; built by Makefile `build` and CI.
- **Inputs**: ldflags `-X main.version/commit/buildDate/targetOS/targetArch`; env `CGO_ENABLED=0`.
- **Outputs**: Statically linked ELF/Mach-O/PE binary.
- **Dependencies**: `internal/cli` (cobra, app, apperr, advisory).
- **Used by**: end users, Makefile, CI (doctor, conformance, bench).
- **Status**: active.

#### `mx` binary
- **Path**: `cmd/mx/main.go`
- **Type**: Go (`package main`)
- **Category**: build (primary product)
- **Platforms**: Linux, macOS, Windows
- **Purpose**: Package-runner CLI binary (`mx`). Entrypoint delegates to `internal/cli.ExecuteMX`.
- **Invocation**: `go build -o bin/mx ./cmd/mx`.
- **Outputs**: Statically linked binary.
- **Dependencies**: `internal/cli`.
- **Used by**: end users; built by Makefile and CI alongside `m`.
- **Status**: active.

### Test tools

#### `tools/testexec`
- **Path**: `tools/testexec/main.go` (+ `config.go`, `discover.go`, `schedule.go`, `run.go`)
- **Type**: Go (`package main`)
- **Category**: test / orchestration
- **Platforms**: any with Go
- **Purpose**: Canonical adaptive Go test orchestrator. Discovers tests, assigns them to workers (round-robin or LPT with optional timing data), and runs workers as parallel processes. Heavy packages use `go test -c` + `-test.run` for process-level sharding with isolated environments. Light packages use standard `go test -p N` for package-level parallelism.
- **Invocation**: `go run ./tools/testexec [flags] [packages...]`
- **Flags**: `-workers` (auto|1|N), `-short`, `-race`, `-tags`, `-run`, `-timeout`, `-v`, `-json`, `-cpu`
- **Dependencies**: stdlib only; shells out to `go test`, `go list`, `go test -c`.
- **Used by**: Makefile `test`, `test-short`, `test-unit`, `test-integration`, `test-e2e`, `test-crash`, `test-race`, `test-all` targets.
- **Status**: active (canonical test orchestrator).

#### `test` (Makefile target)
- **Path**: `Makefile`
- **Type**: Makefile target
- **Category**: test
- **Platforms**: Unix (Linux, macOS), Windows
- **Purpose**: Full hermetic test suite with adaptive parallel execution.
- **Invocation**: `make test [TESTEXEC_WORKERS=auto|1|N]`
- **Command**: `go run ./tools/testexec -workers $(TESTEXEC_WORKERS) -timeout 25m`
- **Dependencies**: Go 1.26.5+.
- **Used by**: developer manual; CI `test` job uses direct `go test ./... -short` for PR gate.
- **Status**: active.

#### `test-crash` (Makefile target)
- **Path**: `Makefile`
- **Type**: Makefile target
- **Category**: test
- **Platforms**: Unix (Linux, macOS), Windows
- **Purpose**: Run crash recovery suite (`crash` build tag).
- **Invocation**: `make test-crash`
- **Command**: `go run ./tools/testexec -workers auto -tags crash -timeout 30m ./tests/integration/...`
- **Used by**: developer manual; CI `crash-integration` job in full.yml.
- **Status**: active.

#### Go benchmark driver (CI-embedded)
- **Path**: `.github/workflows/full.yml` (line 134, job `benchmarks`)
- **Type**: CI-embedded command
- **Category**: test / benchmark
- **Platforms**: ubuntu-latest
- **Purpose**: Run all Go benchmarks at reduced iterations for CI regression detection.
- **Command**: `go test ./... -run '^$' -bench . -benchtime 10x -count=1 -timeout 35m`
- **Used by**: full.yml nightly and on `v*` tags.
- **Status**: active.

#### `race` (Makefile target)
- **Path**: `Makefile` (line 18)
- **Type**: Makefile target
- **Category**: test
- **Platforms**: Unix (requires CGO)
- **Purpose**: Go race detector across all packages.
- **Invocation**: `make race`
- **Command**: `go test -race ./... -count=1`
- **Dependencies**: Go with CGO enabled (`CGO_ENABLED=1`).
- **Used by**: developer manual; CI `race` job in full.yml (`go test -race ./... -timeout 40m`), plus per-OS race jobs.
- **Status**: active.

#### `fuzz_smoke.py`
- **Path**: `tools/fuzz_smoke.py`
- **Type**: Python
- **Category**: test
- **Platforms**: any with Python 3 and Go
- **Purpose**: Discover and smoke all `func Fuzz*` targets. Runs `go list ./...`, scans each package's `*_test.go`, and executes `go test <pkg> -fuzz=. -fuzztime=1s` for each fuzz target.
- **Outputs**: `fuzz-smoke: <pkg>` per target; `no Fuzz* targets; ok` if none found. Nonzero exit on fuzz failure.
- **Dependencies**: Python 3 stdlib; Go toolchain.
- **Used by**: Makefile `fuzz-smoke`; developer manual; documented in `CONTRIBUTING.md`.
- **Status**: active.

#### `vuln` / govulncheck (Makefile target)
- **Path**: `Makefile` (line 42)
- **Type**: Makefile target
- **Category**: security
- **Platforms**: any with `govulncheck` installed
- **Purpose**: Vulnerability scan of all dependencies.
- **Invocation**: `make vuln`
- **Command**: `govulncheck ./...`
- **Dependencies**: `govulncheck` binary (pinned `v1.1.4` in `tools/versions.env`; installed via `make tools`).
- **Used by**: developer manual; CI `security` job in full.yml uses `go run "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" ./...`.
- **Status**: active.

### Lint and quality tools

#### `vet` (Makefile target)
- **Path**: `Makefile` (line 12)
- **Type**: Makefile target
- **Category**: lint
- **Platforms**: any with Go
- **Purpose**: Go static analysis.
- **Invocation**: `make vet`
- **Command**: `go vet ./...`
- **Used by**: developer manual; CI `quality` job.
- **Status**: active.

#### `lint` (Makefile target)
- **Path**: `Makefile` (line 15)
- **Type**: Makefile target
- **Category**: lint
- **Platforms**: any with `golangci-lint` installed
- **Purpose**: Full linter suite via golangci-lint (config: `.golangci.yml`).
- **Invocation**: `make lint`
- **Command**: `golangci-lint run ./...`
- **Dependencies**: `golangci-lint` binary (pinned `v2.12.2` in `tools/versions.env`; installed via `make tools`).
- **Used by**: developer manual; CI `quality` job uses `golangci/golangci-lint-action@v9` with the pinned version.
- **Status**: active.

#### `gofmt check` (CI-embedded)
- **Path**: `.github/workflows/ci.yml` (lines 49-55, step "Check formatting")
- **Type**: CI-embedded shell script
- **Category**: lint
- **Platforms**: ubuntu-latest
- **Purpose**: Verify all `.go` files are `gofmt`-compliant. Exits 1 with file list if any differ.
- **Command**:
  ```bash
  set -euo pipefail
  files="$(git ls-files -z '*.go' | xargs -0 gofmt -l)"
  if [[ -n "$files" ]]; then
    echo "The following files need gofmt:" >&2
    echo "$files" >&2
    exit 1
  fi
  ```
- **Used by**: CI `quality` job only.
- **Status**: active.

#### `go mod tidy` check (CI-embedded)
- **Path**: `.github/workflows/ci.yml` (lines 59-61, step "Verify module files")
- **Type**: CI-embedded command
- **Category**: lint
- **Platforms**: ubuntu-latest
- **Purpose**: Verify `go.mod`/`go.sum` are tidy and committed.
- **Command**: `go mod tidy && git diff --exit-code -- go.mod go.sum`
- **Used by**: CI `quality` job only.
- **Status**: active.

#### `tools/check-deps`
- **Path**: `tools/check-deps/main.go`
- **Type**: Go (`package main`)
- **Category**: lint / allowlist
- **Platforms**: any with Go
- **Purpose**: Fails if any module from `go list -m all` is absent from `tools/allowlist/modules.txt` (skips the repo's own module). Prints `ok: N modules allowlisted`.
- **Invocation**: `go run ./tools/check-deps` (canonical); `make allowlist`.
- **Inputs**: Reads `tools/allowlist/modules.txt` (one module path per line, `#` comments).
- **Outputs**: stdout status line; exit 1 on unknown dependency.
- **Dependencies**: stdlib only; shells out to `go list -m all`.
- **Used by**: CI `quality` job (also runs `go test ./tools/check-deps` for its unit test); Makefile `allowlist`; `core-manifest.json` step `allowlist`.
- **Status**: active.

#### `tools/check-license`
- **Path**: `tools/check-license/main.go`
- **Type**: Go (`package main`)
- **Category**: lint / compliance
- **Platforms**: any with Go
- **Purpose**: Asserts root `LICENSE` contains "Apache License / Version 2.0" and `NOTICE` contains the correct copyright line and Apache-2.0 declaration.
- **Invocation**: `go run ./tools/check-license` (canonical); `make allowlist`.
- **Outputs**: stdout on success; exit 1 on mismatch.
- **Dependencies**: stdlib only.
- **Used by**: CI `quality` job; Makefile `allowlist`; `core-manifest.json` step `allowlist`.
- **Status**: active.

#### Production architecture check (CI-embedded)
- **Path**: `.github/workflows/ci.yml` (line 81, step "Verify production architecture")
- **Type**: CI-embedded command
- **Category**: lint / architecture
- **Platforms**: ubuntu-latest
- **Purpose**: Verify import-layer rules via `internal/archcheck`.
- **Command**: `go test ./internal/archcheck/... -count=1 -run TestProduction`
- **Used by**: CI `quality` job only.
- **Status**: active.

### CI gate tools

#### `tools/ci/verify-crash-shards`
- **Path**: `tools/ci/verify-crash-shards/main.go`
- **Type**: Go (`package main`)
- **Category**: CI gate
- **Platforms**: any with Go
- **Purpose**: Lists crash tests via `go test -tags crash ./tests/integration/... -list Crash`, asserts each matches exactly one Windows shard regex, mirroring the full.yml crash-integration matrix.
- **Invocation**: `go run ./tools/ci/verify-crash-shards`.
- **Dependencies**: stdlib only; shells out to `go test -list`.
- **Used by**: CI `quality` job; `core-manifest.json` step `crash-shard-verify`.
- **Status**: active.

#### `tools/conformance/verify-fixtures`
- **Path**: `tools/conformance/verify-fixtures/main.go` (+ `main_test.go`)
- **Type**: Go (`package main`)
- **Category**: CI gate
- **Platforms**: any with Go
- **Purpose**: Validates committed lock-bridge fixtures under `fixtures/locks/generated/`: pnpm 9/10/11 families against pins in `tools/conformance/pnpm-versions.env` (metadata schema, lockfileVersion 9.0, SHA-256 digests, sourceTreeDigest) plus `nub-*` derived families.
- **Invocation**: `go run ./tools/conformance/verify-fixtures`.
- **Dependencies**: `tools/conformance/fixturemeta` (internal package); stdlib.
- **Used by**: CI `quality` job; `core-manifest.json` step `fixture-verify`.
- **Status**: active.

#### CI gate aggregator (CI-embedded)
- **Path**: `.github/workflows/ci.yml` (lines 160-179, job `gate`); `.github/workflows/full.yml` (lines 526-561, job `gate`)
- **Type**: CI-embedded shell script
- **Category**: CI orchestration
- **Platforms**: ubuntu-latest
- **Purpose**: Aggregation gate that asserts all required upstream jobs passed. Runs with `if: always()`, loops over `needs.*.result`, exits 1 if any job is not `success`.
- **Used by**: CI only.
- **Status**: active.

### Certification tools

#### `tools/certification/run_core_cert.py`
- **Path**: `tools/certification/run_core_cert.py`
- **Type**: Python
- **Category**: certification
- **Platforms**: Linux, macOS, Windows (with appropriate tools installed)
- **Purpose**: Certification runner driven by `tools/certification/core-manifest.json`. Reads `targets` (core-cert-fast, core-cert, core-cert-security, core-cert-crash, core-cert-performance) and their `steps`; runs each step via subprocess; skips steps when required tools (`govulncheck`, `node`, `pnpm`) are missing (non-blocking) or errors (blocking exits nonzero).
- **Inputs**: `core-manifest.json` (defines targets and steps).
- **Dependencies**: Python 3; pwsh (for pwsh steps); per-step tools (Go, node, pnpm, govulncheck).
- **Used by**: Makefile; developer manual; documented in `docs/core-certification.md`.
- **Status**: active.

#### `tools/bench/check_correctness.py`
- **Path**: `tools/bench/check_correctness.py`
- **Type**: Python
- **Category**: certification / benchmark
- **Platforms**: any with Python 3 and Go
- **Purpose**: Runs a single install bench (`go run ./cmd/m bench install --{mode} --json`), validates required fields (`case`, `mode`, `fixtureDigest`, `medianMs`) and minimum 7 samples, writes `bench-result.json`.
- **Outputs**: `bench-result.json` artifact; stdout status line.
- **Dependencies**: Python 3 stdlib; Go toolchain.
- **Used by**: `core-manifest.json` step `bench-correctness` (blocking); documented in `docs/performance.md`.
- **Status**: active.

#### `tools/bench/check_regression.py`
- **Path**: `tools/bench/check_regression.py`
- **Type**: Python
- **Category**: certification / benchmark
- **Platforms**: any with Python 3 and Go
- **Purpose**: Runs install bench; compares median and p95 against `benchmarks/install-baseline.json` for matching case `medium-graph-{mode}` + os/arch/runnerClass. 10% noise budget. Supports structured waivers from `benchmarks/waivers.json` (expiry-aware) and legacy env `BENCH_WAIVER=1` (deprecated). Writes `bench-result.json`.
- **Inputs**: `benchmarks/install-baseline.json`, `benchmarks/waivers.json` (optional); env `MEW_BENCH_RUNNER_CLASS` or `GITHUB_ACTIONS`/`RUNNER_OS` for CI runner inference.
- **Outputs**: Pass: `ok: within baseline median and p95`; fail: exit error with regression details (or `WARN:` if waived). Artifact at `bench-result.json`.
- **Dependencies**: Python 3 stdlib; Go toolchain; baseline/waiver data files.
- **Used by**: `core-manifest.json` step `bench-regression` (advisory, non-blocking); `docs/performance.md`.
- **Status**: active.

#### `tools/soak/install_loop.py`
- **Path**: `tools/soak/install_loop.py`
- **Type**: Python
- **Category**: certification / soak
- **Platforms**: any with Python 3 and Go
- **Purpose**: Repeated install-bench loop. Runs `go run ./cmd/m bench install --{mode} --json [--fixture project]` N times. Sets `MEW_EXPERIMENTAL_WORKSPACES=1` + `MEW_EXPERIMENTAL_ISOLATED_LINKER=1` when project name contains `workspace`.
- **Dependencies**: Python 3 stdlib; Go toolchain.
- **Used by**: `core-manifest.json` step `soak-short` (count=10, mode=warm); `docs/core-certification.md`, `docs/performance.md`.
- **Status**: active.

#### `tools/ci/verify_plan_generation.py`
- **Path**: `tools/ci/verify_plan_generation.py`
- **Type**: Python
- **Category**: certification / CI
- **Platforms**: any with Python 3, git
- **Purpose**: Idempotency check for the plans pipeline. Runs `python3 plans/scripts/enrich_and_generate.py` twice and asserts `git diff --quiet plans/` before and after both runs. `--self-check` verifies git, python3, and the generator script exist.
- **Dependencies**: Python 3 stdlib; git.
- **Used by**: manual / evidence docs.
- **Status**: active.

### Conformance tools

#### `tools/conformance/generate_lock_fixtures.py`
- **Path**: `tools/conformance/generate_lock_fixtures.py`
- **Type**: Python
- **Category**: conformance / fixture generation
- **Platforms**: any with Python 3, Node, pnpm (via corepack)
- **Purpose**: Regenerates committed lock bridge conformance fixtures from pinned pnpm binaries. Reads family sources from `fixtures/locks/sources/pnpm/<family>/` and writes `fixtures/locks/generated/pnpm-{9,10,11}/<family>/` with honest `metadata.json`. When `--generate` is passed, runs pnpm `install --lockfile-only` in isolated temp homes; otherwise verify-only. Also derives Nub `.lock` fixtures from pnpm-9 generated locks.
- **Invocation**: `python3 tools/conformance/generate_lock_fixtures.py [--generate] [--families ...] [--majors ...]`
- **Outputs**: writes lockfiles and `metadata.json` under `fixtures/locks/generated/`.
- **Dependencies**: Python 3 stdlib; Go (`fixturemeta/cmd` digest tool); pnpm, corepack, Node (for `--generate`).
- **Used by**: `fixturemeta/cmd` (digest helper); manual fixture regeneration.
- **Status**: active (manual fixture regeneration).

#### `tools/conformance/fixturemeta
#### `tools/conformance/fixturemeta/cmd`
- **Path**: `tools/conformance/fixturemeta/cmd/main.go`
- **Type**: Go (`package main`; binary name "fixturemeta-digest")
- **Category**: conformance / utility
- **Platforms**: any with Go
- **Purpose**: Tiny CLI for computing fixture digests. Subcommands: `fixturemeta-digest source-tree <path>` (source-tree digest), `fixturemeta-digest file <path>` (SHA-256). Exit 2 on usage error.
- **Invocation**: `go run <tools/conformance/fixturemeta/cmd> <mode> <path>`
- **Dependencies**: `tools/conformance/fixturemeta` (internal package).
- **Used by**: `tools/conformance/generate-lock-fixtures.ps1` (`Invoke-FixtureDigest` helper).
- **Status**: active; internal support tool.

### Dev installation tools

#### `scripts/install-dev.sh`
- **Path**: `scripts/install-dev.sh`
- **Type**: Shell (bash)
- **Category**: dev installation
- **Platforms**: Unix (Linux, macOS); Git-bash on Windows (treated as `windows` target; cross-install rejected without `--build-only`)
- **Purpose**: Dev installer. Sources `scripts/lib/devinstall.sh`. Detects host/target, validates prerequisites, builds `m`/`mx` (+ copies `mew`/`mewx`) with `CGO_ENABLED=0`, installs to `$XDG_DATA_HOME/mewjs/bin` (default `~/.local/share/mewjs/bin`), upserts a managed PATH block into the shell profile (bash/zsh/fish), generates completions, verifies via `version --json`.
- **Invocation**: `./scripts/install-dev.sh [flags]`. Flags: `--build-only`, `--skip-path`, `--skip-completion`, `--skip-verify`, `--install-dir <path>`, `--goos`, `--goarch`, `--version`, `--force`, `-h/--help`. Env: `GOOS`, `GOARCH`, `MEW_VERSION`, `XDG_DATA_HOME`, `HOME`, `SHELL`.
- **Outputs**: `bin/m`, `bin/mx`, `bin/mew`, `bin/mewx`; install-dir copies; profile edits; completion files; summary block.
- **Dependencies**: bash, `go` >= 1.26.5, `git`, `awk`, `grep`, `uname`, `python3` (verify path); built binaries' own completion output.
- **Used by**: Makefile `install-dev` (non-MINGW/MSYS branch); `tools/devinstall/smoke.py` (`--build-only`); manual.
- **Status**: active.

#### `scripts/install-dev.ps1`
- **Path**: `scripts/install-dev.ps1`
- **Type**: PowerShell (requires 5.1+)
- **Category**: dev installation
- **Platforms**: Windows
- **Purpose**: Windows dev installer, mirroring `install-dev.sh`. Imports `lib/DevInstall.psm1`; installs to `%LOCALAPPDATA%\MewJS\bin`; writes `.cmd` shims + PowerShell completions; updates User `Path` env var and PS `$PROFILE` managed block; verifies `.exe` and `.cmd` via `version --json`.
- **Invocation**: `pwsh -NoProfile -File scripts/install-dev.ps1 [-BuildOnly] [-SkipPath] [-SkipCompletion] [-SkipVerify] [-InstallDir <path>] [-GoOS] [-GoArch] [-Version] [-Force]`
- **Outputs**: staged progress lines; `bin/m.exe`, `bin/mx.exe`, `bin/mew.exe`, `bin/mewx.exe`; install-dir copies; `.cmd` shims; PowerShell completion files; summary block.
- **Dependencies**: PowerShell 5.1+, go, git, `LOCALAPPDATA`.
- **Used by**: Makefile `install-dev` (MINGW/MSYS branch); `tools/devinstall/smoke.py` (`-BuildOnly`); manual.
- **Status**: active.

#### `scripts/uninstall-dev.sh`
- **Path**: `scripts/uninstall-dev.sh`
- **Type**: Shell (bash)
- **Category**: dev installation
- **Platforms**: Unix (Linux, macOS)
- **Purpose**: Unix dev uninstaller. Sources `scripts/lib/devinstall.sh`; removes owned files `m|mx|mew|mewx`, completion tree, and managed PATH/completion blocks from the shell profile.
- **Invocation**: `./scripts/uninstall-dev.sh [--install-dir <path>] [--keep-path] [--keep-completion] [-h/--help]`
- **Dependencies**: bash, awk (via lib), git.
- **Used by**: Makefile `uninstall-dev` (non-MINGW branch); manual.
- **Status**: active.

#### `scripts/uninstall-dev.ps1`
- **Path**: `scripts/uninstall-dev.ps1`
- **Type**: PowerShell (requires 5.1+)
- **Category**: dev installation
- **Platforms**: Windows
- **Purpose**: Windows dev uninstaller. Removes owned files + `.cmd` shims, PowerShell completions, profile block, User PATH entry.
- **Invocation**: `pwsh -NoProfile -File scripts/uninstall-dev.ps1 [-InstallDir <path>] [-KeepPath] [-KeepCompletion]`
- **Used by**: Makefile `uninstall-dev` (MINGW/MSYS branch); manual.
- **Status**: active.

#### `scripts/lib/devinstall.sh`
- **Path**: `scripts/lib/devinstall.sh`
- **Type**: Shell library (bash; sourced, not executed directly)
- **Category**: dev installation
- **Platforms**: Unix (Linux, macOS)
- **Purpose**: Shared POSIX library. ~40 `devinstall_*` functions: host/target detection, GOOS/GOARCH normalization, version compare, build with ldflags, atomic copies, managed-block insert/replace/remove in profiles, completion generation, verify, uninstall, summary.
- **Invocation**: `source scripts/lib/devinstall.sh` (sourced by installer/uninstaller/test scripts).
- **Dependencies**: bash, awk, grep, git, uname, date, go, python3 (verify path).
- **Used by**: `install-dev.sh`, `uninstall-dev.sh`, `devinstall_test.sh`.
- **Status**: active.

#### `scripts/lib/DevInstall.psm1`
- **Path**: `scripts/lib/DevInstall.psm1`
- **Type**: PowerShell module
- **Category**: dev installation
- **Platforms**: Windows
- **Purpose**: Shared PowerShell module with all `DevInstall*` / `*-DevInstall*` helpers (detect, build, atomic copy, managed blocks, PATH, completions, verify, uninstall, summary). Exports ~43 functions + 18 variables.
- **Invocation**: `Import-Module <path>/DevInstall.psm1 -Force` (imported by installer/uninstaller/test scripts).
- **Dependencies**: PowerShell 5.1+, go, git.
- **Used by**: `install-dev.ps1`, `uninstall-dev.ps1`, `devinstall_test.ps1`.
- **Status**: active.

#### `scripts/lib/devinstall_test.sh`
- **Path**: `scripts/lib/devinstall_test.sh`
- **Type**: Shell (bash)
- **Category**: dev installation / test
- **Platforms**: Unix
- **Purpose**: Self-contained logic tests for `devinstall.sh` (no test framework): OS/arch normalization, cross-detect, managed-block insert/replace/remove, default paths, metadata/commit resolution, idempotency, atomic install/uninstall.
- **Invocation**: `bash scripts/lib/devinstall_test.sh`
- **Outputs**: `ok:`/`FAIL:` lines; final `results: N passed, N failed`; exit 1 on any failure.
- **Dependencies**: bash, git, mktemp, grep.
- **Used by**: developer manual (documented in `CONTRIBUTING.md`).
- **Status**: active.

#### `scripts/lib/devinstall_test.ps1`
- **Path**: `scripts/lib/devinstall_test.ps1`
- **Type**: PowerShell (requires 5.1+)
- **Category**: dev installation / test
- **Platforms**: Windows
- **Purpose**: Logic tests for `DevInstall.psm1` (no Pester): OS/arch normalize, PATH dedupe, managed blocks, shim content, cross-compile canInstall, metadata resolution, UTF-8 no-BOM writes.
- **Invocation**: `pwsh -NoProfile -File scripts/lib/devinstall_test.ps1`
- **Outputs**: `ok:`/`FAIL:` lines; `results: N passed, N failed`; exit 1 on failure.
- **Dependencies**: PowerShell 5.1+.
- **Used by**: developer manual (documented in `CONTRIBUTING.md`).
- **Status**: active.

#### `tools/devinstall/smoke.py`
- **Path**: `tools/devinstall/smoke.py`
- **Type**: Python
- **Category**: dev installation / test
- **Platforms**: any with Python 3, bash (or Git-bash on Windows), pwsh, go
- **Purpose**: Smoke test for the dev-install scripts. `--self-check` verifies install scripts, bash, and pwsh exist. `--build-only` runs `install-dev.sh --build-only` (Unix) or `install-dev.ps1 -BuildOnly` (Windows) under an isolated `HOME`/`XDG_DATA_HOME`/`LOCALAPPDATA`, then runs `bin/m version`.
- **Invocation**: `python tools/devinstall/smoke.py [--self-check] [--build-only]`
- **Dependencies**: Python 3; bash (or Git-bash on Windows); pwsh; go.
- **Used by**: manual only (no in-repo caller).
- **Status**: active (manual smoke harness).



### Code generation and fixture tools

#### `tools/gen_0008_fixtures.go`
- **Path**: `tools/gen_0008_fixtures.go`
- **Type**: Go (`package main`; `//go:build ignore`)
- **Category**: code generation / fixtures
- **Platforms**: any with Go
- **Purpose**: Regenerates hermetic fixtures: `fixtures/registry/v1` (lodash 4.17.21 tarball + packument + `manifest.json` with SHA-256s), `fixtures/security/evil-archives/` (path-traversal members + README), and sample projects (`basic-cjs`, `basic-esm`, `typescript-app`, `workspace-simple`). Hardcoded deterministic 2020-01-01 tar mtimes.
- **Invocation**: `go run tools/gen_0008_fixtures.go` (from repo root; hardcoded `root := "."`).
- **Outputs**: writes to `fixtures/registry/v1/`, `fixtures/security/`, fixture project directories.
- **Dependencies**: stdlib only.
- **Used by**: manual one-shot; zero in-repo references. Superseded in part by `gen_fixture_tarballs.go` for registry packages.
- **Status**: legacy (initial commit artifact; most functions superseded).

#### `tools/gen_archives.go`
- **Path**: `tools/gen_archives.go`
- **Type**: Go (`package main`; `//go:build ignore`)
- **Category**: code generation / fixtures
- **Platforms**: any with Go
- **Purpose**: Writes `fixtures/archives/traversal-attack.tgz` (hostile path members) and `corrupt-hash.tgz` (valid tar, wrong hash) for archive fail-closed tests.
- **Invocation**: `go run tools/gen_archives.go <repo-root>` (arg required; exit 2 if missing).
- **Outputs**: two `.tgz` files under `fixtures/archives/`.
- **Dependencies**: stdlib only.
- **Used by**: manual one-shot; no in-repo callers.
- **Status**: manual one-shot generator.

#### `tools/gen_fixture_tarballs.go`
- **Path**: `tools/gen_fixture_tarballs.go`
- **Type**: Go (`package main`; `//go:build ignore`)
- **Category**: code generation / fixtures
- **Platforms**: any with Go
- **Purpose**: Writes registry tarballs `pkg-a`, `pkg-b`, `pkg-c`, `@scope/pkg` under `fixtures/registry/v1/tarballs/`, patches each packument's `dist.integrity`, regenerates `manifest.json`, and re-hashes all blobs.
- **Invocation**: `go run tools/gen_fixture_tarballs.go [<root>]` (defaults to `.`).
- **Dependencies**: stdlib only.
- **Used by**: manual one-shot; no in-repo callers.
- **Status**: manual one-shot generator (newer than `gen_0008_fixtures.go`).

#### `tools/gen_help_golden.go`
- **Path**: `tools/gen_help_golden.go`
- **Type**: Go (`package main`; `//go:build ignore`)
- **Category**: code generation / test data
- **Platforms**: any with Go
- **Purpose**: Regenerates `testdata/cli/help-golden/m-root.txt` and `mx-root.txt` by running `cli.NewMRoot`/`NewMXRoot` with `--help` (fixed BuildInfo version 0.0.0-test, CRLF-normalized).
- **Invocation**: `go run tools/gen_help_golden.go` (from repo root).
- **Outputs**: `testdata/cli/help-golden/m-root.txt`, `testdata/cli/help-golden/mx-root.txt`.
- **Dependencies**: `internal/cli` (transitively cobra).
- **Used by**: manual one-shot; output consumed by `internal/cli/help_test.go:62` and `internal/cli/foundation_test.go:45`.
- **Status**: active (manual; downstream tests depend on its output).

#### `tools/update-runtime-assets.py`
- **Path**: `tools/update-runtime-assets.py`
- **Type**: Python
- **Category**: code generation / validation
- **Platforms**: any with Python 3
- **Purpose**: Regenerates `internal/runtime/assets/manifest.json` from the asset files on disk (`credential-grabber.cjs`, `preload.cjs`, `preload.mjs`, `loader-register.mjs`, `ts-loader.mjs`). Hashes every asset with SHA-256, records byte size, auto-detects `moduleType` from file extension (`.cjs` → `cjs`, otherwise `esm`). Validates manifest schema, duplicate detection, path-traversal rejection, case-collision detection on case-insensitive filesystems. Atomic write via temp file + rename.
- **CLI**: `--write` (regenerate and update), `--check` (fail if stale, default), `--manifest <path>` / `--assets-dir <path>` (overrides for tests), `--verbose` (print changes), `--help`. Exit codes: 0 (ok), 1 (stale), 2 (usage), 3 (invalid manifest), 4 (scan/hash failure), 5 (write failure).
- **Outputs**: writes `internal/runtime/assets/manifest.json`.
- **Dependencies**: Python 3 stdlib only.
- **Derived vs manual fields**: `size` and `sha256` are derived from file bytes. `schemaVersion`, `bundleVersion`, `name`, `path`, `role`, `moduleType` are preserved from existing manifest entries. For newly discovered assets, `moduleType` is derived from file extension and `role` defaults to `preload-cjs`/`preload-esm` based on module type.
- **Tests**: `python3 -m unittest tools/test_update-runtime-assets.py` (29 tests).
- **Status**: active (generator + CI gate).

### Plans and documentation tools

#### `plans/scripts/enrich_and_generate.py`
- **Path**: `plans/scripts/enrich_and_generate.py`
- **Type**: Python
- **Category**: documentation generation
- **Platforms**: any with Python 3
- **Purpose**: Enriches all `plans/00xx-*.md` files from `enrichment-*.json` catalogs, then generates `CHECKLIST.md` and `manifest.json`. Reads `status.json` for MVP state. Replaces five PowerShell scripts (`enrich-and-generate.ps1`, `enrichment-catalog.ps1`, `Read-Status.ps1`, `generate-checklist.ps1`, `update-manifest.ps1`).
- **Invocation**: `python3 plans/scripts/enrich_and_generate.py` (canonical). Makefile: `make plans`, `make plans-check`.
- **Outputs**: writes enrichment blocks into plan files, `plans/CHECKLIST.md`, `plans/manifest.json`.
- **Dependencies**: Python 3 stdlib only.
- **Used by**: Makefile `plans` / `generate`; `tools/ci/verify_plan_generation.py` (downstream check); `plans/INDEX.md`, `plans/0000-README.md`.
- **Status**: active (generator; downstream CI verification depends on its output).

### Validation tools
### Validation tools

#### `tools/sbom/validate.py`
- **Path**: `tools/sbom/validate.py`
- **Type**: Python
- **Category**: validation
- **Platforms**: any with Python 3
- **Purpose**: Validates CycloneDX SBOM golden structure (`fixtures/sbom/medium-graph-cyclonedx-golden.json`). Checks `bomFormat`, `specVersion`, `bom-ref` on metadata component, existence of components/dependencies arrays, and cross-references all dependency refs. No external schema CLI required.
- **Invocation**: `python3 tools/sbom/validate.py [<path>]` (default: `fixtures/sbom/medium-graph-cyclonedx-golden.json`).
- **Outputs**: `OK: <path>`; throws `SystemExit` on validation failure.
- **Dependencies**: Python 3 stdlib only.
- **Used by**: manual SBOM validation; no in-repo CI caller.
- **Status**: active.

---


---

## Tool dependency and wrapper relationships

### Wrapper scripts pointing to canonical Python entrypoints

| Wrapper | Canonical | Type |
|---|---|---|
| `tools/fuzz-smoke.sh` | `tools/fuzz_smoke.py` | POSIX sh exec |
| `tools/certification/run-core-cert.sh` | `tools/certification/run_core_cert.py` | POSIX sh exec |
| `tools/update-runtime-assets.sh` | `tools/update-runtime-assets.py` | POSIX sh exec |

### Library sourcing

| Library | Sourced/imported by | Language |
|---|---|---|
| `scripts/lib/devinstall.sh` | `install-dev.sh`, `uninstall-dev.sh`, `devinstall_test.sh` | bash |
| `scripts/lib/DevInstall.psm1` | `install-dev.ps1`, `uninstall-dev.ps1`, `devinstall_test.ps1` | PowerShell |

### Makefile targets mapping to scripts/tools

| Makefile target | Canonical command | Direct or via wrapper? |
|---|---|---|
| `fuzz-smoke` | `python3 tools/fuzz_smoke.py` | Direct |
| `core-cert-fast` | `python3 tools/certification/run_core_cert.py core-cert-fast` | Direct |
| `core-cert` | `python3 tools/certification/run_core_cert.py core-cert` | Direct |
| `core-cert-security` | `python3 tools/certification/run_core_cert.py core-cert-security` | Direct |
| `core-cert-crash` | `python3 tools/certification/run_core_cert.py core-cert-crash` | Direct |
| `core-cert-performance` | `python3 tools/certification/run_core_cert.py core-cert-performance` | Direct |
| `install-dev` | `scripts/install-dev.ps1` (Windows) or `scripts/install-dev.sh` (Unix) | Direct |
| `uninstall-dev` | `scripts/uninstall-dev.ps1` (Windows) or `scripts/uninstall-dev.sh` (Unix) | Direct |
| `allowlist` | `go run ./tools/check-license; go run ./tools/check-deps` | Direct |
| `update-runtime-assets` | `python3 tools/update-runtime-assets.py --write` | Direct |
| `check-runtime-assets` | `python3 tools/update-runtime-assets.py --check` | Direct |

---

## CI-only and workflow-embedded tools

These tools exist only as inline shell scripts or commands inside `.github/workflows/ci.yml` and `full.yml`. They are not extractable as standalone scripts.

| Name | Workflow | Type | Purpose |
|---|---|---|---|
| gofmt check | ci.yml `quality` | inline bash | `gofmt -l` on all `.go` files |
| go mod tidy check | ci.yml `quality` | inline command | `go mod tidy` + `git diff --exit-code` |
| Arch check | ci.yml `quality` | inline command | `go test ./internal/archcheck/... -run TestProduction` |
| CI gate (default) | ci.yml `gate` | inline bash | Assert quality + test + windows-smoke all `success` |
| CI gate (full) | full.yml `gate` | inline bash | Assert all 11 upstream jobs `success` |
| Cross-compile loop | full.yml `cross-compile` | inline bash | Build `m`/`mx` for 6 GOOS/GOARCH pairs |
| pnpm shim installer | full.yml `core-certification` | inline bash | corepack prepare pnpm 9/10/11, `ln -sf` as `pnpm9`/`pnpm10`/`pnpm11` |
| Fail-closed cert proof | full.yml `core-certification` | inline bash | Two negative-filter cert runs expecting failure |
| CRLF guard | full.yml `platform-certification` | inline command | `git config --global core.autocrlf false` (Windows) |
| Race detector (per-OS) | full.yml `platform` | conditional commands | `-race` on transaction/store/fsx/resolver per OS |
| Lock conformance shard selectors | full.yml `lock-pnpm`, `lock-adapters` | inline bash | `-run` regex selection per pnpm major and per adapter |

---

## Platform-specific tools

| Tool | Unix (Linux/macOS) | Windows |
|---|---|---|
| `build` (Makefile) | `bin/m`, `bin/mx` | `bin/m.exe`, `bin/mx.exe` |
| `install-dev` | `scripts/install-dev.sh` (bash) | `scripts/install-dev.ps1` (PowerShell) |
| `uninstall-dev` | `scripts/uninstall-dev.sh` (bash) | `scripts/uninstall-dev.ps1` (PowerShell) |
| `devinstall_test` | `scripts/lib/devinstall_test.sh` (bash) | `scripts/lib/devinstall_test.ps1` (PowerShell) |
| DevInstall libs | `scripts/lib/devinstall.sh` (bash) | `scripts/lib/DevInstall.psm1` (PowerShell) |
| `tools` (dev tools) | `make tools` (go install) | `make tools` (go install) |
| `race` (Makefile) | `go test -race` (CGO required) | Not in Makefile; manual `go test -race` possible |
| `generate-lock-fixtures` | `python3 tools/conformance/generate_lock_fixtures.py` | `python3 tools/conformance/generate_lock_fixtures.py` |
| Plan enrichment | `python3 plans/scripts/enrich_and_generate.py` | `python3 plans/scripts/enrich_and_generate.py` |
| SBOM validation | `python3 tools/sbom/validate.py` | `python3 tools/sbom/validate.py` |

---

## Generated, legacy, experimental, or unclear tools

### Legacy

| Tool | Path | Reason |
|---|---|---|
| `gen_0008_fixtures.go` | `tools/gen_0008_fixtures.go` | Initial commit artifact; partially superseded by `gen_fixture_tarballs.go`; no in-repo callers |
| `enrichment-catalog.ps1` | `plans/scripts/enrichment-catalog.ps1` | Data module superseded by `enrichment-*.json`; not dot-sourced by current pipeline |

### One-shot generators (`//go:build ignore`)

Excluded from normal builds; run manually when fixture data needs regeneration:

- `tools/gen_0008_fixtures.go`
- `tools/gen_archives.go`
- `tools/gen_fixture_tarballs.go`
- `tools/gen_help_golden.go`

### Runtime assets (bundled, not development tools)

- `internal/runtime/assets/credential-grabber.cjs`
- `internal/runtime/assets/loader-register.mjs`
- `internal/runtime/assets/preload.cjs`
- `internal/runtime/assets/preload.mjs`
- `internal/runtime/assets/ts-loader.mjs`
- `internal/runtime/assets/manifest.json`

