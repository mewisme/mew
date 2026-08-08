package runtime_test

import (
	"strings"
	"testing"
)

// --- Syntax/transform conformance ---
// Every case exercises the production CLI path: m <entrypoint> against a
// fixture project. No internal Go APIs are called directly for the behavior
// being certified. Exit code 0 proves successful execution; content
// verification uses file-based output (output.txt) where available.

func TestConformanceHelloTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestConformanceHelloTSX(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "hello.tsx")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "greeting") || !strings.Contains(got, "hello from tsx") {
		t.Fatalf("unexpected output: %s", got)
	}
}

func TestConformanceHelloMTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "hello.mts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestConformanceHelloCTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "hello.cts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

// --- JS/plain execution (no transform needed) ---

func TestConformanceHelloJS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "hello.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestConformanceHelloMJS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "hello.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestConformanceHelloCJS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "hello.cjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

// --- Import resolution with transform (file-based output) ---

func TestConformanceImportJSToTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "import-js-to-ts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "resolved-lib-ts" {
		t.Fatalf("expected 'resolved-lib-ts', got %q", got)
	}
}

func TestConformanceImportMJSToMTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "import-mjs-to-mts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "resolved-mod-mts" {
		t.Fatalf("expected 'resolved-mod-mts', got %q", got)
	}
}

func TestConformanceImportCJSToCTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "import-cjs-to-cts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestConformanceImportJSXToTSX(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "import-jsx-to-tsx.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "resolved-component-tsx" {
		t.Fatalf("expected 'resolved-component-tsx', got %q", got)
	}
}

// --- Import with existing JS taking precedence over TS ---

func TestConformanceImportExistingJS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "import-existing-js.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "real-js-wins" {
		t.Fatalf("expected 'real-js-wins', got %q", got)
	}
}
