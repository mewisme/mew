// Package runtime: inspector flag normalization.
//
// All --inspect and --inspect-brk flags flow through ParseInspectorFlags
// for host/port validation, duplicate detection, and safe default bind
// policy enforcement. In --node mode (ZeroAugmentation) validation is
// minimal — only malformed values are rejected.

package runtime

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/mewisme/mew/internal/apperr"
)

// InspectorConfig is the normalized result of parsing inspector flags.
type InspectorConfig struct {
	// Mode is the inspector mode: none, run, or break-on-start.
	Mode InspectorMode
	// Host is the bind address (default "127.0.0.1").
	Host string
	// Port is the inspector port (0 means Node chooses ephemeral).
	Port int
	// ExplicitBind is true when the user explicitly specified host:port.
	ExplicitBind bool
}

// InspectorMode describes the inspector launch mode.
type InspectorMode int

const (
	InspectorNone InspectorMode = iota
	InspectorRun                // --inspect
	InspectorBrk                // --inspect-brk
)

// String returns a compact representation for diagnostics.
func (m InspectorMode) String() string {
	switch m {
	case InspectorRun:
		return "run"
	case InspectorBrk:
		return "brk"
	default:
		return "none"
	}
}

// Flag returns the Node CLI flag fragment for this mode.
func (m InspectorMode) Flag() string {
	switch m {
	case InspectorRun:
		return "--inspect"
	case InspectorBrk:
		return "--inspect-brk"
	default:
		return ""
	}
}

// DefaultInspectorHost is the safe default bind address.
const DefaultInspectorHost = "127.0.0.1"

// DefaultInspectorPort signals Node's default (9229) or ephemeral allocation.
const DefaultInspectorPort = 0

// remoteInspectorEnv is the env var that permits non-loopback inspector binds.
const remoteInspectorEnv = "MEW_EXPERIMENTAL_REMOTE_INSPECTOR=1"

// remoteInspectorAllowed reports whether non-loopback inspector binds are permitted.
func remoteInspectorAllowed() bool {
	for _, kv := range os.Environ() {
		if kv == remoteInspectorEnv {
			return true
		}
	}
	return false
}

// isLoopback reports whether host is a loopback address.
func isLoopback(host string) bool {
	if host == "" || host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	// Resolve hostname to check if it maps to loopback.
	if addrs, err := net.LookupHost(host); err == nil {
		for _, a := range addrs {
			if ip := net.ParseIP(a); ip != nil && ip.IsLoopback() {
				return true
			}
		}
	}
	return false
}

// ParseInspectorFlags extracts and validates inspector flags from raw V8 args.
//
// Returns the normalized InspectorConfig and a slice of remaining
// non-inspector V8 args. When no inspector flags are present, cfg is nil.
//
// Validation rules:
//   - At most one --inspect and one --inspect-brk flag.
//   - Duplicate flags of the same type with different values are rejected.
//   - Port must be in range 1–65535 (0 = default/auto, accepted).
//   - Host must be a valid IP or hostname.
//   - Non-loopback binds require MEW_EXPERIMENTAL_REMOTE_INSPECTOR=1.
//
// In --node mode (zeroAugmentation), only malformed values are rejected;
// the loopback policy is not enforced since native Node behavior is desired.
func ParseInspectorFlags(v8Args []string, zeroAugmentation bool) (*InspectorConfig, []string, error) {
	var (
		cfg       InspectorConfig
		runAddr   string // address from --inspect
		brkAddr   string // address from --inspect-brk
		runCount  int
		brkCount  int
		others    []string
		runValues []string // raw values for duplicate detection
		brkValues []string
	)

	for _, arg := range v8Args {
		name, value := splitInspectorArg(arg)
		switch name {
		case "--inspect":
			runCount++
			runValues = append(runValues, value)
			if value != "" {
				runAddr = value
			}
		case "--inspect-brk":
			brkCount++
			brkValues = append(brkValues, value)
			if value != "" {
				brkAddr = value
			}
		default:
			others = append(others, arg)
		}
	}

	if runCount == 0 && brkCount == 0 {
		return nil, others, nil
	}

	// Duplicate detection: same flag with different values.
	if err := checkDuplicateValues("--inspect", runValues); err != nil {
		return nil, nil, err
	}
	if err := checkDuplicateValues("--inspect-brk", brkValues); err != nil {
		return nil, nil, err
	}

	// Determine mode: --inspect-brk wins for mode when both are present.
	if brkCount > 0 {
		cfg.Mode = InspectorBrk
	} else {
		cfg.Mode = InspectorRun
	}

	// Resolve bind address. When both --inspect and --inspect-brk specify
	// addresses, --inspect-brk wins (matches Node behavior).
	addr := brkAddr
	if addr == "" {
		addr = runAddr
	}

	host, port, explicit, err := parseInspectorAddr(addr)
	if err != nil {
		return nil, nil, err
	}
	cfg.Host = host
	cfg.Port = port
	cfg.ExplicitBind = explicit

	// Security: non-loopback binds require explicit opt-in.
	// Skip this check in --node mode to preserve native Node behavior.
	if !zeroAugmentation && !isLoopback(cfg.Host) && !remoteInspectorAllowed() {
		return nil, nil, apperr.New(apperr.InspectorBind, "runtime.inspect", cfg.Host,
			fmt.Sprintf("non-loopback inspector bind %q requires %s (security policy: debugger must not be exposed to non-local interfaces by default)",
				cfg.Host, remoteInspectorEnv))
	}

	return &cfg, others, nil
}

// splitInspectorArg splits an arg into the flag name and its value.
// "--inspect" returns ("--inspect", "")
// "--inspect=127.0.0.1:9229" returns ("--inspect", "127.0.0.1:9229")
// "--max-old-space-size=4096" returns ("--max-old-space-size", "4096") — not an inspector flag.
func splitInspectorArg(arg string) (name, value string) {
	if !strings.HasPrefix(arg, "--") {
		return arg, ""
	}
	body := arg[2:]
	if idx := strings.IndexByte(body, '='); idx >= 0 {
		name = "--" + body[:idx]
		value = body[idx+1:]
	} else {
		name = arg
	}
	return name, value
}

// parseInspectorAddr parses a Node inspector address value.
// Format: [host]:port where both host and port are optional.
// Empty string returns defaults.
// IPv6 addresses are tried as bare addresses (Node inspector accepts "::1:9229"
// where the last colon-separated segment is the port).
func parseInspectorAddr(raw string) (host string, port int, explicit bool, err error) {
	if raw == "" {
		return DefaultInspectorHost, DefaultInspectorPort, false, nil
	}

	explicit = true

	// Special case: raw is just a port number (e.g., "9229")
	if looksLikePort(raw) {
		p, perr := parsePort(raw)
		if perr != nil {
			return "", 0, false, perr
		}
		return DefaultInspectorHost, p, true, nil
	}

	// Split host from port: find the last colon, check if the
	// segment after it is a port number. This handles bare IPv6
	// addresses like "::1:9229" (host=::1, port=9229) and
	// "::" (host=::, port default).
	lastColon := strings.LastIndex(raw, ":")
	if lastColon >= 0 {
		portPart := raw[lastColon+1:]
		hostPart := raw[:lastColon]

		// If the segment after the last colon looks like a port number,
		// split there. Otherwise treat the whole thing as host.
		if looksLikePort(portPart) && portPart != "" {
			p, perr := parsePort(portPart)
			if perr != nil {
				return "", 0, false, perr
			}
			port = p
			host = hostPart
		} else {
			host = raw
		}
	} else {
		host = raw
	}

	// Leading colon means default host (e.g., ":9229").
	if host == "" {
		host = DefaultInspectorHost
	}

	// Validate host.
	if host != "" && host != DefaultInspectorHost {
		if err := validateHost(host); err != nil {
			return "", 0, false, err
		}
	}

	if host == "" {
		host = DefaultInspectorHost
	}

	return host, port, explicit, nil
}

// looksLikePort reports whether s looks like a bare port number.
func looksLikePort(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parsePort parses and validates a port number.
func parsePort(s string) (int, error) {
	if s == "" || s == "0" {
		return 0, nil
	}
	p, err := strconv.Atoi(s)
	if err != nil {
		return 0, apperr.New(apperr.InspectorPort, "runtime.inspect", s,
			fmt.Sprintf("invalid inspector port %q: must be 1–65535", s))
	}
	if p < 1 || p > 65535 {
		return 0, apperr.New(apperr.InspectorPort, "runtime.inspect", s,
			fmt.Sprintf("inspector port %d out of range (1–65535)", p))
	}
	return p, nil
}

// validateHost checks that the host is a valid IP or hostname.
func validateHost(host string) error {
	if host == "" {
		return nil
	}
	// Check if it's a valid IP.
	if ip := net.ParseIP(host); ip != nil {
		return nil
	}
	// Check if it looks like a valid hostname.
	if len(host) > 253 {
		return apperr.New(apperr.InspectorHost, "runtime.inspect", host,
			fmt.Sprintf("invalid inspector host %q: hostname too long", host))
	}
	// Reject obviously malformed: contains spaces, starts/ends with dot/hyphen.
	if strings.ContainsAny(host, " \t\n") {
		return apperr.New(apperr.InspectorHost, "runtime.inspect", host,
			fmt.Sprintf("invalid inspector host %q", host))
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") ||
		strings.HasPrefix(host, "-") || strings.HasSuffix(host, "-") {
		return apperr.New(apperr.InspectorHost, "runtime.inspect", host,
			fmt.Sprintf("invalid inspector host %q", host))
	}
	return nil
}

// checkDuplicateValues checks that all values are the same.
// If there are multiple flags of the same type with different values, it's an error.
func checkDuplicateValues(flag string, values []string) error {
	if len(values) <= 1 {
		return nil
	}
	first := values[0]
	for _, v := range values[1:] {
		if v != first {
			return apperr.New(apperr.InspectorDup, "runtime.inspect", flag,
				fmt.Sprintf("conflicting %s values: %q and %q", flag, first, v))
		}
	}
	return nil
}

// BuildInspectorArgv returns the normalized inspector argv for the config.
// Returns nil when inspector is not active.
func (cfg *InspectorConfig) BuildInspectorArgv() []string {
	if cfg == nil || cfg.Mode == InspectorNone {
		return nil
	}

	var flag string
	switch cfg.Mode {
	case InspectorRun:
		flag = "--inspect"
	case InspectorBrk:
		flag = "--inspect-brk"
	default:
		return nil
	}

	if cfg.ExplicitBind || cfg.Port != 0 || cfg.Host != DefaultInspectorHost {
		if cfg.Port != 0 {
			flag += fmt.Sprintf("=%s:%d", cfg.Host, cfg.Port)
		} else if cfg.Host != DefaultInspectorHost {
			flag += fmt.Sprintf("=%s", cfg.Host)
		}
	}

	return []string{flag}
}
