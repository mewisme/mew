package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/testkit"
)

// resolveModuleFixture creates a temp project from a fixture and returns the project dir.
func resolveModuleFixture(t *testing.T, rel string) string {
	t.Helper()
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	testkit.CopyFixture(t, "runner/"+rel, projDir)
	return projDir
}

// TestResolveModuleJSON_ValidOutput proves --json emits valid JSON with no text contamination.
func TestResolveModuleJSON_ValidOutput(t *testing.T) {
	proj := resolveModuleFixture(t, "resolve-module-paths")
	code, out := runMProject(t, proj, "resolve-module", "--json", "@app/helpers")
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, out)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\nGot: %s", err, out)
	}

	// Verify no text renderer contamination.
	for _, forbidden := range []string{"Specifier:", "From:", "Tsconfig:", "Resolution trace:"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("--json output contains text renderer string %q", forbidden)
		}
	}

	// Verify required JSON fields.
	if result["schemaVersion"] == nil {
		t.Error("missing schemaVersion")
	}
	if result["specifier"] != "@app/helpers" {
		t.Errorf("specifier = %v", result["specifier"])
	}
	if result["resolved"] != true {
		t.Errorf("expected resolved=true for @app/helpers, got resolved=%v", result["resolved"])
	}
	if result["target"] == nil {
		t.Fatal("missing target")
	}
	target := result["target"].(map[string]interface{})
	if target["format"] != "module" {
		t.Errorf("expected format=module for .ts file, got %v", target["format"])
	}
}

// TestResolveModuleJSON_DeterministicOutput proves repeated runs produce identical JSON.
func TestResolveModuleJSON_DeterministicOutput(t *testing.T) {
	proj := resolveModuleFixture(t, "resolve-module-paths")
	code1, out1 := runMProject(t, proj, "resolve-module", "--json", "@app/helpers")
	if code1 != 0 {
		t.Fatalf("run 1 exit code %d: %s", code1, out1)
	}
	code2, out2 := runMProject(t, proj, "resolve-module", "--json", "@app/helpers")
	if code2 != 0 {
		t.Fatalf("run 2 exit code %d: %s", code2, out2)
	}
	if out1 != out2 {
		t.Errorf("--json output not deterministic:\nrun1: %s\nrun2: %s", out1, out2)
	}
}

// TestResolveModuleJSON_MissingModule proves unresolved modules exit non-zero with error.
func TestResolveModuleJSON_MissingModule(t *testing.T) {
	proj := resolveModuleFixture(t, "resolve-module-paths")
	code, out := runMProject(t, proj, "resolve-module", "--json", "nonexistent-pkg-xyz")
	if code != 1 {
		t.Errorf("expected exit code 1 for missing module, got %d", code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nGot: %s", err, out)
	}
	if result["resolved"] != false {
		t.Error("expected resolved=false for missing module")
	}
	if result["error"] == nil {
		t.Error("missing error field")
	}
}

// TestResolveModuleHuman_Output proves human output contains expected sections.
func TestResolveModuleHuman_Output(t *testing.T) {
	proj := resolveModuleFixture(t, "resolve-module-paths")
	code, out := runMProject(t, proj, "resolve-module", "@app/helpers")
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, out)
	}

	if !strings.Contains(out, "Specifier: @app/helpers") {
		t.Error("human output missing specifier")
	}
	if !strings.Contains(out, "Resolution trace:") {
		t.Error("human output missing trace header")
	}
	if !strings.Contains(out, "Resolved:") {
		t.Error("human output missing resolved line")
	}
	if !strings.Contains(out, "Format:") {
		t.Error("human output missing format line")
	}
}

// TestResolveModule_FromFlag proves --from flag changes the importer directory.
func TestResolveModule_FromFlag(t *testing.T) {
	proj := resolveModuleFixture(t, "resolve-module-paths")
	srcDir := filepath.Join(proj, "src")

	// Create a dummy file in src dir so it exists as importer context.
	if err := os.WriteFile(filepath.Join(srcDir, "index.ts"), []byte("import { helper } from '@app/helpers';"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := runMProject(t, proj, "resolve-module", "--from", srcDir, "--json", "./helpers")
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, out)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nGot: %s", err, out)
	}
	if result["resolved"] != true {
		t.Errorf("expected resolved=true for ./helpers, got %v", result["resolved"])
	}
}

// TestResolveModule_Builtin proves builtins resolve correctly.
func TestResolveModule_Builtin(t *testing.T) {
	proj := resolveModuleFixture(t, "resolve-module-paths")
	code, out := runMProject(t, proj, "resolve-module", "--json", "fs")
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, out)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nGot: %s", err, out)
	}
	if result["resolved"] != true {
		t.Error("expected resolved=true for builtin fs")
	}
	target := result["target"].(map[string]interface{})
	if target["format"] != "builtin" {
		t.Errorf("expected format=builtin for node:fs, got %v", target["format"])
	}
}

// TestResolveModule_RelativeFile proves relative file resolution works.
func TestResolveModule_RelativeFile(t *testing.T) {
	proj := resolveModuleFixture(t, "resolve-module-paths")
	srcDir := filepath.Join(proj, "src")

	code, out := runMProject(t, proj, "resolve-module", "--from", srcDir, "--json", "./helpers.ts")
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, out)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nGot: %s", err, out)
	}
	if result["resolved"] != true {
		t.Errorf("expected resolved=true for ./helpers.ts, got %v (output: %s)", result["resolved"], out)
	}
	target, ok := result["target"].(map[string]interface{})
	if !ok || target == nil {
		t.Fatalf("missing or invalid target in: %s", out)
	}
	// .ts files under a package.json with "type":"module" → "module" format.
	if target["format"] != "module" {
		t.Errorf("expected format=module for .ts file, got %v (full output: %s)", target["format"], out)
	}
}
