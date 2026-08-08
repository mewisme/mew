# Performance tuning

MVP **0029**. Measure install phases, tune concurrency, and guard regressions
with the bench harness.

## Install phase timing

Pass `--debug` (or set `MEW_DEBUG=1` / `M_LOG=debug`) to emit per-phase elapsed
time at the end of each install transaction phase:

```text
debug: install phase phase=resolve elapsed=120ms
debug: install phase phase=fetch elapsed=1.2s
debug: install phase phase=link elapsed=450ms
debug: install phase phase=validate elapsed=80ms
debug: install phase phase=commit elapsed=200ms
```

Phases: `resolve`, `fetch`, `link`, `validate`, `commit`. Progress events
(`phase=…` on stderr) continue regardless of debug mode.

NDJSON / JSON reporters emit structured `type=debug` records with `phase` and
`elapsed` attributes when debug is enabled.

## Worker concurrency

Fetch and registry metadata use the same default worker count:

```text
workers = min(max(runtime.NumCPU(), 1), 16)
```

Implemented in `fetch.DefaultWorkers()` and `registry.defaultMaxWorkers()`.

| Surface | Default |
|---|---|
| Tarball download pool | `fetch.Downloader.Workers` |
| Registry `Packuments` batch | `registry.Options.MaxWorkers` |

`ponytail:` `MEW_FETCH_WORKERS` env override is deferred to the cross-cutting
performance program (MVP **0081**); today only the NumCPU default applies.

## Hot-path profiling (Phase 5)

After baseline timing landed, resolver `selectVersion` buffer reuse and linker
placement `strings.Split` avoidance were evaluated against `m bench install`
and package-level `go test -bench` suites. Neither showed a clear win on the
checked-in medium-graph fixture, so no hot-path diff was applied.

`ponytail:` revisit when a 1k+ workspace fixture corpus exists (MVP **0081**)
or bench medians regress without an obvious network cause.

Integrity verification, tarball hash checks, and store validation are never
skipped for speed.

## `m bench install`

End-to-end install benchmark against a fixture project (default
`fixtures/bench/medium-graph`):

```text
m bench install              # cold (default): clear bench home cache first
m bench install --cold
m bench install --warm       # reuse cache from prior bench run
m bench install --fixture <path>
m bench install --json       # BenchResult JSON on stdout
```

Bench runs use an isolated home under `.cache/mew/bench/<fixture>/` and a local
fixture registry. Lifecycle scripts are ignored (`IgnoreScripts`).

JSON shape:

```json
{
  "case": "medium-graph-cold",
  "mode": "cold",
  "phases": { "resolve": 120, "fetch": 800, "link": 200 },
  "totalMs": 1500
}
```

Phase keys mirror install progress; values are milliseconds.

## Baselines and CI gates

Published medians live in [`benchmarks/install-baseline.json`](../benchmarks/install-baseline.json)
(schema v3: keyed by `name`, `os`, `arch`, `goVersion`, `runnerClass`, `benchmarkMode`).

| Gate | Blocking? | Command |
|---|---|---|
| Bench correctness | **Yes** | `python tools/bench/check_correctness.py --mode warm` |
| Bench regression | **No** (advisory until Ubuntu baseline exists) | `python tools/bench/check_regression.py --mode warm` |

Correctness validates JSON shape, fixture digest presence, and **≥7** measured samples
(default warmup 1 → 8 total iterations). Regression compares median/p95 against a
**platform-matched** baseline row; fixture digest mismatch fails closed.

Structured waivers live in [`benchmarks/waivers.json`](../benchmarks/waivers.json).
`BENCH_WAIVER=1` is deprecated — use waivers with owner, reason, issue, and expiry.

Check regression locally:

```text
python tools/bench/check_regression.py --mode warm
python tools/bench/check_regression.py --mode cold
```

Soak repeated installs:

```text
python tools/soak/install_loop.py --count 10 --mode cold
```

## `m bench runtime`

Structured runtime benchmark with isolated cache/state:

```text
m bench runtime --cold --warm             # cold + warm measurements
m bench runtime --warm --samples 5        # warm only, 5 samples
m bench runtime --cold --json             # JSON output
m bench runtime --warm --compare path     # baseline comparison
```

All state (cache, store, config, temp) is isolated in a bench-owned
temporary directory. The user's real Mew cache is never mutated.

### Metrics

| ID | Unit | Category | Description |
|----|------|----------|-------------|
| `runtime.startup.latency` | ns | n/a | `runtime.Plan` construction (Node launch plan) |
| `runtime.transform.latency` | ns | cold, warm | TypeScript transform latency |
| `runtime.execution.walltime` | ns | warm | `m run` process wall-clock time |

**Cold** measurement: bench-owned transform cache is cleared before timing.
**Warm** measurement: cache is primed, then cache-hit path is timed.

### Sampling

`--samples N` measured iterations (default 5). `--warmup N` discarded
warmup iterations (default 1). Warmup samples are excluded from aggregates.

Aggregates: median, min, max computed deterministically from raw samples.
Zero samples, negative durations, and NaN/Inf values fail the benchmark.

### Baseline comparison

```text
m bench runtime --warm --compare benchmarks/runtime-baseline.json
```

Baseline schema v1. Comparison checks:
- Schema version, OS, and arch must match
- Every current metric must exist in baseline with matching unit
- Duplicate metric IDs in baseline fail
- 10% default regression threshold (configurable in baseline)

Output includes per-metric delta, threshold, and verdict
(`pass` / `regression` / `improvement`). Overall status is `regression`
if any required metric exceeds threshold.

### JSON schema

```json
{
  "schemaVersion": 1,
  "environment": { "goVersion": "...", "os": "...", "arch": "...", "logicalCpus": 4 },
  "cold": true, "warm": true,
  "samples": 5, "warmup": 1,
  "measurements": [
    {
      "id": "runtime.transform.latency",
      "unit": "ns",
      "category": "cold",
      "rawSamplesNs": [500000, 520000, 510000],
      "aggregate": { "medianNs": 510000, "minNs": 500000, "maxNs": 520000 },
      "sampleCount": 3, "warmupCount": 1
    }
  ]
}
```

Human output derives from the same typed model.

### Limitations

Wall-clock measurements on shared hardware are inherently noisy. Baseline
comparison uses generous default thresholds suitable for developer laptops.
CI baselines should be captured on dedicated, quiesced runners.

## Package-level benchmarks

Lower-level `go test -bench` suites (transaction, resolver, store, linker) are
documented in [`tests/benchmarks/README.md`](../tests/benchmarks/README.md).

## Related

- [`offline.md`](offline.md) — warm cache and capsule bootstrap
- [`cli.md`](cli.md) — `m bench`, `m capsule`
- [`install.md`](install.md) — install pipeline
