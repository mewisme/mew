# Runtime Protocol Versions

Current as of 0052-0057 implementation. Subject to change until the 0057 runtime stabilization gate is complete and exact-head certification is observed. Once frozen, all versions will require a migration path for changes.

## Transform IPC

| Component | Version | Source |
|---|---|---|
| Protocol | `2` | `internal/transform/protocol.go:ProtocolVersion` |
| Frame encoding | u32 little-endian length-prefixed JSON | `internal/transform/protocol.go` |
| Transport | TCP loopback (127.0.0.1, random port via `MEW_TRANSFORM_ENDPOINT`) | `internal/transform/service.go` |
| Auth | random hex token per service instance (`MEW_TRANSFORM_TOKEN`) | `internal/transform/service.go` |
| Handshake | `HelloRequest{V, Token}` → `HelloResponse{V, OK, ErrCode}` | `internal/transform/service.go` |

**Request/response types** (protocol v2):

| Type | Direction | Description |
|---|---|---|
| `hello` | C→S | Authenticate session (first frame on connection) |
| `health` | C→S | No-op health check |
| `transform` | C→S | Single-file transform request; response carries `ok` field for success/error |
| `cancel` | C→S | Cancel an in-flight transform by cancel token |
| `shutdown` | C→S | Graceful connection shutdown |

## Transform Cache

| Component | Version | Source |
|---|---|---|
| Cache schema | `1` | `internal/transform/cache.go:CacheSchemaVersion` |
| Key algorithm | SHA-256(source + path + loader + format + normalizedOpts + optsDigest + engineName + engineVersion + sourceMapMode + targetNodeMajor) | `internal/transform/cache.go:CacheKey` |
| Layout | `<cacheDir>/transform/v1/<prefix2>/<sha64>.{code,map,meta}` | `internal/transform/cache.go:CacheKeyPath` |
| Commit record | JSON metadata written last (atomic via temp+rename) | `internal/transform/cache.go:WriteCache` |
| Integrity | SHA-256 of code + map stored in metadata; verified on read | `internal/transform/cache.go:computeOutputDigest` |

## Runtime Assets (Loader Bridge)

| Component | Version | Source |
|---|---|---|
| Manifest schema | `2` (v1 accepted for backward compat) | `internal/runtime/assets/assets.go:LoadManifest` |
| Bundle version | `8` | `internal/runtime/assets/manifest.json` |
| Asset roles | `preload-cjs`, `preload-esm`, `loader-registration`, `loader-support`, `credential-grabber` | `internal/runtime/assets/assets.go:AssetRole` |
| Integrity | SHA-256 per asset, verified on extraction | `internal/runtime/assets/assets.go:VerifyAsset` |

**Embedded assets** (bundle v8):

| Asset | Role | Module type |
|---|---|---|
| `credential-grabber.cjs` | credential-grabber | cjs |
| `preload.cjs` | preload-cjs | cjs |
| `preload.mjs` | preload-esm | esm |
| `loader-register.mjs` | loader-registration | esm |
| `ts-loader.mjs` | loader-support | esm |

**Environment variables** (credential/loader bridge):

| Variable | Purpose |
|---|---|
| `MEW_TRANSFORM_ENDPOINT` | TCP host:port for transform service (127.0.0.1:&lt;port&gt;) |
| `MEW_TRANSFORM_TOKEN` | One-time auth token (cleared after grab) |
| `MEW_TRANSFORM_OPTIONS` | JSON-serialized NormalizedOptions (cleared after grab) |
| `MEW_TRANSFORM_OPTS_DIGEST` | SHA-256 of options JSON |
| `MEW_TRANSFORM_CONFIG_DIR` | tsconfig directory for path resolution |

## Conformance Reports

| Component | Version | Source |
|---|---|---|
| Go-test report schema | `2` | `internal/conformance/report.go:ReportSchemaVersion` |
| Runner report schema | `1` | `internal/conformance/runner_report.go` (if exists) |

## Related Persistent Formats

Versions of formats the runtime stabilization gate depends on (defined in their own packages):

| Format | Version | Source |
|---|---|---|
| Manifest (`package.json` normalized) | `1` | `internal/manifest/manifest.go:SchemaVersion` |
| Lockfile | `1` | `internal/lockfile/interface.go` |
| Graph | `1` | `internal/graph/graph.go` |
| Loss report | `1` | `internal/lockfile/interface.go:LossReportSchemaVersion` |
| Release train config | `1` | `internal/releasetrain/releasetrain.go:SchemaVersion` |
| Registry packument cache | `1` | `internal/registry/packument.go:CacheSchemaVersion` |
| Registry test manifest | `1` | `internal/testkit/registry.go:RegistryManifestSchemaVersion` |

## Change Policy

- Versions in this document are not yet frozen. Freeze will occur when the 0057 stabilization gate is complete.
- Bumping a version requires: (1) a migration path for existing data, (2) a backward-compat window or explicit break notice, (3) an update to this document in the same commit.
- New persistent formats must be versioned from their first commit.
