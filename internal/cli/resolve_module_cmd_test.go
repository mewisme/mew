package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/transform"
)

// ── matchPathPattern tests (existing) ──────────────────────────────────

func TestMatchPathPattern(t *testing.T) {
	tests := []struct {
		name      string
		specifier string
		pattern   string
		want      []string
	}{
		{
			name:      "exact match",
			specifier: "@app/core",
			pattern:   "@app/core",
			want:      []string{""},
		},
		{
			name:      "exact no match",
			specifier: "@app/other",
			pattern:   "@app/core",
			want:      nil,
		},
		{
			name:      "prefix wildcard",
			specifier: "@app/helpers",
			pattern:   "@app/*",
			want:      []string{"helpers"},
		},
		{
			name:      "prefix wildcard no match",
			specifier: "@other/helpers",
			pattern:   "@app/*",
			want:      nil,
		},
		{
			name:      "middle wildcard",
			specifier: "@app/helpers/utils",
			pattern:   "@app/*/utils",
			want:      []string{"helpers"},
		},
		{
			name:      "catch-all wildcard",
			specifier: "@scope/pkg",
			pattern:   "*",
			want:      []string{"@scope/pkg"},
		},
		{
			name:      "empty captures",
			specifier: "@app/",
			pattern:   "@app/*",
			want:      []string{""},
		},
		{
			name:      "no wildcard in pattern",
			specifier: "@app/helpers",
			pattern:   "@app/helpers",
			want:      []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPathPattern(tt.specifier, tt.pattern)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("matchPathPattern(%q, %q) = %v, want %v", tt.specifier, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMatchPathPatternOverlappingPrefixSuffix(t *testing.T) {
	got := matchPathPattern("abc", "ab*bc")
	if got != nil {
		t.Errorf("matchPathPattern(%q, %q) = %v, want nil (overlapping prefix/suffix)", "abc", "ab*bc", got)
	}
}

func TestMatchPathPatternMultiWildcard(t *testing.T) {
	got := matchPathPattern("@scope/pkg/sub/file", "@*/*/*")
	if got == nil {
		t.Fatal("expected match")
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 captures, got %d: %v", len(got), got)
	}
	if got[0] != "scope" || got[1] != "pkg" || got[2] != "sub/file" {
		t.Errorf("captures: %v", got)
	}
}

// ── parseNodeMajor tests ───────────────────────────────────────────────

func TestParseNodeMajor(t *testing.T) {
	tests := []struct {
		version string
		want    int
	}{
		{"v22.14.0", 22},
		{"v20.6.1", 20},
		{"v18.19.0", 18},
		{"v16.0.0", 16},
		{"22.14.0", 22},
		{"", 0},
		{"v", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		got := parseNodeMajor(tt.version)
		if got != tt.want {
			t.Errorf("parseNodeMajor(%q) = %d, want %d", tt.version, got, tt.want)
		}
	}
}

// ── nodeImportMetaResolveFlag tests ────────────────────────────────────

func TestNodeImportMetaResolveFlag(t *testing.T) {
	tests := []struct {
		version string
		wantLen int
	}{
		{"v22.14.0", 0},
		{"v20.6.1", 0},
		{"v18.19.0", 1}, // needs --experimental-import-meta-resolve
		{"v16.0.0", 0},
	}
	for _, tt := range tests {
		got := nodeImportMetaResolveFlag(tt.version)
		if len(got) != tt.wantLen {
			t.Errorf("nodeImportMetaResolveFlag(%q) len = %d, want %d", tt.version, len(got), tt.wantLen)
		}
	}
}

// ── buildDiagnosticOptions tests ───────────────────────────────────────

func TestBuildDiagnosticOptions_NoTsconfig(t *testing.T) {
	dir := t.TempDir()
	opts := buildDiagnosticOptions("", dir)
	if opts["configDir"] != dir {
		t.Errorf("configDir = %v, want %v", opts["configDir"], dir)
	}
}

func TestBuildDiagnosticOptions_WithPaths(t *testing.T) {
	dir := t.TempDir()
	tsconfig := filepath.Join(dir, "tsconfig.json")
	data := `{"compilerOptions":{"baseUrl":".","paths":{"@app/*":["./src/*"]}}}`
	if err := os.WriteFile(tsconfig, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := buildDiagnosticOptions(tsconfig, dir)
	if opts["configDir"] != dir {
		t.Errorf("configDir = %v, want %v", opts["configDir"], dir)
	}
	if opts["baseUrl"] != "." {
		t.Errorf("baseUrl = %v, want .", opts["baseUrl"])
	}
	mappings, ok := opts["pathMappings"].([]transform.PathMapping)
	if !ok || len(mappings) == 0 {
		t.Fatal("expected pathMappings")
	}
	if mappings[0].Pattern != "@app/*" {
		t.Errorf("pattern = %v, want @app/*", mappings[0].Pattern)
	}
}

// ── JSON renderer tests ────────────────────────────────────────────────

func TestRenderResolveModuleJSON_NoResult(t *testing.T) {
	var buf bytes.Buffer
	cmd := newResolveModuleCmd()
	cmd.SetOut(&buf)

	err := renderResolveModuleJSON(cmd, "test-dep", "/tmp/test", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Verify valid JSON.
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	// Verify no text contamination.
	if bytes.Contains(buf.Bytes(), []byte("Specifier:")) {
		t.Error("JSON output contains text renderer contamination ('Specifier:')")
	}
	if bytes.Contains(buf.Bytes(), []byte("Resolution trace:")) {
		t.Error("JSON output contains text renderer contamination ('Resolution trace:')")
	}

	// Verify required fields.
	if result["schemaVersion"] == nil {
		t.Error("missing schemaVersion")
	}
	if result["specifier"] != "test-dep" {
		t.Errorf("specifier = %v, want test-dep", result["specifier"])
	}
	if result["resolved"] != false {
		t.Error("expected resolved=false when no result")
	}
}

func TestRenderResolveModuleJSON_WithResult(t *testing.T) {
	var buf bytes.Buffer
	cmd := newResolveModuleCmd()
	cmd.SetOut(&buf)

	result := &diagResult{
		SchemaVersion: 1,
		Specifier:     "@app/core",
		Importer:      "/tmp/test/src/index.ts",
		Resolved:      true,
		Target: &diagTarget{
			URL:    "file:///tmp/test/src/core.ts",
			Path:   "/tmp/test/src/core.ts",
			Format: "module",
		},
		Trace: []diagStep{
			{Stage: "builtins", Outcome: "skipped"},
			{Stage: "tsconfig-paths", Outcome: "resolved", Resolved: "/tmp/test/src/core.ts", Pattern: "@app/*", Format: "module"},
		},
	}

	err := renderResolveModuleJSON(cmd, "@app/core", "/tmp/test/src", "/tmp/test/tsconfig.json", result)
	if err != nil {
		t.Fatal(err)
	}

	var parsed diagResult
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", parsed.SchemaVersion)
	}
	if !parsed.Resolved {
		t.Error("expected resolved=true")
	}
	if parsed.Target == nil || parsed.Target.Format != "module" {
		t.Error("target format mismatch")
	}
	if len(parsed.Trace) != 2 {
		t.Errorf("trace length = %d, want 2", len(parsed.Trace))
	}

	// Verify trace ordering is preserved (deterministic).
	if parsed.Trace[0].Stage != "builtins" {
		t.Error("first trace step should be builtins")
	}
	if parsed.Trace[1].Stage != "tsconfig-paths" {
		t.Error("second trace step should be tsconfig-paths")
	}
}

// ── Text renderer tests ────────────────────────────────────────────────

func TestRenderResolveModuleText_WithResult(t *testing.T) {
	var buf bytes.Buffer
	cmd := newResolveModuleCmd()
	cmd.SetOut(&buf)

	result := &diagResult{
		SchemaVersion: 1,
		Specifier:     "@app/core",
		Importer:      "/tmp/test/src/index.ts",
		Resolved:      true,
		Target: &diagTarget{
			URL:    "file:///tmp/test/src/core.ts",
			Path:   "/tmp/test/src/core.ts",
			Format: "module",
		},
		Trace: []diagStep{
			{Stage: "builtins", Outcome: "skipped"},
			{Stage: "tsconfig-paths", Outcome: "resolved", Resolved: "/tmp/test/src/core.ts", Pattern: "@app/*", Format: "module"},
		},
	}

	err := renderResolveModuleText(cmd, "@app/core", "/tmp/test/src", "/tmp/test/tsconfig.json", result)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Specifier: @app/core") {
		t.Error("text output missing specifier")
	}
	if !strings.Contains(out, "Resolution trace:") {
		t.Error("text output missing trace header")
	}
	if !strings.Contains(out, "Resolved: file:///tmp/test/src/core.ts") {
		t.Error("text output missing resolved URL")
	}
	if !strings.Contains(out, "Format:   module") {
		t.Error("text output missing format")
	}
}

func TestRenderResolveModuleText_ErrorResult(t *testing.T) {
	var buf bytes.Buffer
	cmd := newResolveModuleCmd()
	cmd.SetOut(&buf)

	result := &diagResult{
		SchemaVersion: 1,
		Specifier:     "nonexistent",
		Importer:      "/tmp/test/src/index.ts",
		Resolved:      false,
		Target:        nil,
		Error: &diagError{
			Code:    "ERR_MODULE_NOT_FOUND",
			Message: "Cannot find module 'nonexistent'",
		},
		Trace: []diagStep{
			{Stage: "builtins", Outcome: "skipped"},
			{Stage: "node-native", Outcome: "miss", Error: "ERR_MODULE_NOT_FOUND"},
		},
	}

	err := renderResolveModuleText(cmd, "nonexistent", "/tmp/test/src", "/tmp/test/tsconfig.json", result)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Error:") {
		t.Error("text output missing error line")
	}
	if !strings.Contains(out, "ERR_MODULE_NOT_FOUND") {
		t.Error("text output missing error code")
	}
}

func TestRenderResolveModuleText_NoNode(t *testing.T) {
	var buf bytes.Buffer
	cmd := newResolveModuleCmd()
	cmd.SetOut(&buf)

	// When result is nil, the renderer falls back to static analysis.
	err := renderResolveModuleText(cmd, "@app/core", "/tmp/test", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Diagnostic resolution unavailable") {
		t.Error("text output missing fallback notice")
	}
}

// ── findPnpRoot tests ──────────────────────────────────────────────────

func TestFindPnpRoot_Found(t *testing.T) {
	dir := t.TempDir()
	pnpPath := filepath.Join(dir, ".pnp.cjs")
	if err := os.WriteFile(pnpPath, []byte("module.exports = {};"), 0o644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(dir, "src", "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	root := findPnpRoot(subDir)
	if root != dir {
		t.Errorf("findPnpRoot = %q, want %q", root, dir)
	}
}

func TestFindPnpRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	root := findPnpRoot(dir)
	if root != "" {
		t.Errorf("findPnpRoot should be empty, got %q", root)
	}
}

// ── JSON renderer regression: --json emits valid JSON with no text ─────

func TestResolveModuleJSON_NoTextContamination(t *testing.T) {
	// Construct a proper result and verify JSON output is pure JSON.
	var buf bytes.Buffer
	cmd := newResolveModuleCmd()
	cmd.SetOut(&buf)

	result := &diagResult{
		SchemaVersion: 1,
		Specifier:     "test-dep",
		Importer:      "/tmp/project/app.ts",
		Resolved:      true,
		Target: &diagTarget{
			URL:    "file:///tmp/project/node_modules/test-dep/index.js",
			Path:   "/tmp/project/node_modules/test-dep/index.js",
			Format: "commonjs",
		},
		Trace: []diagStep{
			{Stage: "builtins", Outcome: "skipped"},
			{Stage: "node-native", Outcome: "resolved", Resolved: "/tmp/project/node_modules/test-dep/index.js", Format: "commonjs"},
		},
	}

	_ = renderResolveModuleJSON(cmd, "test-dep", "/tmp/project", "/tmp/project/tsconfig.json", result)

	out := buf.String()
	// Must be valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\nGot: %s", err, out)
	}
	// Must not contain text renderer strings.
	for _, forbidden := range []string{"Specifier:", "From:", "Tsconfig:", "Resolution trace:", "Resolved:", "Format:", "Error:", "PnP root:"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("--json output contains text renderer string %q", forbidden)
		}
	}
	// Must have required JSON fields.
	if v, ok := parsed["schemaVersion"]; !ok || v == nil {
		t.Error("JSON output missing schemaVersion")
	}
	if v, ok := parsed["resolved"]; !ok || v != true {
		t.Error("JSON output missing/wrong resolved")
	}
}

// ── Deterministic trace order test ────────────────────────────────────

func TestResolveModuleJSON_DeterministicTraceOrder(t *testing.T) {
	var buf bytes.Buffer
	cmd := newResolveModuleCmd()
	cmd.SetOut(&buf)

	trace := []diagStep{
		{Stage: "builtins", Outcome: "skipped"},
		{Stage: "node-native", Outcome: "miss", Error: "ERR_MODULE_NOT_FOUND"},
		{Stage: "extension-probe", Outcome: "miss"},
		{Stage: "pnp", Outcome: "skipped", Reason: "no .pnp.cjs found"},
		{Stage: "tsconfig-paths", Outcome: "resolved", Resolved: "/tmp/src/helpers.ts", Pattern: "@app/*", Format: "module"},
	}

	result := &diagResult{
		SchemaVersion: 1,
		Specifier:     "@app/helpers",
		Importer:      "/tmp/src/index.ts",
		Resolved:      true,
		Target:        &diagTarget{URL: "file:///tmp/src/helpers.ts", Path: "/tmp/src/helpers.ts", Format: "module"},
		Trace:         trace,
	}

	// Run twice and verify identical output (deterministic).
	var out1, out2 bytes.Buffer
	cmd1 := newResolveModuleCmd()
	cmd1.SetOut(&out1)
	_ = renderResolveModuleJSON(cmd1, "@app/helpers", "/tmp/src", "/tmp/tsconfig.json", result)

	cmd2 := newResolveModuleCmd()
	cmd2.SetOut(&out2)
	_ = renderResolveModuleJSON(cmd2, "@app/helpers", "/tmp/src", "/tmp/tsconfig.json", result)

	if out1.String() != out2.String() {
		t.Errorf("JSON output not deterministic:\nrun1: %s\nrun2: %s", out1.String(), out2.String())
	}
}
