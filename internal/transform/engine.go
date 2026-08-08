package transform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"
)

// Engine transforms TypeScript source in memory.
type Engine interface {
	Transform(ctx context.Context, req TransformRequest) (TransformResult, error)
	Identity() EngineIdentity
}

// esbuildEngine implements Engine using the esbuild Go API.
type esbuildEngine struct{}

// NewEsbuildEngine returns a new esbuild-backed transform engine.
func NewEsbuildEngine() Engine {
	return &esbuildEngine{}
}

func (e *esbuildEngine) Identity() EngineIdentity {
	return EngineIdentity{
		Name:    "esbuild",
		Version: "0.28.1",
	}
}

func (e *esbuildEngine) Transform(ctx context.Context, req TransformRequest) (TransformResult, error) {
	var zero TransformResult
	start := time.Now()

	if err := ctx.Err(); err != nil {
		return zero, err
	}

	loader := mapLoader(req.Loader)
	esbuildTarget := mapTarget(req.NormalizedOpts.Target, req.TargetNodeMajor)

	sourceMap := api.SourceMapNone
	switch req.SourceMapMode {
	case SourceMapInline:
		sourceMap = api.SourceMapInline
	case SourceMapExternal:
		sourceMap = api.SourceMapExternal
	}

	// If sourceMap mode is none but tsconfig has sourceMap:true, upgrade to external.
	if sourceMap == api.SourceMapNone && req.NormalizedOpts.SourceMap {
		sourceMap = api.SourceMapExternal
	}
	// tsconfig inlineSourceMap:true can override to inline.
	if req.NormalizedOpts.InlineSourceMap && sourceMap != api.SourceMapExternal {
		sourceMap = api.SourceMapInline
	}

	format := mapFormat(req.Format)
	// Respect tsconfig module setting for format when not explicit.
	if req.NormalizedOpts.Module != "" {
		switch strings.ToUpper(req.NormalizedOpts.Module) {
		case "COMMONJS":
			format = api.FormatCommonJS
		case "ES6", "ES2015", "ES2020", "ES2022", "ESNEXT", "NODENEXT", "NODE16":
			format = api.FormatESModule
		case "PRESERVE":
			// Keep whatever format was mapped from the request.
		}
	}

	jsxMode := api.JSXTransform // default
	jsxSet := false
	jsxDev := false
	if req.NormalizedOpts.JSX != "" {
		switch strings.ToLower(req.NormalizedOpts.JSX) {
		case "react":
			jsxMode = api.JSXTransform
			jsxSet = true
		case "react-jsx":
			jsxMode = api.JSXAutomatic
			jsxSet = true
		case "react-jsxdev":
			jsxMode = api.JSXAutomatic
			jsxSet = true
			jsxDev = true
		case "preserve":
			jsxMode = api.JSXPreserve
			jsxSet = true
		}
	}

	sourcesContent := api.SourcesContentInclude
	// inlineSources: nil (absent) or true → include source content (tsc default).
	// Explicit false → exclude source content.
	if req.NormalizedOpts.InlineSources != nil && !*req.NormalizedOpts.InlineSources {
		sourcesContent = api.SourcesContentExclude
	}

	transformOpts := api.TransformOptions{
		Loader:            loader,
		Target:            esbuildTarget,
		Format:            format,
		Sourcemap:         sourceMap,
		SourcesContent:    sourcesContent,
		Sourcefile:        req.SourcePath,
		Define:            nil,
		Pure:              nil,
		MinifyWhitespace:  false,
		MinifyIdentifiers: false,
		MinifySyntax:      false,
		TreeShaking:       api.TreeShakingFalse,
		Platform:          api.PlatformNode,
		Charset:           api.CharsetUTF8,
	}
	if jsxSet {
		transformOpts.JSX = jsxMode
	}
	if jsxDev {
		transformOpts.JSXDev = true
	}

	// JSX classic runtime: factory and fragment functions.
	if req.NormalizedOpts.JSXFactory != "" {
		transformOpts.JSXFactory = req.NormalizedOpts.JSXFactory
	}
	if req.NormalizedOpts.JSXFragmentFactory != "" {
		transformOpts.JSXFragment = req.NormalizedOpts.JSXFragmentFactory
	}

	// JSX automatic runtime: custom import source (default is "react").
	if req.NormalizedOpts.JSXImportSource != "" {
		transformOpts.JSXImportSource = req.NormalizedOpts.JSXImportSource
	}

	// Source root for external source maps.
	if req.NormalizedOpts.SourceRoot != "" {
		transformOpts.SourceRoot = req.NormalizedOpts.SourceRoot
	}

	// Pass compiler options through TsconfigRaw so esbuild can read
	// options we don't map to dedicated api.TransformOptions fields.
	// esbuild handles: experimentalDecorators, useDefineForClassFields,
	// verbatimModuleSyntax.
	var tsconfigOpts []string
	if req.NormalizedOpts.ExperimentalDecorators {
		tsconfigOpts = append(tsconfigOpts, `"experimentalDecorators":true`)
	}
	if req.NormalizedOpts.UseDefineForClassFields != nil {
		if *req.NormalizedOpts.UseDefineForClassFields {
			tsconfigOpts = append(tsconfigOpts, `"useDefineForClassFields":true`)
		} else {
			tsconfigOpts = append(tsconfigOpts, `"useDefineForClassFields":false`)
		}
	}
	if req.NormalizedOpts.VerbatimModuleSyntax != nil {
		if *req.NormalizedOpts.VerbatimModuleSyntax {
			tsconfigOpts = append(tsconfigOpts, `"verbatimModuleSyntax":true`)
		} else {
			tsconfigOpts = append(tsconfigOpts, `"verbatimModuleSyntax":false`)
		}
	}
	if len(tsconfigOpts) > 0 {
		transformOpts.TsconfigRaw = `{"compilerOptions":{` + strings.Join(tsconfigOpts, ",") + `}}`
	}

	result := api.Transform(string(req.SourceBytes), transformOpts)

	if len(result.Errors) > 0 {
		return zero, transformErrors(result.Errors)
	}

	code := result.Code
	sourceMapBytes := result.Map

	// Compute output digest
	h := sha256.New()
	h.Write(code)
	h.Write(sourceMapBytes)
	outputDigest := hex.EncodeToString(h.Sum(nil))

	diags := convertDiagnostics(result.Warnings, SeverityWarning)

	return TransformResult{
		Code:         code,
		SourceMap:    sourceMapBytes,
		OutputDigest: outputDigest,
		Diagnostics:  diags,
		CacheStatus:  CacheStatusBypass,
		Transformer:  e.Identity(),
		Elapsed:      time.Since(start),
	}, nil
}

func mapLoader(l LoaderKind) api.Loader {
	switch l {
	case LoaderTS:
		return api.LoaderTS
	case LoaderTSX:
		return api.LoaderTSX
	case LoaderMTS:
		return api.LoaderTS // esbuild handles MTS same as TS
	case LoaderCTS:
		return api.LoaderTS // esbuild handles CTS same as TS
	default:
		return api.LoaderTS
	}
}

func mapFormat(f ModuleFormat) api.Format {
	switch f {
	case FormatESM:
		return api.FormatESModule
	case FormatCJS:
		return api.FormatCommonJS
	default:
		return api.FormatESModule
	}
}

func mapTarget(tsconfigTarget string, nodeMajor int) api.Target {
	// Translate tsconfig target to esbuild target when set;
	// otherwise fall back to Node major version.
	if tsconfigTarget != "" {
		switch strings.ToUpper(tsconfigTarget) {
		case "ES3":
			return api.ES5 // esbuild doesn't support ES3; downgrade to ES5
		case "ES5":
			return api.ES5
		case "ES2015", "ES6":
			return api.ES2015
		case "ES2016":
			return api.ES2016
		case "ES2017":
			return api.ES2017
		case "ES2018":
			return api.ES2018
		case "ES2019":
			return api.ES2019
		case "ES2020":
			return api.ES2020
		case "ES2021":
			return api.ES2021
		case "ES2022":
			return api.ES2022
		case "ES2023":
			return api.ES2023
		case "ES2024":
			return api.ES2024
		case "ESNEXT":
			return api.ESNext
		default:
			// Unknown target: fall through to Node-based heuristic.
		}
	}
	// Default to Node major version-based target.
	switch {
	case nodeMajor >= 22:
		return api.ESNext
	case nodeMajor >= 20:
		return api.ES2023
	case nodeMajor >= 18:
		return api.ES2022
	default:
		return api.ES2020
	}
}

func transformErrors(errs []api.Message) error {
	var msgs []string
	for _, err := range errs {
		loc := ""
		if err.Location != nil {
			loc = fmt.Sprintf(" at %s:%d:%d", err.Location.File, err.Location.Line, err.Location.Column)
		}
		msgs = append(msgs, fmt.Sprintf("%s%s: %s", err.PluginName, loc, err.Text))
	}
	return fmt.Errorf("transform errors: %s", strings.Join(msgs, "; "))
}

func convertDiagnostics(msgs []api.Message, sev Severity) []Diagnostic {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]Diagnostic, len(msgs))
	for i, m := range msgs {
		d := Diagnostic{
			Severity: sev,
			Message:  m.Text,
		}
		if m.Location != nil {
			d.Source = m.Location.File
			d.Line = m.Location.Line
			d.Column = m.Location.Column
			d.Length = m.Location.Length
			d.Snippet = m.Location.LineText
		}
		out[i] = d
	}
	return out
}

// SystemInfo returns the host metadata for benchmarks and diagnostics.
func SystemInfo() map[string]string {
	return map[string]string{
		"goos":   runtime.GOOS,
		"goarch": runtime.GOARCH,
		"engine": "esbuild",
	}
}
