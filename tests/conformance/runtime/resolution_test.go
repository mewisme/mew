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

	// Write a script that imports via tsconfig path alias and verifies
	// the resolved module is the expected one.
	writeFile(t, proj, "verify-paths.mjs",
		`import { writeFileSync } from "node:fs";
try {
  // Dynamic import to exercise tsconfig path alias resolution.
  const mod = await import("@app/helpers");
  writeFileSync("output.txt", "paths-ok:" + (mod.helper === true ? "yes" : "no"));
} catch(e) {
  writeFileSync("output.txt", "paths-error:" + e.message);
}`)
	code, _ := runM(t, proj, "verify-paths.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "paths-ok:yes" {
		t.Fatalf("tsconfig paths resolution failed: %s", got)
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
	code, _ := runM(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "subpath:") {
		t.Fatalf("expected 'subpath:...', got %q", got)
	}
}

func TestConformancePnPNested(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e-pnp-nested")
	code, _ := runM(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "nested:") {
		t.Fatalf("expected 'nested:...', got %q", got)
	}
}

func TestConformancePnPMultiProjectIsolation(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")

	// Each project must only resolve its own .pnp.cjs dependencies.
	// Project A should resolve project-a-dep, project B project-b-dep.
	projA := setupRuntimeFixture(t, "runtime-e2e-pnp-multi/project-a")
	codeA, outA := runM(t, projA, "app.mjs")
	if codeA != 0 {
		t.Fatalf("project-a exit=%d out=%s", codeA, outA)
	}
	gotA := readOutput(t, projA)
	if !strings.Contains(gotA, "project-a") {
		t.Fatalf("project-a expected 'project-a' in output, got %q", gotA)
	}

	projB := setupRuntimeFixture(t, "runtime-e2e-pnp-multi/project-b")
	codeB, outB := runM(t, projB, "app.mjs")
	if codeB != 0 {
		t.Fatalf("project-b exit=%d out=%s", codeB, outB)
	}
	gotB := readOutput(t, projB)
	if !strings.Contains(gotB, "project-b") {
		t.Fatalf("project-b expected 'project-b' in output, got %q", gotB)
	}

	// Projects must NOT cross-resolve each other's dependencies.
	if strings.Contains(gotA, "project-b") {
		t.Fatalf("project-a leaked project-b dependency: %s", gotA)
	}
	if strings.Contains(gotB, "project-a") {
		t.Fatalf("project-b leaked project-a dependency: %s", gotB)
	}
}

// --- Module format detection ---

func TestConformanceMTSModuleFormat(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// .mts files always execute as ESM regardless of package type.
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

func TestConformanceCTSModuleFormat(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// .cts files always execute as CJS regardless of package type.
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

// --- Default export handling ---

func TestConformanceDefaultExportTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "dep.cts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "resolved-dep-cts" {
		t.Fatalf("expected 'resolved-dep-cts', got %q", got)
	}
}
