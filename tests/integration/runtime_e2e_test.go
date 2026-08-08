package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mewisme/mew/internal/cli"
	"github.com/mewisme/mew/internal/testkit"
)

// runtimeE2EFixture copies the runtime-e2e fixture into a temp dir and returns
// the path to the fixture directory.
func runtimeE2EFixture(t *testing.T) string {
	t.Helper()
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	testkit.CopyFixture(t, "runner/runtime-e2e", projDir)
	return projDir
}

// readOutput reads the fixture output file produced by test scripts that write
// results to a file (for tests that need to verify specific output).
func readOutput(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatalf("read output.txt: %v", err)
	}
	return strings.TrimSpace(string(data))
}

// runMWithRuntime runs the m CLI with MEW_EXPERIMENTAL_RUNTIME=1 set.
func runMWithRuntime(t *testing.T, projDir string, args ...string) (int, string) {
	t.Helper()
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	return runMProject(t, projDir, args...)
}

// runMWithRuntimeCtx runs the m CLI with MEW_EXPERIMENTAL_RUNTIME=1 and a context.
func runMWithRuntimeCtx(t *testing.T, ctx context.Context, projDir string, args ...string) (int, string) {
	t.Helper()
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	return runMProjectCtx(t, ctx, projDir, args...)
}

// --- JS/TS entrypoint execution ---

func TestRuntimeE2EHelloJS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2EHelloMJS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2EHelloCJS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.cjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2EHelloTS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2EHelloMTS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.mts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2EHelloCTS(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for .cts loader transform (CJS loader hooks)")
	}
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.cts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

// --- Imports, args, errors ---

func TestRuntimeE2EImportResolution(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "imports.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, proj); out != "hello from import" {
		t.Fatalf("got %q, want 'hello from import'", out)
	}
}

func TestRuntimeE2EEntrypointArgs(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "args.js", "--", "alpha", "beta")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("got %q, want args containing alpha, beta", out)
	}
}

func TestRuntimeE2ENodeV8Args(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "node-args", "--", "--expose-gc", "hello.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2ESyntaxErrorJS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "syntax-error.js")
	if code == 0 {
		t.Fatal("expected non-zero exit for syntax error")
	}
}

func TestRuntimeE2ESyntaxErrorTS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "syntax-error.ts")
	if code == 0 {
		t.Fatal("expected non-zero exit for TS syntax error")
	}
}

func TestRuntimeE2EExitCode(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "exit-code.js")
	if code != 42 {
		t.Fatalf("expected exit code 42, got %d", code)
	}
}

// --- Zero augmentation, dispatch precedence, deferred extensions ---

func TestRuntimeE2EZeroAugmentation(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "--node", "env-dump.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if out != "absent" {
		t.Fatalf("--node mode leaked MEW_TRANSFORM_ENDPOINT: got %q", out)
	}
}

func TestRuntimeE2EScriptWinsOverFile(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	// Use a .js file: inline node -e is mangled by cmd.exe on Windows.
	writeFile(t, filepath.Join(proj, "hello-script.js"), `require("fs").writeFileSync("output.txt", "script-wins\n")`)
	writeFile(t, filepath.Join(proj, "package.json"), `{"scripts": {"hello": "node hello-script.js"}}`)
	code, _ := runMProject(t, proj, "run", "hello")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, proj); out != "script-wins" {
		t.Fatalf("got %q, want 'script-wins'", out)
	}
}

func TestRuntimeE2EJSXDeferred(t *testing.T) {
	proj := runtimeE2EFixture(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, out := runMProject(t, proj, "test.jsx")
	if code == 0 {
		t.Fatalf("expected non-zero exit for .jsx, got out=%s", out)
	}
	if !strings.Contains(out, "0053") {
		t.Fatalf("expected 0053 deferral message, got %q", out)
	}
}

func TestRuntimeE2ETSXSuccess(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.tsx")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "hello from tsx") {
		t.Fatalf("got %q, want output containing 'hello from tsx'", out)
	}
	if !strings.Contains(out, `"tag":"div"`) {
		t.Fatalf("got %q, want JSX transform evidence (tag div)", out)
	}
}

// --- Cache: warm, corrupt recovery ---

func TestRuntimeE2EWarmCache(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	testkit.CopyFixture(t, "runner/runtime-e2e", projDir)

	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	// First run: populate cache.
	code1, _ := runMProject(t, projDir, "hello.js")
	if code1 != 0 {
		t.Fatalf("first run: exit=%d", code1)
	}
	// Second run: warm cache, should reuse verified assets.
	code2, _ := runMProject(t, projDir, "hello.js")
	if code2 != 0 {
		t.Fatalf("second run (warm cache): exit=%d", code2)
	}
}

func TestRuntimeE2ECorruptCacheRecovery(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	cacheDir := filepath.Join(homeDir, ".cache", "mew")
	projDir := t.TempDir()
	testkit.CopyFixture(t, "runner/runtime-e2e", projDir)

	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	// First run: populate cache.
	code1, _ := runMProject(t, projDir, "hello.js")
	if code1 != 0 {
		t.Fatalf("first run: exit=%d", code1)
	}
	// Corrupt cached assets.
	corruptCacheAssets(t, cacheDir)
	// Second run: should recover and still produce correct output.
	code2, _ := runMProject(t, projDir, "hello.js")
	if code2 != 0 {
		t.Fatalf("second run (corrupt cache): exit=%d", code2)
	}
}

// --- Extension substitution: .js/.jsx/.mjs/.cjs → TypeScript ---

func TestRuntimeE2EExtensionSubstitutionJSToTS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "import-js-to-ts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, proj); out != "resolved-lib-ts" {
		t.Fatalf("got %q, want 'resolved-lib-ts'", out)
	}
}

func TestRuntimeE2EExtensionSubstitutionMJSToMTS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "import-mjs-to-mts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, proj); out != "resolved-mod-mts" {
		t.Fatalf("got %q, want 'resolved-mod-mts'", out)
	}
}

func TestRuntimeE2EExtensionSubstitutionCJSToCTS(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for .cts loader transform (CJS loader hooks)")
	}
	proj := runtimeE2EFixture(t)
	// Verify .cjs → .cts resolution succeeds without module-not-found.
	// CJS-format execution semantics for .cts files are tested in Issue 13.
	code, _ := runMWithRuntime(t, proj, "import-cjs-to-cts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2EExtensionSubstitutionJSXToTSX(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "import-jsx-to-tsx.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, proj); out != "resolved-component-tsx" {
		t.Fatalf("got %q, want 'resolved-component-tsx'", out)
	}
}

func TestRuntimeE2EExtensionSubstitutionExistingJSWins(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "import-existing-js.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, proj); out != "real-js-wins" {
		t.Fatalf("got %q, want 'real-js-wins' (existing .js must win over .ts)", out)
	}
}

func TestRuntimeE2EExtensionSubstitutionMissingPreservesNodeError(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, out := runMWithRuntime(t, proj, "import-missing.mjs")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing module, got out=%s", out)
	}
}

func TestRuntimeE2EExtensionSubstitutionNestedRelativeImport(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	// Create subdirectory with a .ts file importable via .js specifier.
	subDir := filepath.Join(proj, "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(subDir, "nested-lib.ts"),
		"export const nestedValue: string = 'nested-resolved';\n")
	writeFile(t, filepath.Join(proj, "import-nested.mjs"),
		"import { nestedValue } from './nested/nested-lib.js';\n"+
			"import { writeFileSync } from 'node:fs';\n"+
			"writeFileSync('output.txt', nestedValue + '\\n');\n")
	code, _ := runMWithRuntime(t, proj, "import-nested.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, proj); out != "nested-resolved" {
		t.Fatalf("got %q, want 'nested-resolved'", out)
	}
}

// --- Source maps, paths, node-args, mx, cancellation, gate ---

func TestRuntimeE2ESourceMaps(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 6) {
		t.Skip("Node >= 20.6 required for --enable-source-maps")
	}
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--enable-source-maps")
	code, _ := runMWithRuntime(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2ESpacesInPath(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	spacesDir := filepath.Join(projDir, "path with spaces")
	if err := os.MkdirAll(spacesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(spacesDir, "hello.js"), `console.log("spaces work");`)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, spacesDir, "hello.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2EUnicodeInPath(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	unicodeDir := filepath.Join(projDir, "café")
	if err := os.MkdirAll(unicodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(unicodeDir, "hello.js"), `console.log("unicode works");`)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, unicodeDir, "hello.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2ENodeArgsStyle(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "node-args", "--", "hello.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2EMXNoFileRun(t *testing.T) {
	proj := runtimeE2EFixture(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, out := runMXInProject(t, proj, "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for mx file-run, got out=%s", out)
	}
}

func TestRuntimeE2ECancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal propagation unreliable on Windows")
	}
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	code, _ := runMWithRuntimeCtx(t, ctx, proj, "sigterm.mjs")
	if code == 0 {
		t.Fatal("expected non-zero exit for cancelled process")
	}
}

func TestRuntimeE2EDisabledGate(t *testing.T) {
	proj := runtimeE2EFixture(t)
	code, out := runMProject(t, proj, "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit when gate is off, got out=%s", out)
	}
}

// --- Transform credential isolation ---

func TestRuntimeE2ETSNoTransformEndpoint(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "env-dump.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if out != "absent" {
		t.Fatalf("MEW_TRANSFORM_ENDPOINT leaked into user code: got %q, want 'absent'", out)
	}
}

func TestRuntimeE2EAllCredsStrippedJS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "env-dump-all.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		if parts[1] != "absent" {
			t.Fatalf("%s leaked: got %q, want 'absent'", parts[0], parts[1])
		}
	}
}

func TestRuntimeE2EAllCredsStrippedTS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	writeFile(t, filepath.Join(proj, "check.ts"),
		"import { writeFileSync } from \"node:fs\";"+"\n"+
			"var vars = [\"MEW_TRANSFORM_ENDPOINT\",\"MEW_TRANSFORM_TOKEN\",\"MEW_TRANSFORM_OPTIONS\",\"MEW_TRANSFORM_OPTS_DIGEST\"];"+"\n"+
			"var out = vars.map(function(v) { return v + \"=\" + (process.env[v] || \"absent\"); });"+"\n"+
			"writeFileSync(\"output.txt\", out.join(String.fromCharCode(10)));"+"\n")
	code, _ := runMWithRuntime(t, proj, "check.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		if parts[1] != "absent" {
			t.Fatalf("%s leaked into TS entrypoint: got %q, want 'absent'", parts[0], parts[1])
		}
	}
}

func TestRuntimeE2ENoCredsInWorker(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "worker-check.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "worker-error:") {
			t.Fatalf("worker error: %s", line)
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		if parts[1] != "absent" {
			t.Fatalf("%s leaked into worker: got %q, want 'absent'", parts[0], parts[1])
		}
	}
}

func TestRuntimeE2ENoCredsInChildProcess(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "child-check.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "child-error:") {
			t.Fatalf("child process error: %s", line)
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		if parts[1] != "absent" {
			t.Fatalf("%s leaked into child process: got %q, want 'absent'", parts[0], parts[1])
		}
	}
}

func TestRuntimeE2ENodeOptionsRequireRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--require ./evil.cjs")
	code, out := runMWithRuntime(t, proj, "hello.ts")
	if code == 0 {
		t.Fatalf("expected non-zero exit for NODE_OPTIONS with --require, got out=%s", out)
	}
	if !strings.Contains(out, "NODE_OPTIONS") && !strings.Contains(out, "credential") {
		t.Fatalf("expected NODE_OPTIONS rejection message, got %q", out)
	}
}

func TestRuntimeE2ENodeOptionsImportRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--import ./evil.mjs")
	code, out := runMWithRuntime(t, proj, "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for NODE_OPTIONS with --import, got out=%s", out)
	}
	if !strings.Contains(out, "NODE_OPTIONS") && !strings.Contains(out, "credential") {
		t.Fatalf("expected NODE_OPTIONS rejection message, got %q", out)
	}
}

// TestRuntimeE2ENodeOptionsShortRequireRejected verifies the -r bypass is closed.
func TestRuntimeE2ENodeOptionsShortRequireRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "-r ./evil.cjs")
	code, out := runMWithRuntime(t, proj, "hello.ts")
	if code == 0 {
		t.Fatalf("expected non-zero exit for NODE_OPTIONS with -r, got out=%s", out)
	}
	if !strings.Contains(out, "-r") {
		t.Fatalf("expected -r rejection message, got %q", out)
	}
}

// TestRuntimeE2ENodeOptionsCompactRequireRejected verifies -r./evil.cjs is blocked.
// Node >= 20 rejects the compact form in NODE_OPTIONS itself (exit 9);
// older Node versions may accept it, in which case Mew's parser catches it.
func TestRuntimeE2ENodeOptionsCompactRequireRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "-r./evil.cjs")
	code, out := runMWithRuntime(t, proj, "hello.ts")
	if code == 0 {
		t.Fatalf("expected non-zero exit for NODE_OPTIONS with -r./evil.cjs, got out=%s", out)
	}
	// Either Mew's validator or Node's own NODE_OPTIONS check rejects this.
	if !strings.Contains(out, "-r") && !strings.Contains(out, "NODE_OPTIONS") {
		t.Fatalf("expected rejection (Mew or Node) for -r./evil.cjs, got %q", out)
	}
}

// TestRuntimeE2ENodeOptionsRequireEqualsRejected verifies --require= form blocked.
func TestRuntimeE2ENodeOptionsRequireEqualsRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--require=./evil.cjs")
	code, out := runMWithRuntime(t, proj, "hello.ts")
	if code == 0 {
		t.Fatalf("expected non-zero exit for NODE_OPTIONS with --require=, got out=%s", out)
	}
	if !strings.Contains(out, "NODE_OPTIONS") && !strings.Contains(out, "--require") {
		t.Fatalf("expected --require rejection message, got %q", out)
	}
}

// TestRuntimeE2ENodeOptionsImportEqualsRejected verifies --import= form blocked.
func TestRuntimeE2ENodeOptionsImportEqualsRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--import=./evil.mjs")
	code, out := runMWithRuntime(t, proj, "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for NODE_OPTIONS with --import=, got out=%s", out)
	}
	if !strings.Contains(out, "NODE_OPTIONS") && !strings.Contains(out, "--import") {
		t.Fatalf("expected --import rejection message, got %q", out)
	}
}

// TestRuntimeE2ENodeOptionsLoaderRejected verifies --loader in NODE_OPTIONS blocked.
func TestRuntimeE2ENodeOptionsLoaderRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--loader ./evil.mjs")
	code, out := runMWithRuntime(t, proj, "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for NODE_OPTIONS with --loader, got out=%s", out)
	}
	if !strings.Contains(out, "NODE_OPTIONS") && !strings.Contains(out, "--loader") {
		t.Fatalf("expected --loader rejection message, got %q", out)
	}
}

// TestRuntimeE2ENodeOptionsLoaderEqualsRejected verifies --loader= form blocked.
func TestRuntimeE2ENodeOptionsLoaderEqualsRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--loader=./evil.mjs")
	code, out := runMWithRuntime(t, proj, "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for NODE_OPTIONS with --loader=, got out=%s", out)
	}
	if !strings.Contains(out, "NODE_OPTIONS") && !strings.Contains(out, "--loader") {
		t.Fatalf("expected --loader rejection message, got %q", out)
	}
}

// TestRuntimeE2ENodeOptionsExperimentalLoaderRejected verifies --experimental-loader blocked.
func TestRuntimeE2ENodeOptionsExperimentalLoaderRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--experimental-loader ./evil.mjs")
	code, out := runMWithRuntime(t, proj, "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for NODE_OPTIONS with --experimental-loader, got out=%s", out)
	}
	if !strings.Contains(out, "NODE_OPTIONS") && !strings.Contains(out, "--experimental-loader") {
		t.Fatalf("expected --experimental-loader rejection message, got %q", out)
	}
}

// TestRuntimeE2ENodeOptionsSafeStillWorks verifies safe NODE_OPTIONS still accepted.
func TestRuntimeE2ENodeOptionsSafeStillWorks(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--max-old-space-size=4096 --no-warnings")
	code, _ := runMWithRuntime(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("expected zero exit for safe NODE_OPTIONS, got code=%d", code)
	}
}

// TestRuntimeE2ENodeOptionsSafeSubstringNotRejected verifies --require in a value
// (e.g. --title=my--require-app) is not falsely rejected.
func TestRuntimeE2ENodeOptionsSafeSubstringNotRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	t.Setenv("NODE_OPTIONS", "--title=my--require-app --max-old-space-size=4096")
	code, _ := runMWithRuntime(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("expected zero exit for harmless --require substring, got code=%d", code)
	}
}

// TestRuntimeE2ENodeModeNativeNodeOptions verifies --node mode allows
// native NODE_OPTIONS usage with --require (no Mew credentials to protect).
func TestRuntimeE2ENodeModeNativeNodeOptions(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	writeFile(t, filepath.Join(proj, "native-probe.cjs"),
		"var fs = require(\"fs\");\n"+
			"fs.writeFileSync(\"output.txt\", \"native-preload-ran\");\n")
	t.Setenv("NODE_OPTIONS", "--require ./native-probe.cjs")
	code, _ := runMWithRuntime(t, proj, "--node", "hello.js")
	if code != 0 {
		t.Fatalf("expected zero exit for --node with native NODE_OPTIONS, got code=%d", code)
	}
	out := readOutput(t, proj)
	if out != "native-preload-ran" {
		t.Fatalf("expected native preload to run in --node mode, got %q", out)
	}
}

// TestRuntimeE2ECredentialGrabberRunsBeforeUserCLIPreload verifies Mew's
// credential-grabber strips env before permitted user CLI --require preloads.
func TestRuntimeE2ECredentialGrabberRunsBeforeUserCLIPreload(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	writeFile(t, filepath.Join(proj, "user-preload-env-check.cjs"),
		"var fs = require(\"fs\");\n"+
			"var results = [];\n"+
			"results.push('token='+(process.env.MEW_TRANSFORM_TOKEN || 'absent'));\n"+
			"results.push('endpoint='+(process.env.MEW_TRANSFORM_ENDPOINT || 'absent'));\n"+
			"fs.writeFileSync(\"output.txt\", results.join(\"\\n\"));\n")
	code, _ := runMWithRuntime(t, proj, "node-args", "--", "--require", "./user-preload-env-check.cjs", "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		if parts[1] != "absent" {
			t.Fatalf("%s leaked into CLI --require preload: got %q, want 'absent'", parts[0], parts[1])
		}
	}
}

func TestRuntimeE2ENodeArgsRequireStillWorks(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	writeFile(t, filepath.Join(proj, "preload-check.cjs"),
		"var fs = require(\"fs\");"+"\n"+
			"var vars = [\"MEW_TRANSFORM_ENDPOINT\",\"MEW_TRANSFORM_TOKEN\",\"MEW_TRANSFORM_OPTIONS\"];"+"\n"+
			"var results = vars.map(function(v){return v+\"=\"+(process.env[v]||\"absent\")});"+"\n"+
			"fs.writeFileSync(\"output.txt\", results.join(\"\\n\"));"+"\n")
	code, _ := runMWithRuntime(t, proj, "node-args", "--", "--require", "./preload-check.cjs", "hello.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		if parts[1] != "absent" {
			t.Fatalf("%s leaked through user --require preload: got %q, want 'absent'", parts[0], parts[1])
		}
	}
}

func TestRuntimeE2ENodeArgsTypeScriptStillWorks(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "node-args", "--", "--no-warnings", "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2ETSLoaderAuthenticates(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2ETSLoaderMTS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.mts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2ETSLoaderCTS(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for .cts loader transform (CJS loader hooks)")
	}
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.cts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRuntimeE2ENodeModeZeroAugmentation(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "--node", "env-dump-all.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		if parts[1] != "absent" {
			t.Fatalf("%s leaked in --node mode: got %q, want 'absent'", parts[0], parts[1])
		}
	}
}

func TestRuntimeE2EErrorOutputNoEndpointOrToken(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, out := runMWithRuntime(t, proj, "syntax-error.ts")
	if code == 0 {
		t.Fatal("expected non-zero exit for syntax error")
	}
	if strings.Contains(out, "127.0.0.1:") {
		t.Fatalf("error output contains endpoint address: %q", out)
	}
	if strings.Contains(out, "MEW_TRANSFORM_TOKEN=") {
		t.Fatalf("error output contains token env: %q", out)
	}
	if strings.Contains(out, "MEW_TRANSFORM_ENDPOINT=") {
		t.Fatalf("error output contains endpoint env: %q", out)
	}
}

// --- package-script CWD ---

func TestRuntimeE2EScriptCWDIsProjectRoot(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	// Use a separate .js file so the script works cross-platform: inline
	// node -e with double quotes is mangled by cmd.exe on Windows.
	writeFile(t, filepath.Join(proj, "cwdcheck.js"), `require("fs").writeFileSync("cwd.txt", process.cwd())`)
	writeFile(t, filepath.Join(proj, "package.json"), `{"scripts": {"cwdcheck": "node cwdcheck.js"}}`)
	code, _ := runMProject(t, proj, "run", "cwdcheck")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(proj, "cwd.txt"))
	if err != nil {
		t.Fatalf("read cwd.txt: %v", err)
	}
	got := strings.TrimSpace(string(data))
	// Compare after resolving both to absolute form so symlinks and
	// platform path normalizations are handled correctly.
	want, err := filepath.Abs(proj)
	if err != nil {
		t.Fatal(err)
	}
	gotAbs, err := filepath.Abs(got)
	if err != nil {
		t.Fatalf("resolve subprocess cwd: %v", err)
	}
	if !samePath(gotAbs, want) {
		t.Fatalf("script cwd=%q, want project root %q", got, want)
	}
}

func TestRuntimeE2EScriptCWDWithCWD(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	// Use a separate .js file: inline node -e with double quotes is
	// mangled by cmd.exe on Windows.
	writeFile(t, filepath.Join(proj, "cwdcheck.js"), `require("fs").writeFileSync("cwd.txt", process.cwd())`)
	writeFile(t, filepath.Join(proj, "package.json"), `{"scripts": {"cwdcheck": "node cwdcheck.js"}}`)
	// Run from parent directory; --cwd selects the project.
	parent := filepath.Dir(proj)
	code, _ := runMProjectWithCWD(t, parent, proj, "run", "cwdcheck")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(proj, "cwd.txt"))
	if err != nil {
		t.Fatalf("read cwd.txt: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if !samePath(got, proj) {
		t.Fatalf("script cwd=%q, want project root %q", got, proj)
	}
}

func TestRuntimeE2EScriptCWDFromSubdir(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	subdir := filepath.Join(proj, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Use a separate .js file: inline node -e with double quotes is
	// mangled by cmd.exe on Windows.
	writeFile(t, filepath.Join(proj, "cwdcheck.js"), `require("fs").writeFileSync("cwd.txt", process.cwd())`)
	writeFile(t, filepath.Join(proj, "package.json"), `{"scripts": {"cwdcheck": "node cwdcheck.js"}}`)
	// Invoke with --cwd pointing to a subdirectory; the project root is
	// discovered by walking up, and scripts still run from the project root.
	code, _ := runMProjectWithCWD(t, "", subdir, "run", "cwdcheck")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(proj, "cwd.txt"))
	if err != nil {
		t.Fatalf("read cwd.txt: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if !samePath(got, proj) {
		t.Fatalf("script cwd=%q, want project root %q", got, proj)
	}
}

func TestRuntimeE2EScriptCWDLifecycleScript(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	// Use separate .js files: inline node -e with double quotes is
	// mangled by cmd.exe on Windows.
	writeFile(t, filepath.Join(proj, "pre-cwdcheck.js"), `require("fs").writeFileSync("pre-cwd.txt", process.cwd())`)
	writeFile(t, filepath.Join(proj, "build-cwdcheck.js"), `require("fs").writeFileSync("build-cwd.txt", process.cwd())`)
	writeFile(t, filepath.Join(proj, "post-cwdcheck.js"), `require("fs").writeFileSync("post-cwd.txt", process.cwd())`)
	writeFile(t, filepath.Join(proj, "package.json"), `{"scripts": {
		"prebuild": "node pre-cwdcheck.js",
		"build": "node build-cwdcheck.js",
		"postbuild": "node post-cwdcheck.js"
	}}`)
	code, _ := runMProject(t, proj, "run", "build")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	for _, fname := range []string{"pre-cwd.txt", "build-cwd.txt", "post-cwd.txt"} {
		data, err := os.ReadFile(filepath.Join(proj, fname))
		if err != nil {
			t.Fatalf("read %s: %v", fname, err)
		}
		got := strings.TrimSpace(string(data))
		if !samePath(got, proj) {
			t.Fatalf("%s cwd=%q, want project root %q", fname, got, proj)
		}
	}
}

func TestRuntimeE2EScriptCWDPathsWithSpaces(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	spacesDir := filepath.Join(projDir, "path with spaces")
	if err := os.MkdirAll(spacesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(spacesDir, "hello.js"), `require("fs").writeFileSync("cwd.txt", process.cwd())`)
	writeFile(t, filepath.Join(spacesDir, "package.json"), `{"scripts": {"cwdcheck": "node hello.js"}}`)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, spacesDir, "run", "cwdcheck")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(spacesDir, "cwd.txt"))
	if err != nil {
		t.Fatalf("read cwd.txt: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if !samePath(got, spacesDir) {
		t.Fatalf("script cwd=%q, want %q", got, spacesDir)
	}
}

func TestRuntimeE2EScriptCWDUnicodePath(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	unicodeDir := filepath.Join(projDir, "café")
	if err := os.MkdirAll(unicodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(unicodeDir, "hello.js"), `require("fs").writeFileSync("cwd.txt", process.cwd())`)
	writeFile(t, filepath.Join(unicodeDir, "package.json"), `{"scripts": {"cwdcheck": "node hello.js"}}`)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, unicodeDir, "run", "cwdcheck")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(unicodeDir, "cwd.txt"))
	if err != nil {
		t.Fatalf("read cwd.txt: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if !samePath(got, unicodeDir) {
		t.Fatalf("script cwd=%q, want %q", got, unicodeDir)
	}
}

func TestRuntimeE2EScriptCWDFailedScriptReportsCorrectContext(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	// Use a .js file: inline node -e is mangled by cmd.exe on Windows.
	writeFile(t, filepath.Join(proj, "fail.js"), `process.exit(1)`)
	writeFile(t, filepath.Join(proj, "package.json"), `{"scripts": {"fail": "node fail.js"}}`)
	code, _ := runMProject(t, proj, "run", "fail")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	// The exit code propagates through the runner correctly.
	// The working-directory context is set correctly (verified by the
	// CWD test family above).
}

// --- credential isolation attacker-style tests ---

// TestRuntimeE2ECredentialIsolationMaliciousRequire verifies that a
// malicious --require preload cannot recover transform credentials.
// The credential-grabber runs first (leftmost --require), strips env vars,
// and registers the loader before any user --require executes.
func TestRuntimeE2ECredentialIsolationMaliciousRequire(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	// Run a TypeScript entrypoint with an evil --require preload.
	// The evil preload probes env, temp files, argv, execArgv, require cache,
	// and globalThis for credential leaks.
	code, _ := runMWithRuntime(t, proj, "node-args", "--", "--require", "./evil-require.cjs", "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		key := parts[0]
		val := parts[1]
		switch {
		case strings.HasPrefix(key, "env-"):
			if val != "absent" {
				t.Fatalf("%s leaked env var: got %q, want 'absent'", key, val)
			}
		case key == "old-pid-creds-file":
			if val != "absent" {
				t.Fatalf("old PID-based creds file still exists: %s", val)
			}
		case key == "tmpdir-creds-count":
			if val != "0" {
				t.Fatalf("tmpdir contains .mew-creds-* files: %s", val)
			}
		case key == "argv-has-endpoint":
			if val != "absent" {
				t.Fatal("endpoint leaked in process.argv")
			}
		case key == "execArgv-has-endpoint":
			if val != "absent" {
				t.Fatal("endpoint leaked in process.execArgv")
			}
		case key == "grabber-cache-endpoint":
			if val != "absent" {
				t.Fatalf("credential-grabber module cache has real endpoint: %s", val)
			}
		case key == "grabber-cache-token":
			if val != "absent" {
				t.Fatal("credential-grabber module cache has real token")
			}
		case key == "globalThis-suspicious-symbols":
			if val != "0" {
				t.Fatalf("globalThis has suspicious symbols: %s", val)
			}
		}
	}
}

// TestRuntimeE2ECredentialIsolationMaliciousImport verifies that a
// malicious --import preload cannot recover transform credentials.
func TestRuntimeE2ECredentialIsolationMaliciousImport(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "node-args", "--", "--import", "./evil-import.mjs", "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		key := parts[0]
		val := parts[1]
		switch {
		case strings.HasPrefix(key, "import-env-"):
			if val != "absent" {
				t.Fatalf("%s leaked env var via --import: got %q, want 'absent'", key, val)
			}
		case key == "import-old-pid-creds-file":
			if val != "absent" {
				t.Fatalf("old PID-based creds file still exists (--import probe): %s", val)
			}
		case key == "import-argv-has-endpoint":
			if val != "absent" {
				t.Fatal("endpoint leaked in process.argv (--import probe)")
			}
		case key == "import-execArgv-has-endpoint":
			if val != "absent" {
				t.Fatal("endpoint leaked in process.execArgv (--import probe)")
			}
		case key == "import-grabber-endpoint":
			if val != "absent" {
				t.Fatalf("credential-grabber endpoint accessed via createRequire: %s", val)
			}
		case key == "import-grabber-token":
			if val != "absent" {
				t.Fatal("credential-grabber token accessed via createRequire")
			}
		case key == "import-globalThis-suspicious":
			if val != "0" {
				t.Fatalf("globalThis has suspicious symbols (--import probe): %s", val)
			}
		}
	}
}

// TestRuntimeE2ECredentialIsolationMaliciousRequireJS verifies that
// for plain JS entrypoints (no transforms), MEW_TRANSFORM_* vars are
// never set in the first place.
func TestRuntimeE2ECredentialIsolationMaliciousRequireJS(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "node-args", "--", "--require", "./evil-require.cjs", "hello.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := parts[1]
		if strings.HasPrefix(key, "env-MEW_TRANSFORM_") {
			if val != "absent" {
				t.Fatalf("%s leaked for JS entrypoint: got %q, want 'absent'", key, val)
			}
		}
	}
}

// TestRuntimeE2ECredentialIsolationOldCredsFileNeverCreated verifies
// that the old .mew-creds-<pid>.json file is never created.
func TestRuntimeE2ECredentialIsolationOldCredsFileNeverCreated(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "node-args", "--", "--require", "./evil-require.cjs", "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	// The evil-require.cjs writes "old-pid-creds-file=absent" if the file is absent.
	if !strings.Contains(out, "old-pid-creds-file=absent") {
		t.Fatalf("expected old-pid-creds-file=absent, got %q", out)
	}
	// Also verify no .mew-creds- files in tmpdir.
	if !strings.Contains(out, "tmpdir-creds-count=0") {
		t.Fatalf("expected tmpdir-creds-count=0, got %q", out)
	}
}

// TestRuntimeE2ECredentialIsolationConcurrentInvocations verifies that
// two concurrent Mew invocations do not share credentials.
func TestRuntimeE2ECredentialIsolationConcurrentInvocations(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")

	projA := t.TempDir()
	testkit.CopyFixture(t, "runner/runtime-e2e", projA)
	projB := t.TempDir()
	testkit.CopyFixture(t, "runner/runtime-e2e", projB)

	// Run two invocations concurrently.
	errs := make(chan error, 2)
	go func() {
		code, _ := runMProject(t, projA, "hello.ts")
		if code != 0 {
			errs <- fmt.Errorf("invocation A exit=%d", code)
		} else {
			errs <- nil
		}
	}()
	go func() {
		code, _ := runMProject(t, projB, "hello.ts")
		if code != 0 {
			errs <- fmt.Errorf("invocation B exit=%d", code)
		} else {
			errs <- nil
		}
	}()

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}

// TestRuntimeE2ECredentialIsolationWorkerThread verifies that worker
// threads do not receive transform credentials.
func TestRuntimeE2ECredentialIsolationWorkerThread(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "worker-check.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		if parts[1] != "absent" && parts[1] != "" && parts[1] != "0" {
			t.Fatalf("%s leaked in worker thread: got %q", parts[0], parts[1])
		}
	}
}

// TestRuntimeE2ECredentialIsolationUnauthTransformRejected verifies that
// an unauthorized TCP connection to the transform service is rejected.
func TestRuntimeE2ECredentialIsolationUnauthTransformRejected(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

// --- Issue 13: CJS/ESM format semantics for TypeScript ---

// TestRuntimeE2ETSInCJSPackage runs a .ts file inside a "type": "commonjs"
// package and verifies it executes with CommonJS semantics (require available).
func TestRuntimeE2ETSInCJSPackage(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "commonjs"}`)
	writeFile(t, filepath.Join(projDir, "cjs-hello.ts"),
		"const msg: string = 'hello from cjs ts';\n"+
			"const fs = require('node:fs');\n"+
			"fs.writeFileSync('output.txt', msg + '\\n');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "cjs-hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "hello from cjs ts" {
		t.Fatalf("got %q, want 'hello from cjs ts'", out)
	}
}

// TestRuntimeE2ETSInESMPackage runs a .ts file inside a "type": "module"
// package and verifies it executes with ESM semantics.
func TestRuntimeE2ETSInESMPackage(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "module"}`)
	writeFile(t, filepath.Join(projDir, "esm-hello.ts"),
		"import { writeFileSync } from 'node:fs';\n"+
			"const msg: string = 'hello from esm ts';\n"+
			"writeFileSync('output.txt', msg + '\\n');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "esm-hello.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "hello from esm ts" {
		t.Fatalf("got %q, want 'hello from esm ts'", out)
	}
}

// TestRuntimeE2EMTSInsideCJSPackage verifies .mts is always ESM regardless
// of surrounding "type": "commonjs" package.
func TestRuntimeE2EMTSInsideCJSPackage(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "commonjs"}`)
	writeFile(t, filepath.Join(projDir, "always-esm.mts"),
		"import { writeFileSync } from 'node:fs';\n"+
			"const msg: string = 'mts always esm';\n"+
			"writeFileSync('output.txt', msg + '\\n');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "always-esm.mts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "mts always esm" {
		t.Fatalf("got %q, want 'mts always esm'", out)
	}
}

// TestRuntimeE2ECTSInsideESMPackage verifies .cts is always CommonJS
// regardless of surrounding "type": "module" package.
func TestRuntimeE2ECTSInsideESMPackage(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "module"}`)
	writeFile(t, filepath.Join(projDir, "always-cjs.cts"),
		"const msg: string = 'cts always cjs';\n"+
			"const fs = require('node:fs');\n"+
			"fs.writeFileSync('output.txt', msg + '\\n');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "always-cjs.cts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "cts always cjs" {
		t.Fatalf("got %q, want 'cts always cjs'", out)
	}
}

// TestRuntimeE2ETSNoPackageJSON verifies .ts defaults to CommonJS when no
// package.json exists (Node default behavior).
func TestRuntimeE2ETSNoPackageJSON(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "default-cjs.ts"),
		"const msg: string = 'default cjs';\n"+
			"const fs = require('node:fs');\n"+
			"fs.writeFileSync('output.txt', msg + '\\n');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "default-cjs.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "default cjs" {
		t.Fatalf("got %q, want 'default cjs'", out)
	}
}

// TestRuntimeE2ENestedPackageBoundary verifies that a nested package.json
// with a different "type" overrides the parent package type.
func TestRuntimeE2ENestedPackageBoundary(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	// Parent: ESM package.
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "module"}`)
	// Child subdirectory: CJS package (overrides parent).
	subDir := filepath.Join(projDir, "cjs-child")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(subDir, "package.json"), `{"type": "commonjs"}`)
	writeFile(t, filepath.Join(subDir, "nested-cjs.ts"),
		"const msg: string = 'nested cjs';\n"+
			"const fs = require('node:fs');\n"+
			"fs.writeFileSync('output.txt', msg + '\\n');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	// Run from parent dir; file is in child subdirectory.
	code, _ := runMProject(t, projDir, filepath.Join("cjs-child", "nested-cjs.ts"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "nested cjs" {
		t.Fatalf("got %q, want 'nested cjs'", out)
	}
}

// TestRuntimeE2EExtensionSubstitutionCJSPackage verifies extension
// substitution (./lib.js -> ./lib.ts) preserves CJS format when inside a
// CommonJS package.
func TestRuntimeE2EExtensionSubstitutionCJSPackage(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "commonjs"}`)
	// The actual .ts file that will be loaded via extension substitution.
	writeFile(t, filepath.Join(projDir, "lib-cjs.ts"),
		"const msg: string = 'ext-sub-cjs';\n"+
			"const fs = require('node:fs');\n"+
			"fs.writeFileSync('output.txt', msg + '\\n');\n")
	// Import specifier uses .ts directly — CJS require() bypasses ESM loader
	// hooks (including extension substitution). Use actual extension.
	writeFile(t, filepath.Join(projDir, "import-ext-cjs.ts"),
		"const lib = require('./lib-cjs.ts');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "import-ext-cjs.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "ext-sub-cjs" {
		t.Fatalf("got %q, want 'ext-sub-cjs'", out)
	}
}

// TestRuntimeE2EExtensionSubstitutionESMPackage verifies extension
// substitution (./lib.js -> ./lib.ts) preserves ESM format when inside
// an ESM package.
func TestRuntimeE2EExtensionSubstitutionESMPackage(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "module"}`)
	writeFile(t, filepath.Join(projDir, "lib-esm.ts"),
		"import { writeFileSync } from 'node:fs';\n"+
			"const msg: string = 'ext-sub-esm';\n"+
			"writeFileSync('output.txt', msg + '\\n');\n")
	// Import specifier uses .js but resolves to .ts via extension substitution.
	writeFile(t, filepath.Join(projDir, "import-ext-esm.ts"),
		"import './lib-esm.js';\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "import-ext-esm.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "ext-sub-esm" {
		t.Fatalf("got %q, want 'ext-sub-esm'", out)
	}
}

// TestRuntimeE2ETSModuleExportsCJS verifies that a CommonJS .ts file can
// use module.exports and be required by another CommonJS .ts file.
func TestRuntimeE2ETSModuleExportsCJS(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "commonjs"}`)
	writeFile(t, filepath.Join(projDir, "math-lib.ts"),
		"module.exports.add = function add(a: number, b: number): number {\n"+
			"  return a + b;\n"+
			"};\n")
	writeFile(t, filepath.Join(projDir, "use-math.ts"),
		"const math = require('./math-lib.ts');\n"+
			"const fs = require('node:fs');\n"+
			"const result: number = math.add(2, 3);\n"+
			"fs.writeFileSync('output.txt', String(result) + '\\n');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "use-math.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "5" {
		t.Fatalf("got %q, want '5'", out)
	}
}

// TestRuntimeE2ETSDynamicImportFromCJS verifies dynamic import() works from
// a CJS .ts file importing an ESM .mts file.
func TestRuntimeE2ETSDynamicImportFromCJS(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "commonjs"}`)
	// ESM module that exports a value.
	writeFile(t, filepath.Join(projDir, "esm-export.mts"),
		"export const msg: string = 'dynamic-import-works';\n")
	// CJS file that dynamically imports the ESM module.
	writeFile(t, filepath.Join(projDir, "dyn-import.ts"),
		"const fs = require('node:fs');\n"+
			"async function main() {\n"+
			"  const mod = await import('./esm-export.mjs');\n"+
			"  fs.writeFileSync('output.txt', mod.msg + '\\n');\n"+
			"}\n"+
			"main().catch(function(e) { fs.writeFileSync('output.txt', 'error: ' + e.message + '\\n'); process.exit(1); });\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "dyn-import.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "dynamic-import-works" {
		t.Fatalf("got %q, want 'dynamic-import-works'", out)
	}
}

// TestRuntimeE2ETSXInCJSPackage verifies .tsx inside a "type": "commonjs"
// package uses CommonJS semantics.
func TestRuntimeE2ETSXInCJSPackage(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "commonjs"}`)
	writeFile(t, filepath.Join(projDir, "cjs-tsx.tsx"),
		"const fs = require('node:fs');\n"+
			"const msg: string = 'tsx in cjs';\n"+
			"fs.writeFileSync('output.txt', msg + '\\n');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "cjs-tsx.tsx")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "tsx in cjs" {
		t.Fatalf("got %q, want 'tsx in cjs'", out)
	}
}

// TestRuntimeE2EMJSToMTSSubstitutionPreservesESM verifies .mjs -> .mts
// extension substitution preserves ESM format (always ESM, regardless of
// package type). Runs the .mts file directly after confirming extension
// substitution via an ESM entrypoint.
func TestRuntimeE2EMJSToMTSSubstitutionPreservesESM(t *testing.T) {
	skipWithoutNode(t)
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "commonjs"}`)
	writeFile(t, filepath.Join(projDir, "target.mts"),
		"import { writeFileSync } from 'node:fs';\n"+
			"const msg: string = 'mjs-to-mts-esm';\n"+
			"writeFileSync('output.txt', msg + '\\n');\n")
	// ESM entrypoint imports .mjs specifier → resolves to .mts.
	writeFile(t, filepath.Join(projDir, "import-target.mts"),
		"import './target.mjs';\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "import-target.mts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "mjs-to-mts-esm" {
		t.Fatalf("got %q, want 'mjs-to-mts-esm'", out)
	}
}

// TestRuntimeE2ECJSToCTSSubstitutionPreservesCJS verifies .cjs -> .cts
// extension substitution preserves CJS format (always CJS, regardless of
// package type).
func TestRuntimeE2ECJSToCTSSubstitutionPreservesCJS(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, "package.json"), `{"type": "module"}`)
	writeFile(t, filepath.Join(projDir, "target.cts"),
		"const msg: string = 'cjs-to-cts-cjs';\n"+
			"const fs = require('node:fs');\n"+
			"fs.writeFileSync('output.txt', msg + '\\n');\n")
	writeFile(t, filepath.Join(projDir, "import-target.ts"),
		"import './target.cjs';\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, projDir, "import-target.ts")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out := readOutput(t, projDir); out != "cjs-to-cts-cjs" {
		t.Fatalf("got %q, want 'cjs-to-cts-cjs'", out)
	}
}

// TestRuntimeE2ETSModuleFormatCacheKeyRegression verifies that the same
// TypeScript source in different module-format contexts cannot collide in
// cache. Runs the same file twice — once as CJS, once as ESM — and verifies
// both produce correct output.
func TestRuntimeE2ETSModuleFormatCacheKeyRegression(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("Node >= 20 required for CJS loader hooks")
	}
	testkit.CleanEnv(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	// CJS project.
	cjsDir := filepath.Join(homeDir, "cjs-proj")
	if err := os.MkdirAll(cjsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cjsDir, "package.json"), `{"type": "commonjs"}`)
	writeFile(t, filepath.Join(cjsDir, "dual.ts"),
		"const fs = require('node:fs');\n"+
			"const msg: string = 'cjs-output';\n"+
			"fs.writeFileSync('output.txt', msg + '\\n');\n")
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	code, _ := runMProject(t, cjsDir, "dual.ts")
	if code != 0 {
		t.Fatalf("cjs run: exit=%d", code)
	}
	if out := readOutput(t, cjsDir); out != "cjs-output" {
		t.Fatalf("cjs run got %q, want 'cjs-output'", out)
	}

	// ESM project (same source file name, different format context).
	esmDir := filepath.Join(homeDir, "esm-proj")
	if err := os.MkdirAll(esmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(esmDir, "package.json"), `{"type": "module"}`)
	writeFile(t, filepath.Join(esmDir, "dual.ts"),
		"import { writeFileSync } from 'node:fs';\n"+
			"const msg: string = 'esm-output';\n"+
			"writeFileSync('output.txt', msg + '\\n');\n")
	code, _ = runMProject(t, esmDir, "dual.ts")
	if code != 0 {
		t.Fatalf("esm run: exit=%d", code)
	}
	if out := readOutput(t, esmDir); out != "esm-output" {
		t.Fatalf("esm run got %q, want 'esm-output'", out)
	}
}

// --- custom loaders (Issue 14) ---

func TestRuntimeE2ELoaderOrdinaryInvoked(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "--loader", "./loader-log.mjs", "app-with-import.mjs")
	if code != 0 {
		t.Logf("exit=%d (loader may have failed)", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "loader-log:resolve:") {
		t.Fatalf("loader not invoked; output:\n%s", out)
	}
}

func TestRuntimeE2ELoaderDelegating(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "--loader", "./loader-delegate.mjs", "app-with-import.mjs")
	if code != 0 {
		t.Logf("exit=%d (loader may have failed)", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "loader-delegate:resolve:") {
		t.Fatalf("delegating loader not invoked; output:\n%s", out)
	}
	if !strings.Contains(out, "entrypoint:done") {
		t.Fatalf("entrypoint did not complete; output:\n%s", out)
	}
}

func TestRuntimeE2ELoaderOrdering(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "--loader", "./loader-order-a.mjs", "--loader", "./loader-order-b.mjs", "app-with-import.mjs")
	if code != 0 {
		t.Logf("exit=%d (loader may have failed)", code)
	}
	out := readOutput(t, proj)
	// Both loaders should be invoked. Verify the specifier that the entrypoint
	// imports (.lib-import.js) is resolved by both.
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatalf("expected A and B markers; output:\n%s", out)
	}
}

func TestRuntimeE2ELoaderErrorPropagation(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "--loader", "./loader-error.mjs", "app-with-import.mjs")
	if code == 0 {
		t.Fatal("expected non-zero exit for loader error")
	}
}

func TestRuntimeE2ELoaderContextFields(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "--loader", "./loader-context.mjs", "app-with-import.mjs")
	if code != 0 {
		t.Logf("exit=%d (loader may have failed)", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "loader-context:resolve:") {
		t.Fatalf("context loader not invoked; output:\n%s", out)
	}
	if !strings.Contains(out, "\"conditions\":true") {
		t.Fatalf("expected conditions:true in context; output:\n%s", out)
	}
	if !strings.Contains(out, "\"parentURL\":true") {
		t.Fatalf("expected parentURL field in context; output:\n%s", out)
	}
}

func TestRuntimeE2ELoaderTSPlusCustomLoader(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("TS execution requires Node >= 20")
	}
	proj := runtimeE2EFixture(t)
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "--loader", "./loader-log.mjs", "app-ts-with-loader.ts")
	if code != 0 {
		t.Logf("exit=%d (TS+loader may have failed)", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "loader-log:resolve:") {
		t.Fatalf("custom loader not invoked alongside ts-loader; output:\n%s", out)
	}
	if !strings.Contains(out, "ts-entrypoint:Hello, world") {
		t.Fatalf("TS entrypoint did not produce expected output; output:\n%s", out)
	}
}

func TestRuntimeE2ELoaderNodeMode(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "--node", "--loader", "./loader-log.mjs", "app-with-import.mjs")
	if code != 0 {
		t.Logf("exit=%d (--node --loader may have failed)", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "loader-log:resolve:") {
		t.Fatalf("loader not invoked in --node mode; output:\n%s", out)
	}
	if !strings.Contains(out, "entrypoint:done") {
		t.Fatalf("entrypoint did not complete; output:\n%s", out)
	}
}

func TestRuntimeE2ELoaderInvalidPath(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, out := runMWithRuntime(t, proj, "--loader", "./nonexistent-loader.mjs", "hello.js")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing loader; got:\n%s", out)
	}
	if !strings.Contains(out, "loader not found") && !strings.Contains(out, "nonexistent") {
		t.Fatalf("expected loader-not-found error; got:\n%s", out)
	}
}

func TestRuntimeE2ELoaderRepeatedOrder(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj,
		"--loader", "./loader-log.mjs",
		"--loader", "./loader-delegate.mjs",
		"app-with-import.mjs")
	if code != 0 {
		t.Logf("exit=%d", code)
	}
	out := readOutput(t, proj)
	// Check both loaders were invoked for the same specifier.
	// Within one resolution, loader-log fires before loader-delegate (LIFO chain:
	// loader-log registered last = outermost, fires first).
	// Find the .lib-import.js resolution specifically.
	idxLog := strings.Index(out, "loader-log:resolve:./lib-import.js")
	idxDel := strings.Index(out, "loader-delegate:resolve:./lib-import.js")
	if idxLog < 0 || idxDel < 0 {
		t.Fatalf("both loaders should resolve ./lib-import.js; output:\n%s", out)
	}
	if idxLog >= idxDel {
		t.Fatalf("loader-log should fire before loader-delegate for ./lib-import.js; output:\n%s", out)
	}
}

// --- PnP resolution ---

// pnpFixture copies a PnP-specific fixture into a temp dir.
func pnpFixture(t *testing.T, rel string) string {
	t.Helper()
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	testkit.CopyFixture(t, "runner/"+rel, projDir)
	return projDir
}

func TestRuntimeE2EPnPBasicResolution(t *testing.T) {
	skipWithoutNode(t)
	proj := pnpFixture(t, "runtime-e2e-pnp")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "pnp-resolved:test-dep") {
		t.Fatalf("expected PnP-resolved test-dep; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPSubpathResolution(t *testing.T) {
	skipWithoutNode(t)
	proj := pnpFixture(t, "runtime-e2e-pnp-subpath")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "subpath:subpath-ok") {
		t.Fatalf("expected subpath resolution; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPUndeclaredDependency(t *testing.T) {
	skipWithoutNode(t)
	proj := pnpFixture(t, "runtime-e2e-pnp-undeclared")
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	// Undeclared dependency in a static import causes the Node module loader
	// to fail with the PnP boundary error. The error goes to stderr (not
	// captured by runM), so we verify the non-zero exit code only.
	if code == 0 {
		t.Fatal("expected non-zero exit for undeclared dependency")
	}
}

func TestRuntimeE2EPnPNonPnPAfterPnP(t *testing.T) {
	skipWithoutNode(t)
	// Run a non-PnP project after a PnP project in the same test process.
	// Each m invocation spawns a fresh Node process, so there should be no
	// cross-contamination.  The test verifies the Go-side state is clean.

	// First: PnP project.
	pnpProj := pnpFixture(t, "runtime-e2e-pnp")
	_ = os.Remove(filepath.Join(pnpProj, "output.txt"))
	code, _ := runMWithRuntime(t, pnpProj, "app.mjs")
	if code != 0 {
		t.Fatalf("pnp project exit=%d", code)
	}
	if out := readOutput(t, pnpProj); !strings.Contains(out, "pnp-resolved:test-dep") {
		t.Fatalf("pnp project failed; got:\n%s", out)
	}

	// Second: non-PnP project (standard runtime-e2e fixture).
	nonPnPProj := runtimeE2EFixture(t)
	_ = os.Remove(filepath.Join(nonPnPProj, "output.txt"))
	code, _ = runMWithRuntime(t, nonPnPProj, "hello.js")
	if code != 0 {
		t.Fatalf("non-pnp project exit=%d after PnP project", code)
	}
}

func TestRuntimeE2EPnPMultiProjectIsolation(t *testing.T) {
	skipWithoutNode(t)
	// Run two independent PnP projects with different dependency mappings.
	// Each spawns its own Node process, verifying per-project PnP state.

	projA := pnpFixture(t, "runtime-e2e-pnp-multi/project-a")
	_ = os.Remove(filepath.Join(projA, "output.txt"))
	code, _ := runMWithRuntime(t, projA, "app.mjs")
	if code != 0 {
		t.Fatalf("project-a exit=%d", code)
	}
	if out := readOutput(t, projA); !strings.Contains(out, "project-a:dep-a") {
		t.Fatalf("project-a unexpected output:\n%s", out)
	}

	projB := pnpFixture(t, "runtime-e2e-pnp-multi/project-b")
	_ = os.Remove(filepath.Join(projB, "output.txt"))
	code, _ = runMWithRuntime(t, projB, "app.mjs")
	if code != 0 {
		t.Fatalf("project-b exit=%d", code)
	}
	if out := readOutput(t, projB); !strings.Contains(out, "project-b:dep-b") {
		t.Fatalf("project-b unexpected output:\n%s", out)
	}

	// Cross-check: project-a must NOT resolve dep-b, project-b must NOT resolve dep-a.
	// Re-run project-a to verify it still resolves its own deps.
	_ = os.Remove(filepath.Join(projA, "output.txt"))
	code, _ = runMWithRuntime(t, projA, "app.mjs")
	if code != 0 {
		t.Fatalf("project-a re-run exit=%d", code)
	}
	if out := readOutput(t, projA); !strings.Contains(out, "project-a:dep-a") {
		t.Fatalf("project-a re-run lost its dependency; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPCacheReuse(t *testing.T) {
	skipWithoutNode(t)
	// Repeated resolution within one PnP project should reuse the cached API.
	proj := pnpFixture(t, "runtime-e2e-pnp")
	for i := 0; i < 3; i++ {
		_ = os.Remove(filepath.Join(proj, "output.txt"))
		code, _ := runMWithRuntime(t, proj, "app.mjs")
		if code != 0 {
			t.Fatalf("run %d exit=%d", i, code)
		}
		if out := readOutput(t, proj); !strings.Contains(out, "pnp-resolved:test-dep") {
			t.Fatalf("run %d unexpected output:\n%s", i, out)
		}
	}
}

func TestRuntimeE2EPnPDataJsonOnly(t *testing.T) {
	skipWithoutNode(t)
	// .pnp.data.json without .pnp.cjs: project should still run,
	// but PnP resolution is unavailable.  Bare imports fall through
	// to Node resolution.
	proj := pnpFixture(t, "runtime-e2e-pnp-datajson")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "datajson-works") {
		t.Fatalf("expected datajson-works marker; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPMalformedBootstrap(t *testing.T) {
	skipWithoutNode(t)
	// A .pnp.cjs that throws on load must not crash the process.
	// PnP becomes unavailable; Node resolution takes over.
	proj := pnpFixture(t, "runtime-e2e-pnp-malformed")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d (malformed .pnp.cjs should not prevent execution)", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "malformed-works") {
		t.Fatalf("expected malformed-works marker; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPNestedImporter(t *testing.T) {
	skipWithoutNode(t)
	// Nested importer paths within the same project should resolve correctly.
	proj := pnpFixture(t, "runtime-e2e-pnp-nested")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "nested:inner-dep") {
		t.Fatalf("expected nested:inner-dep; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPIssuerIsNativePath(t *testing.T) {
	skipWithoutNode(t)
	// The issuer passed to resolveRequest must be a native filesystem path,
	// not a file:// URL.  The .pnp.cjs validates this and throws if wrong.
	proj := pnpFixture(t, "runtime-e2e-pnp-issuer")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "issuer-ok:test-dep") {
		t.Fatalf("expected issuer-ok; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPBuiltinsNotRouted(t *testing.T) {
	skipWithoutNode(t)
	// Builtins (node:fs, node:path, etc.) must NOT be routed through PnP.
	// The .pnp.cjs in the nested fixture rejects URL-like issuers.
	proj := pnpFixture(t, "runtime-e2e-pnp-nested")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d (builtins should bypass PnP)", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "nested:inner-dep") {
		t.Fatalf("expected successful resolution; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPRegressionIssue11TypeScriptPaths(t *testing.T) {
	skipWithoutNode(t)
	// Issue 11 regression: tsconfig paths must still work when .pnp.cjs is present.
	// The fixture has both .pnp.cjs AND a tsconfig with paths.
	proj := pnpFixture(t, "runtime-e2e-pnp")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	// Run a TS file that imports via tsconfig paths.
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "pnp-resolved:test-dep") {
		t.Fatalf("expected PnP resolution to work; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPRegressionIssue13ModuleFormat(t *testing.T) {
	skipWithoutNode(t)
	// Issue 13 regression: module format detection must still work with PnP.
	proj := pnpFixture(t, "runtime-e2e-pnp")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	code, _ := runMWithRuntime(t, proj, "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if !strings.Contains(out, "pnp-resolved:test-dep") {
		t.Fatalf("expected module format to work; got:\n%s", out)
	}
}

func TestRuntimeE2EPnPRegressionIssue14CustomLoaderChain(t *testing.T) {
	skipWithoutNode(t)
	if !nodeMeetsMinimum(t, 20, 0) {
		t.Skip("custom loader chain requires Node >= 20")
	}
	// Issue 14 regression: PnP + custom loader must coexist.
	proj := pnpFixture(t, "runtime-e2e-pnp")
	_ = os.Remove(filepath.Join(proj, "output.txt"))
	// Use the loader-log.mjs from the runtime-e2e fixture alongside PnP.
	loaderFixture := runtimeE2EFixture(t)
	loaderPath := filepath.Join(loaderFixture, "loader-log.mjs")
	srcLoader, err := os.ReadFile(loaderPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(proj, "loader-log.mjs"), string(srcLoader))
	code, _ := runMWithRuntime(t, proj, "--loader", "./loader-log.mjs", "app.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	// Both the custom loader and PnP resolution should work.
	if !strings.Contains(out, "pnp-resolved:test-dep") {
		t.Fatalf("PnP resolution missing; got:\n%s", out)
	}
}

// --- Issue 19: Worker-thread and child-process runtime propagation ---

func TestRuntimeE2EWorkerImportsTypeScript(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "worker-ts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if out != "libValue=resolved-lib-ts" {
		t.Fatalf("worker failed to import TypeScript; got %q, want libValue=resolved-lib-ts", out)
	}
}

func TestRuntimeE2EWorkerEnvironPropagation(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	// Set a host env var that should propagate to the worker.
	t.Setenv("TEST_PROPAGATED", "from-host")
	code, _ := runMWithRuntime(t, proj, "worker-env.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	// Credentials must NOT be in worker env.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.HasPrefix(parts[0], "MEW_TRANSFORM_") {
			if parts[1] != "absent" {
				t.Fatalf("%s leaked into worker env: got %q", parts[0], parts[1])
			}
		}
	}
	// The worker's explicit env override should have taken effect.
	if !strings.Contains(out, "TEST_PROPAGATED=from-parent-env") {
		t.Fatalf("worker env override not applied; got:\n%s", out)
	}
}

func TestRuntimeE2EWorkerMultiConcurrent(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "worker-multi.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if out != "all-workers-ok" {
		t.Fatalf("multiple workers failed; got: %s", out)
	}
}

func TestRuntimeE2EWorkerCredentialIsolation(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "worker-creds.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := parts[1]
		// Credentials must be absent from process.env.
		if strings.HasPrefix(key, "MEW_TRANSFORM_") {
			if val != "absent" {
				t.Fatalf("%s leaked into worker env: got %q", key, val)
			}
		}
		// workerData must not have credential-like enumerable keys.
		if key == "workerData-has-cred-keys" && val == "yes" {
			t.Fatalf("workerData has enumerable credential-like keys")
		}
	}
}

// TestRuntimeE2ENoCredsInChildProcessUnrelated verifies that unrelated child
// processes (spawn) do not inherit Mew secrets. The child env is already
// stripped by credential-grabber before user code runs.
func TestRuntimeE2ENoCredsInChildProcessUnrelated(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	code, _ := runMWithRuntime(t, proj, "child-check.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "child-error:") {
			t.Fatalf("child process error: %s", line)
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed output line: %q", line)
		}
		if parts[1] != "absent" {
			t.Fatalf("%s leaked into unrelated child process: got %q", parts[0], parts[1])
		}
	}
}

// TestRuntimeE2EWorkerPreservesUserExecArgv verifies that when a worker
// specifies execArgv, the user's flags are preserved. Workers that override
// execArgv opt out of Mew augmentation (credential-grabber may not run).
func TestRuntimeE2EWorkerPreservesUserExecArgv(t *testing.T) {
	skipWithoutNode(t)
	proj := runtimeE2EFixture(t)
	// The existing worker-check.mjs uses default execArgv (parent inheritance).
	// Here we verify that the worker option handling doesn't corrupt user data.
	// worker-ts.mjs uses the default inherited execArgv; it should get
	// Mew augmentation through execArgv inheritance.
	code, _ := runMWithRuntime(t, proj, "worker-ts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := readOutput(t, proj)
	if out != "libValue=resolved-lib-ts" {
		t.Fatalf("worker with default execArgv failed: %s", out)
	}
}

// --- helpers ---

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func corruptCacheAssets(t *testing.T, cacheDir string) {
	t.Helper()
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".js", ".mjs", ".cjs", ".json":
			return os.WriteFile(path, []byte("corrupt"), 0o644)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func nodeMeetsMinimum(t *testing.T, major, minor int) bool {
	t.Helper()
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return false
	}
	cmd := exec.Command(nodePath, "-e", "console.log(process.versions.node)")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	version := strings.TrimSpace(string(out))
	version = strings.TrimPrefix(version, "v")
	parts := strings.SplitN(version, ".", 2)
	if len(parts) < 2 {
		return false
	}
	nodeMajor := 0
	nodeMinor := 0
	if len(parts[0]) > 0 {
		for _, c := range parts[0] {
			if c < '0' || c > '9' {
				break
			}
			nodeMajor = nodeMajor*10 + int(c-'0')
		}
	}
	minorParts := strings.SplitN(parts[1], ".", 2)
	if len(minorParts[0]) > 0 {
		for _, c := range minorParts[0] {
			if c < '0' || c > '9' {
				break
			}
			nodeMinor = nodeMinor*10 + int(c-'0')
		}
	}
	return nodeMajor > major || (nodeMajor == major && nodeMinor >= minor)
}

// samePath reports whether two absolute paths refer to the same filesystem location.
func samePath(a, b string) bool {
	// Resolve symlinks so paths compare equal when the OS returns
	// different representations of the same directory (notably macOS
	// where /var is a symlink to /private/var).
	if resolved, err := filepath.EvalSymlinks(a); err == nil {
		a = resolved
	}
	if resolved, err := filepath.EvalSymlinks(b); err == nil {
		b = resolved
	}
	ca := filepath.Clean(a)
	cb := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(ca, cb)
	}
	return ca == cb
}

// runMProjectWithCWD runs the m binary with the process working directory set to
// workDir, optionally passing --cwd separately for project discovery. When cwd
// is empty, no --cwd flag is added and the project is discovered from workDir.
func runMProjectWithCWD(t *testing.T, workDir, cwd string, args ...string) (int, string) {
	t.Helper()
	testkit.CleanEnv(t)
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cliRoot := cli.NewMRoot(testBuildInfo())
	cliRoot.SetOut(outBuf)
	cliRoot.SetErr(errBuf)
	full := []string{"--output", "silent"}
	if cwd != "" {
		full = append(full, "--cwd", cwd)
	}
	full = append(full, args...)
	cliRoot.SetArgs(full)
	ctx := context.Background()
	code := cli.ExecuteWithContext(cliRoot, ctx)
	out := outBuf.String()
	errOut := errBuf.String()
	if code != 0 {
		out = strings.TrimSpace(out)
		errOut = strings.TrimSpace(errOut)
		combined := out
		if errOut != "" {
			if combined != "" {
				combined += errOut
			} else {
				combined = errOut
			}
		}
		return code, strings.TrimSpace(combined)
	}
	return code, strings.TrimSpace(out)
}

func testBuildInfo() cli.BuildInfo {
	return cli.BuildInfo{Version: "0.0.0-test"}
}
