package node

import (
	"context"
	"os/exec"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{"22.11.0", "22.11.0", false},
		{"20.18.1", "20.18.1", false},
		{"16.20.2", "16.20.2", false},
		{"12.22.12", "12.22.12", false},
		{"18.0", "18.0.0", false},
		{"22", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := normalizeVersion(tt.raw)
		if tt.wantErr {
			if err == nil {
				t.Errorf("normalizeVersion(%q) expected error", tt.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeVersion(%q) unexpected error: %v", tt.raw, err)
			continue
		}
		if got != tt.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestDetectCapabilities(t *testing.T) {
	tests := []struct {
		version string
		want    []string
	}{
		{"22.11.0", []string{"import-preload", "require-preload", "module-register", "source-maps"}},
		{"20.18.0", []string{"import-preload", "require-preload", "module-register", "source-maps"}},
		{"18.19.0", []string{"import-preload", "require-preload", "module-register"}},
		{"18.0.0", []string{"import-preload", "require-preload"}},
		{"16.0.0", []string{"import-preload", "require-preload"}},
		{"14.0.0", []string{"require-preload"}},
		{"12.0.0", []string{"require-preload"}},
		{"10.0.0", nil},
	}
	for _, tt := range tests {
		got := detectCapabilities(tt.version)
		if !strSliceEq(got, tt.want) {
			t.Errorf("detectCapabilities(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestDetectCapabilitiesBoundaries(t *testing.T) {
	// Boundary version tests for capability gating.
	tests := []struct {
		version string
		has     []string // must-have capabilities
		lacks   []string // must-NOT-have capabilities
	}{
		// module-register: stable from 20.6, experimental from 18.19
		{"20.6.0", []string{"module-register", "source-maps"}, nil},
		{"20.5.0", nil, []string{"module-register", "source-maps"}},
		{"21.0.0", []string{"module-register", "source-maps"}, nil},
		{"18.19.0", []string{"module-register"}, []string{"source-maps"}},
		{"18.18.0", nil, []string{"module-register", "source-maps"}},
		// source-maps: stable from 20.6 (major > 20 OR major == 20 && minor >= 6)
		{"20.6.0", []string{"source-maps"}, nil},
		{"20.5.0", nil, []string{"source-maps"}},
		{"22.0.0", []string{"source-maps"}, nil},
		// import-preload from 16, require-preload from 12
		{"15.0.0", []string{"require-preload"}, []string{"import-preload"}},
		{"11.0.0", nil, []string{"import-preload", "require-preload"}},
	}
	for _, tt := range tests {
		got := detectCapabilities(tt.version)
		gotSet := make(map[string]bool)
		for _, c := range got {
			gotSet[c] = true
		}
		for _, c := range tt.has {
			if !gotSet[c] {
				t.Errorf("detectCapabilities(%q): missing %q (got %v)", tt.version, c, got)
			}
		}
		for _, c := range tt.lacks {
			if gotSet[c] {
				t.Errorf("detectCapabilities(%q): unexpected %q (got %v)", tt.version, c, got)
			}
		}
	}
}

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		a, b string
		want int
		ok   bool
	}{
		// equal
		{"18.19.0", "18.19.0", 0, true},
		{"22.11.0", "22.11.0", 0, true},
		// a < b (major)
		{"18.19.0", "20.0.0", -1, true},
		{"16.0.0", "18.0.0", -1, true},
		// a < b (minor)
		{"18.0.0", "18.19.0", -1, true},
		{"18.18.0", "18.19.0", -1, true},
		// a < b (patch)
		{"18.19.0", "18.19.1", -1, true},
		// a > b
		{"20.6.0", "18.19.0", 1, true},
		{"18.20.0", "18.19.0", 1, true},
		{"18.19.1", "18.19.0", 1, true},
		// floor boundaries
		{"18.18.0", "18.19.0", -1, true},
		{"18.19.0", "18.19.0", 0, true},
		{"18.20.0", "18.19.0", 1, true},
		// Node 20 boundaries
		{"20.5.0", "20.6.0", -1, true},
		{"20.6.0", "20.6.0", 0, true},
		{"20.7.0", "20.6.0", 1, true},
		// invalid
		{"invalid", "18.19.0", 0, false},
		{"18.19.0", "invalid", 0, false},
		{"", "", 0, false},
	}
	for _, tt := range tests {
		got, ok := CompareVersion(tt.a, tt.b)
		if ok != tt.ok {
			t.Errorf("CompareVersion(%q, %q) ok=%v, want ok=%v", tt.a, tt.b, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("CompareVersion(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMinRuntimeVersion(t *testing.T) {
	// Verify the constant parses correctly.
	major, minor, patch, ok := parseSemver(MinRuntimeVersion)
	if !ok {
		t.Fatalf("MinRuntimeVersion %q does not parse", MinRuntimeVersion)
	}
	if major != 18 || minor != 19 || patch != 0 {
		t.Fatalf("MinRuntimeVersion %q = %d.%d.%d, want 18.19.0", MinRuntimeVersion, major, minor, patch)
	}
}

func TestCompareVersionMinFloor(t *testing.T) {
	// Every version below MinRuntimeVersion must compare as less.
	below := []string{"18.18.0", "18.0.0", "16.20.0", "14.0.0", "12.0.0"}
	for _, v := range below {
		cmp, ok := CompareVersion(v, MinRuntimeVersion)
		if !ok {
			t.Errorf("CompareVersion(%q, %q) not ok", v, MinRuntimeVersion)
			continue
		}
		if cmp >= 0 {
			t.Errorf("CompareVersion(%q, %q) = %d, want < 0", v, MinRuntimeVersion, cmp)
		}
	}
	// Every version at or above MinRuntimeVersion must compare as >=.
	above := []string{"18.19.0", "18.19.1", "18.20.0", "20.0.0", "20.6.0", "22.0.0", "24.0.0"}
	for _, v := range above {
		cmp, ok := CompareVersion(v, MinRuntimeVersion)
		if !ok {
			t.Errorf("CompareVersion(%q, %q) not ok", v, MinRuntimeVersion)
			continue
		}
		if cmp < 0 {
			t.Errorf("CompareVersion(%q, %q) = %d, want >= 0", v, MinRuntimeVersion, cmp)
		}
	}
}

func TestDiscoverNodeFound(t *testing.T) {
	// Only run if node is available on PATH.
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH")
	}
	inst, err := Discover(context.Background(), Request{})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if inst.ExePath == "" {
		t.Fatal("empty ExePath")
	}
	if inst.NormalizedVersion == "" {
		t.Fatal("empty NormalizedVersion")
	}
	if inst.DiscoverySource != "PATH" {
		t.Fatalf("DiscoverySource = %q, want PATH", inst.DiscoverySource)
	}
}

func TestDiscoverNodeNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := Discover(ctx, Request{ExplicitCandidate: "no-such-node-exe-999"})
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func strSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
