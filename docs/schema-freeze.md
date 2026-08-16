# Schema freeze — PM core (0031) + Runtime (0057)

Contract freeze for persistent formats and machine-readable outputs shipped in
MVPs **0010–0030** and runtime stabilization (**0050–0057**). Runner MVPs (**0040+**)
and subsequent MVPs depend on these shapes remaining backward compatible unless
an ADR authorizes a breaking change.

See also: [`lockfile.md`](lockfile.md), [`core-certification.md`](core-certification.md), [`runtime/protocol-versions.md`](runtime/protocol-versions.md).

## Frozen artifacts

| Artifact | Version field | Frozen since | Change process |
|---|---|---|---|
| `m.lock` | `lockfileVersion: 3` | MVP 0031 | ADR + migration tool + fixture regen |
| Canonical graph | `graph.SchemaVersion: 3` | MVP 0015 | ADR; lock encoder/decoder must round-trip |
| Install result JSON | implicit (no top-level schemaVersion) | MVP 0016 | Additive fields only; see `app.InstallResult` |
| Mutation plan JSON | `plan.SchemaVersion: 1` | MVP 0028 | ADR for breaking plan shape |
| Audit report | `schemaVersion: 1` | MVP 0030 | ADR; see [`audit.md`](audit.md) |
| SBOM export | format flag (`cyclonedx` / `spdx`) | MVP 0030 | ADR; golden tests in `fixtures/sbom/` |
| Policy report | `schemaVersion: 1` | MVP 0030 | ADR; see [`policy.md`](policy.md) |
| Org policy file | `schemaVersion: 1` | MVP 0030 | ADR |
| Doctor report | `schemaVersion: 1` | MVP 0031 | Additive checks only |
| Core conformance report | `schemaVersion: 2` | MVP 0031 (Pass 32) | Additive suite metadata only; breaking shape requires ADR |
| Install bench baseline | `schemaVersion: 2` | Pass 32 | Regenerate via `m bench install --baseline`; median/p95 fields |
| Transaction journal | `schemaVersion` in lock doc | MVP 0017 | ADR; recovery must handle prior version |
| Transform IPC protocol | `ProtocolVersion: 2` | MVP 0057 | ADR; client+server sync; migration path for v2→v3 |
| Transform cache schema | `CacheSchemaVersion: 1` | MVP 0057 | ADR; stale cache auto-purged on bump |
| Runtime asset manifest | `schemaVersion: 2`, `bundleVersion: "10"` | MVP 0057 | ADR; per-asset SHA-256 integrity |
| Trace event schema | `SchemaVersion: 1` | MVP 0057 | ADR; emitter + consumer sync |
| Conformance report (all matrices) | `ReportSchemaVersion: 2` | MVP 0057 | Additive suite metadata only; breaking shape requires ADR |
| Web Storage schema | `SCHEMA_VERSION: 1` | MVP 0057 | ADR; stale data auto-purged on bump |
| Resolve diagnostic schema | `SCHEMA_VERSION: 1` | MVP 0057 | ADR; loader consumer sync |

## `m.lock` v3 (native)

Frozen fields and semantics:

- Top-level: `lockfileVersion`, `checksum`, `settings`, `importers`, `packages`, `edges`, optional `extensions`
- Package keys encode peer context, patches, and protocols deterministically
- Checksum covers canonical serialized bytes; `m lock format` must not change ordering rules without ADR
- Incumbent lockfiles (`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lock`, `nub.lock`) are **not** frozen — Mew preserves incumbent bytes on no-op paths; semantic rewrite requires certified producer evidence (see [`lockfile.md`](lockfile.md))

## Install result JSON

Emitted by `m install`, `m add`, `m remove`, `m ci`, `m update`, `m dedupe`,
`m prune`, and related flags with `--json`.

Frozen shape (`internal/app.InstallResult`):

```json
{
  "added": 0,
  "removed": 0,
  "changed": 0,
  "packages": 0,
  "plan": { "schemaVersion": 1 },
  "committed": true
}
```

New fields may be appended. Existing field types and meanings must not change
without an ADR.

## Audit / SBOM / policy reports

| Report | Schema | Doc |
|---|---|---|
| `AuditReport` | `schemaVersion: 1` | [`audit.md`](audit.md) |
| CycloneDX / SPDX SBOM | format version in output | [`sbom.md`](sbom.md) |
| `PolicyReport` | `schemaVersion: 1` | [`policy.md`](policy.md) |

## CLI grammar freeze

Shipped PM command names, primary flags, and exit-code contracts documented in
[`pm-commands.md`](pm-commands.md) and [`cli.md`](cli.md) are frozen. New
commands may be added; renaming or removing shipped commands requires an ADR.

Stabilization-only commands (`m doctor`, `m conformance`) are part of the 0031
surface and follow the same freeze from MVP 0031 onward.

## Non-frozen (explicit)

- Runner CLI (`m run`, `mx`) — MVP 0040+ (runner certification run; individual runner schemas are frozen at v1)
- Full 0080 differential conformance report schema
- Live Sigstore attestation verification protocol
- Advisory feed signature format
- Runtime launch/plan/environment-prepared schemas — defined in `internal/runner/envexec/`, each frozen at v1; broader runner surface changes require ADR
