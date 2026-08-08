package benchruntime

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// EncodeResultJSON encodes the result as indented JSON.
func EncodeResultJSON(r Result) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// WriteResultHuman writes a human-readable summary of the result.
func WriteResultHuman(w io.Writer, r Result) error {
	fmt.Fprintf(w, "mode: cold=%v warm=%v samples=%d warmup=%d\n",
		r.Cold, r.Warm, r.Samples, r.Warmup)

	for _, m := range r.Measurements {
		cat := string(m.Category)
		if cat == "" {
			cat = "n/a"
		}
		fmt.Fprintf(w, "%-35s %-6s %10d ns  (min: %d, max: %d, samples: %d)\n",
			m.ID, cat, m.Aggregate.MedianNs, m.Aggregate.MinNs, m.Aggregate.MaxNs, m.SampleCount)
	}

	if r.Compare != nil {
		fmt.Fprintf(w, "\nbaseline: %s  status: %s  threshold: %.1f%%\n",
			r.Compare.Baseline, r.Compare.Status, r.Compare.Threshold)
		for _, d := range r.Compare.Details {
			sign := "+"
			if d.DeltaPct < 0 {
				sign = ""
			}
			fmt.Fprintf(w, "  %-33s %10d ns  baseline: %d  delta: %s%.1f%%  %s\n",
				d.MetricID, d.CurrentMedianNs, d.BaselineMedianNs, sign, d.DeltaPct, d.Verdict)
		}
	}
	return nil
}

// FormatResultHuman returns a human-readable summary string.
func FormatResultHuman(r Result) string {
	var b strings.Builder
	_ = WriteResultHuman(&b, r)
	return b.String()
}
