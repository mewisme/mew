package releasetrain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type planStatus struct {
	SchemaVersion           int      `json:"schemaVersion"`
	CurrentMvp              string   `json:"currentMvp"`
	CompletedMvps           []string `json:"completedMvps"`
	PlannedMvps             []string `json:"plannedMvps"`
	LastCertifiedCoreCommit string   `json:"lastCertifiedCoreCommit"`
	LastUpdated             string   `json:"lastUpdated"`
}

func TestPlanStatusValid(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "plans", "status.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var status planStatus
	if err := json.Unmarshal(b, &status); err != nil {
		t.Fatal(err)
	}
	if status.SchemaVersion != 1 {
		t.Fatalf("schemaVersion=%d want 1", status.SchemaVersion)
	}
	if status.CurrentMvp == "" {
		t.Fatal("currentMvp is empty")
	}

	seen := make(map[string]string)
	for _, id := range status.CompletedMvps {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate MVP id %s", id)
		}
		seen[id] = "completed"
	}
	for _, id := range status.PlannedMvps {
		if prev, ok := seen[id]; ok && prev != "planned" {
			t.Fatalf("duplicate MVP id %s", id)
		}
		seen[id] = "planned"
	}
	if prev, ok := seen[status.CurrentMvp]; ok && prev == "completed" {
		t.Fatalf("currentMvp %s must not be completed", status.CurrentMvp)
	}
	if prev, ok := seen[status.CurrentMvp]; ok && prev == "planned" {
		t.Fatalf("currentMvp %s must not appear in plannedMvps (current is distinct from planned)", status.CurrentMvp)
	}

	all := make([]string, 0, len(seen)+1)
	for id := range seen {
		all = append(all, id)
	}
	if _, ok := seen[status.CurrentMvp]; !ok {
		all = append(all, status.CurrentMvp)
	}

	planDir := filepath.Join(root, "plans")
	entries, err := os.ReadDir(planDir)
	if err != nil {
		t.Fatal(err)
	}
	known := make(map[string]struct{})
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "0") {
			continue
		}
		if len(e.Name()) < 5 || e.Name()[4] != '-' {
			continue
		}
		id := e.Name()[:4]
		if id == "0000" {
			continue
		}
		known[id] = struct{}{}
	}
	for _, id := range all {
		if _, ok := known[id]; !ok {
			t.Errorf("status.json id %s has no plans/00xx-*.md file", id)
		}
	}
}
