package runtime

import (
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/node"
)

func TestEnforceCapabilitiesVersionFloor(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		caps      []string
		wantErr   bool
		errSubstr string // if wantErr, error message must contain this
	}{
		{
			name:      "below floor 18.18.0",
			version:   "18.18.0",
			caps:      []string{"require-preload", "import-preload"},
			wantErr:   true,
			errSubstr: "below minimum supported version",
		},
		{
			name:      "below floor 18.0.0",
			version:   "18.0.0",
			caps:      []string{"require-preload", "import-preload"},
			wantErr:   true,
			errSubstr: "below minimum supported version",
		},
		{
			name:      "below floor 16.20.0",
			version:   "16.20.0",
			caps:      []string{"require-preload", "import-preload"},
			wantErr:   true,
			errSubstr: "below minimum supported version",
		},
		{
			name:    "at floor 18.19.0 with all caps",
			version: "18.19.0",
			caps:    []string{"require-preload", "import-preload", "module-register"},
			wantErr: false,
		},
		{
			name:    "above floor 18.20.0",
			version: "18.20.0",
			caps:    []string{"require-preload", "import-preload", "module-register"},
			wantErr: false,
		},
		{
			name:    "above floor 20.6.0",
			version: "20.6.0",
			caps:    []string{"require-preload", "import-preload", "module-register", "source-maps"},
			wantErr: false,
		},
		{
			name:    "above floor 22.11.0",
			version: "22.11.0",
			caps:    []string{"require-preload", "import-preload", "module-register", "source-maps"},
			wantErr: false,
		},
		// Below-floor with partial capability sets: version check fires first.
		{
			name:      "below floor no caps",
			version:   "16.0.0",
			caps:      nil,
			wantErr:   true,
			errSubstr: "below minimum supported version",
		},
		// Edge case: version at floor but missing module-register capability.
		// This is a synthetic edge — real detectCapabilities won't produce this.
		{
			name:      "at floor but missing module-register",
			version:   "18.19.0",
			caps:      []string{"require-preload", "import-preload"},
			wantErr:   true,
			errSubstr: "lacks required capability",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &node.Installation{
				NormalizedVersion: tt.version,
				Capabilities:      tt.caps,
			}
			err := enforceCapabilities(inst, "test.js")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				code := apperr.CodeOf(err)
				if code != apperr.RuntimeNodeUnsupported {
					t.Errorf("error code = %s, want %s", code, apperr.RuntimeNodeUnsupported)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestEnforceCapabilitiesErrorReportsVersion(t *testing.T) {
	inst := &node.Installation{
		NormalizedVersion: "18.18.0",
		Capabilities:      []string{"require-preload", "import-preload"},
	}
	err := enforceCapabilities(inst, "test.js")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	// Error must include the detected version.
	if !strings.Contains(msg, "18.18.0") {
		t.Errorf("error does not include detected version: %q", msg)
	}
	// Error must include the minimum version.
	if !strings.Contains(msg, node.MinRuntimeVersion) {
		t.Errorf("error does not include minimum version %q: %q", node.MinRuntimeVersion, msg)
	}
	// Error must reference module.register.
	if !strings.Contains(msg, "module.register") {
		t.Errorf("error does not reference module.register: %q", msg)
	}
}
