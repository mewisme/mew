package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/testkit"
	"github.com/mewisme/mew/internal/transaction"
)

func setupDoctorHealthyProject(t *testing.T) (*Context, string) {
	t.Helper()
	testkit.CleanEnv(t)
	t.Setenv("NO_PROXY", "*")

	projDir := t.TempDir()
	fixture := filepath.Join(testkit.ModuleRoot(t), "fixtures", "projects", "mlock-greenfield")
	for _, name := range []string{"package.json", "m.lock"} {
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projDir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfgPath := filepath.Join(projDir, "m.jsonc")
	if err := os.WriteFile(cfgPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ac, err := New(ctx, Options{CWD: projDir, ConfigPath: cfgPath, Env: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	return ac, projDir
}

func TestDoctorHealthyMlockProject(t *testing.T) {
	ac, _ := setupDoctorHealthyProject(t)
	report, err := Doctor(context.Background(), ac, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected ok report: %+v", report)
	}
	for _, id := range []string{"project", "config", "cache", "store", "lock", "filesystem", "transaction", "node"} {
		if findDoctorCheck(report, id) == nil {
			t.Fatalf("missing check %q: %+v", id, report.Checks)
		}
	}
	if findDoctorCheck(report, "project").Status != string(DoctorStatusOK) {
		t.Fatalf("project=%+v", findDoctorCheck(report, "project"))
	}
	if findDoctorCheck(report, "lock").Status != string(DoctorStatusOK) {
		t.Fatalf("lock=%+v", findDoctorCheck(report, "lock"))
	}
}

func TestDoctorStrictTreatsTxnWarnAsFailure(t *testing.T) {
	ac, projDir := setupDoctorHealthyProject(t)
	txnRoot := filepath.Join(projDir, ".mew", "txn", "orphan-test")
	if err := os.MkdirAll(filepath.Join(txnRoot, "stage"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := &transaction.Document{
		SchemaVersion: transaction.SchemaVersion,
		ID:            "orphan-test",
		ProjectRoot:   projDir,
		State:         transaction.StateStaging,
	}
	data, err := transaction.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(txnRoot, "journal.000001.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Doctor(context.Background(), ac, DoctorOptions{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatal("expected strict failure with orphan txn")
	}
	txn := findDoctorCheck(report, "transaction")
	if txn == nil || txn.Status != string(DoctorStatusWarn) {
		t.Fatalf("transaction=%+v", txn)
	}
}

func TestDoctorFailsWithoutLock(t *testing.T) {
	ac, projDir := setupDoctorHealthyProject(t)
	if err := os.Remove(filepath.Join(projDir, "m.lock")); err != nil {
		t.Fatal(err)
	}

	report, err := Doctor(context.Background(), ac, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatal("expected failure without lock")
	}
	lock := findDoctorCheck(report, "lock")
	if lock == nil || lock.Status != string(DoctorStatusFail) {
		t.Fatalf("lock=%+v", lock)
	}
}

func findDoctorCheck(rep DoctorReport, id string) *DoctorCheck {
	for i := range rep.Checks {
		if rep.Checks[i].ID == id {
			return &rep.Checks[i]
		}
	}
	return nil
}

func TestDoctorRuntime(t *testing.T) {
	ac, _ := setupDoctorHealthyProject(t)
	report, err := DoctorRuntime(context.Background(), ac, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// OK depends on environment: may have warnings (missing tsconfig, empty cache).
	// Verify required checks are present and critical ones pass.
	required := []string{
		"node-capabilities",
		"transform-handshake",
		"transform-roundtrip",
		"source-map",
		"tsconfig",
		"loader-bridge",
		"watch-backend",
		"inspector",
		"worker",
	}
	for _, id := range required {
		if findDoctorCheck(report, id) == nil {
			t.Fatalf("missing check %q", id)
		}
	}
	// These probes should always pass in a functioning environment.
	for _, id := range []string{"node-capabilities", "transform-handshake", "transform-roundtrip", "source-map", "loader-bridge", "watch-backend", "inspector", "worker"} {
		c := findDoctorCheck(report, id)
		if c == nil {
			t.Fatalf("missing check %q", id)
		}
		if c.Status != string(DoctorStatusOK) {
			t.Errorf("%s: %s — %s", id, c.Status, c.Message)
		}
	}
	// tsconfig and runtime-cache may warn in test environments without those assets.
}

func TestDoctorRuntimeStrict(t *testing.T) {
	ac, _ := setupDoctorHealthyProject(t)
	report, err := DoctorRuntime(context.Background(), ac, DoctorOptions{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = report
}

func TestDoctorSkippedStatus(t *testing.T) {
	// Verify DoctorStatusSkipped exists and doesn't cause fail/warn aggregation.
	rep := DoctorReport{SchemaVersion: DoctorReportSchemaVersion}
	rep.Checks = append(rep.Checks, DoctorCheck{
		ID: "ok-check", Status: string(DoctorStatusOK), Message: "ok",
	})
	rep.Checks = append(rep.Checks, DoctorCheck{
		ID: "skipped-check", Status: string(DoctorStatusSkipped), Message: "not applicable", Details: "no tsconfig",
	})
	rep.OK = !reportHasStatus(rep, DoctorStatusFail)

	if !rep.OK {
		t.Fatal("report with only ok+skipped should be ok")
	}
}

func TestDoctorFailAggregation(t *testing.T) {
	rep := DoctorReport{SchemaVersion: DoctorReportSchemaVersion}
	rep.Checks = append(rep.Checks, DoctorCheck{
		ID: "ok-check", Status: string(DoctorStatusOK), Message: "ok",
	})
	rep.Checks = append(rep.Checks, DoctorCheck{
		ID: "fail-check", Status: string(DoctorStatusFail), Message: "broken",
	})
	rep.OK = !reportHasStatus(rep, DoctorStatusFail)

	if rep.OK {
		t.Fatal("report with a failure should not be ok")
	}
}

func TestDoctorStrictTurnsWarnIntoFailure(t *testing.T) {
	rep := DoctorReport{SchemaVersion: DoctorReportSchemaVersion}
	rep.Checks = append(rep.Checks, DoctorCheck{
		ID: "ok-check", Status: string(DoctorStatusOK), Message: "ok",
	})
	rep.Checks = append(rep.Checks, DoctorCheck{
		ID: "warn-check", Status: string(DoctorStatusWarn), Message: "caution",
	})
	rep.OK = !reportHasStatus(rep, DoctorStatusFail)
	if !rep.OK {
		t.Fatal("report with only warn should be ok in non-strict mode")
	}

	// Strict mode: warnings count as failures.
	if !reportHasStatus(rep, DoctorStatusWarn) {
		t.Fatal("expected warn status")
	}
	// In strict mode, OK should be false.
	strictOK := !reportHasStatus(rep, DoctorStatusFail) && !reportHasStatus(rep, DoctorStatusWarn)
	if strictOK {
		t.Fatal("strict mode should turn warn into failure")
	}
}

func TestDoctorRuntimeHasTransformHandshake(t *testing.T) {
	ac, _ := setupDoctorHealthyProject(t)
	report, err := DoctorRuntime(context.Background(), ac, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c := findDoctorCheck(report, "transform-handshake")
	if c == nil {
		t.Fatal("missing transform-handshake check")
	}
	if c.Status != string(DoctorStatusOK) {
		t.Fatalf("transform-handshake: %s — %s", c.Status, c.Message)
	}
}

func TestDoctorRuntimeHasSourceMap(t *testing.T) {
	ac, _ := setupDoctorHealthyProject(t)
	report, err := DoctorRuntime(context.Background(), ac, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c := findDoctorCheck(report, "source-map")
	if c == nil {
		t.Fatal("missing source-map check")
	}
	if c.Status != string(DoctorStatusOK) {
		t.Fatalf("source-map: %s — %s", c.Status, c.Message)
	}
}

func TestDoctorRuntimeHasWatchBackend(t *testing.T) {
	report, err := DoctorRuntime(context.Background(), nil, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c := findDoctorCheck(report, "watch-backend")
	if c == nil {
		t.Fatal("missing watch-backend check")
	}
	if c.Status != string(DoctorStatusOK) {
		t.Fatalf("watch-backend: %s — %s", c.Status, c.Message)
	}
}

func TestDoctorRuntimeHasInspector(t *testing.T) {
	report, err := DoctorRuntime(context.Background(), nil, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c := findDoctorCheck(report, "inspector")
	if c == nil {
		t.Fatal("missing inspector check")
	}
	if c.Status != string(DoctorStatusOK) {
		t.Fatalf("inspector: %s — %s", c.Status, c.Message)
	}
}

func TestDoctorRuntimeHasWorker(t *testing.T) {
	report, err := DoctorRuntime(context.Background(), nil, DoctorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c := findDoctorCheck(report, "worker")
	if c == nil {
		t.Fatal("missing worker check")
	}
	// Worker may be ok or warn depending on environment, but never fail on modern Node.
	if c.Status == string(DoctorStatusFail) {
		t.Errorf("worker probe unexpectedly failed: %s", c.Message)
	}
}

func TestDoctorSummaryFormat(t *testing.T) {
	rep := DoctorReport{
		SchemaVersion: DoctorReportSchemaVersion,
		OK:            true,
		Checks: []DoctorCheck{
			{ID: "a", Status: string(DoctorStatusOK), Message: "all good"},
		},
	}
	out := FormatDoctorReport(rep)
	if !strings.Contains(out, "doctor=ok") {
		t.Errorf("expected doctor=ok in output: %s", out)
	}
}

func TestDoctorSummaryFormatFailed(t *testing.T) {
	rep := DoctorReport{
		SchemaVersion: DoctorReportSchemaVersion,
		OK:            false,
		Checks: []DoctorCheck{
			{ID: "a", Status: string(DoctorStatusFail), Message: "broken", Details: "oops", Remediation: "fix it"},
		},
	}
	out := FormatDoctorReport(rep)
	if !strings.Contains(out, "doctor=failed") {
		t.Errorf("expected doctor=failed in output: %s", out)
	}
	if !strings.Contains(out, "details=oops") {
		t.Errorf("expected details in output: %s", out)
	}
	if !strings.Contains(out, "remediation=fix it") {
		t.Errorf("expected remediation in output: %s", out)
	}
}
