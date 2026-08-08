# Doctor

MVP **0031**. End-user health checks for project and package-manager readiness.

Contributor tooling (`m development doctor`) is separate — see
[`development-doctor.md`](development-doctor.md).

See also: [`cli.md`](cli.md), [`errors.md`](errors.md).

## `m doctor`

```text
m doctor [--json] [--strict]
```

| Flag | Effect |
|---|---|
| `--json` | Emit `DoctorReport` JSON (schema v1) |
| `--strict` | Treat warnings as failures (exit **1**) |

Exit **0** when every check is `ok`, or only `warn` without `--strict`.
Exit **1** when any check is `fail`, or any `warn` with `--strict`.

### Checks

| ID | Severity when failing | Notes |
|---|---|---|
| `project` | fail | `package.json` discovery and read |
| `config` | fail | layered configuration loaded |
| `cache` | fail | global cache root writable |
| `store` | fail | content store root writable |
| `lock` | fail | incumbent lock present and `m lock validate` passes |
| `filesystem` | warn | link probe between store and `node_modules`; limited caps warn |
| `transaction` | warn | incomplete `.mew/txn` journals; run `m recover` |
| `node` | warn | Node on `PATH` (runner is MVP **0040**) |

Hard blockers fail closed. Recoverable or optional gaps surface as warnings.

### Human output

One line per check:

```text
check=project status=ok message=package.json readable at /path/to/proj
check=lock status=ok message=m.lock validated
...
doctor=ok
```

### JSON schema v1

```json
{
  "schemaVersion": 1,
  "checkedAt": "2026-07-30T00:00:00Z",
  "checks": [
    {
      "id": "project",
      "status": "ok",
      "message": "package.json readable at /path/to/proj"
    },
    {
      "id": "lock",
      "status": "ok",
      "message": "m.lock validated"
    }
  ],
  "ok": true
}
```

`status` is one of `ok`, `warn`, or `fail`. `remediation` is present when Mew
can suggest a next step.

## Related commands

```text
m doctor runtime                   runtime health checks (0056)
m development doctor              contributor prerequisites (unchanged)
m development doctor filesystem   detailed link-capability probe
m recover                         clear incomplete transaction journals
m lock validate                   lock-only validation used by the lock check
```

## `m doctor runtime`

```text
m doctor runtime [--json] [--strict]
```

Health checks for the runtime subsystem (transform service, Node capabilities,
loader bridge, watch backend, inspector, cache).

| ID | Severity when failing | Notes |
|---|---|---|
| `node-capabilities` | fail | Node ≥18 with required preload/module-register capabilities |
| `transform-handshake` | fail | Real transform session start + auth handshake |
| `transform-roundtrip` | fail | esbuild transforms a minimal TS fixture end-to-end |
| `source-map` | warn | Transform emits a valid source map |
| `tsconfig` | fail/warn | tsconfig discovery + extends-chain load |
| `runtime-cache` | warn/fail | Asset cache integrity + read/write probe |
| `loader-bridge` | fail | Runtime assets + Node discovery for the loader bridge |
| `watch-backend` | fail | Watcher factory succeeds (native or polling) |
| `inspector` | fail/warn | Inspector flag parsing and argv build pipeline |
| `worker` | warn/fail | Node ≥12 (worker_threads availability) |

Exit codes and `--json`/`--strict` semantics match `m doctor`.
