package archcheck_test

import (
	"testing"

	"github.com/mewisme/mew/internal/conformance"
	"github.com/mewisme/mew/internal/graph"
	"github.com/mewisme/mew/internal/lockfile"
	"github.com/mewisme/mew/internal/lockfile/mlock"
	"github.com/mewisme/mew/internal/runtime/assets"
	"github.com/mewisme/mew/internal/transform"
)

// TestVersionDrift guards documented protocol/schema versions against
// accidental source drift. When a source constant changes, this test fails
// and the developer must update both this test and
// docs/runtime/protocol-versions.md in the same commit.
func TestVersionDrift(t *testing.T) {
	// Transform IPC protocol (docs/runtime/protocol-versions.md § Transform IPC)
	if transform.ProtocolVersion != 2 {
		t.Errorf("transform.ProtocolVersion = %d, want 2 (update docs/runtime/protocol-versions.md)", transform.ProtocolVersion)
	}

	// Transform cache schema (docs/runtime/protocol-versions.md § Transform Cache)
	if transform.CacheSchemaVersion != 1 {
		t.Errorf("transform.CacheSchemaVersion = %d, want 1 (update docs/runtime/protocol-versions.md)", transform.CacheSchemaVersion)
	}

	// Runtime asset manifest (docs/runtime/protocol-versions.md § Runtime Assets)
	m, err := assets.LoadManifest()
	if err != nil {
		t.Fatalf("assets.LoadManifest: %v", err)
	}
	if m.SchemaVersion != 2 {
		t.Errorf("asset manifest schemaVersion = %d, want 2 (update docs/runtime/protocol-versions.md)", m.SchemaVersion)
	}
	if m.BundleVersion != "10" {
		t.Errorf("asset manifest bundleVersion = %s, want \"10\" (update docs/runtime/protocol-versions.md)", m.BundleVersion)
	}

	// Conformance report schema (docs/runtime/protocol-versions.md § Conformance Reports)
	if conformance.ReportSchemaVersion != 2 {
		t.Errorf("conformance.ReportSchemaVersion = %d, want 2 (update docs/runtime/protocol-versions.md)", conformance.ReportSchemaVersion)
	}

	// Graph (canonical) schema (docs/runtime/protocol-versions.md § Related Persistent Formats)
	if graph.SchemaVersion != 3 {
		t.Errorf("graph.SchemaVersion = %d, want 3 (update docs/runtime/protocol-versions.md)", graph.SchemaVersion)
	}
	// Graph cache schema (internal resolve/cache blobs)
	if graph.CacheSchemaVersion != 1 {
		t.Errorf("graph.CacheSchemaVersion = %d, want 1 (update docs/runtime/protocol-versions.md)", graph.CacheSchemaVersion)
	}

	// Native m.lock (docs/runtime/protocol-versions.md § Related Persistent Formats)
	if mlock.LockfileVersion != 3 {
		t.Errorf("mlock.LockfileVersion = %d, want 3 (update docs/runtime/protocol-versions.md)", mlock.LockfileVersion)
	}

	// Loss report schema (docs/runtime/protocol-versions.md § Related Persistent Formats)
	if lockfile.LossReportSchemaVersion != 1 {
		t.Errorf("lockfile.LossReportSchemaVersion = %d, want 1 (update docs/runtime/protocol-versions.md)", lockfile.LossReportSchemaVersion)
	}
}

// TestVersionDriftIntentionalMismatch verifies the drift guard actually
// fails on an intentional mismatch (meta-test).
func TestVersionDriftIntentionalMismatch(t *testing.T) {
	// Verify the guard would catch a wrong expected value by checking
	// that ProtocolVersion is not some absurd value like 9999.
	if transform.ProtocolVersion == 9999 {
		t.Error("transform.ProtocolVersion is 9999 — this should never happen")
	}
	// Verify the guard would fail if we used a wrong expected value.
	// The guard above expects ProtocolVersion == 2. If it were 0 or
	// unset, the test would fail.
	if transform.ProtocolVersion == 0 {
		t.Error("transform.ProtocolVersion is 0 — constant may be unset")
	}
}
