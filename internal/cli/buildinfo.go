package cli

import (
	"path/filepath"
	"strings"
)

// BuildInfo is injected at link time for version output.
type BuildInfo struct {
	Version     string
	Commit      string
	ShortCommit string
	Dirty       bool
	BuildDate   string
	TargetOS    string
	TargetArch  string
}

// Short returns the 7-char commit prefix, or the explicit ShortCommit if set.
func (b BuildInfo) Short() string {
	if b.ShortCommit != "" {
		return b.ShortCommit
	}
	if len(b.Commit) >= 7 {
		return b.Commit[:7]
	}
	return b.Commit
}

// DirtyStr returns "+dirty" or "".
func (b BuildInfo) DirtyStr() string {
	if b.Dirty {
		return "+dirty"
	}
	return ""
}

// Target returns "os/arch" or "".
func (b BuildInfo) Target() string {
	if b.TargetOS == "" && b.TargetArch == "" {
		return ""
	}
	if b.TargetOS == "" {
		return b.TargetArch
	}
	if b.TargetArch == "" {
		return b.TargetOS
	}
	return b.TargetOS + "/" + b.TargetArch
}

// versionSuffix returns "(v1.2.3+abc1234)" for the help banner, or "" for dev without commit.
func versionSuffix(info BuildInfo) string {
	if info.Version == "" || info.Version == "dev" {
		if info.Short() == "" {
			return ""
		}
		return "(" + info.Short() + info.DirtyStr() + ")"
	}
	s := "v" + info.Version
	if c := info.Short(); c != "" {
		s += "+" + c
	}
	s += info.DirtyStr()
	return "(" + s + ")"
}

// InvokedBinary returns the logical binary name from os.Args[0] (m|mew|mx|mewx).
func InvokedBinary(argv0, fallback string) string {
	base := filepath.Base(argv0)
	base = strings.TrimSuffix(base, ".exe")
	base = strings.TrimSuffix(base, ".EXE")
	switch strings.ToLower(base) {
	case "m", "mew", "mx", "mewx":
		return strings.ToLower(base)
	default:
		return fallback
	}
}

// DisplayName returns the Use string for help (preserves github.com/mewisme/mew/mewx aliases).
func DisplayName(invoked string) string {
	switch invoked {
	case "mew":
		return "mew"
	case "mewx":
		return "mewx"
	case "mx":
		return "mx"
	default:
		return "m"
	}
}
