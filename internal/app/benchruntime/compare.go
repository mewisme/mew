package benchruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mewisme/mew/internal/apperr"
)

// DefaultRegressionThresholdPct is the default regression threshold (10%).
const DefaultRegressionThresholdPct = 10.0

// compareBaseline loads a baseline file and compares it against the result.
func compareBaseline(path string, result Result) (Compare, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Compare{}, apperr.Wrap(apperr.Manifest, "bench runtime", path, err)
	}

	var bl Baseline
	if err := json.Unmarshal(data, &bl); err != nil {
		return Compare{}, apperr.Wrap(apperr.Manifest, "bench runtime", path,
			fmt.Errorf("invalid baseline JSON: %w", err))
	}

	if bl.SchemaVersion != SchemaVersion {
		return Compare{}, apperr.New(apperr.Manifest, "bench runtime", path,
			fmt.Sprintf("baseline schema version %d != %d", bl.SchemaVersion, SchemaVersion))
	}

	if bl.Environment.OS != "" && !strings.EqualFold(bl.Environment.OS, result.Environment.OS) {
		return Compare{}, apperr.New(apperr.Manifest, "bench runtime", path,
			fmt.Sprintf("baseline OS %q differs from current %q", bl.Environment.OS, result.Environment.OS))
	}
	if bl.Environment.Arch != "" && !strings.EqualFold(bl.Environment.Arch, result.Environment.Arch) {
		return Compare{}, apperr.New(apperr.Manifest, "bench runtime", path,
			fmt.Sprintf("baseline arch %q differs from current %q", bl.Environment.Arch, result.Environment.Arch))
	}

	threshold := bl.ThresholdPct
	if threshold <= 0 {
		threshold = DefaultRegressionThresholdPct
	}

	blByID := make(map[string]BaselineMetric)
	for _, m := range bl.Measurements {
		key := baselineMetricKey(m.ID, m.Unit)
		if _, exists := blByID[key]; exists {
			return Compare{}, apperr.New(apperr.Manifest, "bench runtime", path,
				fmt.Sprintf("duplicate baseline metric %s/%s", m.ID, m.Unit))
		}
		if m.MedianNs <= 0 {
			return Compare{}, apperr.New(apperr.Manifest, "bench runtime", path,
				fmt.Sprintf("baseline metric %s has invalid median %d", m.ID, m.MedianNs))
		}
		blByID[key] = m
	}

	if len(blByID) == 0 {
		return Compare{}, apperr.New(apperr.Manifest, "bench runtime", path,
			"baseline contains no measurements")
	}

	var details []CompareDetail
	allPass := true

	for _, cur := range result.Measurements {
		mKey := baselineMetricKey(cur.ID, cur.Unit)
		bl, ok := blByID[mKey]
		if !ok {
			return Compare{}, apperr.New(apperr.Manifest, "bench runtime", path,
				fmt.Sprintf("baseline missing required metric %s/%s", cur.ID, cur.Unit))
		}
		if !strings.EqualFold(bl.Unit, cur.Unit) {
			return Compare{}, apperr.New(apperr.Manifest, "bench runtime", path,
				fmt.Sprintf("metric %s unit mismatch: baseline %q != current %q", cur.ID, bl.Unit, cur.Unit))
		}

		deltaPct := float64(cur.Aggregate.MedianNs-bl.MedianNs) / float64(bl.MedianNs) * 100.0
		verdict := "pass"
		if deltaPct > threshold {
			verdict = "regression"
			allPass = false
		} else if deltaPct < -threshold {
			verdict = "improvement"
		}

		details = append(details, CompareDetail{
			MetricID:         cur.ID,
			CurrentMedianNs:  cur.Aggregate.MedianNs,
			BaselineMedianNs: bl.MedianNs,
			DeltaPct:         deltaPct,
			ThresholdPct:     threshold,
			Verdict:          verdict,
		})
	}

	status := "pass"
	if !allPass {
		status = "regression"
	}

	return Compare{
		Status:    status,
		Baseline:  path,
		Threshold: threshold,
		Details:   details,
	}, nil
}

// baselineMetricKey builds a unique key for a baseline metric entry.
func baselineMetricKey(id MetricID, unit string) string {
	return string(id) + "/" + unit
}
