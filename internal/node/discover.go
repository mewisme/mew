package node

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mewisme/mew/internal/apperr"
)

// Request is a node resolution request.
type Request struct {
	ProjectRoot          string
	WorkingDir           string
	ExplicitCandidate    string
	RequiredCapabilities []string
}

// Installation is a resolved node installation.
type Installation struct {
	ExePath           string
	NormalizedVersion string
	Capabilities      []string
	DiscoverySource   string
}

// Discover finds a stock Node binary on PATH.
func Discover(ctx context.Context, req Request) (*Installation, error) {
	nodeExe := req.ExplicitCandidate
	if nodeExe == "" {
		nodeExe = "node"
	}

	exePath, err := exec.LookPath(nodeExe)
	if err != nil {
		return nil, apperr.Wrap(apperr.RuntimeNodeNotFound, "node.discover", nodeExe, err)
	}

	version, err := queryVersion(ctx, exePath)
	if err != nil {
		return nil, err
	}

	norm, err := normalizeVersion(version)
	if err != nil {
		return nil, apperr.Wrap(apperr.RuntimeNodeVersion, "node.discover", version, err)
	}

	source := "PATH"
	if req.ExplicitCandidate != "" {
		source = "explicit"
	}

	return &Installation{
		ExePath:           exePath,
		NormalizedVersion: norm,
		Capabilities:      detectCapabilities(norm),
		DiscoverySource:   source,
	}, nil
}

func queryVersion(ctx context.Context, exePath string) (string, error) {
	cmd := exec.CommandContext(ctx, exePath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", apperr.Wrap(apperr.RuntimeNodeVersion, "node.query-version", exePath, err)
	}
	raw := strings.TrimSpace(string(out))
	raw = strings.TrimPrefix(raw, "v")
	return raw, nil
}

func normalizeVersion(raw string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), ".", 3)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid node version %q", raw)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid node major version %q", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid node minor version %q", parts[1])
	}
	patch := 0
	if len(parts) >= 3 {
		patch, _ = strconv.Atoi(parts[2])
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

// MinRuntimeVersion is the minimum Node version required for Mew runtime
// augmentation. Node 18.19.0 is the first 18.x release that includes
// module.register() as an experimental API (stable from 20.6.0).
// Versions below this floor cannot run Mew's augmented runtime and will be
// rejected with ERR_M_RUNTIME_NODE_UNSUPPORTED before any launch attempt.
const MinRuntimeVersion = "18.19.0"

func detectCapabilities(version string) []string {
	var caps []string
	parts := strings.SplitN(version, ".", 2)
	if len(parts) < 2 {
		return caps
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return caps
	}

	if major >= 16 {
		caps = append(caps, "import-preload")
	}
	if major >= 12 {
		caps = append(caps, "require-preload")
	}

	// module.register() is stable from Node 20.6, experimental from 18.19.
	if major > 20 || (major == 20 && parseMinor(parts) >= 6) ||
		(major == 18 && parseMinor(parts) >= 19) {
		caps = append(caps, "module-register")
	}

	// Node >= 20.6 has stable source-map support via --enable-source-maps.
	if major > 20 || (major == 20 && parseMinor(parts) >= 6) {
		caps = append(caps, "source-maps")
	}

	return caps
}

// parseMinor extracts the minor version from a "M.m" or "M.m.p" version split.
func parseMinor(parts []string) int {
	if len(parts) < 2 {
		return 0
	}
	minorParts := strings.SplitN(parts[1], ".", 2)
	n, _ := strconv.Atoi(minorParts[0])
	return n
}

// CompareVersion compares two normalized semver strings (M.m.p).
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// Returns 0 and false when either string is not a valid M.m.p version.
func CompareVersion(a, b string) (int, bool) {
	ma, mia, pa, okA := parseSemver(a)
	mb, mib, pb, okB := parseSemver(b)
	if !okA || !okB {
		return 0, false
	}
	if ma != mb {
		if ma < mb {
			return -1, true
		}
		return 1, true
	}
	if mia != mib {
		if mia < mib {
			return -1, true
		}
		return 1, true
	}
	if pa < pb {
		return -1, true
	}
	if pa > pb {
		return 1, true
	}
	return 0, true
}

// parseSemver parses major.minor.patch from a normalized version string.
func parseSemver(v string) (major, minor, patch int, ok bool) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, 0, false
	}
	var err error
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	if len(parts) >= 3 {
		patch, _ = strconv.Atoi(parts[2])
	}
	return major, minor, patch, true
}
