package conformance

import (
	"path/filepath"
	"testing"

	"github.com/mewisme/mew/internal/testkit"
)

func TestLoadCLIUXManifest(t *testing.T) {
	root := testkit.ModuleRoot(t)
	path := filepath.Join(root, "tests", "conformance", "cli-ux", "manifest.json")
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != SchemaVersion || m.Matrix != CLIUXMatrix {
		t.Fatalf("manifest=%+v", m)
	}
	if len(m.Suites) < 8 {
		t.Fatalf("expected cli-ux suites, got %d", len(m.Suites))
	}
	var sawCrosslink bool
	for _, s := range m.Suites {
		if s.ID == "runner-crosslink" {
			sawCrosslink = true
			if s.Run != "TestCLIUXRunnerMatrixCrosslink" {
				t.Fatalf("crosslink run=%q", s.Run)
			}
		}
	}
	if !sawCrosslink {
		t.Fatal("missing runner-crosslink suite")
	}
}

func TestRunCLIUXDryRun(t *testing.T) {
	root := testkit.ModuleRoot(t)
	report, err := RunCLIUX(t.Context(), RunOptions{RepoRoot: root, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || !report.DryRun || report.Matrix != CLIUXMatrix || len(report.Suites) == 0 {
		t.Fatalf("report=%+v", report)
	}
	for _, s := range report.Suites {
		switch s.Status {
		case StatusPlanned:
			// dry-run inventory
		case StatusNotApplicable:
			if s.SkipReason != "unsupported platform" {
				t.Fatalf("suite %s status=not-applicable reason=%q want unsupported platform", s.ID, s.SkipReason)
			}
		default:
			t.Fatalf("suite %s status=%s want planned (or platform-not-applicable)", s.ID, s.Status)
		}
	}
}

func TestCLIUXRunnerMatrixCrosslink(t *testing.T) {
	root := testkit.ModuleRoot(t)
	m, err := LoadRunnerManifest(RunnerManifestPath(root))
	if err != nil {
		t.Fatal(err)
	}
	need := map[string]bool{
		"runner-stdio":           false,
		"runner-signals-unix":    false,
		"runner-signals-windows": false,
	}
	for _, s := range m.Suites {
		if _, ok := need[s.ID]; ok {
			need[s.ID] = true
		}
	}
	for id, found := range need {
		if !found {
			t.Fatalf("runner-matrix missing %s (cli-ux cross-links; do not duplicate)", id)
		}
	}
}
