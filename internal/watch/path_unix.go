//go:build !windows

package watch

import (
	"path/filepath"
	"strings"
)

// pathKey returns the canonical identity key for a path. On
// case-sensitive filesystems, this is the normalized path as-is.
func pathKey(p string) string {
	return normalizePath(p)
}

// hasPathPrefix reports whether path starts with prefix (as a directory
// boundary). On case-sensitive filesystems, this is a simple string
// prefix check.
func hasPathPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+string(filepath.Separator))
}

// segmentSkipped returns true when a directory segment should be
// excluded from recursive watching.
func segmentSkipped(seg string) bool {
	return skippedSegments[seg]
}
