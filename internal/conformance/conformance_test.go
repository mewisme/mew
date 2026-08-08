package conformance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/testkit"
)

func TestLoadCoreManifest(t *testing.T) {
	root := testkit.ModuleRoot(t)
	path := filepath.Join(root, "tests", "conformance", "core-matrix", "manifest.json")
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != SchemaVersion || m.Matrix != CoreMatrix {
		t.Fatalf("manifest=%+v", m)
	}
	if len(m.Suites) < 10 {
		t.Fatalf("expected many suites, got %d", len(m.Suites))
	}
}

func TestRunCoreDryRun(t *testing.T) {
	root := testkit.ModuleRoot(t)
	report, err := RunCore(t.Context(), RunOptions{RepoRoot: root, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || !report.DryRun || len(report.Suites) == 0 {
		t.Fatalf("report=%+v", report)
	}
	for _, s := range report.Suites {
		if s.Status != StatusPlanned {
			t.Fatalf("suite %s status=%s want planned", s.ID, s.Status)
		}
	}
}

func TestRunCoreFilterIdentity(t *testing.T) {
	root := testkit.ModuleRoot(t)
	report, err := RunCore(t.Context(), RunOptions{RepoRoot: root, Filter: "identity"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Suites) != 1 {
		t.Fatalf("suites=%d want 1", len(report.Suites))
	}
	if report.Suites[0].ID != "lock-bridge-yarn-identity" {
		t.Fatalf("suite id=%s", report.Suites[0].ID)
	}
	if report.Suites[0].Status != StatusPassed {
		t.Fatalf("suite failed: %+v", report.Suites[0])
	}
}

func TestRunCoreCertNegativeZeroMatchProbe(t *testing.T) {
	root := testkit.ModuleRoot(t)
	report, err := RunCore(t.Context(), RunOptions{
		RepoRoot: root,
		Filter:   "integration.cert-negative-zero-match",
	})
	if err == nil {
		t.Fatal("expected probe failure")
	}
	if report.Passed {
		t.Fatalf("report=%+v", report)
	}
	if len(report.Suites) != 1 || report.Suites[0].Status != StatusFailed {
		t.Fatalf("suites=%+v", report.Suites)
	}
	if report.SchemaVersion != ReportSchemaVersion {
		t.Fatalf("schema=%d", report.SchemaVersion)
	}
	if report.CommitSHA == "" {
		t.Fatal("missing commitSHA")
	}
}

func TestRunCoreCertNegativeForcedSkipProbe(t *testing.T) {
	root := testkit.ModuleRoot(t)
	report, err := RunCore(t.Context(), RunOptions{
		RepoRoot: root,
		Filter:   "integration.cert-negative-forced-skip",
	})
	if err == nil {
		t.Fatal("expected probe failure")
	}
	if report.Passed {
		t.Fatalf("report=%+v", report)
	}
	if len(report.Suites) != 1 || report.Suites[0].Status != StatusFailed {
		t.Fatalf("suites=%+v", report.Suites)
	}
}

func TestFilterSuitesPrefix(t *testing.T) {
	suites := []Suite{{ID: "lock-bridge-npm"}, {ID: "lock-bridge-yarn-identity"}}
	got := FilterSuites(suites, "lock-bridge-n")
	if len(got) != 1 || got[0].ID != "lock-bridge-npm" {
		t.Fatalf("got=%v", got)
	}
}

func TestReportPassed(t *testing.T) {
	tests := []struct {
		name     string
		suites   []SuiteResult
		dryRun   bool
		wantPass bool
	}{
		{
			name:     "empty fails",
			suites:   nil,
			wantPass: false,
		},
		{
			name: "single required pass succeeds",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusPassed},
			},
			wantPass: true,
		},
		{
			name: "single required fail fails",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusFailed},
			},
			wantPass: false,
		},
		{
			name: "single required skipped fails",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusSkipped},
			},
			wantPass: false,
		},
		{
			name: "required not-applicable succeeds",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusNotApplicable},
			},
			wantPass: true,
		},
		{
			name: "all optional fails",
			suites: []SuiteResult{
				{ID: "a", Required: false, Status: StatusPassed},
				{ID: "b", Required: false, Status: StatusPassed},
			},
			wantPass: false,
		},
		{
			name: "mixed required+optional with required pass succeeds",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusPassed},
				{ID: "b", Required: false, Status: StatusFailed},
			},
			wantPass: true,
		},
		{
			name: "mixed required fail + optional pass fails",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusFailed},
				{ID: "b", Required: false, Status: StatusPassed},
			},
			wantPass: false,
		},
		{
			name: "dry run required planned succeeds",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusPlanned},
			},
			dryRun:   true,
			wantPass: true,
		},
		{
			name: "dry run required not-applicable succeeds",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusNotApplicable},
			},
			dryRun:   true,
			wantPass: true,
		},
		{
			name: "dry run required failed fails",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusFailed},
			},
			dryRun:   true,
			wantPass: false,
		},
		{
			name: "dry run required skipped fails",
			suites: []SuiteResult{
				{ID: "a", Required: true, Status: StatusSkipped},
			},
			dryRun:   true,
			wantPass: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reportPassed(tc.suites, tc.dryRun)
			if got != tc.wantPass {
				t.Fatalf("reportPassed() = %v, want %v", got, tc.wantPass)
			}
		})
	}
}

func TestLoadManifestDuplicateID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	data := `{"schemaVersion":1,"matrix":"core","suites":[{"id":"dup","title":"a","package":"./x","run":"Test"},{"id":"dup","title":"b","package":"./y","run":"Test"}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	data := `{"schemaVersion":1,"matrix":"core","suites":[{"id":"a","title":"a","package":"./x","run":"[invalid"}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected invalid regex error")
	}
	if !strings.Contains(err.Error(), "invalid run regex") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestInvalidPlatform(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	data := `{"schemaVersion":1,"matrix":"core","suites":[{"id":"a","title":"a","package":"./x","run":"Test","platforms":["freebsd"]}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected invalid platform error")
	}
	if !strings.Contains(err.Error(), "invalid platform") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadManifestUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	data := `{"schemaVersion":1,"matrix":"core","suites":[{"id":"a","title":"a","package":"./x","run":"Test"}],"bogus":true}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestLoadManifestEmptySuites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	data := `{"schemaVersion":1,"matrix":"core","suites":[]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected empty suites error")
	}
	if !strings.Contains(err.Error(), "no suites") {
		t.Fatalf("unexpected error: %v", err)
	}
}
