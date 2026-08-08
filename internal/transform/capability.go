// Package transform: canonical capability registry for compiler options.
//
// This registry is the single source of truth for compiler option support status.
// It drives validation diagnostics and the CLI parity report. Every recognized
// compiler option must have an entry here; unrecognized compilerOptions keys
// are rejected during normalization.
package transform

import "sort"

// Status classifies compiler option support.
type Status string

const (
	StatusSupported   Status = "supported"
	StatusPartial     Status = "partial"
	StatusUnsupported Status = "unsupported"
)

// Category groups compiler options by transform area.
type Category string

const (
	CategoryTypeScript Category = "TypeScript"
	CategoryJSX        Category = "JSX"
	CategoryDecorators Category = "Decorators"
	CategorySourceMaps Category = "SourceMaps"
)

// Entry describes a single compiler option in the capability registry.
type Entry struct {
	Option     string   `json:"option"`
	Status     Status   `json:"status"`
	Category   Category `json:"category"`
	Values     []string `json:"values,omitempty"`
	Limitation string   `json:"limitation,omitempty"`
}

// registry returns the canonical capability table.
// Every compiler option parsed by applyCompilerOptions must appear here.
// Options not in this table are unrecognized and rejected during normalization.
func registry() []Entry {
	return []Entry{
		// TypeScript target and module.
		{
			Option:   "target",
			Status:   StatusSupported,
			Category: CategoryTypeScript,
			Values:   []string{"ES5", "ES2015", "ES2016", "ES2017", "ES2018", "ES2019", "ES2020", "ES2021", "ES2022", "ES2023", "ES2024", "ESNext"},
			Limitation: "ES3 is downgraded to ES5 (esbuild minimum); unrecognized targets " +
				"fall back to Node version heuristic",
		},
		{
			Option:   "module",
			Status:   StatusSupported,
			Category: CategoryTypeScript,
			Values:   []string{"CommonJS", "ES6", "ES2015", "ES2020", "ES2022", "ESNext", "NodeNext", "Node16", "Preserve"},
			Limitation: "NodeNext/Node16 always map to ESM regardless of file extension; " +
				"unrecognized modules silently fall back to format inferred from the request",
		},
		{
			Option:     "useDefineForClassFields",
			Status:     StatusSupported,
			Category:   CategoryTypeScript,
			Limitation: "Passed through to esbuild via TsconfigRaw; esbuild defaults to true",
		},
		{
			Option:     "verbatimModuleSyntax",
			Status:     StatusSupported,
			Category:   CategoryTypeScript,
			Limitation: "Passed through to esbuild via TsconfigRaw; esbuild always elides type-only imports",
		},
		{
			Option:     "importHelpers",
			Status:     StatusUnsupported,
			Category:   CategoryTypeScript,
			Limitation: "esbuild does not support tslib helper imports; helpers are always inlined",
		},

		// Path resolution (resolution only, not transform).
		{
			Option:     "baseUrl",
			Status:     StatusPartial,
			Category:   CategoryTypeScript,
			Limitation: "Resolution only via 0053 resolve hook; not consumed by transform engine",
		},
		{
			Option:     "paths",
			Status:     StatusPartial,
			Category:   CategoryTypeScript,
			Limitation: "Resolution only via 0053 resolve hook; not consumed by transform engine",
		},

		// JSX.
		{
			Option:     "jsx",
			Status:     StatusSupported,
			Category:   CategoryJSX,
			Values:     []string{"react", "react-jsx", "react-jsxdev", "preserve"},
			Limitation: "Unrecognized values silently fall back to classic (react) transform",
		},
		{
			Option:     "jsxFactory",
			Status:     StatusSupported,
			Category:   CategoryJSX,
			Limitation: "Only applies when jsx mode is classic (react)",
		},
		{
			Option:     "jsxFragmentFactory",
			Status:     StatusSupported,
			Category:   CategoryJSX,
			Limitation: "Only applies when jsx mode is classic (react)",
		},
		{
			Option:     "jsxImportSource",
			Status:     StatusSupported,
			Category:   CategoryJSX,
			Limitation: "Only applies when jsx mode is automatic (react-jsx/react-jsxdev)",
		},

		// Decorators.
		{
			Option:   "experimentalDecorators",
			Status:   StatusSupported,
			Category: CategoryDecorators,
			Values:   []string{"true", "false"},
			Limitation: "When true enables legacy TypeScript decorators (__decorateClass); " +
				"absent or false enables TC39 standard decorators (__decorateElement)",
		},
		{
			Option:   "emitDecoratorMetadata",
			Status:   StatusUnsupported,
			Category: CategoryDecorators,
			Limitation: "Metadata emission requires type-checker information unavailable to a transpiler; " +
				"set to true produces ERR_M_TRANSFORM_UNSUPPORTED diagnostic",
		},

		// Source maps.
		{
			Option:     "sourceMap",
			Status:     StatusSupported,
			Category:   CategorySourceMaps,
			Limitation: "Upgrades no-map to external map; inlineSourceMap can further upgrade to inline",
		},
		{
			Option:     "inlineSourceMap",
			Status:     StatusSupported,
			Category:   CategorySourceMaps,
			Limitation: "Upgrades no-map to inline; does not override explicit external request",
		},
		{
			Option:     "inlineSources",
			Status:     StatusSupported,
			Category:   CategorySourceMaps,
			Values:     []string{"true", "false"},
			Limitation: "Tri-state *bool: nil/absent → include (tsc default), true → include, false → exclude",
		},
		{
			Option:     "sourceRoot",
			Status:     StatusSupported,
			Category:   CategorySourceMaps,
			Limitation: "Passed through to esbuild sourceRoot field on emitted source map",
		},
		{
			Option:     "mapRoot",
			Status:     StatusPartial,
			Category:   CategorySourceMaps,
			Limitation: "Cache-keyed; esbuild has no MapRoot option. Informational only, not applied to maps",
		},
	}
}

// CapabilityReport returns a deterministically sorted copy of the registry.
func CapabilityReport() []Entry {
	r := registry()
	sorted := make([]Entry, len(r))
	copy(sorted, r)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Category != sorted[j].Category {
			return sorted[i].Category < sorted[j].Category
		}
		return sorted[i].Option < sorted[j].Option
	})
	return sorted
}

// OptionSet returns compiler option keys that Mew recognizes.
// Keys not in this set are rejected during normalization.
func OptionSet() map[string]bool {
	r := registry()
	m := make(map[string]bool, len(r))
	for _, e := range r {
		m[e.Option] = true
	}
	return m
}

// UnsupportedSet returns option keys that are recognized but unsupported.
// Setting these to a non-default/true value must fail.
func UnsupportedSet() map[string]bool {
	r := registry()
	m := make(map[string]bool)
	for _, e := range r {
		if e.Status == StatusUnsupported {
			m[e.Option] = true
		}
	}
	return m
}
