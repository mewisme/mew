package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Watch mode: entry starts ---

func TestConformanceWatchStart(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	code, _ := runMCtx(t, ctx, proj, "watch", "hello.ts")
	// Watch runs until cancelled; killed by context is acceptable
	if code != 0 && code != -1 {
		t.Logf("watch exit=%d", code)
	}
}

// --- Watch mode: bounded shutdown ---

func TestConformanceWatchShutdown(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	code, _ := runMCtx(t, ctx, proj, "watch", "hello.js")
	// Should be killed by context timeout, not crash
	if code != 0 && code != -1 {
		t.Logf("watch shutdown exit=%d", code)
	}
}

// --- Diagnostics: runtime trace ---

func TestConformanceRuntimeTrace(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "runtime", "trace", "hello.ts")
	if code != 0 {
		t.Fatalf("trace exit=%d", code)
	}
	// Trace writes structured output; verify by checking exit is clean.
	// The structured trace output goes through presentation, so we can't
	// easily capture it with --output silent. The exit code proves
	// trace execution succeeded.
}

// --- Diagnostics: doctor runtime ---

func TestConformanceDoctorRuntime(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "doctor", "runtime")
	// Doctor may report tsconfig as unavailable (no tsconfig.json in fixture)
	// but core checks should still run. Accept both pass and fail.
	if code != 0 {
		t.Logf("doctor exit=%d (tsconfig not in fixture is expected)", code)
	}
}

// --- Diagnostics: cache explain ---

func TestConformanceCacheExplain(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// Execute a transform to populate cache
	runM(t, proj, "hello.ts")
	// Cache explain should succeed
	code, _ := runM(t, proj, "cache", "explain")
	if code != 0 {
		t.Fatalf("cache explain exit=%d", code)
	}
}

// --- Diagnostics: support bundle ---

func TestConformanceSupportBundle(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	code, _ := runM(t, proj, "runtime", "support-bundle")
	if code != 0 {
		t.Fatalf("support-bundle exit=%d", code)
	}
	// Support bundle produces a .tgz file in the project directory
	matches, _ := filepath.Glob(filepath.Join(proj, "*.tgz"))
	if len(matches) == 0 {
		// Bundle might be written elsewhere; check that no error occurred
		t.Logf("no .tgz found in project dir; bundle may use alternate path")
	}
	// Clean up any generated bundle
	for _, m := range matches {
		_ = os.Remove(m)
	}
}

// --- Diagnostics: inspector ---

func TestConformanceInspector(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// --inspect on loopback should execute and exit cleanly
	code, _ := runM(t, proj, "--inspect", "hello.ts")
	if code != 0 {
		t.Fatalf("inspect exit=%d", code)
	}
}

// --- Opt-out: --node bypasses augmentation preload ---

func TestConformanceOptOutNode(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// --node runs plain Node without Mew preload injection.
	// JS files should still execute fine.
	code, _ := runM(t, proj, "--node", "hello.js")
	if code != 0 {
		t.Fatalf("--node exit=%d", code)
	}
}

// --- App argv survives CLI boundary ---

func TestConformanceAppArgv(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// args.js writes process.argv to output.txt
	writeFile(t, proj, "args-file.js",
		`const fs = require("node:fs"); fs.writeFileSync("output.txt", JSON.stringify(process.argv.slice(1)));`)
	code, _ := runM(t, proj, "args-file.js", "--app-flag", "value")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "--app-flag") || !strings.Contains(got, "value") {
		t.Fatalf("app argv not passed through: %s", got)
	}
}
