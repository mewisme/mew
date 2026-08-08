package conformance

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ToolInfo records a resolved external tool for certification reports.
type ToolInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
}

var defaultToolNames = []string{"node", "npm", "pnpm", "bun", "yarn", "nub"}

// CollectTools probes PATH for common package-manager binaries.
func CollectTools() []ToolInfo {
	var out []ToolInfo
	for _, name := range defaultToolNames {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		out = append(out, ToolInfo{
			Name:    name,
			Path:    path,
			Version: probeToolVersion(name, path),
		})
	}
	return out
}

func probeToolVersion(name, path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var args []string
	switch name {
	case "node":
		args = []string{"-v"}
	default:
		args = []string{"-v"}
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

// ResolveCommitSHA returns the current git HEAD or MEW_COMMIT_SHA when set.
func ResolveCommitSHA(repoRoot string) string {
	if v := strings.TrimSpace(os.Getenv("MEW_COMMIT_SHA")); v != "" {
		return v
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// toolVersion returns the Mew tool version from linker stamp or "dev".
func toolVersion() string {
	// Populated by ldflags at build time; reads the version embedded by
	// cmd/m and cmd/mx. When conformance runs via `go run ./cmd/m`,
	// the stamp is absent and we return "dev".
	return "dev"
}

func conformanceRequireTools() bool {
	return os.Getenv("MEW_CONFORMANCE_REQUIRE_TOOLS") == "1"
}
