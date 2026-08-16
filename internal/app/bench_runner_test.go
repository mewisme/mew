package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunnerBenchCasesSmokeDefault(t *testing.T) {
	cases := runnerBenchCases(RunnerBenchOptions{Profile: RunnerBenchProfileSmoke})
	if len(cases) != 1 || cases[0].ID != "project-script" {
		t.Fatalf("smoke cases = %#v, want project-script only", cases)
	}
}

func TestRunnerBenchCasesFullProfile(t *testing.T) {
	cases := runnerBenchCases(RunnerBenchOptions{Profile: RunnerBenchProfileFull})
	if len(cases) != 2 {
		t.Fatalf("full profile len = %d, want 2", len(cases))
	}
}

func TestRunnerBenchCasesCaseID(t *testing.T) {
	cases := runnerBenchCases(RunnerBenchOptions{CaseID: "dlx-warm"})
	if len(cases) != 1 || cases[0].ID != "dlx-warm" {
		t.Fatalf("case override = %#v", cases)
	}
}

func TestCompareRunnerBaselineComparable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(path, []byte(`{
  "schemaVersion": 1,
  "fixtureDigest": "x",
  "commandVersion": "runner-bench-v1",
  "environment": {"os":"linux","arch":"amd64","machineClass":"ci","goVersion":"go1.26","nodeVersion":"v22"},
  "recordedAt": "2026-07-31T00:00:00Z",
  "cases": []
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmp, err := compareRunnerBaseline(path, RunnerBenchResult{
		Environment: RunnerBenchEnvironment{OS: "linux", Arch: "amd64"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Status != "pass" {
		t.Fatalf("status = %q, want pass", cmp.Status)
	}
}

func TestCompareRunnerBaselineNotComparable(t *testing.T) {
	dir := t.TempDir()

	// Incompatible schema version.
	path := filepath.Join(dir, "bad-schema.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion": 99}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := compareRunnerBaseline(path, RunnerBenchResult{})
	if err == nil {
		t.Fatal("expected error for incompatible schema version")
	}

	// Incompatible OS.
	path2 := filepath.Join(dir, "bad-os.json")
	if err := os.WriteFile(path2, []byte(`{
  "schemaVersion": 1,
  "commandVersion": "runner-bench-v1",
  "environment": {"os": "darwin", "arch": "amd64"},
  "cases": []
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = compareRunnerBaseline(path2, RunnerBenchResult{
		Environment: RunnerBenchEnvironment{OS: "linux", Arch: "amd64"},
	})
	if err == nil {
		t.Fatal("expected error for incompatible OS")
	}
}

func TestCompareRunnerBaselineRegression(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(path, []byte(`{
  "schemaVersion": 1,
  "commandVersion": "runner-bench-v1",
  "environment": {"os":"linux","arch":"amd64"},
  "cases": [{"id":"startup","medianNs":100,"p95Ns":120}],
  "thresholdPct": 10
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmp, err := compareRunnerBaseline(path, RunnerBenchResult{
		Environment: RunnerBenchEnvironment{OS: "linux", Arch: "amd64"},
		Cases: []RunnerBenchCase{
			{ID: "startup", MedianNs: 150, P95Ns: 180},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Status != "regression" {
		t.Fatalf("status = %q, want regression", cmp.Status)
	}
	if len(cmp.Details) != 1 || cmp.Details[0].Verdict != "regression" {
		t.Fatalf("details = %+v", cmp.Details)
	}
}

func TestCompareRunnerBaselineMissingCase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(path, []byte(`{
  "schemaVersion": 1,
  "commandVersion": "runner-bench-v1",
  "environment": {"os":"linux","arch":"amd64"},
  "cases": []
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := compareRunnerBaseline(path, RunnerBenchResult{
		Environment: RunnerBenchEnvironment{OS: "linux", Arch: "amd64"},
		Cases:       []RunnerBenchCase{{ID: "startup", MedianNs: 100}},
	})
	if err == nil {
		t.Fatal("expected error for missing case in baseline")
	}
}

func TestBenchRunnerMutuallyExclusiveCaseProfile(t *testing.T) {
	_, err := BenchRunner(t.Context(), &Context{}, RunnerBenchOptions{
		Profile: RunnerBenchProfileFull,
		CaseID:  "dlx-warm",
	})
	if err == nil {
		t.Fatal("expected error for case + profile")
	}
}

func TestBenchRunnerInvalidSamples(t *testing.T) {
	// Negative samples must fail (validated before any real work).
	_, err := BenchRunner(t.Context(), &Context{}, RunnerBenchOptions{
		Profile: RunnerBenchProfileSmoke,
		Samples: -1,
	})
	if err == nil {
		t.Fatal("expected error for negative samples")
	}
}

func TestBenchRunnerInvalidWarmup(t *testing.T) {
	// Negative warmup must fail.
	_, err := BenchRunner(t.Context(), &Context{}, RunnerBenchOptions{
		Profile: RunnerBenchProfileSmoke,
		Samples: 1,
		Warmup:  -1,
	})
	if err == nil {
		t.Fatal("expected error for negative warmup")
	}
}

func TestRunnerBenchEnvironment(t *testing.T) {
	env := runnerBenchEnvironment("abc123")
	if env.OS == "" || env.Arch == "" || env.GoVersion == "" {
		t.Fatalf("incomplete environment: %+v", env)
	}
	if env.Commit != "abc123" {
		t.Fatalf("commit = %q, want abc123", env.Commit)
	}
	if env.LogicalCPUs <= 0 {
		t.Fatalf("logicalCpus = %d, want > 0", env.LogicalCPUs)
	}
}
