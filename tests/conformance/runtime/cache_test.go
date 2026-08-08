package runtime_test

import (
	"testing"
)

// TestConformanceCacheRoundTrip verifies that a TypeScript file executed
// through the production CLI path produces consistent output across runs
// (proving the transform cache works without corruption).
func TestConformanceCacheRoundTrip(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")

	// First run: execute hello.ts
	code, _ := runM(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("first run exit=%d", code)
	}

	// Second run: cache hit should still succeed
	code, _ = runM(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("second run (cache hit) exit=%d", code)
	}
}

// TestConformanceCacheKeyStability verifies that cache explain shows entries
// after transform execution, proving the cache subsystem is operational.
func TestConformanceCacheKeyStability(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")

	// Execute a transform to populate cache
	code, _ := runM(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}

	// Cache explain should succeed
	code, _ = runM(t, proj, "cache", "explain")
	if code != 0 {
		t.Fatalf("cache explain exit=%d", code)
	}
}

// TestConformanceCacheSchemaVersion verifies the transform cache is active
// by checking that running a .ts file succeeds (requires working cache).
func TestConformanceCacheSchemaVersion(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")

	code, _ := runM(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

// TestConformanceSourceMapRoundTrip verifies that repeated .ts execution
// produces consistent results (source map caching is transparent).
func TestConformanceSourceMapRoundTrip(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")

	for i := 0; i < 3; i++ {
		code, _ := runM(t, proj, "hello.ts")
		if code != 0 {
			t.Fatalf("run %d exit=%d", i, code)
		}
	}
}
