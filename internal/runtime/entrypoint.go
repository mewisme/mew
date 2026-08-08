package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mewisme/mew/internal/apperr"
)

// runtimeExts is the set of extensions recognized as runtime entrypoints.
var runtimeExts = map[string]bool{
	".js":  true,
	".mjs": true,
	".cjs": true,
	".ts":  true,
	".tsx": true,
	".mts": true,
	".cts": true,
}

// nextPlanExts are extensions not yet supported by the runtime.
var nextPlanExts = map[string]string{
	".jsx": "future",
}

// IsJSFile reports whether the selector looks like a runtime file (has a supported
// extension or contains a directory separator).
// Deprecated: use IsRuntimeFile for new code.
func IsJSFile(selector string) bool {
	return IsRuntimeFile(selector)
}

// IsRuntimeFile reports whether the selector looks like a runtime file.
func IsRuntimeFile(selector string) bool {
	if selector == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(selector))
	if runtimeExts[ext] {
		return true
	}
	// Unsupported extensions are still recognized as runtime files so the
	// dispatcher can give an actionable message instead of "unknown command".
	if _, ok := nextPlanExts[ext]; ok {
		return true
	}
	// A path with a directory separator is likely a file reference.
	if strings.ContainsAny(selector, "/\\") {
		return true
	}
	return false
}

// IsNextPlanExt reports whether the extension is not yet supported and
// returns the deferral key.
func IsNextPlanExt(selector string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(selector))
	plan, ok := nextPlanExts[ext]
	return plan, ok
}

// ResolveEntrypoint resolves a selector to an absolute filesystem path.
// Returns the absolute path, or a typed error if the entrypoint is invalid.
func ResolveEntrypoint(cwd, selector string) (string, error) {
	if selector == "" {
		return "", apperr.New(apperr.RuntimeEntrypoint, "runtime.entrypoint", "", "empty entrypoint")
	}
	if !filepath.IsAbs(selector) {
		selector = filepath.Join(cwd, selector)
	}
	abs, err := filepath.Abs(selector)
	if err != nil {
		return "", apperr.Wrap(apperr.RuntimeEntrypoint, "runtime.entrypoint", selector, err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", apperr.New(apperr.RuntimeEntrypoint, "runtime.entrypoint", abs,
				fmt.Sprintf("entrypoint not found: %s", abs))
		}
		return "", apperr.Wrap(apperr.RuntimeEntrypoint, "runtime.entrypoint", abs, err)
	}

	if info.IsDir() {
		return "", apperr.New(apperr.RuntimeEntrypoint, "runtime.entrypoint", abs,
			"entrypoint is a directory, not a file")
	}

	ext := strings.ToLower(filepath.Ext(abs))
	if !runtimeExts[ext] {
		return "", apperr.New(apperr.RuntimeEntrypoint, "runtime.entrypoint", abs,
			fmt.Sprintf("unsupported file extension %q; expected .js, .mjs, .cjs, .ts, .tsx, .mts, or .cts", ext))
	}

	return abs, nil
}
