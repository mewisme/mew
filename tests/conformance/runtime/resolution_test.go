package runtime_test

import (
	"strings"
	"testing"
)

// --- Custom loader chaining ---

func TestConformanceLoaderOrdinary(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// loader-log.mjs writes to output.txt on each resolve/load hook
	code, _ := runM(t, proj, "--loader", "./loader-log.mjs", "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "loader-log:resolve:") {
		t.Fatalf("loader hooks not invoked, output: %s", got)
	}
}

func TestConformanceLoaderDelegating(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// loader-delegate.mjs chains to nextResolve/nextLoad
	code, _ := runM(t, proj, "--loader", "./loader-delegate.mjs", "hello.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "loader-delegate:resolve:") {
		t.Fatalf("delegate hooks not invoked, output: %s", got)
	}
}

func TestConformanceLoaderOrdering(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// Two loaders; both should be invoked and write to output.txt
	code, _ := runM(t, proj,
		"--loader", "./loader-order-a.mjs",
		"--loader", "./loader-order-b.mjs",
		"hello.js",
	)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") {
		t.Fatalf("both loaders not invoked, output: %s", got)
	}
}

func TestConformanceLoaderError(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// loader-error.mjs throws on load
	code, out := runM(t, proj, "--loader", "./loader-error.mjs", "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit from loader error, got out=%s", out)
	}
}

// --- tsconfig path alias resolution ---

func TestConformanceTsconfigPaths(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "resolve-module-paths")
	// This fixture has tsconfig.json with paths: @app/*, @utils/*, @lib
	// The entrypoint is src/helpers.ts — just verify it can be executed
	code, out := runM(t, proj, "src/helpers.ts")
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
}

// --- PnP resolution ---

func TestConformancePnPBasic(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e-pnp")
	code, _ := runM(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "pnp-resolved:test-dep") {
		t.Fatalf("expected 'pnp-resolved:test-dep', got %q", got)
	}
}

func TestConformancePnPSubpath(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e-pnp-subpath")
	code, out := runM(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
}

func TestConformancePnPNested(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e-pnp-nested")
	code, out := runM(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
}

func TestConformancePnPMultiProjectIsolation(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	// Each project should only resolve its own .pnp.cjs dependencies.
	// Run project-a.
	projA := setupRuntimeFixture(t, "runtime-e2e-pnp-multi/project-a")
	codeA, outA := runM(t, projA, "app.mjs")
	if codeA != 0 {
		t.Fatalf("project-a exit=%d out=%s", codeA, outA)
	}
	// Run project-b in separate temp dir.
	projB := setupRuntimeFixture(t, "runtime-e2e-pnp-multi/project-b")
	codeB, outB := runM(t, projB, "app.mjs")
	if codeB != 0 {
		t.Fatalf("project-b exit=%d out=%s", codeB, outB)
	}
}

// --- Module format detection ---

func TestConformanceMTSModuleFormat(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// .mts files always execute as ESM regardless of package type.
	// The runtime-e2e fixture has no package.json (defaults to CJS for .js),
	// but .mts must still work as ESM.
	code, _ := runM(t, proj, "hello.mts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestConformanceCTSModuleFormat(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// .cts files always execute as CJS regardless of package type
	code, _ := runM(t, proj, "hello.cts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

// --- Default export handling ---

func TestConformanceDefaultExportTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// dep.cts is a CJS file that writes to output.txt via require('fs')
	code, _ := runM(t, proj, "dep.cts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "resolved-dep-cts" {
		t.Fatalf("expected 'resolved-dep-cts', got %q", got)
	}
}
