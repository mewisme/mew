package runtime_test

import (
	"strings"
	"testing"
)

// --- Transform/syntax failure ---

func TestConformanceFailureSyntaxErrorTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// syntax-error.ts has deliberate syntax errors
	code, out := runM(t, proj, "syntax-error.ts")
	if code == 0 {
		t.Fatalf("expected non-zero exit for syntax error, got out=%s", out)
	}
}

func TestConformanceFailureSyntaxErrorJS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// syntax-error.js has deliberate syntax errors
	code, out := runM(t, proj, "syntax-error.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for JS syntax error, got out=%s", out)
	}
}

// --- Module resolution failure ---

func TestConformanceFailureImportMissing(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// import-missing.mjs imports a non-existent module
	code, out := runM(t, proj, "import-missing.mjs")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing import, got out=%s", out)
	}
}

// --- Explicit env-file missing ---

func TestConformanceFailureEnvFileMissing(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// Explicit --env-file pointing to non-existent file should fail
	code, out := runM(t, proj, "--env-file", "nonexistent.env", "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing --env-file, got out=%s", out)
	}
}

// --- Bad loader ---

func TestConformanceFailureLoaderInvalidPath(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, out := runM(t, proj, "--loader", "./nonexistent-loader.mjs", "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing loader, got out=%s", out)
	}
}

// --- Non-zero exit propagation ---

func TestConformanceFailureNonZeroExit(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "exit-code.js")
	if code != 42 {
		t.Fatalf("expected exit=42, got exit=%d", code)
	}
}

// --- Entrypoint: file not found ---

func TestConformanceFailureFileNotFound(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, out := runM(t, proj, "does-not-exist.ts")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing file, got out=%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "not found") && !strings.Contains(out, "ERR_") {
		t.Fatalf("expected 'not found' or error code in output, got %q", out)
	}
}
