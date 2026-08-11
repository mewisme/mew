# 0057 Runtime Stabilization Gate — Certification Evidence

Date: 2026-08-12. Branch: `implement-005x-plans`. Plan: `plans/0057-runtime-stabilization.md`.

This document records the evidence produced by the runtime certification gate.
The authoritative evidence is the set of machine-readable certification reports
produced by `.github/workflows/runtime-cert.yml` for each push to this branch.

## Certification Contract

- **Matrix**: 3 OS × 4 Node versions = 12 required cells
- **OS**: ubuntu-latest, macos-latest, windows-latest
- **Node**: 18.x, 20.x, 22.x (LTS), 24.x
- **Commit binding**: Exact HEAD SHA. Report `commitSHA` must match the workflow `github.sha`.
  Reports bound to any other SHA are rejected as STALE.
- **Fail-closed**: Missing report, wrong SHA, zero matched required tests, failed/skipped
  required suite, malformed/stale report → aggregate fails.

### SHA Binding Semantics

The certification workflow uses `actions/checkout@v7` (default behavior), which checks
out the merge commit for pull request events and the branch HEAD for push events. The
certification report records `commitSHA` via `MEW_COMMIT_SHA` environment variable
(set to `${{ github.sha }}`), which is the commit SHA that triggered the workflow run.

For PR events, `github.sha` is the merge commit SHA (last commit on the PR's merge ref).
For push events, `github.sha` is the branch HEAD commit SHA.

The aggregate verification step (`runtime-certification-aggregate`) validates that every
report's `commitSHA` matches `github.sha` exactly. Reports from different workflow runs
or cached artifacts from prior SHA are rejected.

**Decision**: Certification binds to the workflow trigger SHA (`github.sha`), which is
the merge commit for PRs and branch HEAD for pushes. This is the GitHub Actions default
and the correct target: certification must verify the exact code that was tested.

### CI Workflow Reference

- Workflow: `.github/workflows/runtime-cert.yml`
- Called by: `ci.yml` (PR/push gate) and `full.yml` (nightly/release)
- Aggregate artifact: `runtime-certification-summary`
- Per-cell artifacts: `runtime-certification-{os}-node-{ver}`

## Conformance Matrix

| Suite ID | Coverage | Required |
|---|---|---|
| `runtime-syntax` | TS, TSX, MTS, CTS, JS, MJS, CJS, imports | Yes |
| `runtime-resolution` | Loaders, PnP, tsconfig paths, module format | Yes |
| `runtime-env` | Environment loading, precedence, expansion | Yes |
| `runtime-workers` | Worker isolation, credentials, child processes | Yes |
| `runtime-storage` | localStorage, sessionStorage | Yes |
| `runtime-watch` | Watch start, restart, shutdown | Yes |
| `runtime-diagnostics` | Trace, doctor, cache explain, support, inspector, opt-out | Yes |
| `runtime-cache` | Transform cache round-trip, key stability, source maps | Yes |
| `runtime-failure` | Syntax errors, missing modules, exit codes | Yes |

Probe suites (not required, run explicitly):

| Suite ID | Coverage | Required |
|---|---|---|
| `runtime-soak-watch` | Watch mode soak: rapid start/stop, goroutine leak detection | No (probe) |
| `runtime-soak-worker` | Worker soak: rapid create/destroy, goroutine leak detection | No (probe) |
| `runtime-soak-transform` | Transform recovery soak: repeated invocations, leak detection | No (probe) |

## How to Verify Locally

```bash
# Build the binary
make build

# Run runtime conformance
go run ./cmd/m conformance run runtime --json

# Run runtime benchmarks with baseline comparison
go run ./cmd/m benchmark runtime --cold --warm --samples 5 --warmup 1 \
  --json --compare benchmarks/runtime-baseline.json

# Run soak tests (short mode)
go test ./tests/conformance/runtime/... -run 'Soak' -short -count=1 -v

# Long soak (manual, set MEW_SOAK_CYCLES=1000 for multi-hour run)
MEW_SOAK_CYCLES=1000 go test ./tests/conformance/runtime/... -run 'Soak' -count=1 -v -timeout 24h
```

## Frozen Protocol Versions

See `docs/runtime/protocol-versions.md` and `docs/schema-freeze.md` for the frozen
runtime protocol and schema versions (0057 stabilization gate).

| Protocol/Schema | Version | Source |
|---|---|---|
| Transform IPC | 2 | `internal/transform/protocol.go:ProtocolVersion` |
| Transform cache schema | 1 | `internal/transform/cache.go:CacheSchemaVersion` |
| Runtime asset manifest | schema v2, bundle v10 | `internal/runtime/assets/manifest.json` |
| Trace event schema | 1 | `internal/trace/event.go:SchemaVersion` |
| Conformance report schema | 2 | `internal/conformance/report.go:ReportSchemaVersion` |
| Web Storage schema | 1 | `web-storage.cjs` |
| Resolve diagnostic schema | 1 | `resolve-diagnostic.mjs` |

## Benchmark Baselines

- Baseline: `benchmarks/runtime-baseline.json` (schema v1, linux/amd64)
- Metrics: `runtime.startup.latency`, `runtime.transform.latency` (cold/warm), `runtime.execution.walltime`
- Threshold: 10% regression
- Command: `go run ./cmd/m benchmark runtime --cold --warm --compare benchmarks/runtime-baseline.json`

## Known Limitations

See `docs/runtime/known-limitations.md` for the full catalog. Key limitations carried
forward from 0050-0056:

- OXC divergence (esbuild, intentional, permanent)
- `emitDecoratorMetadata` unsupported (transpile-only)
- JSX entrypoints via extension substitution only
- PnP unplugged mode not detected
- Worker-specific tsconfig not supported (planned 0060+)
- Custom loader propagation to workers deferred
- No persistent transform daemon (planned 0060+)
- Watch mode polling fallback on network filesystems (500ms)

## Waivers

No 0057-specific waivers are recorded. All conformance suites are expected to pass on
all 12 platform/Node combinations. If a platform-specific failure occurs in CI that
cannot be fixed in this stabilization pass, a waiver will be recorded here with owner,
expiry, and rationale.

## Exit Criteria Status

| Criterion | Status | Evidence |
|---|---|---|
| Supported syntax and Node versions have published certification | PENDING CI | `runtime-cert.yml` exact-head run on this branch |
| No known transform cache corruption or source-map integrity bug | PASS | `runtime-cache` suite; `runtime-diagnostics` source-map tests |
| Watch and workers do not leak processes, services, or file descriptors | PASS | `runtime-workers`, `runtime-watch`, soak probe suites |
| Plain Node escape hatch remains behaviorally plain | PASS | `runtime-diagnostics` opt-out tests; `TestConformanceOptOut` |
| Runtime protocol versions frozen | PASS | `docs/runtime/protocol-versions.md`, `docs/schema-freeze.md` |
| Runtime support matrix published with evidence | PASS | `docs/support-matrix.md` |
| Benchmark baseline checked in | PASS | `benchmarks/runtime-baseline.json` |
| Worker/watch soak evidence published | PASS | `tests/conformance/runtime/soak_test.go` (automated) |
