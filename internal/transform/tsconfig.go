package transform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConfigErrorKind classifies tsconfig failures for stable error mapping.
type ConfigErrorKind string

const (
	ConfigErrIO                ConfigErrorKind = "io"
	ConfigErrParse             ConfigErrorKind = "parse"
	ConfigErrExtendsMissing    ConfigErrorKind = "extends_missing"
	ConfigErrExtendsCycle      ConfigErrorKind = "extends_cycle"
	ConfigErrExtendsDepth      ConfigErrorKind = "extends_depth"
	ConfigErrExtendsPackage    ConfigErrorKind = "extends_package"
	ConfigErrExtendsInvalid    ConfigErrorKind = "extends_invalid"
	ConfigErrOptionInvalid     ConfigErrorKind = "option_invalid"
	ConfigErrOptionUnsupported ConfigErrorKind = "option_unsupported"
)

// ConfigError is a typed tsconfig failure that preserves the config path
// without exposing file contents.
type ConfigError struct {
	Kind ConfigErrorKind
	Path string
	Err  error
}

func (e *ConfigError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("tsconfig %s: %s: %v", e.Path, e.Kind, e.Err)
	}
	return fmt.Sprintf("tsconfig %s: %s", e.Path, e.Kind)
}

func (e *ConfigError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NormalizedOptions collects transform-relevant tsconfig options.
// Only options that affect transform output are included; options that
// only affect type-checking (noEmit) are excluded.
// baseUrl and paths affect module resolution (0052+) and are carried
// for cache key stability.

// PathMapping is a single tsconfig paths entry in canonical order.
// Pattern is the alias key (e.g. "@app/*", "@app/internal/*", "*").
// Targets are the replacement values in declared order.
type PathMapping struct {
	Pattern string   `json:"pattern"`
	Targets []string `json:"targets"`
}

// sortPathMappings orders path mappings by TypeScript specificity:
//  1. Exact patterns (no wildcard) first, sorted alphabetically.
//  2. Wildcard patterns sorted by:
//     a. Longer prefix before the first '*' (more specific) first.
//     b. Shorter suffix after the last '*' (more specific) first.
//     c. Alphabetically for deterministic tie-breaking.
func sortPathMappings(mappings []PathMapping) {
	sort.Slice(mappings, func(i, j int) bool {
		pi, pj := mappings[i].Pattern, mappings[j].Pattern
		hasStarI := strings.Contains(pi, "*")
		hasStarJ := strings.Contains(pj, "*")

		// Exact patterns (no '*') come before wildcard patterns.
		if hasStarI != hasStarJ {
			return !hasStarI
		}

		if hasStarI {
			// Both wildcard: compare prefix length before first '*'.
			preI := strings.Index(pi, "*")
			preJ := strings.Index(pj, "*")
			if preI != preJ {
				return preI > preJ // longer prefix first
			}
			// Compare suffix length after last '*'.
			sufI := len(pi) - strings.LastIndex(pi, "*") - 1
			sufJ := len(pj) - strings.LastIndex(pj, "*") - 1
			if sufI != sufJ {
				return sufI < sufJ // shorter suffix first
			}
		}

		// Tie-break: alphabetical.
		return pi < pj
	})
}

type NormalizedOptions struct {
	Target string `json:"target,omitempty"`
	Module string `json:"module,omitempty"`
	// UseDefineForClassFields uses *bool to distinguish absent from explicit false.
	// nil (absent)    → esbuild default (true)
	// &true (explicit) → pass through to esbuild
	// &false (explicit) → pass through to esbuild
	UseDefineForClassFields *bool `json:"useDefineForClassFields,omitempty"`
	// VerbatimModuleSyntax uses *bool to distinguish absent from explicit false.
	// nil (absent)    → esbuild default
	// &true (explicit) → pass through to esbuild
	// &false (explicit) → pass through to esbuild
	VerbatimModuleSyntax *bool               `json:"verbatimModuleSyntax,omitempty"`
	ImportHelpers        bool                `json:"importHelpers,omitempty"`
	BaseURL              string              `json:"baseUrl,omitempty"`
	Paths                map[string][]string `json:"paths,omitempty"`
	// PathMappings is the canonical ordered representation of Paths,
	// sorted by TypeScript specificity (exact first, then longest prefix).
	// The JS loader reads this field for deterministic resolution.
	// Paths is retained for parsing/merge and backward compatibility.
	PathMappings []PathMapping `json:"pathMappings,omitempty"`

	// JSX
	JSX                string `json:"jsx,omitempty"`
	JSXFactory         string `json:"jsxFactory,omitempty"`
	JSXFragmentFactory string `json:"jsxFragmentFactory,omitempty"`
	JSXImportSource    string `json:"jsxImportSource,omitempty"`

	// Decorators
	ExperimentalDecorators bool `json:"experimentalDecorators,omitempty"`
	EmitDecoratorMetadata  bool `json:"emitDecoratorMetadata,omitempty"`

	// Source maps.
	// InlineSources uses *bool to distinguish:
	//   nil (absent)      → default (include source content, matching tsc default)
	//   &true (explicit)  → include source content
	//   &false (explicit) → exclude source content
	SourceMap       bool   `json:"sourceMap,omitempty"`
	InlineSourceMap bool   `json:"inlineSourceMap,omitempty"`
	InlineSources   *bool  `json:"inlineSources,omitempty"`
	SourceRoot      string `json:"sourceRoot,omitempty"`
	MapRoot         string `json:"mapRoot,omitempty"`
	// mapRoot is parsed and included in cache keys but NOT forwarded to esbuild.
	// esbuild has no MapRoot option. mapRoot specifies where external tooling
	// expects to find .map files; Mew manages map lifecycle through the cache,
	// so mapRoot is informational (cache-keyed) only.
}

// NormalizedOptionsDigest returns a stable SHA-256 of the normalized options.
func (o NormalizedOptions) Digest() string {
	data, _ := json.Marshal(o)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// maxTsconfigDepth is the maximum extends chain depth.
const maxTsconfigDepth = 20

// DiscoverTsconfig searches upward from sourceDir to find the nearest tsconfig.json.
// Permission and I/O errors during discovery are propagated as ConfigError.
func DiscoverTsconfig(sourceDir string) (string, error) {
	dir := filepath.Clean(sourceDir)
	for {
		candidate := filepath.Join(dir, "tsconfig.json")
		info, err := os.Stat(candidate)
		if err == nil {
			if info.IsDir() {
				return "", &ConfigError{Kind: ConfigErrIO, Path: candidate, Err: fmt.Errorf("tsconfig path is a directory")}
			}
			return candidate, nil
		}
		if !os.IsNotExist(err) {
			return "", &ConfigError{Kind: ConfigErrIO, Path: candidate, Err: err}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil // reached root, no tsconfig
		}
		dir = parent
	}
}

// LoadTsconfigChain loads a tsconfig and resolves its extends chain.
func LoadTsconfigChain(configPath string) ([]TsconfigFile, error) {
	return resolveExtends(configPath, nil, 0)
}

// TsconfigFile is a parsed tsconfig with metadata.
type TsconfigFile struct {
	Path   string
	Raw    map[string]any
	Digest string
	Parent *TsconfigFile
}

// resolveExtends recursively loads the extends chain.
func resolveExtends(path string, visited map[string]bool, depth int) ([]TsconfigFile, error) {
	if depth > maxTsconfigDepth {
		return nil, &ConfigError{Kind: ConfigErrExtendsDepth, Path: path, Err: fmt.Errorf("extends chain exceeds maximum depth %d", maxTsconfigDepth)}
	}
	if visited == nil {
		visited = make(map[string]bool)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, &ConfigError{Kind: ConfigErrIO, Path: path, Err: err}
	}
	if visited[abs] {
		return nil, &ConfigError{Kind: ConfigErrExtendsCycle, Path: abs, Err: fmt.Errorf("cycle detected")}
	}
	visited[abs] = true

	raw, digest, err := parseTsconfigFile(abs)
	if err != nil {
		return nil, err
	}

	tsc := TsconfigFile{Path: abs, Raw: raw, Digest: digest}

	// resolve extends
	extends, ok := raw["extends"]
	if !ok {
		return []TsconfigFile{tsc}, nil
	}
	parentPath, ok := extends.(string)
	if !ok || parentPath == "" {
		return nil, &ConfigError{Kind: ConfigErrExtendsInvalid, Path: abs, Err: fmt.Errorf("extends must be a non-empty string")}
	}

	// relative extends
	resolved, err := resolveExtendsPath(abs, parentPath)
	if err != nil {
		return nil, err
	}
	parents, err := resolveExtends(resolved, visited, depth+1)
	if err != nil {
		// If the extends target doesn't exist on disk, classify as extends_missing.
		if isExtendsFileNotFound(err) {
			return nil, &ConfigError{Kind: ConfigErrExtendsMissing, Path: resolved, Err: err}
		}
		return nil, err
	}
	tsc.Parent = &parents[len(parents)-1]
	return append(parents, tsc), nil
}

// isExtendsFileNotFound reports whether err is caused by a missing extends target file.
func isExtendsFileNotFound(err error) bool {
	var cfgErr *ConfigError
	if errors.As(err, &cfgErr) && cfgErr.Kind == ConfigErrIO && cfgErr.Err != nil {
		return os.IsNotExist(cfgErr.Err)
	}
	return os.IsNotExist(err)
}

// resolveExtendsPath resolves an extends path relative to the config file.
// Package-style extends (e.g. "@scope/tsconfig") return ConfigErrExtendsPackage.
func resolveExtendsPath(configPath, extends string) (string, error) {
	if strings.HasPrefix(extends, ".") {
		base := filepath.Dir(configPath)
		resolved := filepath.Join(base, extends)
		if filepath.Ext(resolved) == "" {
			resolved += ".json"
		}
		return resolved, nil
	}
	return "", &ConfigError{Kind: ConfigErrExtendsPackage, Path: configPath, Err: fmt.Errorf("package-style extends %q not yet supported", extends)}
}

// parseTsconfigFile reads and JSONC-parses a tsconfig.json file.
func parseTsconfigFile(path string) (map[string]any, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", &ConfigError{Kind: ConfigErrIO, Path: path, Err: err}
	}
	cleaned := stripJSONC(data)

	var raw map[string]any
	if err := json.Unmarshal(cleaned, &raw); err != nil {
		return nil, "", &ConfigError{Kind: ConfigErrParse, Path: path, Err: err}
	}

	// Extract compilerOptions for normalization; keep extends at top level.
	h := sha256.New()
	h.Write(cleaned)
	digest := hex.EncodeToString(h.Sum(nil))
	return raw, digest, nil
}

// TsconfigChainDigest returns a stable digest combining all config digests in the chain.
func TsconfigChainDigest(chain []TsconfigFile) string {
	h := sha256.New()
	for _, tsc := range chain {
		h.Write([]byte(tsc.Digest))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// NormalizeOptions extracts and normalizes transform-relevant options from a chain.
// Returns ConfigErrOptionInvalid when a compiler option has an unexpected type.
func NormalizeOptions(chain []TsconfigFile) (NormalizedOptions, error) {
	opts := NormalizedOptions{}
	for _, tsc := range chain {
		if err := applyCompilerOptions(&opts, tsc.Path, tsc.Raw); err != nil {
			return NormalizedOptions{}, err
		}
	}
	// importHelpers requires tslib integration. esbuild does not support
	// importing helpers from tslib; helpers are always inlined.
	if opts.ImportHelpers {
		return NormalizedOptions{}, &ConfigError{
			Kind: ConfigErrOptionUnsupported,
			Path: chain[len(chain)-1].Path,
			Err:  fmt.Errorf("importHelpers is not supported: esbuild always inlines helper functions; tslib imports are not available"),
		}
	}
	// emitDecoratorMetadata requires type information only available to a
	// type checker. Mew is a transpiler; reject it explicitly rather than
	// silently ignoring it.
	if opts.EmitDecoratorMetadata {
		return NormalizedOptions{}, &ConfigError{
			Kind: ConfigErrOptionUnsupported,
			Path: chain[len(chain)-1].Path,
			Err:  fmt.Errorf("emitDecoratorMetadata is not supported: Mew is a transpiler, not a type checker; metadata emission requires compiler type information"),
		}
	}

	// Build canonical ordered path mappings for deterministic resolution.
	if len(opts.Paths) > 0 {
		opts.PathMappings = make([]PathMapping, 0, len(opts.Paths))
		for pattern, targets := range opts.Paths {
			opts.PathMappings = append(opts.PathMappings, PathMapping{
				Pattern: pattern,
				Targets: targets,
			})
		}
		sortPathMappings(opts.PathMappings)
	}

	return opts, nil
}

// applyCompilerOptions applies compilerOptions from a tsconfig raw document.
// Options later in the chain (child configs) override earlier ones (parent configs).
// A child key that is absent does not clear the parent value.
func applyCompilerOptions(opts *NormalizedOptions, path string, raw map[string]any) error {
	co, ok := raw["compilerOptions"]
	if !ok {
		return nil
	}
	coMap, ok := co.(map[string]any)
	if !ok {
		return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("compilerOptions must be an object")}
	}

	// String options: child overwrites parent.
	if v, ok := coMap["target"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("target must be a string")}
		}
		opts.Target = s
	}
	if v, ok := coMap["module"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("module must be a string")}
		}
		opts.Module = s
	}
	if v, ok := coMap["baseUrl"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("baseUrl must be a string")}
		}
		opts.BaseURL = s
	}
	if v, ok := coMap["jsx"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("jsx must be a string")}
		}
		opts.JSX = s
	}
	if v, ok := coMap["jsxFactory"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("jsxFactory must be a string")}
		}
		opts.JSXFactory = s
	}
	if v, ok := coMap["jsxFragmentFactory"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("jsxFragmentFactory must be a string")}
		}
		opts.JSXFragmentFactory = s
	}
	if v, ok := coMap["jsxImportSource"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("jsxImportSource must be a string")}
		}
		opts.JSXImportSource = s
	}

	// Boolean options: child overrides parent. Distinguish explicit false from absent.
	if v, ok := coMap["useDefineForClassFields"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("useDefineForClassFields must be a boolean")}
		}
		val := b
		opts.UseDefineForClassFields = &val
	}
	if v, ok := coMap["verbatimModuleSyntax"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("verbatimModuleSyntax must be a boolean")}
		}
		val := b
		opts.VerbatimModuleSyntax = &val
	}
	if v, ok := coMap["importHelpers"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("importHelpers must be a boolean")}
		}
		opts.ImportHelpers = b
	}
	if v, ok := coMap["experimentalDecorators"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("experimentalDecorators must be a boolean")}
		}
		opts.ExperimentalDecorators = b
	}
	if v, ok := coMap["emitDecoratorMetadata"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("emitDecoratorMetadata must be a boolean")}
		}
		opts.EmitDecoratorMetadata = b
	}
	if v, ok := coMap["sourceMap"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("sourceMap must be a boolean")}
		}
		opts.SourceMap = b
	}
	if v, ok := coMap["inlineSourceMap"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("inlineSourceMap must be a boolean")}
		}
		opts.InlineSourceMap = b
	}
	if v, ok := coMap["inlineSources"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("inlineSources must be a boolean")}
		}
		val := b
		opts.InlineSources = &val
	}
	if v, ok := coMap["sourceRoot"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("sourceRoot must be a string")}
		}
		opts.SourceRoot = s
	}
	if v, ok := coMap["mapRoot"]; ok {
		s, isStr := v.(string)
		if !isStr {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("mapRoot must be a string")}
		}
		opts.MapRoot = s
	}

	// Paths: child paths replace parent paths for the same key.
	if v, ok := coMap["paths"]; ok {
		pathsMap, ok := v.(map[string]any)
		if !ok {
			return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("paths must be an object")}
		}
		if opts.Paths == nil {
			opts.Paths = make(map[string][]string)
		}
		for k, pv := range pathsMap {
			switch pvs := pv.(type) {
			case []any:
				vals := make([]string, 0, len(pvs))
				for _, p := range pvs {
					if ps, ok := p.(string); ok {
						vals = append(vals, ps)
					}
				}
				opts.Paths[k] = vals
			case []string:
				opts.Paths[k] = append([]string(nil), pvs...)
			default:
				return &ConfigError{Kind: ConfigErrOptionInvalid, Path: path, Err: fmt.Errorf("paths.%s must be an array of strings", k)}
			}
		}
	}

	return nil
}

// UnsupportedOptions returns tsconfig option names that are unsupported.
func UnsupportedOptions(raw map[string]any) []string {
	recognized := OptionSet()
	var unsupported []string
	for k := range raw {
		if !recognized[strings.TrimSpace(k)] {
			unsupported = append(unsupported, k)
		}
	}
	sort.Strings(unsupported)
	return unsupported
}
