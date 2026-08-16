package runtime_test

import (
	"strings"
	"testing"
)

// --- Syntax/transform conformance ---
// Every case proves the runtime can transform and execute TypeScript
// variants. Exit code 0 alone is insufficient; each test asserts
// observable output (stdout or fixture output.txt).

func TestConformanceHelloTS(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "hello from ts") {
		t.Fatalf("expected 'hello from ts', got %q", out)
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
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "hello.mts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "hello from mts") {
		t.Fatalf("expected 'hello from mts', got %q", out)
	}
}

func TestConformanceHelloCTS(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "hello.cts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "hello from cts") {
		t.Fatalf("expected 'hello from cts', got %q", out)
	}
}

// --- JS/plain execution (no transform needed) ---

func TestConformanceHelloJS(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "hello.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "hello from js") {
		t.Fatalf("expected 'hello from js', got %q", out)
	}
}

func TestConformanceHelloMJS(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "hello.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "hello from mjs") {
		t.Fatalf("expected 'hello from mjs', got %q", out)
	}
}

func TestConformanceHelloCJS(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := deadline(t)
	defer cancel()
	code, out, _ := runMBinary(t, ctx, proj, "hello.cjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "hello from cjs") {
		t.Fatalf("expected 'hello from cjs', got %q", out)
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
	got := readOutput(t, proj)
	if got != "resolved-dep-cts" {
		t.Fatalf("expected 'resolved-dep-cts', got %q", got)
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
