package conformance

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mewisme/mew/internal/testkit"
)

func TestLoadRunnerManifest(t *testing.T) {
	root := testkit.ModuleRoot(t)
	m, err := LoadRunnerManifest(RunnerManifestPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if m.Matrix != RunnerMatrix || len(m.Suites) < 20 {
		t.Fatalf("manifest=%+v", m)
	}
	for _, suite := range m.Suites {
		if !runnerSuiteSupportedOnPlatform(suite, runtime.GOOS) {
			continue
		}
		if err := ValidateExpectedTestsRegex(root, suite); err != nil {
			t.Fatalf("suite %s: %v", suite.ID, err)
		}
	}
}

func TestRunnerManifestDigestsStable(t *testing.T) {
	root := testkit.ModuleRoot(t)
	m, err := LoadRunnerManifest(RunnerManifestPath(root))
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadWaiverManifest(RunnerWaiverPath(root))
	if err != nil {
		t.Fatal(err)
	}
	md, err := RunnerManifestDigest(m)
	if err != nil || len(md) != 64 {
		t.Fatalf("manifest digest=%q err=%v", md, err)
	}
	wd, err := RunnerWaiverManifestDigest(w)
	if err != nil || len(wd) != 64 {
		t.Fatalf("waiver digest=%q err=%v", wd, err)
	}
}

func TestSelectRunnerSuitesIntersection(t *testing.T) {
	root := testkit.ModuleRoot(t)
	m, err := LoadRunnerManifest(RunnerManifestPath(root))
	if err != nil {
		t.Fatal(err)
	}
	selected, err := SelectRunnerSuites(m.Suites, []string{"dispatch"}, []string{"runner-dispatch-collisions"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].ID != "runner-dispatch-collisions" {
		t.Fatalf("selected=%v", selected)
	}
}

func TestValidateRunRegexAnchored(t *testing.T) {
	if err := validateRunRegex("TestFoo"); err == nil {
		t.Fatal("expected anchored regex error")
	}
	if err := validateRunRegex(`^(TestA\|TestB)$`); err == nil {
		t.Fatal("expected escaped pipe error")
	}
	if err := validateRunRegex("^(TestA|TestB)$"); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerManifestPath(t *testing.T) {
	root := testkit.ModuleRoot(t)
	want := filepath.Join(root, "tests", "conformance", "runner-matrix", "manifest.json")
	if RunnerManifestPath(root) != want {
		t.Fatalf("got %s", RunnerManifestPath(root))
	}
}

func TestRunnerOverallResult(t *testing.T) {
	tests := []struct {
		name   string
		suites []RunnerSuiteResult
		want   string
	}{
		{
			name: "empty fails",
			want: RunnerResultFail,
		},
		{
			name: "single required pass succeeds",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Result: RunnerResultPass},
			},
			want: RunnerResultPass,
		},
		{
			name: "single required fail fails",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Result: RunnerResultFail},
			},
			want: RunnerResultFail,
		},
		{
			name: "single required skip fails",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Result: RunnerResultSkip},
			},
			want: RunnerResultFail,
		},
		{
			name: "single required not-run fails",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Result: RunnerResultNotRun},
			},
			want: RunnerResultFail,
		},
		{
			name: "single required timeout fails",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Result: RunnerResultTimeout},
			},
			want: RunnerResultFail,
		},
		{
			name: "single required not-applicable succeeds",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Result: RunnerResultNotApplicable},
			},
			want: RunnerResultPass,
		},
		{
			name: "single required pass-with-waiver succeeds",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Result: RunnerResultPassWithWaiver},
			},
			want: RunnerResultPass,
		},
		{
			name: "required probe skip succeeds",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Probe: true, Result: RunnerResultProbeSkip},
			},
			want: RunnerResultPass,
		},
		{
			name: "required non-probe probe-skip fails",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Probe: false, Result: RunnerResultProbeSkip},
			},
			want: RunnerResultFail,
		},
		{
			name: "all optional no required fails",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: false, Result: RunnerResultPass},
			},
			want: RunnerResultFail,
		},
		{
			name: "required pass + optional fail fails overall",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Result: RunnerResultPass},
				{ID: "b", Required: false, Result: RunnerResultFail},
			},
			want: RunnerResultFail,
		},
		{
			name: "required fail and optional pass fails",
			suites: []RunnerSuiteResult{
				{ID: "a", Required: true, Result: RunnerResultFail},
				{ID: "b", Required: false, Result: RunnerResultPass},
			},
			want: RunnerResultFail,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runnerOverallResult(tc.suites)
			if got != tc.want {
				t.Fatalf("runnerOverallResult() = %q, want %q", got, tc.want)
			}
		})
	}
}
