package benchruntime

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateSamplesRejectsZero verifies zero samples are rejected.
func TestValidateSamplesRejectsZero(t *testing.T) {
	err := validateSamples([]int64{})
	if err == nil {
		t.Fatal("expected error for zero samples")
	}
}

// TestValidateSamplesRejectsInvalid verifies negative, zero, NaN, Inf rejected.
func TestValidateSamplesRejectsInvalid(t *testing.T) {
	tests := []struct {
		name    string
		samples []int64
	}{
		{"negative", []int64{-1}},
		{"zero_value", []int64{0}},
		{"negative_in_slice", []int64{100, -5, 200}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateSamples(tt.samples); err == nil {
				t.Errorf("expected error for %v", tt.samples)
			}
		})
	}
}

// TestValidateIterationsRejectsInvalid verifies invalid iteration/warmup rejected.
func TestValidateIterationsRejectsInvalid(t *testing.T) {
	if err := validateIterations(0, 0); err == nil {
		t.Error("expected error for samples=0")
	}
	if err := validateIterations(1, -1); err == nil {
		t.Error("expected error for warmup=-1")
	}
}

// TestComputeAggregate verifies aggregate computation is deterministic.
func TestComputeAggregate(t *testing.T) {
	samples := []int64{10, 20, 30, 40, 50}
	agg := computeAggregate(samples)
	if agg.MedianNs != 30 {
		t.Errorf("median: want 30, got %d", agg.MedianNs)
	}
	if agg.MinNs != 10 {
		t.Errorf("min: want 10, got %d", agg.MinNs)
	}
	if agg.MaxNs != 50 {
		t.Errorf("max: want 50, got %d", agg.MaxNs)
	}

	// Even-length.
	samples2 := []int64{10, 20, 30, 40}
	agg2 := computeAggregate(samples2)
	if agg2.MedianNs != 30 {
		t.Errorf("median even: want 30, got %d", agg2.MedianNs)
	}
}

// TestSortSamplesDoesNotMutateOriginal verifies sort returns a copy.
func TestSortSamplesDoesNotMutateOriginal(t *testing.T) {
	orig := []int64{30, 10, 20}
	sorted := sortSamples(orig)
	if sorted[0] != 10 || sorted[1] != 20 || sorted[2] != 30 {
		t.Errorf("sort failed: %v", sorted)
	}
	if orig[0] != 30 {
		t.Errorf("original mutated: %v", orig)
	}
}

// TestMeasurementSchemaHasRequiredFields verifies the Measurement JSON schema.
func TestMeasurementSchemaHasRequiredFields(t *testing.T) {
	m := Measurement{
		ID:           MetricTransformLatency,
		Unit:         UnitNanoseconds,
		Category:     CategoryCold,
		RawSamplesNs: []int64{100, 200, 300},
		Aggregate:    Aggregate{MedianNs: 200, MinNs: 100, MaxNs: 300},
		SampleCount:  3,
		WarmupCount:  1,
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "unit", "category", "rawSamplesNs", "aggregate", "sampleCount", "warmupCount"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing required field %q in JSON", key)
		}
	}
	if decoded["unit"] != "ns" {
		t.Errorf("unit: want ns, got %v", decoded["unit"])
	}
}

// TestResultJSONContainsTypedMeasurements verifies JSON structure.
func TestResultJSONContainsTypedMeasurements(t *testing.T) {
	r := Result{
		SchemaVersion: SchemaVersion,
		Environment: Environment{
			GoVersion:   "go1.26.5",
			OS:          "linux",
			Arch:        "amd64",
			LogicalCPUs: 8,
		},
		Measurements: []Measurement{
			{
				ID:           MetricStartupLatency,
				Unit:         UnitNanoseconds,
				Category:     "",
				RawSamplesNs: []int64{1000, 2000, 3000},
				Aggregate:    Aggregate{MedianNs: 2000, MinNs: 1000, MaxNs: 3000},
				SampleCount:  3,
				WarmupCount:  1,
			},
		},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["schemaVersion"].(float64) != float64(SchemaVersion) {
		t.Errorf("schemaVersion mismatch")
	}
	env := decoded["environment"].(map[string]interface{})
	if env["goVersion"] != "go1.26.5" {
		t.Errorf("goVersion missing")
	}
	measurements := decoded["measurements"].([]interface{})
	if len(measurements) != 1 {
		t.Fatalf("expected 1 measurement, got %d", len(measurements))
	}
	m0 := measurements[0].(map[string]interface{})
	rawSamples := m0["rawSamplesNs"].([]interface{})
	if len(rawSamples) != 3 {
		t.Errorf("expected 3 raw samples, got %d", len(rawSamples))
	}
}

// TestHumanAndJSONUseSameModel verifies both outputs derive from same Result.
func TestHumanAndJSONUseSameModel(t *testing.T) {
	r := Result{
		SchemaVersion: SchemaVersion,
		Environment: Environment{
			OS:   "linux",
			Arch: "amd64",
		},
		Cold: true,
		Measurements: []Measurement{
			{
				ID:           MetricTransformLatency,
				Unit:         UnitNanoseconds,
				Category:     CategoryCold,
				RawSamplesNs: []int64{100, 200, 300},
				Aggregate:    Aggregate{MedianNs: 200, MinNs: 100, MaxNs: 300},
				SampleCount:  3,
			},
		},
	}

	// JSON.
	jsonData, err := EncodeResultJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonData), "runtime.transform.latency") {
		t.Error("JSON missing metric ID")
	}

	// Human.
	human := FormatResultHuman(r)
	if !strings.Contains(human, "runtime.transform.latency") {
		t.Error("human output missing metric ID")
	}
	if !strings.Contains(human, "200") {
		t.Error("human output missing median value")
	}
}

// TestValidateSamplesRejectsNaNInf verifies NaN values detected.
func TestValidateSamplesRejectsNaNInf(t *testing.T) {
	nanVal := int64(math.NaN())
	if err := validateSamples([]int64{nanVal}); err == nil {
		t.Error("expected error for NaN value (as int64, NaN becomes 0 or min int64)")
	}
	// int64 can't represent NaN directly; -9223372036854775808 is returned by math.NaN()
	// So test with the actual value that math.NaN() converts to.
	// Also test with math.Inf which as int64 is also min int64.
	infVal := int64(math.Inf(1))
	if err := validateSamples([]int64{infVal}); err == nil {
		t.Error("expected error for Inf value")
	}
}

// TestSortMeasurementsDeterministic verifies deterministic ordering.
func TestSortMeasurementsDeterministic(t *testing.T) {
	measurements := []Measurement{
		{ID: MetricExecutionWallTime, Category: CategoryWarm, Unit: UnitNanoseconds,
			RawSamplesNs: []int64{500}, Aggregate: Aggregate{MedianNs: 500}, SampleCount: 1},
		{ID: MetricStartupLatency, Category: "", Unit: UnitNanoseconds,
			RawSamplesNs: []int64{100}, Aggregate: Aggregate{MedianNs: 100}, SampleCount: 1},
		{ID: MetricTransformLatency, Category: CategoryCold, Unit: UnitNanoseconds,
			RawSamplesNs: []int64{200}, Aggregate: Aggregate{MedianNs: 200}, SampleCount: 1},
		{ID: MetricTransformLatency, Category: CategoryWarm, Unit: UnitNanoseconds,
			RawSamplesNs: []int64{300}, Aggregate: Aggregate{MedianNs: 300}, SampleCount: 1},
	}
	// Sort twice; result must be identical.
	sortMeasurements(measurements)
	first := make([]string, len(measurements))
	for i, m := range measurements {
		first[i] = string(m.ID) + "/" + string(m.Category)
	}
	sortMeasurements(measurements)
	for i, m := range measurements {
		key := string(m.ID) + "/" + string(m.Category)
		if first[i] != key {
			t.Errorf("ordering not deterministic: pos %d: %q vs %q", i, first[i], key)
		}
	}
	// Verify sorted order: execution < startup < transform/cold < transform/warm
	if measurements[0].ID != MetricExecutionWallTime {
		t.Errorf("first should be execution.walltime, got %s", measurements[0].ID)
	}
	if measurements[1].ID != MetricStartupLatency {
		t.Errorf("second should be startup.latency, got %s", measurements[1].ID)
	}
}

// TestBenchRootIsolation verifies bench root is isolated and cleaned up.
func TestBenchRootIsolation(t *testing.T) {
	br, err := createBenchRoot()
	if err != nil {
		t.Fatal(err)
	}
	// Verify directories exist.
	for _, d := range []string{br.Home, br.CacheDir, br.StoreDir} {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", d)
		}
	}
	// Transform cache path should be under CacheDir.
	tcPath := br.transformCachePath()
	if !strings.HasPrefix(tcPath, br.CacheDir) {
		t.Errorf("transform cache %q not under cache dir %q", tcPath, br.CacheDir)
	}
	// Cleanup removes the home directory.
	if err := br.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(br.Home); !os.IsNotExist(err) {
		t.Error("home directory not cleaned up")
	}
}

// TestClearBenchCacheOnlyRemovesTransform verifies selective cache clearing.
func TestClearBenchCacheOnlyRemovesTransform(t *testing.T) {
	br, err := createBenchRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = br.cleanup() }()

	// Create a file in CacheDir outside transform.
	otherFile := filepath.Join(br.CacheDir, "other-data")
	if err := os.WriteFile(otherFile, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a transform cache file.
	tcPath := br.transformCachePath()
	if err := os.MkdirAll(tcPath, 0o755); err != nil {
		t.Fatal(err)
	}
	tcFile := filepath.Join(tcPath, "test.cache")
	if err := os.WriteFile(tcFile, []byte("delete"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Clear only the transform cache.
	if err := br.clearTransformCache(); err != nil {
		t.Fatal(err)
	}

	// Verify transform directory is gone.
	if _, err := os.Stat(tcPath); !os.IsNotExist(err) {
		t.Error("transform cache not cleared")
	}

	// Verify other file survives.
	if _, err := os.Stat(otherFile); os.IsNotExist(err) {
		t.Error("unrelated cache file was removed")
	}
}

// TestBenchRootDoesNotTouchUserCache verifies user's real cache is never touched.
func TestBenchRootDoesNotTouchUserCache(t *testing.T) {
	// Record the real user cache path before creating a bench root.
	// The bench root must only operate under its own temp directory.
	br, err := createBenchRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = br.cleanup() }()

	// Verify bench home is under os.TempDir(), not user home.
	tmpDir := os.TempDir()
	if !strings.HasPrefix(br.Home, tmpDir) {
		t.Errorf("bench home %q not under temp dir %q", br.Home, tmpDir)
	}

	// Verify the transform cache path is within bench home.
	tcPath := br.transformCachePath()
	if !strings.HasPrefix(tcPath, br.Home) {
		t.Errorf("transform cache path %q not under bench home %q", tcPath, br.Home)
	}
}

// TestBuildMeasurementExcludesWarmup verifies warmup samples are excluded.
func TestBuildMeasurementExcludesWarmup(t *testing.T) {
	raw := []int64{5, 10, 100, 200, 300} // first 2 are warmup
	m, err := buildMeasurement(MetricTransformLatency, CategoryCold, raw, 2)
	if err != nil {
		t.Fatal(err)
	}
	if m.SampleCount != 3 {
		t.Errorf("sample count: want 3, got %d", m.SampleCount)
	}
	if m.WarmupCount != 2 {
		t.Errorf("warmup count: want 2, got %d", m.WarmupCount)
	}
	if len(m.RawSamplesNs) != 3 {
		t.Errorf("raw samples len: want 3, got %d", len(m.RawSamplesNs))
	}
	// Warmup values (5, 10) should not appear in measured samples.
	for _, v := range m.RawSamplesNs {
		if v == 5 || v == 10 {
			t.Errorf("warmup value %d leaked into measured samples", v)
		}
	}
}

// TestCompareBaselineMissingFile fails when baseline path doesn't exist.
func TestCompareBaselineMissingFile(t *testing.T) {
	_, err := compareBaseline("/nonexistent/path/baseline.json", Result{})
	if err == nil {
		t.Error("expected error for missing baseline file")
	}
}

// TestCompareBaselineMalformedJSON fails on malformed baseline.
func TestCompareBaselineMalformedJSON(t *testing.T) {
	tmp, err := os.CreateTemp("", "mew-bench-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString("{not json}")
	tmp.Close()

	_, err = compareBaseline(tmp.Name(), Result{})
	if err == nil {
		t.Error("expected error for malformed baseline JSON")
	}
}

// TestCompareBaselineSchemaMismatch fails on wrong schema version.
func TestCompareBaselineSchemaMismatch(t *testing.T) {
	bl := Baseline{
		SchemaVersion: 999,
		Environment:   BaselineEnv{OS: "linux", Arch: "amd64"},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	_, err := compareBaseline(tmp, Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
	})
	if err == nil {
		t.Error("expected error for schema version mismatch")
	}
}

// TestCompareBaselineOSMismatch fails on OS difference.
func TestCompareBaselineOSMismatch(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "windows", Arch: "amd64"},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	_, err := compareBaseline(tmp, Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
	})
	if err == nil {
		t.Error("expected error for OS mismatch")
	}
}

// TestCompareBaselineArchMismatch fails on arch difference.
func TestCompareBaselineArchMismatch(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "linux", Arch: "arm64"},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	_, err := compareBaseline(tmp, Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
	})
	if err == nil {
		t.Error("expected error for arch mismatch")
	}
}

// TestCompareBaselineMissingMetric fails when required metric absent from baseline.
func TestCompareBaselineMissingMetric(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "linux", Arch: "amd64"},
		Measurements: []BaselineMetric{
			{ID: MetricStartupLatency, Unit: UnitNanoseconds, MedianNs: 1000},
		},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	result := Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
		Measurements: []Measurement{
			{
				ID: MetricStartupLatency, Unit: UnitNanoseconds, Category: "",
				RawSamplesNs: []int64{1000}, Aggregate: Aggregate{MedianNs: 1000}, SampleCount: 1,
			},
			{
				ID: MetricTransformLatency, Unit: UnitNanoseconds, Category: CategoryCold,
				RawSamplesNs: []int64{2000}, Aggregate: Aggregate{MedianNs: 2000}, SampleCount: 1,
			},
		},
	}
	_, err := compareBaseline(tmp, result)
	if err == nil {
		t.Error("expected error for missing baseline metric")
	}
}

// TestCompareBaselineUnitMismatch fails on unit difference.
func TestCompareBaselineUnitMismatch(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "linux", Arch: "amd64"},
		Measurements: []BaselineMetric{
			{ID: MetricStartupLatency, Unit: "ms", MedianNs: 1000},
		},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	result := Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
		Measurements: []Measurement{
			{
				ID: MetricStartupLatency, Unit: UnitNanoseconds, Category: "",
				RawSamplesNs: []int64{1000}, Aggregate: Aggregate{MedianNs: 1000}, SampleCount: 1,
			},
		},
	}
	_, err := compareBaseline(tmp, result)
	if err == nil {
		t.Error("expected error for unit mismatch")
	}
}

// TestCompareBaselineRegressionAboveThreshold fails regression.
func TestCompareBaselineRegressionAboveThreshold(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "linux", Arch: "amd64"},
		ThresholdPct:  10.0,
		Measurements: []BaselineMetric{
			{ID: MetricStartupLatency, Unit: UnitNanoseconds, MedianNs: 1000},
		},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	result := Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
		Measurements: []Measurement{
			{
				ID: MetricStartupLatency, Unit: UnitNanoseconds, Category: "",
				RawSamplesNs: []int64{1500}, Aggregate: Aggregate{MedianNs: 1500}, SampleCount: 1,
			},
		},
	}
	cmp, err := compareBaseline(tmp, result)
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Status != "regression" {
		t.Errorf("expected regression, got %s", cmp.Status)
	}
}

// TestCompareBaselineWithinThreshold passes when within bounds.
func TestCompareBaselineWithinThreshold(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "linux", Arch: "amd64"},
		ThresholdPct:  10.0,
		Measurements: []BaselineMetric{
			{ID: MetricStartupLatency, Unit: UnitNanoseconds, MedianNs: 1000},
		},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	result := Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
		Measurements: []Measurement{
			{
				ID: MetricStartupLatency, Unit: UnitNanoseconds, Category: "",
				RawSamplesNs: []int64{1050}, Aggregate: Aggregate{MedianNs: 1050}, SampleCount: 1,
			},
		},
	}
	cmp, err := compareBaseline(tmp, result)
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Status != "pass" {
		t.Errorf("expected pass, got %s", cmp.Status)
	}
}

// TestCompareBaselineOverallVerdictFailsWhenAnyMetricFails verifies overall verdict.
func TestCompareBaselineOverallVerdictFailsWhenAnyMetricFails(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "linux", Arch: "amd64"},
		ThresholdPct:  10.0,
		Measurements: []BaselineMetric{
			{ID: MetricStartupLatency, Unit: UnitNanoseconds, MedianNs: 1000},
			{ID: MetricTransformLatency, Unit: UnitNanoseconds, MedianNs: 1000},
		},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	result := Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
		Measurements: []Measurement{
			{
				ID: MetricStartupLatency, Unit: UnitNanoseconds, Category: "",
				RawSamplesNs: []int64{1050}, Aggregate: Aggregate{MedianNs: 1050}, SampleCount: 1,
			},
			{
				ID: MetricTransformLatency, Unit: UnitNanoseconds, Category: CategoryCold,
				RawSamplesNs: []int64{1500}, Aggregate: Aggregate{MedianNs: 1500}, SampleCount: 1,
			},
		},
	}
	cmp, err := compareBaseline(tmp, result)
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Status != "regression" {
		t.Errorf("expected regression (one metric failing), got %s", cmp.Status)
	}
}

// TestCompareBaselineDuplicateMetricID fails on duplicate IDs.
func TestCompareBaselineDuplicateMetricID(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "linux", Arch: "amd64"},
		Measurements: []BaselineMetric{
			{ID: MetricStartupLatency, Unit: UnitNanoseconds, MedianNs: 1000},
			{ID: MetricStartupLatency, Unit: UnitNanoseconds, MedianNs: 2000},
		},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	_, err := compareBaseline(tmp, Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
	})
	if err == nil {
		t.Error("expected error for duplicate metric IDs")
	}
}

// TestCompareBaselineInvalidMedian fails on invalid median in baseline.
func TestCompareBaselineInvalidMedian(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "linux", Arch: "amd64"},
		Measurements: []BaselineMetric{
			{ID: MetricStartupLatency, Unit: UnitNanoseconds, MedianNs: 0},
		},
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	_, err := compareBaseline(tmp, Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
	})
	if err == nil {
		t.Error("expected error for invalid median (0)")
	}
}

// TestEnvironmentMetadataNoSecrets verifies metadata excludes sensitive fields.
func TestEnvironmentMetadataNoSecrets(t *testing.T) {
	env := buildEnvironment("test-fixture")
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"HOME", "SECRET", "TOKEN", "PASSWORD", "PID", "/home/", "/Users/"} {
		if strings.Contains(text, secret) {
			t.Errorf("environment metadata contains sensitive marker %q", secret)
		}
	}
	for _, required := range []string{"goVersion", "os", "arch", "logicalCpus"} {
		if !strings.Contains(text, required) {
			t.Errorf("environment metadata missing required field %q", required)
		}
	}
}

// writeBaselineTemp writes a baseline to a temp file and returns the path.
func writeBaselineTemp(t *testing.T, bl Baseline) string {
	t.Helper()
	data, err := json.Marshal(bl)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := os.CreateTemp("", "mew-bench-baseline-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		t.Fatal(err)
	}
	tmp.Close()
	return tmp.Name()
}

// TestRunRejectsZeroSamples verifies Run rejects zero samples.
func TestRunRejectsZeroSamples(t *testing.T) {
	_, err := Run(context.Background(), Options{Samples: 0, Warmup: 1, Warm: true})
	if err == nil {
		t.Error("expected error for samples=0")
	}
}

// TestRunRejectsNegativeWarmup verifies Run rejects negative warmup.
func TestRunRejectsNegativeWarmup(t *testing.T) {
	_, err := Run(context.Background(), Options{Samples: 1, Warmup: -1, Warm: true})
	if err == nil {
		t.Error("expected error for negative warmup")
	}
}

// TestCompareBaselineEmptyMeasurements fails on empty baseline measurements.
func TestCompareBaselineEmptyMeasurements(t *testing.T) {
	bl := Baseline{
		SchemaVersion: SchemaVersion,
		Environment:   BaselineEnv{OS: "linux", Arch: "amd64"},
		Measurements:  nil,
	}
	tmp := writeBaselineTemp(t, bl)
	defer os.Remove(tmp)

	_, err := compareBaseline(tmp, Result{
		SchemaVersion: SchemaVersion,
		Environment:   Environment{OS: "linux", Arch: "amd64"},
	})
	if err == nil {
		t.Error("expected error for empty baseline measurements")
	}
}
