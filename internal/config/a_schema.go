package config

import (
	"sort"
	"time"
)

// ValueType classifies config value semantics.
type ValueType string

const (
	TypeString   ValueType = "string"
	TypeBool     ValueType = "bool"
	TypeInt      ValueType = "int"
	TypeEnum     ValueType = "enum"
	TypeDuration ValueType = "duration"
)

// Scope names a configuration layer.
type Scope string

const (
	ScopeUser      Scope = "user"
	ScopeProject   Scope = "project"
	ScopeEffective Scope = "effective"
)

// ConfigKeySpec is the canonical typed specification for one config key.
type ConfigKeySpec struct {
	Key         string    // canonical dotted snake_case
	Group       string    // display group ("Installation", "Registry", etc.)
	Description string    // one-line human description
	Type        ValueType // value type
	Default     any       // default value
	Enum        []string  // allowed values for TypeEnum
	Minimum     *int64    // minimum for TypeInt (nil = no min)
	Maximum     *int64    // maximum for TypeInt (nil = no max)
	Scopes      []Scope   // writable scopes; empty = all writable scopes
	Secret      bool      // redact in output
	Deprecated  bool
	Replacement string   // canonical replacement key
	Commands    []string // e.g. "install", "add"
}

// CanonicalKey returns the canonical form for a possibly-legacy key.
// Returns "" when key is completely unknown.
func CanonicalKey(raw string) string {
	if c, ok := legacyToCanonical[raw]; ok {
		return c
	}
	if _, ok := keyRegistry[raw]; ok {
		return raw
	}
	return ""
}

// LegacyKey returns the legacy name for a canonical key, or "".
func LegacyKey(canonical string) string {
	for leg, can := range legacyToCanonical {
		if can == canonical {
			return leg
		}
	}
	return ""
}

// IsCanonical reports whether key is a registered canonical key.
func IsCanonical(key string) bool {
	_, ok := keyRegistry[key]
	return ok
}

// IsKnown reports whether a key is recognized (canonical or legacy).
func IsKnown(key string) bool {
	if _, ok := keyRegistry[key]; ok {
		return true
	}
	_, ok := legacyToCanonical[key]
	return ok
}

// KeySpec returns the spec for a canonical key, or nil.
func KeySpec(key string) *ConfigKeySpec {
	s, ok := keyRegistry[key]
	if !ok {
		return nil
	}
	return &s
}

// RegisteredKeys returns sorted canonical keys.
func RegisteredKeys() []string {
	keys := make([]string, 0, len(keyRegistry))
	for k := range keyRegistry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// CanonicalDefaults returns defaults keyed by canonical name.
func CanonicalDefaults() map[string]any {
	m := make(map[string]any, len(keyRegistry))
	for k, s := range keyRegistry {
		m[k] = s.Default
	}
	return m
}

// CanonicalTypes returns type strings keyed by canonical name.
func CanonicalTypes() map[string]string {
	m := make(map[string]string, len(keyRegistry))
	for k, s := range keyRegistry {
		m[k] = string(s.Type)
	}
	return m
}

// Groups returns sorted unique group names from the registry.
func Groups() []string {
	seen := map[string]bool{}
	var groups []string
	for _, s := range keyRegistry {
		if s.Group == "" || seen[s.Group] {
			continue
		}
		seen[s.Group] = true
		groups = append(groups, s.Group)
	}
	sort.Strings(groups)
	return groups
}

// KeysByGroup returns sorted canonical keys for a group.
func KeysByGroup(group string) []string {
	var keys []string
	for k, s := range keyRegistry {
		if s.Group == group {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// legacyToCanonical maps recognized legacy forms to canonical keys.
var legacyToCanonical = map[string]string{
	// kebab-case → snake_case
	"prefer-offline": "prefer_offline",

	// camelCase → snake_case
	"resolve.autoInstallPeers":       "resolve.auto_install_peers",
	"resolve.strictPeerDependencies": "resolve.strict_peer_dependencies",
	"resolve.rejectDeprecated":       "resolve.reject_deprecated",
	"resolve.minimumReleaseAge":      "resolve.minimum_release_age",

	// ms-suffixed int → duration
	"network.timeout_ms": "network.timeout",
}

var secZero int64 = 0

// keyRegistry is the single source of truth for every config key.
var keyRegistry = map[string]ConfigKeySpec{
	// ── Registry ──────────────────────────────────────────────
	"registry": {
		Key: "registry", Group: "Registry", Type: TypeString,
		Default:     "https://registry.npmjs.org",
		Description: "Default package registry URL",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update", "dlx"},
	},
	"registry.auth_token_env": {
		Key: "registry.auth_token_env", Group: "Registry", Type: TypeString,
		Default:     "",
		Description: "Environment variable name holding the registry auth token",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Secret:      true,
		Commands:    []string{"install", "add", "update"},
	},

	// ── Network ───────────────────────────────────────────────
	"network.timeout": {
		Key: "network.timeout", Group: "Network", Type: TypeDuration,
		Default:     "60s",
		Description: "Network request timeout",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update", "fetch"},
	},
	"network.proxy": {
		Key: "network.proxy", Group: "Network", Type: TypeString,
		Default:     "",
		Description: "HTTPS proxy URL for registry requests",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},
	"network.ca_file": {
		Key: "network.ca_file", Group: "Network", Type: TypeString,
		Default:     "",
		Description: "Path to custom CA certificate bundle",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},

	// ── Installation ──────────────────────────────────────────
	"install.linker": {
		Key: "install.linker", Group: "Installation", Type: TypeEnum,
		Default:     "auto",
		Enum:        []string{"auto", "hoisted", "isolated"},
		Description: "Node_modules linking strategy",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update", "ci"},
	},
	"offline": {
		Key: "offline", Group: "Installation", Type: TypeBool,
		Default:     false,
		Description: "Run entirely from cache; fail on network miss",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update", "dlx"},
	},
	"prefer_offline": {
		Key: "prefer_offline", Group: "Installation", Type: TypeBool,
		Default:     false,
		Description: "Prefer cached artifacts; fall through to network on miss",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update", "dlx"},
	},

	// ── Resolve ───────────────────────────────────────────────
	"resolve.auto_install_peers": {
		Key: "resolve.auto_install_peers", Group: "Resolve", Type: TypeBool,
		Default:     false,
		Description: "Enqueue missing peers from the importer during resolution",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},
	"resolve.strict_peer_dependencies": {
		Key: "resolve.strict_peer_dependencies", Group: "Resolve", Type: TypeBool,
		Default:     true,
		Description: "Fail when required peers are unsatisfied",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},
	"resolve.reject_deprecated": {
		Key: "resolve.reject_deprecated", Group: "Resolve", Type: TypeBool,
		Default:     false,
		Description: "Reject packages marked as deprecated by the registry",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},
	"resolve.minimum_release_age": {
		Key: "resolve.minimum_release_age", Group: "Resolve", Type: TypeInt,
		Default:     0,
		Minimum:     &secZero,
		Description: "Minimum age in milliseconds before a release is eligible",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},

	// ── Lifecycle ─────────────────────────────────────────────
	"lifecycle.enabled": {
		Key: "lifecycle.enabled", Group: "Lifecycle", Type: TypeBool,
		Default:     false,
		Description: "Enable lifecycle script execution (experimental)",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},
	"lifecycle.ignore_scripts": {
		Key: "lifecycle.ignore_scripts", Group: "Lifecycle", Type: TypeBool,
		Default:     false,
		Description: "Skip all package lifecycle scripts",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},
	"lifecycle.script_trust": {
		Key: "lifecycle.script_trust", Group: "Lifecycle", Type: TypeEnum,
		Default:     "deny",
		Enum:        []string{"allow", "deny", "ask"},
		Description: "Default script trust policy",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},
	"lifecycle.script_timeout": {
		Key: "lifecycle.script_timeout", Group: "Lifecycle", Type: TypeDuration,
		Default:     "10m",
		Description: "Per-script execution timeout",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"run", "exec"},
	},

	// ── Transaction ───────────────────────────────────────────
	"transaction.snapshot_retention": {
		Key: "transaction.snapshot_retention", Group: "Transaction", Type: TypeInt,
		Default:     10,
		Minimum:     &secZero,
		Description: "Number of transaction snapshots to retain",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},

	// ── Storage ───────────────────────────────────────────────
	"cache.dir": {
		Key: "cache.dir", Group: "Storage", Type: TypeString,
		Default:     "",
		Description: "Package cache directory (empty = platform default)",
		Scopes:      []Scope{ScopeUser},
		Commands:    []string{"install", "add", "update", "dlx"},
	},
	"store.dir": {
		Key: "store.dir", Group: "Storage", Type: TypeString,
		Default:     "",
		Description: "Global content-addressable store directory",
		Scopes:      []Scope{ScopeUser},
		Commands:    []string{"install", "add", "update"},
	},
	"link.use_global_store": {
		Key: "link.use_global_store", Group: "Storage", Type: TypeBool,
		Default:     false,
		Description: "Use a global content-addressable store for packages (experimental)",
		Scopes:      []Scope{ScopeUser},
		Commands:    []string{"install", "add", "update"},
	},

	// ── Workspaces ────────────────────────────────────────────
	"workspaces.enabled": {
		Key: "workspaces.enabled", Group: "Workspaces", Type: TypeBool,
		Default:     false,
		Description: "Enable workspace support (experimental)",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"install", "add", "update"},
	},

	// ── Runner ────────────────────────────────────────────────
	"runner.direct_scripts.enabled": {
		Key: "runner.direct_scripts.enabled", Group: "Runner", Type: TypeBool,
		Default:     true,
		Description: "Enable direct script execution from CLI",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"run"},
	},
	"runner.mx.cache.retention_days": {
		Key: "runner.mx.cache.retention_days", Group: "Runner", Type: TypeInt,
		Default:     7,
		Minimum:     &secZero,
		Description: "DLX cache retention in days",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{"mx", "dlx"},
	},
	"runner.mx.cache.dir": {
		Key: "runner.mx.cache.dir", Group: "Runner", Type: TypeString,
		Default:     "",
		Description: "DLX cache directory (empty = platform default)",
		Scopes:      []Scope{ScopeUser},
		Commands:    []string{"mx", "dlx"},
	},

	// ── Provenance ────────────────────────────────────────────
	"provenance.trusted_public_key": {
		Key: "provenance.trusted_public_key", Group: "Provenance", Type: TypeString,
		Default:     "",
		Description: "Base64-encoded ed25519 public key for attestation verification",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Secret:      true,
		Commands:    []string{"install", "add"},
	},

	// ── Interface ─────────────────────────────────────────────
	"ui.theme": {
		Key: "ui.theme", Group: "Interface", Type: TypeEnum,
		Default:     "auto",
		Enum:        []string{"auto", "light", "dark"},
		Description: "Terminal color theme",
		Scopes:      []Scope{ScopeUser},
		Commands:    []string{},
	},
	"ui.pager": {
		Key: "ui.pager", Group: "Interface", Type: TypeString,
		Default:     "",
		Description: "Pager command (empty = auto-detect or disable)",
		Scopes:      []Scope{ScopeUser},
		Commands:    []string{},
	},

	// ── Logging ───────────────────────────────────────────────
	"log.level": {
		Key: "log.level", Group: "Logging", Type: TypeEnum,
		Default:     "error",
		Enum:        []string{"error", "warn", "info", "debug"},
		Description: "Minimum log level",
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Commands:    []string{},
	},
}

// resolveKey maps any recognized form (canonical or legacy) to its canonical key.
func resolveKey(raw string) (canonical string, legacy bool, ok bool) {
	if _, isCanon := keyRegistry[raw]; isCanon {
		return raw, false, true
	}
	if c, isLegacy := legacyToCanonical[raw]; isLegacy {
		return c, true, true
	}
	return "", false, false
}

// ParseDuration parses a Go duration string, or returns the zero value on empty.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}
