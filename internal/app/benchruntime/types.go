// Package benchruntime implements the runtime benchmark command with
// structured measurements, isolation, and fail-closed semantics.
//
// Regenerate: go run ./tools/plangen ./internal/app/benchruntime
package benchruntime

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"time"
)

// SchemaVersion is the current benchmark result schema version.
const SchemaVersion = 1

// MetricID identifies a specific benchmark measurement.
type MetricID string

const (
	// MetricStartupLatency measures runtime.Plan construction (Mew-side launch prep).
	MetricStartupLatency MetricID = "runtime.startup.latency"

	// MetricTransformLatency measures TypeScript transform latency.
	MetricTransformLatency MetricID = "runtime.transform.latency"

	// MetricExecutionWallTime measures end-to-end wall-clock time of "m run".
	MetricExecutionWallTime MetricID = "runtime.execution.walltime"
)

// Category labels the cache state under which a measurement was taken.
type Category string

const (
	CategoryCold Category = "cold"
	CategoryWarm Category = "warm"
)

// Unit is the measurement unit.
const UnitNanoseconds = "ns"

// Aggregate holds computed statistics from raw samples.
type Aggregate struct {
	MedianNs int64 `json:"medianNs"`
	MinNs    int64 `json:"minNs"`
	MaxNs    int64 `json:"maxNs"`
}

// Measurement is a single benchmark measurement with raw samples and aggregates.
type Measurement struct {
	ID           MetricID  `json:"id"`
	Unit         string    `json:"unit"`
	Category     Category  `json:"category"`
	RawSamplesNs []int64   `json:"rawSamplesNs"`
	Aggregate    Aggregate `json:"aggregate"`
	SampleCount  int       `json:"sampleCount"`
	WarmupCount  int       `json:"warmupCount"`
}

// Environment records reproducibility metadata for a benchmark run.
type Environment struct {
	SchemaVersion int    `json:"schemaVersion"`
	MewVersion    string `json:"mewVersion,omitempty"`
	MewCommit     string `json:"mewCommit,omitempty"`
	GoVersion     string `json:"goVersion"`
	NodeVersion   string `json:"nodeVersion,omitempty"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	LogicalCPUs   int    `json:"logicalCpus"`
	Fixture       string `json:"fixture"`
}

// Result is the complete benchmark output for the runtime command.
type Result struct {
	SchemaVersion int           `json:"schemaVersion"`
	Environment   Environment   `json:"environment"`
	Cold          bool          `json:"cold"`
	Warm          bool          `json:"warm"`
	Samples       int           `json:"samples"`
	Warmup        int           `json:"warmup"`
	RecordedAt    string        `json:"recordedAt"`
	Measurements  []Measurement `json:"measurements"`
	Compare       *Compare      `json:"compare,omitempty"`
}

// Compare holds baseline comparison results.
type Compare struct {
	Status    string          `json:"status"`
	Baseline  string          `json:"baseline"`
	Threshold float64         `json:"threshold"`
	Details   []CompareDetail `json:"details"`
}

// CompareDetail is a single metric's comparison against a baseline.
type CompareDetail struct {
	MetricID         MetricID `json:"metricId"`
	CurrentMedianNs  int64    `json:"currentMedianNs"`
	BaselineMedianNs int64    `json:"baselineMedianNs"`
	DeltaPct         float64  `json:"deltaPct"`
	ThresholdPct     float64  `json:"thresholdPct"`
	Verdict          string   `json:"verdict"`
}

// Baseline is the external baseline file format.
type Baseline struct {
	SchemaVersion int               `json:"schemaVersion"`
	Environment   BaselineEnv       `json:"environment"`
	ThresholdPct  float64           `json:"thresholdPct"`
	Measurements  []BaselineMetric  `json:"measurements"`
}

// BaselineEnv carries the environment identity for compatibility checks.
type BaselineEnv struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// BaselineMetric is a single metric entry in a baseline file.
type BaselineMetric struct {
	ID       MetricID `json:"id"`
	Unit     string   `json:"unit"`
	MedianNs int64    `json:"medianNs"`
}

// computeAggregate derives aggregate statistics from sorted raw samples.
// samples must be sorted ascending and non-empty.
func computeAggregate(samples []int64) Aggregate {
	n := len(samples)
	return Aggregate{
		MedianNs: samples[n/2],
		MinNs:    samples[0],
		MaxNs:    samples[n-1],
	}
}

// validateSamples rejects invalid raw sample values.
func validateSamples(samples []int64) error {
	if len(samples) == 0 {
		return fmt.Errorf("zero samples: at least one measured sample required")
	}
	for i, v := range samples {
		if v <= 0 {
			return fmt.Errorf("sample %d: non-positive duration %d ns", i, v)
		}
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return fmt.Errorf("sample %d: invalid value %d", i, v)
		}
	}
	return nil
}

// sortSamples returns a sorted copy of samples.
func sortSamples(samples []int64) []int64 {
	sorted := make([]int64, len(samples))
	copy(sorted, samples)
	slices.Sort(sorted)
	return sorted
}

// validateIterations rejects invalid iteration/warmup counts.
func validateIterations(samples, warmup int) error {
	if samples < 1 {
		return fmt.Errorf("samples must be >= 1, got %d", samples)
	}
	if warmup < 0 {
		return fmt.Errorf("warmup must be >= 0, got %d", warmup)
	}
	return nil
}

// measureDuration runs f and returns the elapsed wall-clock duration in nanoseconds.
func measureDuration(f func() error) (int64, error) {
	start := time.Now()
	err := f()
	elapsed := time.Since(start).Nanoseconds()
	if elapsed <= 0 {
		return 0, fmt.Errorf("invalid elapsed duration %d ns", elapsed)
	}
	return elapsed, err
}

// sortMeasurements returns a deterministic ordering for measurements.
func sortMeasurements(measurements []Measurement) {
	slices.SortFunc(measurements, func(a, b Measurement) int {
		if n := cmp.Compare(string(a.ID), string(b.ID)); n != 0 {
			return n
		}
		return cmp.Compare(string(a.Category), string(b.Category))
	})
}
