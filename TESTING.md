# Testing

Quick reference for running tests in this repository. Strategy and architecture:
[`docs/testing.md`](docs/testing.md). Tool inventory: [`TOOLS.md`](TOOLS.md).

## Quick start

```sh
make test-short        # fast suite, skips soak/wall-clock (PR gate)
make test              # full suite, adaptive parallel
make test-race         # race detector, requires CGO
make ci                # normal PR CI locally (quality + test-short + build)
```

## Test tiers

| Target | What runs | Use case |
|---|---|---|
| `make test-short` | `./...` with `-short` | Pre-commit, PR CI |
| `make test` | `./...` full | Pre-push, release-check |
| `make test-unit` | All internal packages, no `/tests/` | Quick package feedback |
| `make test-integration` | `./tests/integration/...` | Integration-only debugging |
| `make test-e2e` | Runtime E2E + Node version tests | Loader/transform validation |
| `make test-crash` | Crash recovery suite (`crash` build tag) | Transaction durability |
| `make test-race` | `./...` with `-race` | Concurrency safety |
| `make test-all` | `test` + `test-race` | Full local gate |

## Adaptive parallelism

`tools/testexec` is the canonical test orchestrator. Make targets use it by default.

### Worker count

```sh
make test                          # auto (adapts to CPU count and workload)
make test TESTEXEC_WORKERS=1       # serial (debugging, low-resource)
make test TESTEXEC_WORKERS=4       # explicit 4 workers
```

`auto` caps: unit=NCPU, integration≤4, crash≤3, race≤2.

### Direct testexec use

```sh
go run ./tools/testexec -short ./internal/resolver/...
go run ./tools/testexec -race -workers 2 ./internal/transaction/...
go run ./tools/testexec -tags crash ./tests/integration/...
go run ./tools/testexec -run 'TestIncremental' ./internal/resolver/...
```

### Per-worker CPU budget

Each worker gets `GOMAXPROCS = floor(logicalCPUs / workers)`, minimum 1.
4 CPUs / 4 workers = GOMAXPROCS=1 each.

## Single package (plain go test)

Direct `go test` always works — testexec is an orchestration layer, not a replacement:

```sh
go test ./internal/resolver/... -count=1
go test ./internal/resolver/... -run TestIncrementalDiff -count=1 -v
go test ./internal/app/... -short -count=1
```

## Crash tests

Crash tests use the `crash` build tag. Excluded from normal `./...`.

```sh
make test-crash
go test -tags crash ./tests/integration/... -run Crash -count=1 -timeout 30m
```

Shard verification: `make crash-shards-check` ensures every crash test maps to exactly one CI shard.

## Race detector

```sh
make test-race                            # full suite with -race
CGO_ENABLED=1 go test -race ./internal/transaction/... -count=1
```

Race tests are the only CGO exception. Production builds use `CGO_ENABLED=0`.

## Conformance

```sh
make conformance                          # lock bridge conformance suite
go test ./tests/conformance/... -count=1
```

## Package-specific targets

```sh
make test-runtime       # runtime, transform, node
make test-transform     # transform only
make test-cli           # CLI only
make test-runner        # runner, process, lifecycle
make test-workspace     # workspace, snapshot
```

## Test isolation

Integration/conformance tests use `testkit.CleanEnv` for hermetic environments.
Each heavy-package worker in testexec gets an isolated HOME/XDG/MEW_* temp directory.
No test may depend on the developer's real home, cache, or config.

## CI

Normal PR CI (`ci.yml`): lint + `go test ./... -short` on Linux + Windows smoke (config/project/cli/fsx).

Full CI (`full.yml`, nightly/on-demand): 3-OS platform matrix, race detector, cross-compile, lock conformance, crash integration, runtime Node matrix, benchmarks, certification, vulnerability scan.

## Adding tests

1. Write hermetic tests: use `testkit.CleanEnv` or `t.TempDir`, never mutate checked-in fixtures.
2. Integration tests go in `tests/integration/`.
3. Run `make generate-check` after changing anything that affects generated files.
4. Run `make crash-shards-check` after adding crash tests.
5. Verify with both serial and parallel modes: `TESTEXEC_WORKERS=1 make test-short` then `make test-short`.
