package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	SchemaVersion = 1
	CoreMatrix    = "core"
	CLIUXMatrix   = "cli-ux"
	RuntimeMatrix = "runtime"
)

// Manifest is a go-test certification matrix definition (core or cli-ux).
type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Matrix        string `json:"matrix"`
	Suites        []Suite
}

// Suite describes one go test invocation in the matrix.
type Suite struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Package      string   `json:"package"`
	Run          string   `json:"run"`
	Required     bool     `json:"required"`
	RequireTools bool     `json:"requireTools,omitempty"`
	Probe        bool     `json:"probe,omitempty"`
	Platforms    []string `json:"platforms,omitempty"`
}

// UnmarshalJSON accepts the manifest "suites" array.
func (m *Manifest) UnmarshalJSON(data []byte) error {
	var raw struct {
		SchemaVersion int     `json:"schemaVersion"`
		Matrix        string  `json:"matrix"`
		Suites        []Suite `json:"suites"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	m.SchemaVersion = raw.SchemaVersion
	m.Matrix = raw.Matrix
	m.Suites = raw.Suites
	return nil
}

// LoadManifest reads and validates a core-matrix or cli-ux manifest file.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, err
	}

	if m.SchemaVersion != SchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported manifest schema %d", m.SchemaVersion)
	}
	switch m.Matrix {
	case CoreMatrix, CLIUXMatrix, RuntimeMatrix:
	default:
		return Manifest{}, fmt.Errorf("unsupported matrix %q", m.Matrix)
	}
	if len(m.Suites) == 0 {
		return Manifest{}, fmt.Errorf("manifest has no suites")
	}
	seen := map[string]struct{}{}
	for _, s := range m.Suites {
		if strings.TrimSpace(s.ID) == "" {
			return Manifest{}, fmt.Errorf("suite missing id")
		}
		if _, ok := seen[s.ID]; ok {
			return Manifest{}, fmt.Errorf("duplicate suite id %q", s.ID)
		}
		seen[s.ID] = struct{}{}
		if strings.TrimSpace(s.Package) == "" {
			return Manifest{}, fmt.Errorf("suite %q missing package", s.ID)
		}
		if strings.TrimSpace(s.Run) == "" {
			return Manifest{}, fmt.Errorf("suite %q missing run pattern", s.ID)
		}
		if _, err := regexp.Compile(s.Run); err != nil {
			return Manifest{}, fmt.Errorf("suite %q invalid run regex %q: %w", s.ID, s.Run, err)
		}
		for _, p := range s.Platforms {
			if p != "linux" && p != "darwin" && p != "windows" {
				return Manifest{}, fmt.Errorf("suite %q invalid platform %q", s.ID, p)
			}
		}
	}
	return m, nil
}

// CoreManifestPath returns tests/conformance/core-matrix/manifest.json under repoRoot.
func CoreManifestPath(repoRoot string) string {
	return filepath.Join(repoRoot, "tests", "conformance", "core-matrix", "manifest.json")
}

// CLIUXManifestPath returns tests/conformance/cli-ux/manifest.json under repoRoot.
func CLIUXManifestPath(repoRoot string) string {
	return filepath.Join(repoRoot, "tests", "conformance", "cli-ux", "manifest.json")
}

// RuntimeManifestPath returns tests/conformance/runtime-matrix/manifest.json under repoRoot.
func RuntimeManifestPath(repoRoot string) string {
	return filepath.Join(repoRoot, "tests", "conformance", "runtime-matrix", "manifest.json")
}

// FilterSuites returns suites whose id matches filter (exact, prefix, or substring). Empty filter returns all.
func FilterSuites(suites []Suite, filter string) []Suite {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return append([]Suite(nil), suites...)
	}
	var out []Suite
	for _, s := range suites {
		if s.ID == filter || strings.HasPrefix(s.ID, filter) || strings.Contains(s.ID, filter) {
			out = append(out, s)
		}
	}
	return out
}

func suiteSupportedOnPlatform(s Suite) bool {
	if len(s.Platforms) == 0 {
		return true
	}
	goos := runtime.GOOS
	for _, p := range s.Platforms {
		if p == goos {
			return true
		}
	}
	return false
}
