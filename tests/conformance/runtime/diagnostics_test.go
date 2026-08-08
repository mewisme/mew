package runtime_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Watch conformance ---

func TestConformanceWatchStart(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Launch watch via black-box binary. Watch produces observable output
	// when the initial build completes.
	code, out, _ := runMBinary(t, ctx, proj, "watch", "hello.ts")

	// Watch runs until cancelled; context timeout is the expected
	// termination. Killed-by-signal (code -1) is acceptable, unexpected
	// non-zero exit is a crash.
	if code != 0 && code != -1 {
		t.Fatalf("watch crashed with exit=%d, output:\n%s", code, out)
	}

	// The initial generation must have completed before cancellation.
	if !strings.Contains(out, "hello") && !strings.Contains(out, "ready") && !strings.Contains(out, "started") {
		t.Fatalf("watch did not show initial generation, output:\n%s", out)
	}
}

func TestConformanceWatchShutdown(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	code, out, _ := runMBinary(t, ctx, proj, "watch", "hello.js")

	if code != 0 && code != -1 {
		t.Fatalf("watch shutdown crashed with exit=%d, output:\n%s", code, out)
	}

	// Must show evidence of starting before cancellation.
	if !strings.Contains(out, "hello") && !strings.Contains(out, "ready") && !strings.Contains(out, "started") && !strings.Contains(out, "watch") {
		t.Fatalf("watch did not show startup evidence, output:\n%s", out)
	}
}

// --- Runtime trace conformance ---

func TestConformanceRuntimeTrace(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")

	ctx, cancel := deadline(t)
	defer cancel()

	// Execute trace via black-box binary with --json for machine-readable
	// NDJSON events on stdout.
	code, out, _ := runMBinary(t, ctx, proj, "runtime", "trace", "--json", "hello.ts")
	if code != 0 {
		t.Fatalf("trace exit=%d, output:\n%s", code, out)
	}

	// Trace output must contain at least one valid JSON line with a
	// recognizable event structure.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	jsonEvents := 0
	requiredCategories := map[string]bool{
		"plan": false, "start": false, "launch": false, "cleanup": false,
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("malformed trace event: %v\nline: %s", err, line)
		}
		jsonEvents++

		for _, key := range []string{"cat", "category", "phase", "type", "event"} {
			if v, ok := ev[key]; ok {
				if s, ok := v.(string); ok {
					for cat := range requiredCategories {
						if strings.Contains(strings.ToLower(s), cat) {
							requiredCategories[cat] = true
						}
					}
				}
			}
		}
	}
	if jsonEvents == 0 {
		t.Fatalf("no JSON trace events in output:\n%s", out)
	}

	for cat, found := range requiredCategories {
		if !found {
			t.Errorf("trace missing required category %q", cat)
		}
	}

	// No private credential markers in trace output.
	requireNoOutput(t, out, "MEW_TRANSFORM_TOKEN", "MEW_TRANSFORM_ENDPOINT")
}

// --- Doctor runtime conformance ---

func TestConformanceDoctorRuntime(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")

	ctx, cancel := deadline(t)
	defer cancel()

	// Run doctor runtime via black-box binary.
	code, out, _ := runMBinary(t, ctx, proj, "doctor", "runtime")

	// Doctor must produce recognizable check output.
	if !strings.Contains(out, "check") && !strings.Contains(out, "health") && !strings.Contains(out, "runtime") {
		t.Fatalf("doctor output has no recognizable check results, exit=%d:\n%s", code, out)
	}

	// Required check domains for a working runtime.
	requiredChecks := map[string]bool{
		"node": false, "transform": false, "cache": false,
	}
	for check := range requiredChecks {
		if strings.Contains(strings.ToLower(out), check) {
			requiredChecks[check] = true
		}
	}
	for check, found := range requiredChecks {
		if !found {
			t.Errorf("doctor output missing check domain %q", check)
		}
	}
}

// --- Cache explain conformance ---

func TestConformanceCacheExplain(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")

	// Execute a transform to populate cache (in-process is fine since
	// we only need to create cache state, not certify the execution).
	code, _ := runM(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("priming run exit=%d", code)
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

	// Output must reference cache information.
	hasCacheContent := strings.Contains(strings.ToLower(out), "cache") ||
		strings.Contains(out, "key") ||
		strings.Contains(out, "entry") ||
		strings.Contains(out, "hit") ||
		strings.Contains(out, "miss") ||
		strings.Contains(out, "size") ||
		strings.Contains(out, "count")
	if !hasCacheContent {
		t.Fatalf("cache explain output does not contain cache information:\n%s", out)
	}
}

// --- Support bundle conformance ---

func TestConformanceSupportBundle(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")

	ctx, cancel := deadline(t)
	defer cancel()

	code, out, _ := runMBinary(t, ctx, proj, "runtime", "support-bundle")
	if code != 0 {
		t.Fatalf("support-bundle exit=%d, output:\n%s", code, out)
	}

	// Support bundle must produce a .tgz artifact.
	tgzPath, err := waitForFile(t, proj, "*.tgz", 5*time.Second)
	if err != nil {
		t.Fatalf("support-bundle produced no .tgz artifact in %s. output:\n%s", proj, out)
	}
	defer func() { _ = os.Remove(tgzPath) }()

	fi, err := os.Stat(tgzPath)
	if err != nil {
		t.Fatalf("cannot stat bundle %s: %v", tgzPath, err)
	}
	if fi.Size() == 0 {
		t.Fatalf("support bundle is empty: %s", tgzPath)
	}

	// Must be at least a minimal valid gzip/tar (20 bytes is conservative).
	if fi.Size() < 20 {
		t.Fatalf("support bundle too small to be valid: %d bytes", fi.Size())
	}

	// Cleanup any additional generated bundles.
	matches, _ := filepath.Glob(filepath.Join(proj, "*.tgz"))
	for _, m := range matches {
		_ = os.Remove(m)
	}
}

// --- Inspector conformance ---

func TestConformanceInspector(t *testing.T) {
	skipWithoutNode(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// --inspect on loopback must execute the script and produce
	// inspector-related output.
	code, out, _ := runMBinary(t, ctx, proj, "--inspect", "hello.ts")
	if code != 0 {
		t.Fatalf("inspect exit=%d, output:\n%s", code, out)
	}

	// Inspector activation must produce debugger evidence in output.
	hasInspector := strings.Contains(out, "inspector") ||
		strings.Contains(out, "debugger") ||
		strings.Contains(out, "ws://") ||
		strings.Contains(out, "DevTools") ||
		strings.Contains(out, "Debugger") ||
		strings.Contains(out, "9229")
	if !hasInspector {
		t.Fatalf("inspector did not produce debugger evidence, output:\n%s", out)
	}

	// The target script must still execute.
	if !strings.Contains(out, "hello") {
		t.Fatalf("inspector ran but script did not execute, output:\n%s", out)
	}
}

// --- Native Node opt-out conformance ---

func TestConformanceOptOutNode(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")

	ctx, cancel := deadline(t)
	defer cancel()

	// --node runs plain Node without Mew preload injection.
	// 1. JS file must execute correctly via the binary.
	code, out, _ := runMBinary(t, ctx, proj, "--node", "hello.js")
	if code != 0 {
		t.Fatalf("--node JS exit=%d, output:\n%s", code, out)
	}

	// 2. Mew transform credentials must be absent from --node execution.
	//    Use runM (in-process) because we verify via output.txt.
	writeFile(t, proj, "optout-env.js",
		`const fs = require("node:fs");
fs.writeFileSync("output.txt",
  "has_mew=" + (typeof process.env.MEW_TRANSFORM_TOKEN !== "undefined" ? "yes" : "no"));
`)
	code, _ = runM(t, proj, "--node", "optout-env.js")
	if code != 0 {
		t.Fatalf("optout-env.js exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "has_mew=no" {
		t.Fatalf("--node leaked Mew credentials: %s", got)
	}

	// 3. Mew-specific runtime augmentation (localStorage) must be
	//    absent under --node. Native Node has no localStorage global.
	writeFile(t, proj, "optout-storage.js",
		`const fs = require("node:fs");
try {
  const has = typeof localStorage !== "undefined";
  fs.writeFileSync("output.txt", "has_localStorage=" + (has ? "yes" : "no"));
} catch(e) {
  fs.writeFileSync("output.txt", "error:" + e.message);
}`)
	code, _ = runM(t, proj, "--node", "optout-storage.js")
	if code != 0 {
		t.Fatalf("optout-storage.js exit=%d", code)
	}
	got = readOutput(t, proj)
	if got == "has_localStorage=yes" {
		t.Fatalf("--node leaked localStorage augmentation: %s", got)
	}
}

// --- Application argv preservation ---

func TestConformanceAppArgv(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")

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
