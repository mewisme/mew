package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/transform"
)

func TestTransformReportHumanOutput(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"transform", "report"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if out == "" {
		t.Fatal("expected human-readable output")
	}
	// Human output must not contain JSON-like structured content.
	if strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Fatal("human output must not be JSON")
	}
	// Must include some expected headers.
	if !strings.Contains(out, "OPTION") || !strings.Contains(out, "STATUS") {
		t.Fatal("human output missing table headers")
	}
}

func TestTransformReportJSONOutput(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"transform", "report", "--json"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("expected JSON output")
	}
	var entries []transform.Entry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput:\n%s", err, out)
	}
	if len(entries) == 0 {
		t.Fatal("JSON report must not be empty")
	}

	// Validate every entry has required fields.
	for i, e := range entries {
		if e.Option == "" {
			t.Errorf("entry %d: empty option name", i)
		}
		if e.Status == "" {
			t.Errorf("entry %d (%s): empty status", i, e.Option)
		}
		if e.Category == "" {
			t.Errorf("entry %d (%s): empty category", i, e.Option)
		}
	}
}

func TestTransformReportJSONNoProse(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"transform", "report", "--json"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	// JSON output must start with '[' (array).
	if !strings.HasPrefix(out, "[") {
		t.Fatalf("JSON output must start with '[', got: %s", out[:min(80, len(out))])
	}
	// Must be valid JSON with no extra prose.
	var entries []transform.Entry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("JSON output contains invalid JSON or extra prose: %v", err)
	}
}

func TestTransformReportDeterministic(t *testing.T) {
	run := func() []transform.Entry {
		root := NewMRoot(testBuildInfo())
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetArgs([]string{"transform", "report", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var entries []transform.Entry
		if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
			t.Fatalf("JSON parse: %v", err)
		}
		return entries
	}
	r1 := run()
	r2 := run()
	if len(r1) != len(r2) {
		t.Fatalf("length differs: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i].Option != r2[i].Option {
			t.Errorf("entry %d: option %q vs %q", i, r1[i].Option, r2[i].Option)
		}
		if r1[i].Status != r2[i].Status {
			t.Errorf("entry %d (%s): status %q vs %q", i, r1[i].Option, r1[i].Status, r2[i].Status)
		}
		if r1[i].Category != r2[i].Category {
			t.Errorf("entry %d (%s): category %q vs %q", i, r1[i].Option, r1[i].Category, r2[i].Category)
		}
	}
}

func TestTransformReportJSONMatchesCapabilityReport(t *testing.T) {
	// The CLI JSON output must match what the capability registry returns
	// directly, proving the CLI doesn't have its own hard-coded table.
	root := NewMRoot(testBuildInfo())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"transform", "report", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var cliEntries []transform.Entry
	if err := json.Unmarshal(buf.Bytes(), &cliEntries); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	directEntries := transform.CapabilityReport()
	if len(cliEntries) != len(directEntries) {
		t.Fatalf("CLI report has %d entries, capability report has %d", len(cliEntries), len(directEntries))
	}
	for i := range cliEntries {
		if cliEntries[i].Option != directEntries[i].Option {
			t.Errorf("entry %d: CLI %q vs registry %q", i, cliEntries[i].Option, directEntries[i].Option)
		}
		if cliEntries[i].Status != directEntries[i].Status {
			t.Errorf("entry %d (%s): CLI status %q vs registry %q", i, cliEntries[i].Option, cliEntries[i].Status, directEntries[i].Status)
		}
	}
}

func TestTransformReportNoArgs(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	root.SetArgs([]string{"transform", "report", "extra"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for extra args")
	}
}

func TestTransformReportInvalidFlag(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	root.SetArgs([]string{"transform", "report", "--nonexistent"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestTransformHelp(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"transform", "--help"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "transform") {
		t.Fatal("help output missing command name")
	}
	if !strings.Contains(out, "report") {
		t.Fatal("help output missing report subcommand")
	}
}
