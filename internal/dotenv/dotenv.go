// Package dotenv parses .env files with variable expansion and provides
// mode-aware discovery following the precedence: .env.[mode].local >
// .env.[mode] > .env.local > .env.
package dotenv

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Parse reads a .env file from r and returns key-value pairs.
// Lines use the format KEY=value or export KEY=value.
// Values may be unquoted, double-quoted, or single-quoted.
// Double-quoted values support \n, \r, \t, \\, \" escapes and ${VAR}/$VAR expansion.
// Single-quoted values are literal.
// Unquoted values support variable expansion and strip trailing whitespace.
// Blank lines and lines starting with # (outside quotes) are skipped.
// source is used in error messages only.
func Parse(r io.Reader, source string) (map[string]string, error) {
	return parseInto(r, source, nil)
}

// parseInto reads a .env file from r, using baseVars as the initial expansion
// context so that later files can expand variables defined by earlier files.
// Variables defined in this file are also available to subsequent lines within
// the same file (sequential evaluation).
func parseInto(r io.Reader, source string, baseVars map[string]string) (map[string]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("dotenv: reading %s: %w", source, err)
	}
	// Build expansion context: copy baseVars so we don't mutate caller's map.
	expVars := make(map[string]string, len(baseVars))
	for k, v := range baseVars {
		expVars[k] = v
	}
	result := make(map[string]string)
	lines := strings.SplitSeq(string(data), "\n")
	lineNo := 0
	for line := range lines {
		lineNo++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Strip "export " prefix.
		trimmed = strings.TrimPrefix(trimmed, "export ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eq := strings.IndexByte(trimmed, '=')
		if eq < 0 {
			return nil, fmt.Errorf("dotenv: %s:%d: missing '=' in assignment: %q", source, lineNo, trimmed)
		}
		key := strings.TrimSpace(trimmed[:eq])
		if key == "" {
			return nil, fmt.Errorf("dotenv: %s:%d: empty key before '='", source, lineNo)
		}
		if !validKey(key) {
			return nil, fmt.Errorf("dotenv: %s:%d: invalid key %q", source, lineNo, key)
		}
		rawValue := trimmed[eq+1:]
		value, err := parseValue(rawValue, expVars)
		if err != nil {
			return nil, fmt.Errorf("dotenv: %s:%d: %w", source, lineNo, err)
		}
		result[key] = value
		expVars[key] = value
	}
	return result, nil
}

func parseValue(raw string, vars map[string]string) (string, error) {
	raw = strings.TrimLeft(raw, " \t")
	if raw == "" {
		return "", nil
	}
	switch raw[0] {
	case '"':
		return parseDoubleQuoted(raw, vars)
	case '\'':
		return parseSingleQuoted(raw)
	default:
		return parseUnquoted(raw, vars)
	}
}

func parseDoubleQuoted(raw string, vars map[string]string) (string, error) {
	// Must end with unescaped ".
	if len(raw) < 2 {
		return "", fmt.Errorf("unterminated double-quoted value")
	}
	var b strings.Builder
	i := 1 // skip opening "
	for i < len(raw) {
		c := raw[i]
		if c == '"' {
			i++
			// Only trailing whitespace after closing quote.
			rest := strings.TrimSpace(raw[i:])
			if rest != "" && !strings.HasPrefix(rest, "#") {
				return "", fmt.Errorf("unexpected content after closing quote: %q", rest)
			}
			return b.String(), nil
		}
		if c == '\\' && i+1 < len(raw) {
			next := raw[i+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\', '"', '$':
				b.WriteByte(next)
			default:
				b.WriteByte('\\')
				b.WriteByte(next)
			}
			i += 2
			continue
		}
		if c == '$' {
			rest, err := expandVar(raw, i, vars)
			if err != nil {
				return "", err
			}
			b.WriteString(rest.val)
			i = rest.next
			continue
		}
		b.WriteByte(c)
		i++
	}
	return "", fmt.Errorf("unterminated double-quoted value")
}

func parseSingleQuoted(raw string) (string, error) {
	if len(raw) < 2 {
		return "", fmt.Errorf("unterminated single-quoted value")
	}
	end := strings.IndexByte(raw[1:], '\'')
	if end < 0 {
		return "", fmt.Errorf("unterminated single-quoted value")
	}
	value := raw[1 : 1+end]
	rest := strings.TrimSpace(raw[1+end+1:])
	if rest != "" && !strings.HasPrefix(rest, "#") {
		return "", fmt.Errorf("unexpected content after closing quote: %q", rest)
	}
	return value, nil
}

func parseUnquoted(raw string, vars map[string]string) (string, error) {
	var b strings.Builder
	i := 0
	for i < len(raw) {
		c := raw[i]
		if c == '#' && (i == 0 || raw[i-1] == ' ') {
			// Comment starts after whitespace.
			break
		}
		if c == '\\' && i+1 < len(raw) {
			next := raw[i+1]
			switch next {
			case '\\', '$', '#', ' ', '\t', '"', '\'':
				b.WriteByte(next)
				i += 2
				continue
			default:
				b.WriteByte(c)
			}
		} else if c == '$' {
			rest, err := expandVar(raw, i, vars)
			if err != nil {
				return "", err
			}
			b.WriteString(rest.val)
			i = rest.next
			continue
		} else {
			b.WriteByte(c)
		}
		i++
	}
	return strings.TrimSpace(b.String()), nil
}

type expandResult struct {
	val  string
	next int
}

func expandVar(raw string, start int, vars map[string]string) (expandResult, error) {
	if start+1 >= len(raw) {
		return expandResult{val: "$", next: start + 1}, nil
	}
	next := raw[start+1]
	switch {
	case next == '{':
		// ${VAR} or ${VAR:-default}
		closing := strings.IndexByte(raw[start+2:], '}')
		if closing < 0 {
			return expandResult{}, fmt.Errorf("unterminated variable expansion: ${")
		}
		name := raw[start+2 : start+2+closing]
		defaultVal := ""
		if colon := strings.Index(name, ":-"); colon >= 0 {
			defaultVal = name[colon+2:]
			name = name[:colon]
		}
		val, ok := vars[name]
		if !ok {
			val = os.Getenv(name)
		}
		if val == "" && defaultVal != "" {
			val = defaultVal
		}
		return expandResult{val: val, next: start + 2 + closing + 1}, nil
	case isIdentByte(next):
		// $VAR
		end := start + 1
		for end < len(raw) && isIdentByte(raw[end]) {
			end++
		}
		name := raw[start+1 : end]
		val, ok := vars[name]
		if !ok {
			val = os.Getenv(name)
		}
		return expandResult{val: val, next: end}, nil
	default:
		return expandResult{val: "$", next: start + 1}, nil
	}
}

func isIdentByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}

// validKey checks that key is a valid environment variable name.
// Must start with a letter or underscore, followed by letters, digits, or underscores.
func validKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if i == 0 && !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_') {
			return false
		}
		if !isIdentByte(c) {
			return false
		}
	}
	return true
}

// Open is the function used to open .env files. It is a variable so that
// tests can replace it with a failing implementation to simulate I/O errors
// without relying on platform-specific permission mechanisms.
var Open = os.Open

// Load parses multiple .env files and returns merged KEY=VALUE strings.
// Files are loaded in order; later files override earlier ones.
// Missing files are silently skipped. Use LoadRequired when every file must exist.
func Load(files []string) ([]string, error) {
	merged := make(map[string]string)
	for _, f := range files {
		fh, err := Open(f)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("dotenv: opening %s: %w", f, err)
		}
		parsed, err := parseInto(fh, f, merged)
		_ = fh.Close()
		if err != nil {
			return nil, err
		}
		for k, v := range parsed {
			merged[k] = v
		}
	}
	return mapToEnviron(merged), nil
}

// LoadRequired parses multiple .env files and returns merged KEY=VALUE strings.
// Files are loaded in order; later files override earlier ones.
// Unlike Load, missing files are NOT silently skipped — every file must exist
// and be readable/parseable, or an error is returned.
func LoadRequired(files []string) ([]string, error) {
	merged := make(map[string]string)
	for _, f := range files {
		fh, err := Open(f)
		if err != nil {
			return nil, fmt.Errorf("dotenv: opening %s: %w", f, err)
		}
		parsed, err := parseInto(fh, f, merged)
		_ = fh.Close()
		if err != nil {
			return nil, err
		}
		for k, v := range parsed {
			merged[k] = v
		}
	}
	return mapToEnviron(merged), nil
}

// Discover finds .env files in dir following mode-aware precedence.
// Mode can be "" (no mode-specific files). Returns paths in load order
// (lowest precedence first); loading in order merges correctly.
func Discover(dir, mode string) []string {
	var paths []string
	base := filepath.Join(dir, ".env")
	if _, err := os.Stat(base); err == nil {
		paths = append(paths, base)
	}
	local := filepath.Join(dir, ".env.local")
	if _, err := os.Stat(local); err == nil {
		paths = append(paths, local)
	}
	if mode != "" {
		modeFile := filepath.Join(dir, ".env."+mode)
		if _, err := os.Stat(modeFile); err == nil {
			paths = append(paths, modeFile)
		}
		modeLocal := filepath.Join(dir, ".env."+mode+".local")
		if _, err := os.Stat(modeLocal); err == nil {
			paths = append(paths, modeLocal)
		}
	}
	return paths
}

// MergeEnviron merges envOverlay entries into the current process environment.
// envOverlay entries are KEY=VALUE strings; later entries with the same key win.
// Returns the merged environment as KEY=VALUE strings.
func MergeEnviron(envOverlay []string) []string {
	if len(envOverlay) == 0 {
		return os.Environ()
	}
	overlay := make(map[string]string, len(envOverlay))
	for _, kv := range envOverlay {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				overlay[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	base := os.Environ()
	out := make([]string, 0, len(base)+len(overlay))
	for _, kv := range base {
		key := kv
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				key = kv[:i]
				break
			}
		}
		if _, replaced := overlay[key]; replaced {
			continue
		}
		out = append(out, kv)
	}
	// Add overlay in deterministic order.
	keys := make([]string, 0, len(overlay))
	for k := range overlay {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+overlay[k])
	}
	return out
}

func mapToEnviron(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}
