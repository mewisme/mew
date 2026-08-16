package transform

import (
	"testing"
)

func TestCapabilityReportDeterministic(t *testing.T) {
	// Two calls return identical results.
	r1 := CapabilityReport()
	r2 := CapabilityReport()
	if len(r1) != len(r2) {
		t.Fatalf("length mismatch: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i].Option != r2[i].Option {
			t.Fatalf("entry %d: option %q vs %q", i, r1[i].Option, r2[i].Option)
		}
	}
}

func TestCapabilityReportSorted(t *testing.T) {
	// Report must be sorted by category then option name.
	r := CapabilityReport()
	if len(r) == 0 {
		t.Fatal("empty report")
	}
	for i := 1; i < len(r); i++ {
		prev := r[i-1]
		cur := r[i]
		if prev.Category > cur.Category {
			t.Errorf("category order: %q before %q at index %d", prev.Category, cur.Category, i)
		}
		if prev.Category == cur.Category && prev.Option >= cur.Option {
			t.Errorf("option order: %q before %q at index %d", prev.Option, cur.Option, i)
		}
	}
}

func TestCapabilityReportNoDuplicates(t *testing.T) {
	r := CapabilityReport()
	seen := make(map[string]bool)
	for _, e := range r {
		if seen[e.Option] {
			t.Errorf("duplicate option: %q", e.Option)
		}
		seen[e.Option] = true
	}
}

func TestCapabilityReportValidStatus(t *testing.T) {
	r := CapabilityReport()
	validStatus := map[Status]bool{
		StatusSupported:   true,
		StatusPartial:     true,
		StatusUnsupported: true,
	}
	for _, e := range r {
		if !validStatus[e.Status] {
			t.Errorf("option %q has invalid status: %q", e.Option, e.Status)
		}
	}
}

func TestCapabilityReportValidCategory(t *testing.T) {
	r := CapabilityReport()
	validCategory := map[Category]bool{
		CategoryTypeScript: true,
		CategoryJSX:        true,
		CategoryDecorators: true,
		CategorySourceMaps: true,
	}
	for _, e := range r {
		if !validCategory[e.Category] {
			t.Errorf("option %q has invalid category: %q", e.Option, e.Category)
		}
	}
}

func TestCapabilityReportUnsupportedHaveReason(t *testing.T) {
	r := CapabilityReport()
	for _, e := range r {
		if e.Status == StatusUnsupported && e.Limitation == "" {
			t.Errorf("unsupported option %q must have a limitation/reason", e.Option)
		}
	}
}

func TestCapabilityReportPartialHaveReason(t *testing.T) {
	r := CapabilityReport()
	for _, e := range r {
		if e.Status == StatusPartial && e.Limitation == "" {
			t.Errorf("partial option %q must have a limitation/reason", e.Option)
		}
	}
}

func TestOptionSetCoversAllReportEntries(t *testing.T) {
	r := CapabilityReport()
	s := OptionSet()
	for _, e := range r {
		if !s[e.Option] {
			t.Errorf("option %q in report but not in OptionSet", e.Option)
		}
	}
	if len(s) != len(r) {
		t.Errorf("OptionSet has %d entries, report has %d", len(s), len(r))
	}
}

func TestUnsupportedSetConsistent(t *testing.T) {
	r := CapabilityReport()
	u := UnsupportedSet()
	for _, e := range r {
		expectInSet := e.Status == StatusUnsupported
		if u[e.Option] != expectInSet {
			t.Errorf("option %q: UnsupportedSet=%v, want %v (status=%s)", e.Option, u[e.Option], expectInSet, e.Status)
		}
	}
	for k := range u {
		found := false
		for _, e := range r {
			if e.Option == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("UnsupportedSet key %q not in report", k)
		}
	}
}
