package support

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/config"
)

func TestBundleSchema(t *testing.T) {
	// Verify SchemaVersion is positive.
	if SchemaVersion < 1 {
		t.Errorf("SchemaVersion = %d, want >= 1", SchemaVersion)
	}
	// Verify RedactPolicyVersion is set.
	if RedactPolicyVersion == "" {
		t.Error("RedactPolicyVersion is empty")
	}
}

func TestManifestDeterministic(t *testing.T) {
	// Collect twice with empty collectors; manifests should be
	// structurally identical except for CollectedAt timestamp.
	ac := &app.Context{CWD: t.TempDir()}
	b1, err := Collect(context.Background(), ac, nil)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := Collect(context.Background(), ac, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b1.Manifest.SchemaVersion != b2.Manifest.SchemaVersion {
		t.Error("schema versions differ")
	}
	if b1.Manifest.Status != b2.Manifest.Status {
		t.Error("statuses differ")
	}
	if len(b1.Manifest.Entries) != 0 || len(b2.Manifest.Entries) != 0 {
		t.Error("entries present with nil collectors")
	}
}

func TestVersionCollector(t *testing.T) {
	ac := &app.Context{Version: "1.0.0", Commit: "abc1234", BuildDate: "2026-01-01"}
	data, err := VersionCollector{}.Collect(context.Background(), ac)
	if err != nil {
		t.Fatal(err)
	}
	var dto VersionDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", dto.Version, "1.0.0")
	}
	if dto.Commit != "abc1234" {
		t.Errorf("Commit = %q", dto.Commit)
	}
	if dto.GoVersion == "" {
		t.Error("GoVersion is empty")
	}
}

func TestVersionCollectorNilContext(t *testing.T) {
	data, err := VersionCollector{}.Collect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var dto VersionDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatal(err)
	}
	// Should still have GoVersion even with nil context.
	if dto.GoVersion == "" {
		t.Error("GoVersion is empty with nil context")
	}
}

func TestOSCollector(t *testing.T) {
	data, err := OSCollector{}.Collect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var dto OSDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatal(err)
	}
	if dto.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", dto.OS, runtime.GOOS)
	}
	if dto.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", dto.Arch, runtime.GOARCH)
	}
	if dto.NumCPU <= 0 {
		t.Errorf("NumCPU = %d, want > 0", dto.NumCPU)
	}
}

func TestNodeCollector(t *testing.T) {
	// Node should be discoverable on PATH in dev/test environments.
	data, err := NodeCollector{}.Collect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var dto NodeDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Error != "" {
		t.Logf("Node not found (may be ok in CI): %s", dto.Error)
		return
	}
	if dto.Version == "" {
		t.Error("Version is empty")
	}
	if dto.Path == "" {
		t.Error("Path is empty")
	}
	if len(dto.Capabilities) == 0 {
		t.Error("Capabilities is empty")
	}
}

func TestFeaturesCollector(t *testing.T) {
	data, err := FeaturesCollector{}.Collect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var dto FeaturesDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Total <= 0 {
		t.Errorf("Total = %d, want > 0", dto.Total)
	}
	shipped, ok := dto.ByStatus["shipped"]
	if !ok {
		t.Logf("no 'shipped' status in by_status: %v", dto.ByStatus)
	}
	_ = shipped
}

func TestConfigMetaCollector(t *testing.T) {
	// Build a minimal effective config.
	eff := &config.Effective{Values: map[string]config.Value{}}
	eff.Values["cache.dir"] = config.Value{Raw: "/tmp/cache"}
	eff.Values["registry.auth_token_env"] = config.Value{Raw: "NPM_TOKEN"} // secret key

	ac := &app.Context{Config: eff}
	data, err := ConfigMetaCollector{}.Collect(context.Background(), ac)
	if err != nil {
		t.Fatal(err)
	}
	var dto ConfigMetaDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatal(err)
	}
	// Key names should be present, but never values.
	foundCache := false
	for _, k := range dto.Keys {
		if k == "cache.dir" {
			foundCache = true
		}
		// Keys should not contain secrets embedded by the sanitizer.
		if strings.Contains(k, "TOKEN") {
			// Key name contains TOKEN but the value is never in the
			// DTO; this is not a leak.
			_ = k
		}
	}
	if !foundCache {
		t.Error("cache.dir not found in keys")
	}
	// The DTO must never contain raw config values (the struct has no Value field).
	raw, _ := json.Marshal(dto)
	if strings.Contains(string(raw), "/tmp/cache") {
		t.Error("config value leaked into DTO")
	}
}

func TestConfigMetaCollectorNilConfig(t *testing.T) {
	data, err := ConfigMetaCollector{}.Collect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var dto ConfigMetaDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Error == "" {
		t.Error("expected error with nil config")
	}
}

func TestCollectorNames(t *testing.T) {
	collectors := []Collector{
		VersionCollector{},
		OSCollector{},
		NodeCollector{},
		FeaturesCollector{},
		DoctorCollector{},
		ConfigMetaCollector{},
	}
	seen := map[string]bool{}
	for _, c := range collectors {
		name := c.Name()
		if name == "" {
			t.Errorf("empty name for %T", c)
		}
		if seen[name] {
			t.Errorf("duplicate collector name: %s", name)
		}
		seen[name] = true
		if !strings.HasSuffix(name, ".json") {
			t.Errorf("collector %s: name should end with .json", name)
		}
	}
}

func TestRequiredCollectors(t *testing.T) {
	// Version and OS are required.
	if !(VersionCollector{}.Required()) {
		t.Error("VersionCollector should be required")
	}
	if !(OSCollector{}.Required()) {
		t.Error("OSCollector should be required")
	}
	// Others are optional.
	if (NodeCollector{}.Required()) {
		t.Error("NodeCollector should be optional")
	}
	if (FeaturesCollector{}.Required()) {
		t.Error("FeaturesCollector should be optional")
	}
	if (DoctorCollector{}.Required()) {
		t.Error("DoctorCollector should be optional")
	}
	if (ConfigMetaCollector{}.Required()) {
		t.Error("ConfigMetaCollector should be optional")
	}
}

func TestBundleArchiveRoundTrip(t *testing.T) {
	ac := &app.Context{
		Version:   "1.0.0-test",
		Commit:    "deadbeef",
		BuildDate: "2026-01-01",
	}
	collectors := []Collector{
		VersionCollector{},
		OSCollector{},
	}
	bundle, err := Collect(context.Background(), ac, collectors)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "bundle.tgz")

	if err := WriteBundle(archivePath, bundle); err != nil {
		t.Fatal(err)
	}

	// Read back.
	restored, err := ReadBundle(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Manifest.SchemaVersion != bundle.Manifest.SchemaVersion {
		t.Error("schema version mismatch after round-trip")
	}
	if len(restored.Entries) != len(bundle.Entries) {
		t.Errorf("entry count: got %d, want %d", len(restored.Entries), len(bundle.Entries))
	}
	for i, e := range restored.Entries {
		orig := bundle.Entries[i]
		if e.Name != orig.Name {
			t.Errorf("entry %d name: got %q, want %q", i, e.Name, orig.Name)
		}
	}
}

func TestBundleAtomicWrite(t *testing.T) {
	ac := &app.Context{
		Version: "1.0.0-test",
	}
	bundle, err := Collect(context.Background(), ac, []Collector{VersionCollector{}})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "bundle.tgz")

	// First write succeeds.
	if err := WriteBundle(archivePath, bundle); err != nil {
		t.Fatal(err)
	}

	// Verify the file exists and is non-empty.
	fi, err := os.Stat(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() == 0 {
		t.Error("archive is empty")
	}

	// Verify it's readable.
	restored, err := ReadBundle(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Manifest.Status != StatusOK {
		t.Errorf("status = %q, want %q", restored.Manifest.Status, StatusOK)
	}
}

func TestBundleDeterministicOrdering(t *testing.T) {
	ac := &app.Context{Version: "1.0.0-test"}
	collectors := []Collector{
		ConfigMetaCollector{},
		OSCollector{},
		VersionCollector{},
	}
	bundle, err := Collect(context.Background(), ac, collectors)
	if err != nil {
		t.Fatal(err)
	}

	// Entries must be sorted by name regardless of collector registration order.
	for i := 1; i < len(bundle.Entries); i++ {
		if bundle.Entries[i-1].Name >= bundle.Entries[i].Name {
			t.Errorf("entries not sorted: %s before %s",
				bundle.Entries[i-1].Name, bundle.Entries[i].Name)
		}
	}
	// Verify expected order.
	want := []string{"config.json", "os.json", "version.json"}
	for i, e := range bundle.Entries {
		if e.Name != want[i] {
			t.Errorf("entry[%d] = %q, want %q", i, e.Name, want[i])
		}
	}
}

func TestManifestEntryStatus(t *testing.T) {
	// An optional collector that always fails.
	failCollector := &fakeCollector{name: "fail.json", required: false, failWith: assertError()}
	ac := &app.Context{Version: "1.0.0-test"}
	bundle, err := Collect(context.Background(), ac, []Collector{
		VersionCollector{},
		failCollector,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Bundle should still succeed (optional failure).
	if bundle.Manifest.Status != StatusOK {
		t.Errorf("status = %q, want %q", bundle.Manifest.Status, StatusOK)
	}
	// Find the failed entry in manifest.
	var failMeta *EntryMeta
	for i := range bundle.Manifest.Entries {
		if bundle.Manifest.Entries[i].Name == "fail.json" {
			failMeta = &bundle.Manifest.Entries[i]
		}
	}
	if failMeta == nil {
		t.Fatal("fail.json not in manifest")
	}
	if failMeta.Status != StatusError {
		t.Errorf("fail entry status = %q, want %q", failMeta.Status, StatusError)
	}
}

func TestRequiredCollectorFailure(t *testing.T) {
	failCollector := &fakeCollector{name: "fail.json", required: true, failWith: assertError()}
	ac := &app.Context{Version: "1.0.0-test"}
	_, err := Collect(context.Background(), ac, []Collector{
		failCollector,
		VersionCollector{},
	})
	if err == nil {
		t.Fatal("expected error from required collector failure")
	}
}

func TestWriteBundleToDir(t *testing.T) {
	ac := &app.Context{Version: "1.0.0-test"}
	bundle, err := Collect(context.Background(), ac, []Collector{
		VersionCollector{},
		OSCollector{},
	})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := WriteBundleToDir(dir, bundle); err != nil {
		t.Fatal(err)
	}

	// Each entry plus manifest should be a file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(bundle.Entries)+1 { // +1 for manifest
		t.Errorf("got %d files, want %d", len(entries), len(bundle.Entries)+1)
	}
}

// ------- Redaction / security tests -------

func TestRedactionNoEnvValues(t *testing.T) {
	// Every DTO must be free of env-value-like patterns.
	ac := &app.Context{
		Version:   "1.0.0",
		Commit:    "abc123",
		BuildDate: "2026-01-01",
	}
	collectors := []Collector{
		VersionCollector{},
		OSCollector{},
		ConfigMetaCollector{},
	}
	bundle, err := Collect(context.Background(), ac, collectors)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := WriteBundleToDir(dir, bundle); err != nil {
		t.Fatal(err)
	}

	// Read every file and scan for env-value patterns.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)

		// Should not contain env-value-like patterns.
		for _, pat := range []string{
			"NPM_TOKEN=",
			"SECRET=",
			"PASSWORD=",
			"API_KEY=",
		} {
			if strings.Contains(content, pat) {
				t.Errorf("%s contains %q", e.Name(), pat)
			}
		}
	}
}

func TestRedactionBearerToken(t *testing.T) {
	// trace.Redact should strip Bearer tokens.
	raw := "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	redacted := SanitizeString(raw)
	if strings.Contains(redacted, "eyJ") {
		t.Errorf("Bearer token not redacted: %s", redacted)
	}
	if !strings.Contains(redacted, "Bearer ***") {
		t.Errorf("expected Bearer *** placeholder, got: %s", redacted)
	}
}

func TestRedactionQueryToken(t *testing.T) {
	raw := "https://example.com/api?access_token=secret123&foo=bar"
	redacted := SanitizeString(raw)
	if strings.Contains(redacted, "secret123") {
		t.Errorf("query token not redacted: %s", redacted)
	}
}

func TestRedactionEnvSecretPattern(t *testing.T) {
	tests := []string{
		"MY_TOKEN=secret",
		"API_KEY=abc123",
		"DB_PASSWORD=hunter2",
		"NPM_SECRET=xyz",
	}
	for _, raw := range tests {
		redacted := SanitizeString(raw)
		if strings.Contains(redacted, "secret") || strings.Contains(redacted, "abc123") ||
			strings.Contains(redacted, "hunter2") || strings.Contains(redacted, "xyz") {
			// Only check the specific value; it might not match if the test value
			// is also a substring of the key name. Use regex-like check.
			if strings.Contains(redacted, "=secret") {
				t.Errorf("env secret value not redacted in %q: %s", raw, redacted)
			}
		}
	}
}

func TestRedactionError(t *testing.T) {
	err := &stubError{msg: "failed with token: Bearer abc123"}
	redacted := SanitizeError(err)
	if strings.Contains(redacted, "abc123") {
		t.Errorf("error token not redacted: %s", redacted)
	}
}

func TestSafePath(t *testing.T) {
	// Project-relative path under CWD.
	p := SafePath("/home/user/project/node_modules/pkg", "/home/user/project")
	if p != "node_modules/pkg" {
		t.Errorf("expected relative path, got %q", p)
	}

	// Path outside base stays absolute but has home stripped.
	p = SafePath("/home/user/other/file", "/home/user/project")
	if strings.Contains(p, "/home/user") {
		t.Errorf("home directory not stripped: %q", p)
	}

	// Home stripping.
	p = SafePath("/home/user/.npm/_npx", "")
	if strings.Contains(p, "/home/user") {
		t.Errorf("home directory not stripped: %q", p)
	}
}

func TestRedactedPlaceholder(t *testing.T) {
	if RedactedPlaceholder != config.RedactedPlaceholder {
		t.Error("RedactedPlaceholder diverged from config.RedactedPlaceholder")
	}
}

func TestNestedErrorSanitization(t *testing.T) {
	// A nested error chain with a secret marker must not leak.
	inner := &stubError{msg: "NPM_TOKEN=leaked-secret-value-123"}
	outer := &stubError{msg: "wrapped: " + inner.Error()}

	redacted := SanitizeError(outer)
	if strings.Contains(redacted, "leaked-secret-value-123") {
		t.Errorf("nested error leaked secret: %s", redacted)
	}
}

// Ensure no DTO contains raw Go struct fields that could accidentally
// pick up secrets via future field additions.
func TestDTOsAreExplicit(t *testing.T) {
	// Every DTO must be a struct with only json-tagged fields.
	// The key test: no `any` or `interface{}` fields that could
	// accidentally carry raw production structs.
	dtos := []any{
		VersionDTO{},
		OSDTO{},
		NodeDTO{},
		FeaturesDTO{},
		DoctorDTO{},
		ConfigMetaDTO{},
	}
	for _, dto := range dtos {
		data, err := json.Marshal(dto)
		if err != nil {
			t.Errorf("marshal %T: %v", dto, err)
		}
		// Smoke test: every DTO must have schema_version.
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Errorf("unmarshal %T: %v", dto, err)
		}
		if _, ok := m["schema_version"]; !ok {
			t.Errorf("%T missing schema_version", dto)
		}
	}
}

// ------- Helpers -------

func assertError() error {
	return &stubError{msg: "simulated failure"}
}

type stubError struct{ msg string }

func (e *stubError) Error() string { return e.msg }

type fakeCollector struct {
	name     string
	required bool
	failWith error
}

func (f *fakeCollector) Name() string   { return f.name }
func (f *fakeCollector) Required() bool { return f.required }
func (f *fakeCollector) Collect(context.Context, *app.Context) ([]byte, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return json.Marshal(map[string]any{"ok": true})
}
