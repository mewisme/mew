package trace

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reBearer   = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`)
	reQueryTok = regexp.MustCompile(`(?i)([?&](?:access_token|authToken|token|password|secret)=)([^&\s]+)`)
	reEnvVal   = regexp.MustCompile(`(?i)([A-Z0-9_]*(?:TOKEN|PASSWORD|SECRET|API_KEY|CREDENTIAL)=)([^\s&]+)`)
)

// Redact removes credentials and common secret shapes from s.
// Uses the same patterns as diagnostics.Redact.
func Redact(s string) string {
	if s == "" {
		return s
	}
	s = redactURLs(s)
	s = reBearer.ReplaceAllString(s, "Bearer ***")
	s = reQueryTok.ReplaceAllString(s, "${1}***")
	s = reEnvVal.ReplaceAllString(s, "${1}***")
	return s
}

// RedactError sanitizes an error message for trace event inclusion.
// Returns empty string when err is nil.
func RedactError(err error) string {
	if err == nil {
		return ""
	}
	return Redact(err.Error())
}

func redactURLs(s string) string {
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" && u.User != nil {
		u.User = url.UserPassword("***", "***")
		return u.String()
	}
	parts := strings.Fields(s)
	changed := false
	for i, p := range parts {
		u, err := url.Parse(p)
		if err != nil || u.Scheme == "" || u.Host == "" || u.User == nil {
			continue
		}
		u.User = url.UserPassword("***", "***")
		parts[i] = u.String()
		changed = true
	}
	if changed {
		return strings.Join(parts, " ")
	}
	return s
}

// SafeFilePath returns a path suitable for trace inclusion. When the target
// path is under base, a relative path is returned to avoid leaking absolute
// machine-specific paths. If the relative path would escape the base (starts
// with ".."), the original absolute target is returned as-is.
func SafeFilePath(target, base string) string {
	if base == "" || target == "" {
		return target
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == "" {
		return target
	}
	if strings.HasPrefix(rel, "..") {
		return target
	}
	return rel
}
