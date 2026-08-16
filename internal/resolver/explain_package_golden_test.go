package resolver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mewisme/mew/internal/policy"
	"github.com/mewisme/mew/internal/registry"
	"github.com/mewisme/mew/internal/resolver"
)

func TestExplainPackageGolden(t *testing.T) {
	cases := []struct {
		name        string
		packageName string
		fixture     string
		packs       func() map[string]registry.Packument
		opts        resolver.ResolveOptions
	}{
		{
			name:        "override-chain",
			packageName: "pkg-b",
			fixture:     "explain/override-chain/package.json",
			packs:       overridePackuments,
		},
		{
			name:        "peer-conflict",
			packageName: "react-dom",
			fixture:     "explain/peer-conflict/package.json",
			packs:       reactPackuments,
			opts: resolver.ResolveOptions{
				Policy: &policy.Policy{StrictPeerDependencies: true},
			},
		},
	}

	goldenDir := filepath.Join("..", "..", "testdata", "explain")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng, _ := engineWithPackuments(t, tc.packs())
			root := writeProject(t, readExplainFixture(t, tc.fixture))
			ex, err := eng.ExplainPackage(context.Background(), root, tc.packageName, tc.opts)
			if err != nil {
				t.Fatal(err)
			}

			jsonGolden := filepath.Join(goldenDir, tc.name+".json")
			txtGolden := filepath.Join(goldenDir, tc.name+".txt")

			gotJSON, err := json.MarshalIndent(ex, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			gotJSON = append(gotJSON, '\n')

			var buf bytes.Buffer
			if err := resolver.FormatPackageExplanation(ex, &buf); err != nil {
				t.Fatal(err)
			}
			gotHuman := buf.String()

			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(jsonGolden, gotJSON, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(txtGolden, []byte(gotHuman), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Log("updated explain package goldens")
			}

			wantJSON, err := os.ReadFile(jsonGolden)
			if err != nil {
				t.Fatalf("read json golden (set UPDATE_GOLDEN=1): %v", err)
			}
			if !bytes.Equal(gotJSON, wantJSON) {
				t.Fatalf("json golden mismatch\n--- got ---\n%s\n--- want ---\n%s", gotJSON, wantJSON)
			}

			wantHuman, err := os.ReadFile(txtGolden)
			if err != nil {
				t.Fatalf("read txt golden (set UPDATE_GOLDEN=1): %v", err)
			}
			if string(wantHuman) != gotHuman {
				t.Fatalf("human golden mismatch\n--- got ---\n%s\n--- want ---\n%s", gotHuman, wantHuman)
			}
		})
	}
}
