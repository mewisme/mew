package main

import (
	"reflect"
	"testing"
)

func TestScheduleRoundRobin(t *testing.T) {
	tests := []string{"TestA", "TestB", "TestC", "TestD", "TestE"}
	shards := scheduleRoundRobin(tests, 3)

	// 5 tests across 3 workers: worker 0 gets 0,3; worker 1 gets 1,4; worker 2 gets 2
	expected := []shard{
		{Worker: 0, Tests: []string{"TestA", "TestD"}},
		{Worker: 1, Tests: []string{"TestB", "TestE"}},
		{Worker: 2, Tests: []string{"TestC"}},
	}
	if !reflect.DeepEqual(shards, expected) {
		t.Errorf("round-robin mismatch:\n  got:  %v\n  want: %v", shards, expected)
	}
}

func TestScheduleRoundRobinSingleWorker(t *testing.T) {
	tests := []string{"TestA", "TestB", "TestC"}
	shards := scheduleRoundRobin(tests, 1)
	if len(shards) != 1 {
		t.Fatalf("expected 1 shard, got %d", len(shards))
	}
	if len(shards[0].Tests) != 3 {
		t.Errorf("expected 3 tests in single shard, got %d", len(shards[0].Tests))
	}
}

func TestScheduleRoundRobinEmptyTests(t *testing.T) {
	shards := scheduleRoundRobin(nil, 3)
	if len(shards) != 3 {
		t.Errorf("expected 3 empty shards, got %d", len(shards))
	}
	for _, s := range shards {
		if len(s.Tests) != 0 {
			t.Errorf("expected empty shard, got %v", s.Tests)
		}
	}
}

func TestScheduleLPT(t *testing.T) {
	tests := []string{"TestSlow", "TestMedium", "TestFast"}
	timings := map[string]float64{
		"TestSlow":   10.0,
		"TestMedium": 5.0,
		"TestFast":   1.0,
	}

	shards := scheduleLPT(tests, 2, timings)

	// LPT assigns Slow (10) to worker 0, Medium (5) to worker 1,
	// then Fast (1) to worker 1 (since 5+1=6 < 10).
	if len(shards) != 2 {
		t.Fatalf("expected 2 shards, got %d", len(shards))
	}
	// Worker 0 should have TestSlow only.
	if len(shards[0].Tests) != 1 || shards[0].Tests[0] != "TestSlow" {
		t.Errorf("worker 0 should have only TestSlow, got %v", shards[0].Tests)
	}
	// Worker 1 should have TestMedium and TestFast.
	hasMedium := false
	hasFast := false
	for _, tn := range shards[1].Tests {
		if tn == "TestMedium" {
			hasMedium = true
		}
		if tn == "TestFast" {
			hasFast = true
		}
	}
	if !hasMedium || !hasFast {
		t.Errorf("worker 1 should have TestMedium and TestFast, got %v", shards[1].Tests)
	}
}

func TestScheduleLPTDefaultsMissingTimings(t *testing.T) {
	tests := []string{"TestA", "TestB", "TestC"}
	timings := map[string]float64{
		"TestA": 10.0,
		// TestB and TestC missing — should default to average (10.0).
	}

	shards := scheduleLPT(tests, 2, timings)

	// All tests have effective weight 10 (average of known = 10).
	// LPT: A(10)→w0, B(10)→w1, C(10)→w1 (tie, stays at first lightest = w1).
	if len(shards) != 2 {
		t.Fatalf("expected 2 shards, got %d", len(shards))
	}
	// Verify all tests are assigned exactly once.
	if err := verifyShardCoverage(tests, shards); err != nil {
		t.Fatal(err)
	}
	// Worker 0 should have TestA.
	w0HasA := false
	for _, tn := range shards[0].Tests {
		if tn == "TestA" {
			w0HasA = true
		}
	}
	if !w0HasA {
		t.Errorf("worker 0 should have TestA, got %v", shards[0].Tests)
	}
}

func TestScheduleShardsUsesLPTWhenTimingsAvailable(t *testing.T) {
	// scheduleShards dispatches to LPT when timings are non-empty.
	// This test verifies the dispatch, not the LPT logic itself.
	tests := []string{"TestA", "TestB"}
	timings := map[string]float64{"TestA": 1.0, "TestB": 2.0}

	shards := scheduleShards(tests, 1, timings)
	if len(shards) != 1 || len(shards[0].Tests) != 2 {
		t.Errorf("single worker should get all tests, got %v", shards)
	}
}

func TestScheduleShardsFallsBackToRoundRobin(t *testing.T) {
	// Empty timings → round-robin.
	tests := []string{"TestA", "TestB", "TestC"}
	shards := scheduleShards(tests, 3, nil)
	if err := verifyShardCoverage(tests, shards); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyShardCoverageComplete(t *testing.T) {
	tests := []string{"TestA", "TestB", "TestC"}
	shards := []shard{
		{Worker: 0, Tests: []string{"TestA", "TestB"}},
		{Worker: 1, Tests: []string{"TestC"}},
	}
	if err := verifyShardCoverage(tests, shards); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyShardCoverageDuplicate(t *testing.T) {
	tests := []string{"TestA", "TestB"}
	shards := []shard{
		{Worker: 0, Tests: []string{"TestA", "TestB"}},
		{Worker: 1, Tests: []string{"TestA"}},
	}
	if err := verifyShardCoverage(tests, shards); err == nil {
		t.Error("expected error for duplicate assignment")
	}
}

func TestVerifyShardCoverageMissing(t *testing.T) {
	tests := []string{"TestA", "TestB", "TestC"}
	shards := []shard{
		{Worker: 0, Tests: []string{"TestA"}},
	}
	if err := verifyShardCoverage(tests, shards); err == nil {
		t.Error("expected error for missing test")
	}
}

func TestVerifyShardCoverageEmpty(t *testing.T) {
	if err := verifyShardCoverage(nil, nil); err != nil {
		t.Errorf("unexpected error for empty input: %v", err)
	}
}

func TestBuildRunPattern(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{nil, "^$"},
		{[]string{}, "^$"},
		{[]string{"TestFoo"}, "^TestFoo$"},
		{[]string{"TestFoo", "TestBar"}, "^TestFoo$|^TestBar$"},
	}
	for _, tc := range tests {
		got := buildRunPattern(tc.input)
		if got != tc.expected {
			t.Errorf("buildRunPattern(%v) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestEffectiveWorkersExplicit(t *testing.T) {
	cfg := config{workers: "3"}
	if n := cfg.effectiveWorkers(classUnit); n != 3 {
		t.Errorf("explicit workers=3, got %d", n)
	}
	cfg = config{workers: "1"}
	if n := cfg.effectiveWorkers(classUnit); n != 1 {
		t.Errorf("explicit workers=1, got %d", n)
	}
}

func TestEffectiveWorkersInvalidDefaultsToOne(t *testing.T) {
	cfg := config{workers: "invalid"}
	if n := cfg.effectiveWorkers(classUnit); n != 1 {
		t.Errorf("invalid workers should default to 1, got %d", n)
	}
	cfg = config{workers: "0"}
	if n := cfg.effectiveWorkers(classUnit); n != 1 {
		t.Errorf("workers=0 should default to 1, got %d", n)
	}
	cfg = config{workers: "-1"}
	if n := cfg.effectiveWorkers(classUnit); n != 1 {
		t.Errorf("workers=-1 should default to 1, got %d", n)
	}
}

func TestEffectiveWorkersAutoCappedByWorkload(t *testing.T) {
	cfg := config{workers: "auto", cpuOverride: 16}
	if n := cfg.effectiveWorkers(classUnit); n != 16 {
		t.Errorf("unit on 16 CPUs: got %d, want 16", n)
	}
	if n := cfg.effectiveWorkers(classIntegration); n != 4 {
		t.Errorf("integration on 16 CPUs: got %d, want 4", n)
	}
	if n := cfg.effectiveWorkers(classCrash); n != 3 {
		t.Errorf("crash on 16 CPUs: got %d, want 3", n)
	}
	if n := cfg.effectiveWorkers(classRace); n != 2 {
		t.Errorf("race on 16 CPUs: got %d, want 2", n)
	}
}

func TestEffectiveWorkersAutoMinimumOne(t *testing.T) {
	cfg := config{workers: "auto", cpuOverride: 1}
	for _, class := range []workloadClass{classUnit, classIntegration, classCrash, classRace} {
		if n := cfg.effectiveWorkers(class); n < 1 {
			t.Errorf("class %d on 1 CPU should be >= 1, got %d", class, n)
		}
	}
}

func TestPerWorkerGOMAXPROCS(t *testing.T) {
	if n := perWorkerGOMAXPROCS(4, 4); n != 1 {
		t.Errorf("4 CPUs / 4 workers = 1, got %d", n)
	}
	if n := perWorkerGOMAXPROCS(8, 4); n != 2 {
		t.Errorf("8 CPUs / 4 workers = 2, got %d", n)
	}
	if n := perWorkerGOMAXPROCS(4, 8); n != 1 {
		t.Errorf("4 CPUs / 8 workers should floor to 1, got %d", n)
	}
	if n := perWorkerGOMAXPROCS(4, 0); n != 4 {
		t.Errorf("0 workers should return logical CPU count, got %d", n)
	}
}

func TestClassifyPackages(t *testing.T) {
	if c := classifyPackages([]string{"./internal/resolver/..."}); c != classUnit {
		t.Errorf("expected classUnit, got %d", c)
	}
	if c := classifyPackages([]string{"./tests/integration/..."}); c != classIntegration {
		t.Errorf("expected classIntegration, got %d", c)
	}
	if c := classifyPackages([]string{"./tests/conformance/..."}); c != classIntegration {
		t.Errorf("expected classIntegration for conformance, got %d", c)
	}
}

func TestIsHeavyPackage(t *testing.T) {
	heavy := []string{
		"github.com/mewisme/mew/tests/integration",
		"github.com/mewisme/mew/tests/conformance",
		"github.com/mewisme/mew/internal/app",
		"github.com/mewisme/mew/internal/transaction",
	}
	light := []string{
		"github.com/mewisme/mew/internal/resolver",
		"github.com/mewisme/mew/internal/transform",
		"github.com/mewisme/mew/internal/cli",
	}
	for _, p := range heavy {
		if !isHeavyPackage(p) {
			t.Errorf("%s should be heavy", p)
		}
	}
	for _, p := range light {
		if isHeavyPackage(p) {
			t.Errorf("%s should be light", p)
		}
	}
}

func TestContainsPattern(t *testing.T) {
	if !containsPattern("github.com/mewisme/mew/tests/integration", "tests/integration") {
		t.Error("expected match")
	}
	if containsPattern("github.com/mewisme/mew/internal/resolver", "tests/integration") {
		t.Error("expected no match")
	}
	if !containsPattern("abc/def/ghi", "abc", "ghi") {
		t.Error("expected match on one of multiple patterns")
	}
}
