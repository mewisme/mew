package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/dotenv"
)

func TestBuildEnvOverlayNoFlags(t *testing.T) {
	dir := t.TempDir()
	writeEnvTestFile(t, filepath.Join(dir, ".env"), "BASE=from_dotenv\n")

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(overlay) == 0 {
		t.Fatal("expected at least one env var from auto-discovery")
	}
	m := overlayToMap(overlay)
	if m["BASE"] != "from_dotenv" {
		t.Errorf("BASE = %q, want from_dotenv", m["BASE"])
	}
}

func TestBuildEnvOverlayNoEnvFile(t *testing.T) {
	dir := t.TempDir()
	writeEnvTestFile(t, filepath.Join(dir, ".env"), "BASE=should_not_load\n")

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{noEnvFile: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := overlayToMap(overlay)
	if _, ok := m["BASE"]; ok {
		t.Error("BASE should not be loaded with --no-env-file")
	}
}

func TestBuildEnvOverlayExplicitFile(t *testing.T) {
	dir := t.TempDir()
	writeEnvTestFile(t, filepath.Join(dir, ".env"), "BASE=auto\n")
	explicit := filepath.Join(dir, "custom.env")
	writeEnvTestFile(t, explicit, "BASE=explicit\nCUSTOM=yes\n")

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{envFile: []string{explicit}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := overlayToMap(overlay)
	if m["BASE"] != "explicit" {
		t.Errorf("BASE = %q, want explicit", m["BASE"])
	}
	if m["CUSTOM"] != "yes" {
		t.Errorf("CUSTOM = %q, want yes", m["CUSTOM"])
	}
}

func TestBuildEnvOverlayMode(t *testing.T) {
	dir := t.TempDir()

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{mode: "production"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := overlayToMap(overlay)
	if m["NODE_ENV"] != "production" {
		t.Errorf("NODE_ENV = %q, want production", m["NODE_ENV"])
	}
}

func TestBuildEnvOverlayModeWithDiscovery(t *testing.T) {
	dir := t.TempDir()
	writeEnvTestFile(t, filepath.Join(dir, ".env"), "BASE=from_dotenv\n")
	writeEnvTestFile(t, filepath.Join(dir, ".env.production"), "BASE=from_production\nMODE_SPECIFIC=yes\n")

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{mode: "production"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := overlayToMap(overlay)
	if m["BASE"] != "from_production" {
		t.Errorf("BASE = %q, want from_production (mode-specific should override base)", m["BASE"])
	}
	if m["MODE_SPECIFIC"] != "yes" {
		t.Errorf("MODE_SPECIFIC = %q, want yes", m["MODE_SPECIFIC"])
	}
	if m["NODE_ENV"] != "production" {
		t.Errorf("NODE_ENV = %q, want production", m["NODE_ENV"])
	}
}

func TestBuildEnvOverlayNoEnvFileWithMode(t *testing.T) {
	dir := t.TempDir()
	writeEnvTestFile(t, filepath.Join(dir, ".env"), "BASE=should_not_load\n")

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{noEnvFile: true, mode: "development"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := overlayToMap(overlay)
	if _, ok := m["BASE"]; ok {
		t.Error("BASE should not load with --no-env-file")
	}
	if m["NODE_ENV"] != "development" {
		t.Errorf("NODE_ENV = %q, want development", m["NODE_ENV"])
	}
}

func TestBuildEnvOverlayEmptyDir(t *testing.T) {
	dir := t.TempDir()
	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(overlay) != 0 {
		t.Errorf("expected empty overlay for empty dir, got %d entries", len(overlay))
	}
}

// ── Failure tests (0054) ──────────────────────────────────────────────

func TestBuildEnvOverlayExplicitFileNotFound(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nonexistent.env")

	_, err := buildEnvOverlay(dir, leadingDispatchFlags{envFile: []string{missing}})
	if err == nil {
		t.Fatal("expected error for missing explicit env-file")
	}
	if apperr.CodeOf(err) != apperr.EnvFileNotFound {
		t.Errorf("expected EnvFileNotFound, got %s", apperr.CodeOf(err))
	}
}

func TestBuildEnvOverlayExplicitFileUnreadable(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "unreadable.env")
	writeEnvTestFile(t, bad, "KEY=value\n")

	// Inject a failing open for this specific file so the test works
	// cross-platform without relying on Unix permission bits.
	orig := dotenv.Open
	dotenv.Open = func(name string) (*os.File, error) {
		if name == bad {
			return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrPermission}
		}
		return orig(name)
	}
	defer func() { dotenv.Open = orig }()

	_, err := buildEnvOverlay(dir, leadingDispatchFlags{envFile: []string{bad}})
	if err == nil {
		t.Fatal("expected error for unreadable explicit env-file")
	}
	code := apperr.CodeOf(err)
	if code != apperr.EnvFileRead && code != apperr.EnvFileNotFound {
		t.Errorf("expected EnvFileRead or EnvFileNotFound, got %s", code)
	}
}

func TestBuildEnvOverlayExplicitFileMalformed(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.env")
	writeEnvTestFile(t, bad, "KEY=\"unterminated\n")

	_, err := buildEnvOverlay(dir, leadingDispatchFlags{envFile: []string{bad}})
	if err == nil {
		t.Fatal("expected error for malformed explicit env-file")
	}
	if apperr.CodeOf(err) != apperr.EnvFileParse {
		t.Errorf("expected EnvFileParse, got %s", apperr.CodeOf(err))
	}
}

func TestBuildEnvOverlayMultipleExplicitFilesOrder(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.env")
	second := filepath.Join(dir, "second.env")
	writeEnvTestFile(t, first, "KEY=first\nFIRST_ONLY=yes\n")
	writeEnvTestFile(t, second, "KEY=second\n")

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{envFile: []string{first, second}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := overlayToMap(overlay)
	if m["KEY"] != "second" {
		t.Errorf("KEY = %q, want second (later file overrides)", m["KEY"])
	}
	if m["FIRST_ONLY"] != "yes" {
		t.Errorf("FIRST_ONLY = %q, want yes", m["FIRST_ONLY"])
	}
}

func TestBuildEnvOverlayFirstValidThenFailing(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.env")
	second := filepath.Join(dir, "nonexistent.env")
	writeEnvTestFile(t, first, "KEY=value\n")

	_, err := buildEnvOverlay(dir, leadingDispatchFlags{envFile: []string{first, second}})
	if err == nil {
		t.Fatal("expected error when second explicit file is missing")
	}
	if apperr.CodeOf(err) != apperr.EnvFileNotFound {
		t.Errorf("expected EnvFileNotFound, got %s", apperr.CodeOf(err))
	}
}

func TestBuildEnvOverlayAutoDiscoveryMissingOptional(t *testing.T) {
	dir := t.TempDir()
	// No .env files at all — auto-discovery should return empty, not error.

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{})
	if err != nil {
		t.Fatalf("auto-discovery should not error on missing optional files: %v", err)
	}
	if len(overlay) != 0 {
		t.Errorf("expected empty overlay for dir with no .env files, got %d entries", len(overlay))
	}
}

func TestBuildEnvOverlayRelativeExplicitPath(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "relative.env")
	writeEnvTestFile(t, envFile, "RELATIVE=yes\n")

	// Use a relative path — buildEnvOverlay resolves against cwd.
	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{envFile: []string{"relative.env"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := overlayToMap(overlay)
	if m["RELATIVE"] != "yes" {
		t.Errorf("RELATIVE = %q, want yes", m["RELATIVE"])
	}
}

func TestBuildEnvOverlayPathWithSpaces(t *testing.T) {
	dir := t.TempDir()
	spaceDir := filepath.Join(dir, "path with spaces")
	if err := os.MkdirAll(spaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(spaceDir, "spaced.env")
	writeEnvTestFile(t, envFile, "SPACES=work\n")

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{envFile: []string{envFile}})
	if err != nil {
		t.Fatalf("unexpected error for path with spaces: %v", err)
	}
	m := overlayToMap(overlay)
	if m["SPACES"] != "work" {
		t.Errorf("SPACES = %q, want work", m["SPACES"])
	}
}

func TestBuildEnvOverlayNoEnvFileWithExplicit(t *testing.T) {
	// --no-env-file disables auto-discovery but explicit --env-file still loads.
	dir := t.TempDir()
	writeEnvTestFile(t, filepath.Join(dir, ".env"), "AUTO=ignored\n")
	explicit := filepath.Join(dir, "explicit.env")
	writeEnvTestFile(t, explicit, "EXPLICIT=loaded\n")

	overlay, err := buildEnvOverlay(dir, leadingDispatchFlags{
		noEnvFile: true,
		envFile:   []string{explicit},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := overlayToMap(overlay)
	if _, ok := m["AUTO"]; ok {
		t.Error("AUTO should not be loaded with --no-env-file")
	}
	if m["EXPLICIT"] != "loaded" {
		t.Errorf("EXPLICIT = %q, want loaded", m["EXPLICIT"])
	}
}

func TestBuildEnvOverlayAutoDiscoveryReadError(t *testing.T) {
	// Auto-discovered files with read errors should still propagate errors
	// from the dotenv.Load call (non-ENOENT errors are not swallowed).
	// But dotenv.Discover only adds files that exist (os.Stat succeeds),
	// so this tests the edge case where a file is removed between Stat and Open.
	// In practice this is hard to trigger deterministically, so we verify
	// the error type contract instead.
	dir := t.TempDir()
	badEnv := filepath.Join(dir, ".env")
	writeEnvTestFile(t, badEnv, "KEY=\"unterminated\n")

	_, err := buildEnvOverlay(dir, leadingDispatchFlags{})
	if err == nil {
		t.Fatal("expected parse error for malformed auto-discovered file")
	}
	if apperr.CodeOf(err) != apperr.EnvFileParse {
		t.Errorf("expected EnvFileParse, got %s", apperr.CodeOf(err))
	}
}

func TestBuildEnvOverlayExplicitFileErrorContainsPath(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "definitely-missing.env")

	_, err := buildEnvOverlay(dir, leadingDispatchFlags{envFile: []string{missing}})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "definitely-missing.env") {
		t.Errorf("error message should contain the failing path, got: %s", msg)
	}
	// Must not dump file contents or environment values.
	if strings.Contains(msg, "KEY=") || strings.Contains(msg, "PASSWORD=") {
		t.Errorf("error message should not contain environment values: %s", msg)
	}
}

func TestBuildWatchEnvOverlayExplicitFileNotFound(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nonexistent.env")

	_, err := buildWatchEnvOverlay(dir, []string{missing}, false, "")
	if err == nil {
		t.Fatal("expected error for missing explicit env-file in watch mode")
	}
	if apperr.CodeOf(err) != apperr.EnvFileNotFound {
		t.Errorf("expected EnvFileNotFound, got %s", apperr.CodeOf(err))
	}
}

func TestBuildWatchEnvOverlayAutoDiscoveryMissingOptional(t *testing.T) {
	dir := t.TempDir()

	overlay, err := buildWatchEnvOverlay(dir, nil, false, "")
	if err != nil {
		t.Fatalf("auto-discovery should not error on missing files: %v", err)
	}
	if len(overlay) != 0 {
		t.Errorf("expected empty overlay, got %d entries", len(overlay))
	}
}

func TestBuildWatchEnvOverlayFirstValidThenFailing(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.env")
	second := filepath.Join(dir, "nonexistent.env")
	writeEnvTestFile(t, first, "KEY=value\n")

	_, err := buildWatchEnvOverlay(dir, []string{first, second}, false, "")
	if err == nil {
		t.Fatal("expected error when second explicit file is missing in watch mode")
	}
	if apperr.CodeOf(err) != apperr.EnvFileNotFound {
		t.Errorf("expected EnvFileNotFound, got %s", apperr.CodeOf(err))
	}
}

func TestBuildWatchEnvOverlayDelegatesToBuildEnvOverlay(t *testing.T) {
	dir := t.TempDir()
	writeEnvTestFile(t, filepath.Join(dir, ".env"), "BASE=from_dotenv\n")
	explicit := filepath.Join(dir, "custom.env")
	writeEnvTestFile(t, explicit, "EXPLICIT=yes\n")

	direct, err := buildEnvOverlay(dir, leadingDispatchFlags{
		envFile: []string{explicit},
		mode:    "staging",
	})
	if err != nil {
		t.Fatalf("direct: %v", err)
	}

	watch, err := buildWatchEnvOverlay(dir, []string{explicit}, false, "staging")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	if len(direct) != len(watch) {
		t.Fatalf("length mismatch: direct=%d watch=%d", len(direct), len(watch))
	}
	dm := overlayToMap(direct)
	wm := overlayToMap(watch)
	for k, v := range dm {
		if wm[k] != v {
			t.Errorf("key %s: direct=%q watch=%q", k, v, wm[k])
		}
	}
	for k := range wm {
		if _, ok := dm[k]; !ok {
			t.Errorf("key %s in watch but not direct", k)
		}
	}
}

func writeEnvTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func overlayToMap(overlay []string) map[string]string {
	m := make(map[string]string, len(overlay))
	for _, kv := range overlay {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return m
}
