//go:build windows

package watch

import (
	"path/filepath"
	"strings"
)

// pathKey returns the canonical identity key for a path. On Windows
// this lowercases the normalized path to provide case-insensitive
// identity for maps and comparisons.
func pathKey(p string) string {
	return strings.ToLower(normalizePath(p))
}

// hasPathPrefix reports whether path starts with prefix (as a directory
// boundary), using case-insensitive comparison on Windows.
func hasPathPrefix(path, prefix string) bool {
	sep := string(filepath.Separator)
	pathLower := strings.ToLower(path)
	prefixLower := strings.ToLower(prefix)
	return pathLower == prefixLower ||
		strings.HasPrefix(pathLower, prefixLower+sep)
}

// segmentSkipped returns true when a directory segment should be
// excluded from recursive watching. Uses case-insensitive comparison
// on Windows.
func segmentSkipped(seg string) bool {
	lower := strings.ToLower(seg)
	for name := range skippedSegments {
		if strings.ToLower(name) == lower {
			return true
		}
	}
	return false
}
