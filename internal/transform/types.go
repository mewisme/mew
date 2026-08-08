// Package transform hosts the Go transform service, engine, tsconfig, and cache.
package transform

import "time"

// LoaderKind maps to the module format to emit.
type LoaderKind string

const (
	LoaderTS  LoaderKind = "ts"
	LoaderTSX LoaderKind = "tsx"
	LoaderMTS LoaderKind = "mts"
	LoaderCTS LoaderKind = "cts"
)

// ModuleFormat is the target module format for emitted code.
type ModuleFormat string

const (
	FormatESM ModuleFormat = "esm"
	FormatCJS ModuleFormat = "cjs"
)

// SourceMapMode controls source map generation.
type SourceMapMode string

const (
	SourceMapNone     SourceMapMode = "none"
	SourceMapInline   SourceMapMode = "inline"
	SourceMapExternal SourceMapMode = "external"
)

// TransformRequest is the full input to a single file transform.
type TransformRequest struct {
	RequestID       string
	SourcePath      string
	SourceBytes     []byte
	SourceDigest    string
	Loader          LoaderKind
	Format          ModuleFormat
	NormalizedOpts  NormalizedOptions
	OptsDigest      string
	TargetNodeMajor int
	SourceMapMode   SourceMapMode
}

// TransformResult is the output of a successful transform.
type TransformResult struct {
	Code         []byte
	SourceMap    []byte // empty when SourceMapMode is none
	OutputDigest string
	Diagnostics  []Diagnostic
	CacheStatus  CacheStatus
	Transformer  EngineIdentity
	Elapsed      time.Duration
}

// CacheStatus reports whether the result came from cache.
type CacheStatus int

const (
	CacheStatusMiss   CacheStatus = 0
	CacheStatusHit    CacheStatus = 1
	CacheStatusBypass CacheStatus = 2
)

// Diagnostic is a structured transform diagnostic.
type Diagnostic struct {
	Severity Severity
	Code     string
	Message  string
	Source   string
	Line     int
	Column   int
	Length   int
	Snippet  string
}

// Severity classifies a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// EngineIdentity identifies a transformer.
type EngineIdentity struct {
	Name    string
	Version string
}
