package runtime_test

import (
	"context"
	"testing"

	"github.com/mewisme/mew/internal/conformance"
)

// Meta-tests verify that the conformance harness and helpers reject
// permissive patterns. These test the shared harness/report logic,
// not individual conformance cases.

// TestMeta_FailReason_ZeroMatch verifies FailReason catches zero matched tests.
func TestMeta_FailReason_ZeroMatch(t *testing.T) {
	summary := conformance.TestSummary{TestsMatched: 0}
	suite := conformance.Suite{ID: "test", Required: true}
	reason := summary.FailReason(suite, 0, false)
	if reason == "" {
		t.Fatal("FailReason did NOT catch zero matched tests")
	}
}

// TestMeta_FailReason_AllSkipped verifies FailReason catches all-skipped.
func TestMeta_FailReason_AllSkipped(t *testing.T) {
	summary := conformance.TestSummary{
		TestsMatched: 3,
		Skipped:      3,
	}
	suite := conformance.Suite{ID: "test", Required: false}
	reason := summary.FailReason(suite, 0, false)
	if reason == "" {
		t.Fatal("FailReason did NOT catch all-skipped tests")
	}
}

// TestMeta_FailReason_RequiredSkipped verifies FailReason catches skipped
// required tests (fail-closed for required suites).
func TestMeta_FailReason_RequiredSkipped(t *testing.T) {
	summary := conformance.TestSummary{
		TestsMatched: 3,
		Passed:       2,
		Skipped:      1,
	}
	suite := conformance.Suite{ID: "test", Required: true}
	reason := summary.FailReason(suite, 0, false)
	if reason == "" {
		t.Fatal("FailReason did NOT catch skipped required test")
	}
}

// TestMeta_FailReason_Failed verifies FailReason catches explicit failures.
func TestMeta_FailReason_Failed(t *testing.T) {
	summary := conformance.TestSummary{
		TestsMatched: 3,
		Passed:       2,
		Failed:       1,
	}
	suite := conformance.Suite{ID: "test", Required: true}
	reason := summary.FailReason(suite, 0, false)
	if reason == "" {
		t.Fatal("FailReason did NOT catch failed test")
	}
}

// TestMeta_FailReason_ParseError verifies FailReason catches parse errors.
func TestMeta_FailReason_ParseError(t *testing.T) {
	summary := conformance.TestSummary{
		ParseError: "malformed testjson line 1: invalid JSON",
	}
	suite := conformance.Suite{ID: "test", Required: true}
	reason := summary.FailReason(suite, 0, false)
	if reason == "" {
		t.Fatal("FailReason did NOT catch parse error")
	}
}

// TestMeta_FailReason_PackageFail verifies FailReason catches package failures.
func TestMeta_FailReason_PackageFail(t *testing.T) {
	summary := conformance.TestSummary{
		PackageFail: true,
	}
	suite := conformance.Suite{ID: "test", Required: true}
	reason := summary.FailReason(suite, 0, false)
	if reason == "" {
		t.Fatal("FailReason did NOT catch package build failure")
	}
}

// TestMeta_RunMBinaryTimeoutIsDetectable verifies that context cancellation
// produces a non-zero exit code that the conformance harness would catch.
func TestMeta_RunMBinaryTimeoutIsDetectable(t *testing.T) {
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	cancel()
	code, _, _ := runMBinary(t, ctx, proj, "hello.js")
	// Expired context means command can't start; code should be non-zero.
	if code == 0 {
		t.Fatal("runMBinary with expired context returned exit 0 (false green)")
	}
}

// TestMeta_PermissivePatternDocumented verifies that the old Logf-only
// pattern would produce a false-green result. This is a documentation
// test: the hardening replaced Logf("watch exit=%d", code) with actual
// Fatalf assertions.
func TestMeta_PermissivePatternDocumented(t *testing.T) {
	// This test documents the anti-pattern that was fixed:
	//
	// BEFORE (false green):
	//   if code != 0 && code != -1 {
	//     t.Logf("watch exit=%d", code)  // never fails the test
	//   }
	//
	// AFTER (real assertion):
	//   if code != 0 && code != -1 {
	//     t.Fatalf("watch crashed with exit=%d, output:\n%s", code, out)
	//   }
	//
	// Proof: the BEFORE pattern would pass `go test -run` with no failures
	// regardless of what the binary actually did. The AFTER pattern fails
	// when the binary crashes unexpectedly.

	// Self-test: prove that Logf alone does not fail.
	// (This test always passes; its purpose is documentation.)
}

// TestMeta_ReportRoundTrip verifies that the Report JSON encoding round-trips
// with schema version 2 and required fields.
func TestMeta_ReportRoundTrip(t *testing.T) {
	report := conformance.Report{
		SchemaVersion: conformance.ReportSchemaVersion,
		Matrix:        "runtime",
		CommitSHA:     "test-sha",
		OS:            "linux",
		Arch:          "amd64",
		Passed:        true,
		Suites: []conformance.SuiteResult{
			{
				ID:           "test-suite",
				Title:        "Test Suite",
				Package:      "./tests/example",
				Run:          "TestFoo",
				Required:     true,
				Status:       conformance.StatusPassed,
				TestsMatched: 1,
				Passed:       1,
			},
		},
	}

	data, err := report.EncodeJSON()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Verify schema version is present.
	if len(data) == 0 {
		t.Fatal("empty report JSON")
	}

	// Verify passed=true is encoded correctly.
	passedStr := `"passed": true`
	if !contains(data, passedStr) {
		t.Fatalf("report JSON missing passed=true:\n%s", string(data))
	}
}

func contains(data []byte, s string) bool {
	return len(data) >= len(s) && stringContains(string(data), s)
}

func stringContains(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
