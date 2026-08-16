package archcheck_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRuntimeCertificationEvidence verifies runtime certification evidence
// is consistent with plan status and free of stale claims.
func TestRuntimeCertificationEvidence(t *testing.T) {
	root := repoRoot(t)

	// 1. Evidence document must exist.
	evPath := filepath.Join(root, "docs", "evidence", "runtime", "0050-0051-certification.md")
	ev, err := os.ReadFile(evPath)
	if err != nil {
		t.Fatalf("cannot read runtime certification evidence: %v", err)
	}
	evText := string(ev)

	if !strings.Contains(evText, "**GREEN**") {
		t.Error("runtime certification evidence does not report GREEN status")
	}

	// 2. Status.json: 0050/0051 completed. 0052-0056 may be completed
	// (implementation complete). 0057 must NOT be in completedMvps
	// (exact-head certification pending CI observation).
	st, err := os.ReadFile(filepath.Join(root, "plans", "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	stText := string(st)
	if !strings.Contains(stText, `"0050"`) || !strings.Contains(stText, `"0051"`) {
		t.Error("status.json missing 0050 or 0051 in completedMvps")
	}

	// 0057 must NOT appear in completedMvps (certification pending).
	var status struct {
		CompletedMvps []string `json:"completedMvps"`
	}
	if err := json.Unmarshal(st, &status); err != nil {
		t.Fatalf("cannot parse status.json: %v", err)
	}
	for _, completed := range status.CompletedMvps {
		if completed == "0057" {
			t.Error("status.json: 0057 must not appear in completedMvps (exact-head certification pending CI observation)")
		}
	}

	// 3. status.json must not contain a stale lastCertifiedCoreCommit.
	// The field is optional; if present it must not be a nonexistent SHA.
	if strings.Contains(stText, `"lastCertifiedCoreCommit"`) {
		t.Error("status.json must not contain lastCertifiedCoreCommit (no valid certification SHA exists on this branch); use _certificationNote instead")
	}

	// 4. CHECKLIST.md must not reference a stale certified commit SHA.
	cl, err := os.ReadFile(filepath.Join(root, "plans", "CHECKLIST.md"))
	if err != nil {
		t.Fatal(err)
	}
	clText := string(cl)
	if strings.Contains(clText, "Last certified core commit:") {
		t.Error("CHECKLIST.md header must not claim a certified core commit (no valid certification exists for 0052-0057)")
	}

	// 5. Protocol versions doc must not claim "frozen" before 0057 stabilization.
	pv, err := os.ReadFile(filepath.Join(root, "docs", "runtime", "protocol-versions.md"))
	if err != nil {
		t.Fatal(err)
	}
	pvText := string(pv)
	if strings.Contains(pvText, "Frozen as of 0057") {
		t.Error("protocol-versions.md must not claim versions are frozen (0057 stabilization pending)")
	}

	// 6. Known limitations doc must not claim "as of 0057" stabilization.
	kl, err := os.ReadFile(filepath.Join(root, "docs", "runtime", "known-limitations.md"))
	if err != nil {
		t.Fatal(err)
	}
	klText := string(kl)
	if strings.Contains(klText, "Documented as of 0057") {
		t.Error("known-limitations.md must not claim documentation as of 0057 (stabilization pending)")
	}

	// 7. No stale limitation text in evidence.
	for _, phrase := range []string{
		"macOS and Windows certification pending",
	} {
		if strings.Contains(evText, phrase) {
			t.Errorf("runtime certification evidence contains stale limitation: %q", phrase)
		}
	}
}
