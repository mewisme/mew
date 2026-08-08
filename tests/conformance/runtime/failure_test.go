package runtime_test

import (
	"strings"
	"testing"
)

// --- Transform/syntax failure ---
// Failure tests use runMBinary to capture child process stderr,
// which contains the actual error messages (SyntaxError, etc.).
// runM with --output silent only captures CLI-layer output.

func TestConformanceFailureSyntaxErrorTS(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "syntax-error.ts")
	if code == 0 {
		t.Fatalf("expected non-zero exit for syntax error, got out=%s", out)
	}
	hasError := strings.Contains(strings.ToLower(out), "syntax") ||
		strings.Contains(strings.ToLower(out), "error") ||
		strings.Contains(strings.ToLower(out), "transform") ||
		strings.Contains(out, "ERR_")
	if !hasError {
		t.Fatalf("syntax error output does not indicate failure type:\n%s", out)
	}
}

func TestConformanceFailureSyntaxErrorJS(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "syntax-error.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for JS syntax error, got out=%s", out)
	}
	hasError := strings.Contains(strings.ToLower(out), "syntax") ||
		strings.Contains(strings.ToLower(out), "error") ||
		strings.Contains(strings.ToLower(out), "unexpected") ||
		strings.Contains(out, "ERR_")
	if !hasError {
		t.Fatalf("JS syntax error output does not indicate failure type:\n%s", out)
	}
}

// --- Module resolution failure ---

func TestConformanceFailureImportMissing(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "import-missing.mjs")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing import, got out=%s", out)
	}
	hasResolution := strings.Contains(strings.ToLower(out), "cannot find") ||
		strings.Contains(strings.ToLower(out), "not found") ||
		strings.Contains(strings.ToLower(out), "module") ||
		strings.Contains(strings.ToLower(out), "resolve") ||
		strings.Contains(out, "ERR_MODULE_NOT_FOUND") ||
		strings.Contains(out, "ERR_")
	if !hasResolution {
		t.Fatalf("missing import error does not indicate resolution failure:\n%s", out)
	}
}

// --- Explicit env-file missing ---

func TestConformanceFailureEnvFileMissing(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "--env-file", "nonexistent.env", "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing --env-file, got out=%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "env") && !strings.Contains(strings.ToLower(out), "not found") && !strings.Contains(out, "ERR_") {
		t.Fatalf("missing env-file error does not indicate env failure:\n%s", out)
	}
}

// --- Bad loader ---

func TestConformanceFailureLoaderInvalidPath(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "--loader", "./nonexistent-loader.mjs", "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing loader, got out=%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "loader") && !strings.Contains(strings.ToLower(out), "not found") && !strings.Contains(out, "ERR_") {
		t.Fatalf("loader error does not indicate loader failure:\n%s", out)
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
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "does-not-exist.ts")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing file, got out=%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "not found") && !strings.Contains(out, "ERR_") {
		t.Fatalf("expected 'not found' or error code in output:\n%s", out)
	}
}
