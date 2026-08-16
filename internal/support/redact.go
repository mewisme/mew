package support

import (
	"path/filepath"
	"strings"

	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/trace"
)

// SanitizeError redacts an error for inclusion in a support bundle.
// Uses the canonical trace.RedactError which applies Bearer/query-token/
// env-secret redaction. Never returns the raw error string.
func SanitizeError(err error) string {
	return trace.RedactError(err)
}

// SanitizeString applies trace-level redaction to an arbitrary string.
// Never used on structured data; DTOs must be built field-by-field.
func SanitizeString(s string) string {
	return trace.Redact(s)
}

// SafePath returns a path suitable for support-bundle inclusion.
// Project-relative when under base; sanitizes home-directory prefixes
// and other private path components when they can be detected.
func SafePath(target, base string) string {
	p := trace.SafeFilePath(target, base)
	// If still absolute, strip common sensitive prefixes.
	p = stripHomeDir(p)
	return p
}

// stripHomeDir replaces known home-directory prefixes with "~".
func stripHomeDir(p string) string {
	if !filepath.IsAbs(p) {
		return p
	}
	// Common home paths on Unix.
	for _, prefix := range []string{"/home/", "/Users/"} {
		if strings.HasPrefix(p, prefix) {
			rest := strings.TrimPrefix(p, prefix)
			if idx := strings.IndexByte(rest, filepath.Separator); idx > 0 {
				return "~" + string(filepath.Separator) + rest[idx+1:]
			}
			return "~"
		}
	}
	return p
}

// RedactConfigValue applies config-level redaction. Secret keys
// always return the RedactedPlaceholder sentinel.
func RedactConfigValue(key string, raw any) any {
	return config.RedactValue(key, raw)
}

// RedactedPlaceholder is the sentinel for suppressed values.
const RedactedPlaceholder = config.RedactedPlaceholder
