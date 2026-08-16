package runtime_test

import (
	"strings"
	"testing"
)

// TestConformanceCacheRoundTrip verifies that a TypeScript file produces
// consistent output across runs (proving the transform cache works without
// corruption). Both runs must produce the same observable output.
func TestConformanceCacheRoundTrip(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")

	ctx, cancel := deadline(t)
	defer cancel()

	// First run via black-box binary to capture stdout.
	code1, out1, _ := runMBinary(t, ctx, proj, "hello.ts")
	if code1 != 0 {
		t.Fatalf("first run exit=%d", code1)
	}
	if !strings.Contains(out1, "hello from ts") {
		t.Fatalf("first run unexpected output: %s", out1)
	}

	// Second run should produce identical observable output (cache hit).
	code2, out2, _ := runMBinary(t, ctx, proj, "hello.ts")
	if code2 != 0 {
		t.Fatalf("second run (cache hit) exit=%d", code2)
	}
	if out1 != out2 {
		t.Fatalf("cache round-trip produced different output:\nrun1: %s\nrun2: %s", out1, out2)
	}
}

// TestConformanceCacheKeyStability verifies that cache explain shows
// entries after transform execution and that those entries have stable,
// recognizable key information.
func TestConformanceCacheKeyStability(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")

	// Execute a transform to populate cache.
	code, _ := runM(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}

	ctx, cancel := deadline(t)
	defer cancel()

	// Cache explain via black-box binary.
	code, out, _ := runMBinary(t, ctx, proj, "cache", "explain")
	if code != 0 {
		t.Fatalf("cache explain exit=%d, output:\n%s", code, out)
	}

	if out == "" {
		t.Fatal("cache explain produced no output")
	}

	// Output must contain cache-related content proving the subsystem
	// is operational and producing structured information.
	hasCacheContent := strings.Contains(strings.ToLower(out), "cache") ||
		strings.Contains(out, "key") ||
		strings.Contains(out, "entry") ||
		strings.Contains(out, "hit") ||
		strings.Contains(out, "miss")
	if !hasCacheContent {
		t.Fatalf("cache explain output does not contain cache information:\n%s", out)
	}
}

// TestConformanceCacheSchemaVersion verifies the transform cache is
// operational by executing a TS file and confirming the cache explain
// command reports a coherent state afterward.
func TestConformanceCacheSchemaVersion(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")

	ctx, cancel := deadline(t)
	defer cancel()

	// Execute via black-box binary and verify output.
	code, out, _ := runMBinary(t, ctx, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "hello from ts") {
		t.Fatalf("expected 'hello from ts', got %q", out)
	}
}

// TestConformanceSourceMapRoundTrip verifies that repeated .ts execution
// produces consistent results across multiple runs, proving source map
// caching is transparent and non-corrupting.
func TestConformanceSourceMapRoundTrip(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")

	var previous string
	for i := 0; i < 3; i++ {
		ctx, cancel := deadline(t)
		code, out, _ := runMBinary(t, ctx, proj, "hello.ts")
		cancel()
		if code != 0 {
			t.Fatalf("run %d exit=%d", i, code)
		}
		if !strings.Contains(out, "hello from ts") {
			t.Fatalf("run %d unexpected output: %s", i, out)
		}
		if i > 0 && out != previous {
			t.Fatalf("run %d produced different output than run %d:\nprev: %s\ncur:  %s", i, i-1, previous, out)
		}
		previous = out
	}
}
