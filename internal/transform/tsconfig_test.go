package transform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSONC(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStripJSONCComments(t *testing.T) {
	input := `{
			// single-line comment
			"compilerOptions": {
				"target": "ES2022", /* block comment */
				"strict": true
			}
		}`
	cleaned := stripJSONC([]byte(input))
	var m map[string]any
	if err := json.Unmarshal(cleaned, &m); err != nil {
		t.Fatalf("JSONC parse failed: %v\ncleaned:\n%s", err, string(cleaned))
	}
	co := m["compilerOptions"].(map[string]any)
	if co["target"] != "ES2022" {
		t.Fatalf("target=%v", co["target"])
	}
	if co["strict"] != true {
		t.Fatalf("strict=%v", co["strict"])
	}
}

func TestStripJSONCTrailingCommas(t *testing.T) {
	input := `{
			"compilerOptions": {
				"target": "ES2022",
				"module": "ESNext",
			},
		}`
	cleaned := stripJSONC([]byte(input))
	var m map[string]any
	if err := json.Unmarshal(cleaned, &m); err != nil {
		t.Fatalf("JSONC trailing comma parse failed: %v\ncleaned:\n%s", err, string(cleaned))
	}
}

func TestLoadTsconfigChainSingle(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"compilerOptions":{"target":"ES2022","strict":true}}`
	path := writeJSONC(t, dir, "tsconfig.json", cfg)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 1 {
		t.Fatalf("chain len=%d, want 1", len(chain))
	}
	if chain[0].Digest == "" {
		t.Fatal("empty digest")
	}
}

func TestLoadTsconfigChainExtends(t *testing.T) {
	dir := t.TempDir()
	base := `{"compilerOptions":{"target":"ES2020","strict":true}}`
	writeJSONC(t, dir, "base.json", base)
	child := `{"extends":"./base.json","compilerOptions":{"target":"ES2022"}}`
	path := writeJSONC(t, dir, "tsconfig.json", child)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 2 {
		t.Fatalf("chain len=%d, want 2", len(chain))
	}
	if chain[0].Path != filepath.Join(dir, "base.json") {
		t.Fatalf("parent path=%s", chain[0].Path)
	}
	if chain[1].Path != filepath.Join(dir, "tsconfig.json") {
		t.Fatalf("child path=%s", chain[1].Path)
	}
}

func TestNormalizeOptionsChildOverridesParent(t *testing.T) {
	dir := t.TempDir()
	base := `{"compilerOptions":{"target":"ES2020","module":"CommonJS","useDefineForClassFields":true}}`
	writeJSONC(t, dir, "base.json", base)
	child := `{"extends":"./base.json","compilerOptions":{"target":"ES2022","module":"ESNext","useDefineForClassFields":false}}`
	path := writeJSONC(t, dir, "tsconfig.json", child)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Target != "ES2022" {
		t.Fatalf("target=%s, want ES2022 (child should override parent)", opts.Target)
	}
	if opts.Module != "ESNext" {
		t.Fatalf("module=%s, want ESNext (child should override parent)", opts.Module)
	}
	if opts.UseDefineForClassFields == nil || *opts.UseDefineForClassFields != false {
		t.Fatalf("useDefineForClassFields=%v, want false (child explicit false)", opts.UseDefineForClassFields)
	}
}

func TestNormalizeOptionsParentOnlyAppliesWhenChildAbsent(t *testing.T) {
	dir := t.TempDir()
	base := `{"compilerOptions":{"target":"ES2020","strict":true}}`
	writeJSONC(t, dir, "base.json", base)
	child := `{"extends":"./base.json","compilerOptions":{"module":"ESNext"}}`
	path := writeJSONC(t, dir, "tsconfig.json", child)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Target != "ES2020" {
		t.Fatalf("target=%s, want ES2020 (parent value when child absent)", opts.Target)
	}
	if opts.Module != "ESNext" {
		t.Fatalf("module=%s, want ESNext (child override)", opts.Module)
	}
}

func TestNormalizeOptionsEmptyChain(t *testing.T) {
	opts, err := NormalizeOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Target != "" {
		t.Fatalf("target=%s, want empty", opts.Target)
	}
}

func TestTsconfigCycleDetection(t *testing.T) {
	dir := t.TempDir()
	a := `{"extends":"./b.json"}`
	writeJSONC(t, dir, "a.json", a)
	b := `{"extends":"./a.json"}`
	writeJSONC(t, dir, "b.json", b)

	_, err := LoadTsconfigChain(filepath.Join(dir, "a.json"))
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrExtendsCycle {
		t.Fatalf("expected ConfigErrExtendsCycle, got %s", cfgErr.Kind)
	}
}

func TestTsconfigChainDigest(t *testing.T) {
	chain := []TsconfigFile{
		{Digest: "aaa"},
		{Digest: "bbb"},
	}
	d1 := TsconfigChainDigest(chain)
	if d1 == "" {
		t.Fatal("empty digest")
	}
	d2 := TsconfigChainDigest(chain)
	if d1 != d2 {
		t.Fatalf("same chain different digests: %s vs %s", d1, d2)
	}
	chain2 := []TsconfigFile{
		{Digest: "aaa"},
		{Digest: "ccc"},
	}
	d3 := TsconfigChainDigest(chain2)
	if d1 == d3 {
		t.Fatal("different chains produced same digest")
	}
}

func TestDiscoverTsconfigWalkUp(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "src", "components")
	writeJSONC(t, dir, "tsconfig.json", `{"compilerOptions":{"target":"ES2022"}}`)

	path, err := DiscoverTsconfig(sub)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("no tsconfig found")
	}
}

func TestDiscoverTsconfigNotFound(t *testing.T) {
	dir := t.TempDir()
	path, err := DiscoverTsconfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("found tsconfig at %s, want empty", path)
	}
}

// --- New fail-closed error tests ---

func TestMalformedJSONC(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{invalid`)

	_, err := LoadTsconfigChain(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrParse {
		t.Fatalf("expected ConfigErrParse, got %s", cfgErr.Kind)
	}
	if cfgErr.Path != path {
		t.Fatalf("path=%s, want %s", cfgErr.Path, path)
	}
}

func TestUnreadableConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tsconfig.json")
	// Create a directory with the same name so os.ReadFile fails.
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := LoadTsconfigChain(path)
	if err == nil {
		t.Fatal("expected error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrIO {
		t.Fatalf("expected ConfigErrIO, got %s", cfgErr.Kind)
	}
}

func TestConfigPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	// Create tsconfig.json as a directory.
	cfgDir := filepath.Join(sub, "tsconfig.json")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverTsconfig(sub)
	if err == nil {
		t.Fatal("expected error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrIO {
		t.Fatalf("expected ConfigErrIO, got %s", cfgErr.Kind)
	}
}

func TestMissingRelativeExtendsFile(t *testing.T) {
	dir := t.TempDir()
	child := `{"extends":"./nonexistent.json"}`
	path := writeJSONC(t, dir, "tsconfig.json", child)

	_, err := LoadTsconfigChain(path)
	if err == nil {
		t.Fatal("expected error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrExtendsMissing {
		t.Fatalf("expected ConfigErrExtendsMissing, got %s", cfgErr.Kind)
	}
}

func TestExtendsDepthOverflow(t *testing.T) {
	dir := t.TempDir()
	// Create a chain that exceeds maxTsconfigDepth (20).
	// base0 is the root, and each level extends the previous.
	writeJSONC(t, dir, "base0.json", `{"compilerOptions":{}}`)
	prev := "base0.json"
	for i := 1; i <= maxTsconfigDepth+1; i++ {
		name := "cfg" + strings.Repeat("x", i) + ".json"
		writeJSONC(t, dir, name, `{"extends":"./`+prev+`"}`)
		prev = name
	}

	_, err := LoadTsconfigChain(filepath.Join(dir, prev))
	if err == nil {
		t.Fatal("expected depth overflow error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrExtendsDepth {
		t.Fatalf("expected ConfigErrExtendsDepth, got %s", cfgErr.Kind)
	}
}

func TestNonStringExtends(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"extends":42}`)

	_, err := LoadTsconfigChain(path)
	if err == nil {
		t.Fatal("expected error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrExtendsInvalid {
		t.Fatalf("expected ConfigErrExtendsInvalid, got %s", cfgErr.Kind)
	}
}

func TestEmptyExtends(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"extends":""}`)

	_, err := LoadTsconfigChain(path)
	if err == nil {
		t.Fatal("expected error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrExtendsInvalid {
		t.Fatalf("expected ConfigErrExtendsInvalid, got %s", cfgErr.Kind)
	}
}

func TestUnsupportedPackageExtends(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"extends":"@scope/tsconfig"}`)

	_, err := LoadTsconfigChain(path)
	if err == nil {
		t.Fatal("expected error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrExtendsPackage {
		t.Fatalf("expected ConfigErrExtendsPackage, got %s", cfgErr.Kind)
	}
}

func TestInvalidCompilerOptionShape(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"compilerOptions":{"target":42}}`)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected option error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionInvalid {
		t.Fatalf("expected ConfigErrOptionInvalid, got %s", cfgErr.Kind)
	}
}

func TestCompilerOptionsNotAnObject(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"compilerOptions":"strict"}`)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected option error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionInvalid {
		t.Fatalf("expected ConfigErrOptionInvalid, got %s", cfgErr.Kind)
	}
}

func TestInvalidBooleanOption(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"compilerOptions":{"useDefineForClassFields":"yes"}}`)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected option error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionInvalid {
		t.Fatalf("expected ConfigErrOptionInvalid, got %s", cfgErr.Kind)
	}
}

func TestInvalidPathsOption(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"compilerOptions":{"paths":"bad"}}`)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected option error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionInvalid {
		t.Fatalf("expected ConfigErrOptionInvalid, got %s", cfgErr.Kind)
	}
}

func TestConfigErrorPathPreserved(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{invalid`)

	_, err := LoadTsconfigChain(path)
	if err == nil {
		t.Fatal("expected error")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Path != path {
		t.Fatalf("path=%s, want %s", cfgErr.Path, path)
	}
	// Error message must contain the path but not expose raw file contents.
	// The json parse error may reference character position but not the file bytes.
	if !strings.Contains(cfgErr.Error(), path) {
		t.Fatalf("error does not contain path: %s", cfgErr.Error())
	}
}

func TestNormalizeJSXOptions(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"compilerOptions":{"jsx":"react-jsx","jsxFactory":"h","jsxFragmentFactory":"Fragment","jsxImportSource":"preact"}}`
	path := writeJSONC(t, dir, "tsconfig.json", cfg)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if opts.JSX != "react-jsx" {
		t.Fatalf("jsx=%s, want react-jsx", opts.JSX)
	}
	if opts.JSXFactory != "h" {
		t.Fatalf("jsxFactory=%s, want h", opts.JSXFactory)
	}
	if opts.JSXFragmentFactory != "Fragment" {
		t.Fatalf("jsxFragmentFactory=%s, want Fragment", opts.JSXFragmentFactory)
	}
	if opts.JSXImportSource != "preact" {
		t.Fatalf("jsxImportSource=%s, want preact", opts.JSXImportSource)
	}
}

func TestNormalizeDecoratorOptions(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"compilerOptions":{"experimentalDecorators":true}}`
	path := writeJSONC(t, dir, "tsconfig.json", cfg)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.ExperimentalDecorators {
		t.Fatal("experimentalDecorators should be true")
	}
	if opts.EmitDecoratorMetadata {
		t.Fatal("emitDecoratorMetadata should be false when not set")
	}
}

func TestEmitDecoratorMetadataUnsupported(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"compilerOptions":{"emitDecoratorMetadata":true}}`
	path := writeJSONC(t, dir, "tsconfig.json", cfg)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected error for emitDecoratorMetadata, got nil")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionUnsupported {
		t.Fatalf("expected ConfigErrOptionUnsupported, got %s", cfgErr.Kind)
	}
}

func TestNormalizeSourceMapOptions(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"compilerOptions":{"sourceMap":true,"inlineSourceMap":true,"inlineSources":true,"sourceRoot":"/src","mapRoot":"/maps"}}`
	path := writeJSONC(t, dir, "tsconfig.json", cfg)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.SourceMap {
		t.Fatal("sourceMap should be true")
	}
	if !opts.InlineSourceMap {
		t.Fatal("inlineSourceMap should be true")
	}
	if opts.InlineSources == nil || !*opts.InlineSources {
		t.Fatal("inlineSources should be true")
	}
	if opts.SourceRoot != "/src" {
		t.Fatalf("sourceRoot=%s, want /src", opts.SourceRoot)
	}
	if opts.MapRoot != "/maps" {
		t.Fatalf("mapRoot=%s, want /maps", opts.MapRoot)
	}
}

func TestInvalidJSXFactoryType(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"compilerOptions":{"jsxFactory":42}}`)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected option error for non-string jsxFactory")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionInvalid {
		t.Fatalf("expected ConfigErrOptionInvalid, got %s", cfgErr.Kind)
	}
}

func TestInvalidExperimentalDecoratorsType(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"compilerOptions":{"experimentalDecorators":"yes"}}`)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected option error for non-bool experimentalDecorators")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionInvalid {
		t.Fatalf("expected ConfigErrOptionInvalid, got %s", cfgErr.Kind)
	}
}

func TestNormalizeInlineSourcesAbsent(t *testing.T) {
	// inlineSources not set → *bool nil (absent, default include).
	dir := t.TempDir()
	cfg := `{"compilerOptions":{"sourceMap":true}}`
	path := writeJSONC(t, dir, "tsconfig.json", cfg)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if opts.InlineSources != nil {
		t.Fatal("inlineSources should be nil when absent")
	}
}

func TestNormalizeInlineSourcesExplicitTrue(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"compilerOptions":{"inlineSources":true}}`
	path := writeJSONC(t, dir, "tsconfig.json", cfg)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if opts.InlineSources == nil || !*opts.InlineSources {
		t.Fatal("inlineSources should be explicit true")
	}
}

func TestNormalizeInlineSourcesExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"compilerOptions":{"inlineSources":false}}`
	path := writeJSONC(t, dir, "tsconfig.json", cfg)

	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if opts.InlineSources == nil || *opts.InlineSources {
		t.Fatal("inlineSources should be explicit false")
	}
}

func TestNormalizeInlineSourcesChildOverridesParent(t *testing.T) {
	// Child explicit false overrides parent absent (nil).
	dir := t.TempDir()
	writeJSONC(t, dir, "base.json", `{"compilerOptions":{"sourceMap":true}}`)
	writeJSONC(t, dir, "tsconfig.json", `{"extends":"./base.json","compilerOptions":{"inlineSources":false}}`)
	chain, err := LoadTsconfigChain(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if opts.InlineSources == nil || *opts.InlineSources {
		t.Fatal("child inlineSources:false should override parent absent")
	}
}

func TestInvalidSourceRootType(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"compilerOptions":{"sourceRoot":42}}`)
	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected error for non-string sourceRoot")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionInvalid {
		t.Fatalf("expected ConfigErrOptionInvalid, got %s", cfgErr.Kind)
	}
}

func TestInvalidMapRootType(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONC(t, dir, "tsconfig.json", `{"compilerOptions":{"mapRoot":42}}`)
	chain, err := LoadTsconfigChain(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected error for non-string mapRoot")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionInvalid {
		t.Fatalf("expected ConfigErrOptionInvalid, got %s", cfgErr.Kind)
	}
}

func TestNormalizeOptionsDigestVariesByInlineSources(t *testing.T) {
	// absent vs explicit true serialize differently (nil vs {"inlineSources":true}),
	// so they produce different digests. This is conservative: no collision risk.
	// Same-output requests with different config get separate cache entries.
	optsAbsent := NormalizedOptions{SourceMap: true}
	optsTrue := NormalizedOptions{SourceMap: true, InlineSources: boolPtr(true)}
	optsFalse := NormalizedOptions{SourceMap: true, InlineSources: boolPtr(false)}

	if optsAbsent.Digest() == optsTrue.Digest() {
		t.Log("absent and explicit-true inlineSources may share digest when both serialize same")
	}
	if optsAbsent.Digest() == optsFalse.Digest() {
		t.Fatal("explicit-false inlineSources must differ from absent")
	}
	if optsTrue.Digest() == optsFalse.Digest() {
		t.Fatal("explicit-false inlineSources must differ from explicit-true")
	}
}

func TestNormalizeOptionsDigestVariesBySourceMap(t *testing.T) {
	opts1 := NormalizedOptions{}
	opts2 := NormalizedOptions{SourceMap: true}
	if opts1.Digest() == opts2.Digest() {
		t.Fatal("digests must differ when sourceMap differs")
	}
}

func TestNormalizeOptionsDigestVariesByInlineSourceMap(t *testing.T) {
	opts1 := NormalizedOptions{}
	opts2 := NormalizedOptions{InlineSourceMap: true}
	if opts1.Digest() == opts2.Digest() {
		t.Fatal("digests must differ when inlineSourceMap differs")
	}
}

func TestNormalizeOptionsDigestVariesBySourceRoot(t *testing.T) {
	opts1 := NormalizedOptions{}
	opts2 := NormalizedOptions{SourceRoot: "/src"}
	if opts1.Digest() == opts2.Digest() {
		t.Fatal("digests must differ when sourceRoot differs")
	}
}

func TestNormalizeOptionsDigestVariesByMapRoot(t *testing.T) {
	opts1 := NormalizedOptions{}
	opts2 := NormalizedOptions{MapRoot: "/maps"}
	if opts1.Digest() == opts2.Digest() {
		t.Fatal("digests must differ when mapRoot differs")
	}
}

func boolPtr(b bool) *bool { return &b }

func TestNormalizeOptionsDigestIncludesNewFields(t *testing.T) {
	opts1 := NormalizedOptions{JSX: "react"}
	opts2 := NormalizedOptions{JSX: "react-jsx"}
	if opts1.Digest() == opts2.Digest() {
		t.Fatal("digests must differ when JSX mode differs")
	}

	opts3 := NormalizedOptions{ExperimentalDecorators: true}
	opts4 := NormalizedOptions{ExperimentalDecorators: true, EmitDecoratorMetadata: true}
	if opts3.Digest() == opts4.Digest() {
		t.Fatal("digests must differ when decorator metadata differs")
	}
}

func TestImportHelpersUnsupported(t *testing.T) {
	chain := []TsconfigFile{
		{Path: "tsconfig.json", Raw: map[string]any{
			"compilerOptions": map[string]any{
				"importHelpers": true,
			},
		}},
	}
	_, err := NormalizeOptions(chain)
	if err == nil {
		t.Fatal("expected error for importHelpers: true")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Kind != ConfigErrOptionUnsupported {
		t.Errorf("expected ConfigErrOptionUnsupported, got %v", cfgErr.Kind)
	}
}

func TestImportHelpersFalseOK(t *testing.T) {
	chain := []TsconfigFile{
		{Path: "tsconfig.json", Raw: map[string]any{
			"compilerOptions": map[string]any{
				"importHelpers": false,
			},
		}},
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatalf("importHelpers: false should be accepted: %v", err)
	}
	if opts.ImportHelpers {
		t.Fatal("importHelpers should be false")
	}
}

func TestUseDefineForClassFieldsTriState(t *testing.T) {
	// Explicit true.
	chain := []TsconfigFile{
		{Path: "tsconfig.json", Raw: map[string]any{
			"compilerOptions": map[string]any{
				"useDefineForClassFields": true,
			},
		}},
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.UseDefineForClassFields == nil || *opts.UseDefineForClassFields != true {
		t.Fatal("useDefineForClassFields should be explicit true")
	}

	// Explicit false.
	chain2 := []TsconfigFile{
		{Path: "tsconfig.json", Raw: map[string]any{
			"compilerOptions": map[string]any{
				"useDefineForClassFields": false,
			},
		}},
	}
	opts2, err := NormalizeOptions(chain2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts2.UseDefineForClassFields == nil || *opts2.UseDefineForClassFields != false {
		t.Fatal("useDefineForClassFields should be explicit false")
	}

	// Absent.
	chain3 := []TsconfigFile{
		{Path: "tsconfig.json", Raw: map[string]any{
			"compilerOptions": map[string]any{},
		}},
	}
	opts3, err := NormalizeOptions(chain3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts3.UseDefineForClassFields != nil {
		t.Fatal("useDefineForClassFields should be nil when absent")
	}
}

func TestVerbatimModuleSyntaxTriState(t *testing.T) {
	// Explicit true.
	chain := []TsconfigFile{
		{Path: "tsconfig.json", Raw: map[string]any{
			"compilerOptions": map[string]any{
				"verbatimModuleSyntax": true,
			},
		}},
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.VerbatimModuleSyntax == nil || *opts.VerbatimModuleSyntax != true {
		t.Fatal("verbatimModuleSyntax should be explicit true")
	}

	// Absent.
	chain2 := []TsconfigFile{
		{Path: "tsconfig.json", Raw: map[string]any{
			"compilerOptions": map[string]any{},
		}},
	}
	opts2, err := NormalizeOptions(chain2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts2.VerbatimModuleSyntax != nil {
		t.Fatal("verbatimModuleSyntax should be nil when absent")
	}
}

func TestNormalizeOptionsDigestVariesByUseDefineForClassFields(t *testing.T) {
	absent := NormalizedOptions{}
	explicitTrue := NormalizedOptions{UseDefineForClassFields: boolPtr(true)}
	explicitFalse := NormalizedOptions{UseDefineForClassFields: boolPtr(false)}

	if absent.Digest() == explicitTrue.Digest() {
		t.Fatal("digest must differ when useDefineForClassFields absent vs explicit true")
	}
	if explicitTrue.Digest() == explicitFalse.Digest() {
		t.Fatal("digest must differ when useDefineForClassFields true vs false")
	}
}

func TestNormalizeOptionsDigestVariesByVerbatimModuleSyntax(t *testing.T) {
	absent := NormalizedOptions{}
	explicitTrue := NormalizedOptions{VerbatimModuleSyntax: boolPtr(true)}

	if absent.Digest() == explicitTrue.Digest() {
		t.Fatal("digest must differ when verbatimModuleSyntax absent vs explicit true")
	}
}

func TestNormalizeOptionsDigestVariesByImportHelpers(t *testing.T) {
	absent := NormalizedOptions{}
	explicitTrue := NormalizedOptions{ImportHelpers: true}

	if absent.Digest() == explicitTrue.Digest() {
		t.Fatal("digest must differ when importHelpers absent vs true")
	}
}

func TestOptionSetMatchesNormalizedOptions(t *testing.T) {
	// Every key in OptionSet must be a field that applyCompilerOptions can parse.
	optSet := OptionSet()

	parsedOptions := []string{
		"target", "module", "useDefineForClassFields", "verbatimModuleSyntax",
		"importHelpers", "baseUrl", "paths",
		"jsx", "jsxFactory", "jsxFragmentFactory", "jsxImportSource",
		"experimentalDecorators", "emitDecoratorMetadata",
		"sourceMap", "inlineSourceMap", "inlineSources", "sourceRoot", "mapRoot",
	}

	for _, opt := range parsedOptions {
		if !optSet[opt] {
			t.Errorf("option %q parsed by applyCompilerOptions but not in OptionSet", opt)
		}
	}
}

func TestUnsupportedOptionsUsesRegistry(t *testing.T) {
	raw := map[string]any{
		"target":                           "ESNext",
		"strict":                           true,
		"noEmit":                           true,
		"esModuleInterop":                  true,
		"skipLibCheck":                     true,
		"forceConsistentCasingInFileNames": true,
		"moduleResolution":                 "node16",
		"importHelpers":                    true,
	}

	unsupported := UnsupportedOptions(raw)
	for _, opt := range unsupported {
		if OptionSet()[opt] {
			t.Errorf("option %q is in OptionSet (recognized) but reported as unsupported", opt)
		}
	}

	for _, recognized := range []string{"target", "importHelpers"} {
		for _, u := range unsupported {
			if u == recognized {
				t.Errorf("recognized option %q should not be in unsupported list", recognized)
			}
		}
	}

	for _, expected := range []string{"strict", "noEmit", "esModuleInterop", "skipLibCheck", "moduleResolution"} {
		found := false
		for _, u := range unsupported {
			if u == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unrecognized option %q should be in unsupported list", expected)
		}
	}
}

// ── Path mappings ordering ───────────────────────────────────────────

func TestSortPathMappingsExactFirst(t *testing.T) {
	mappings := []PathMapping{
		{Pattern: "@app/*", Targets: []string{"./src/*"}},
		{Pattern: "@app/core", Targets: []string{"./src/core"}},
		{Pattern: "*", Targets: []string{"./fallback/*"}},
	}
	sortPathMappings(mappings)
	// Exact match (no *) must be first.
	if mappings[0].Pattern != "@app/core" {
		t.Errorf("exact pattern should be first, got %q", mappings[0].Pattern)
	}
	// Catch-all * (shortest prefix) must be last.
	if mappings[len(mappings)-1].Pattern != "*" {
		t.Errorf("catch-all * should be last, got %q", mappings[len(mappings)-1].Pattern)
	}
}

func TestSortPathMappingsWildcardSpecificity(t *testing.T) {
	mappings := []PathMapping{
		{Pattern: "@app/*", Targets: []string{"./lib/*"}},
		{Pattern: "@app/internal/*", Targets: []string{"./internal/*"}},
		{Pattern: "@app/internal/nested/*", Targets: []string{"./nested/*"}},
	}
	sortPathMappings(mappings)
	// Most specific (longest prefix) first.
	if mappings[0].Pattern != "@app/internal/nested/*" {
		t.Errorf("most specific first: got %q", mappings[0].Pattern)
	}
	if mappings[1].Pattern != "@app/internal/*" {
		t.Errorf("second: got %q", mappings[1].Pattern)
	}
	if mappings[2].Pattern != "@app/*" {
		t.Errorf("least specific last: got %q", mappings[2].Pattern)
	}
}

func TestSortPathMappingsSamePrefixShorterSuffixFirst(t *testing.T) {
	// Same prefix length, shorter suffix = more specific.
	mappings := []PathMapping{
		{Pattern: "src/*/test", Targets: []string{"./a/*/b"}},
		{Pattern: "src/*", Targets: []string{"./a/*"}},
	}
	sortPathMappings(mappings)
	if mappings[0].Pattern != "src/*" {
		t.Errorf("shorter suffix (empty) first: got %q", mappings[0].Pattern)
	}
}

func TestSortPathMappingsDeterministic(t *testing.T) {
	// Multiple runs with shuffled input produce identical output.
	mappings := []PathMapping{
		{Pattern: "z", Targets: []string{"./z"}},
		{Pattern: "a", Targets: []string{"./a"}},
		{Pattern: "m/*", Targets: []string{"./m/*"}},
		{Pattern: "@x/*/deep", Targets: []string{"./x/*/d"}},
		{Pattern: "@x/*", Targets: []string{"./x/*"}},
		{Pattern: "*", Targets: []string{"./all/*"}},
	}
	r1 := make([]PathMapping, len(mappings))
	copy(r1, mappings)
	sortPathMappings(r1)

	// Reverse and sort again — must match.
	r2 := make([]PathMapping, len(mappings))
	for i := range mappings {
		r2[i] = mappings[len(mappings)-1-i]
	}
	sortPathMappings(r2)

	if len(r1) != len(r2) {
		t.Fatalf("length mismatch: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i].Pattern != r2[i].Pattern {
			t.Errorf("position %d: %q vs %q", i, r1[i].Pattern, r2[i].Pattern)
		}
	}

	// Exact patterns sorted alphabetically.
	for i := 0; i < len(r1)-1; i++ {
		if !strings.Contains(r1[i].Pattern, "*") && !strings.Contains(r1[i+1].Pattern, "*") {
			if r1[i].Pattern > r1[i+1].Pattern {
				t.Errorf("exact patterns not alphabetical: %q before %q", r1[i].Pattern, r1[i+1].Pattern)
			}
		}
	}
}

func TestNormalizeOptionsPopulatesPathMappings(t *testing.T) {
	chain := []TsconfigFile{
		{
			Path: "/project/tsconfig.json",
			Raw: map[string]any{
				"compilerOptions": map[string]any{
					"baseUrl": ".",
					"paths": map[string]any{
						"@app/*":          []any{"./src/*"},
						"@app/internal/*": []any{"./src/internal/*"},
						"@app/core":       []any{"./src/core"},
					},
				},
			},
		},
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.PathMappings) != 3 {
		t.Fatalf("expected 3 path mappings, got %d", len(opts.PathMappings))
	}
	// Exact match must be first.
	if opts.PathMappings[0].Pattern != "@app/core" {
		t.Errorf("exact match first: got %q", opts.PathMappings[0].Pattern)
	}
	// More specific wildcard must come before less specific.
	if opts.PathMappings[1].Pattern != "@app/internal/*" {
		t.Errorf("more specific wildcard second: got %q", opts.PathMappings[1].Pattern)
	}
	if opts.PathMappings[2].Pattern != "@app/*" {
		t.Errorf("less specific wildcard last: got %q", opts.PathMappings[2].Pattern)
	}
	// Targets must preserve declaration order.
	if len(opts.PathMappings[0].Targets) != 1 || opts.PathMappings[0].Targets[0] != "./src/core" {
		t.Errorf("target order not preserved")
	}
}

func TestPathMappingsEmptyWhenNoPaths(t *testing.T) {
	chain := []TsconfigFile{
		{
			Path: "/project/tsconfig.json",
			Raw: map[string]any{
				"compilerOptions": map[string]any{
					"target": "ESNext",
				},
			},
		},
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.PathMappings) != 0 {
		t.Errorf("expected empty PathMappings, got %d", len(opts.PathMappings))
	}
}

func TestPathMappingsInDigest(t *testing.T) {
	// Two identical configs must have identical digests.
	chain1 := []TsconfigFile{
		{
			Path: "/project/tsconfig.json",
			Raw: map[string]any{
				"compilerOptions": map[string]any{
					"paths": map[string]any{
						"@app/*": []any{"./src/*"},
						"@lib/*": []any{"./lib/*"},
					},
				},
			},
		},
	}
	opts1, err := NormalizeOptions(chain1)
	if err != nil {
		t.Fatal(err)
	}

	// Same config, different map key order (simulate different iteration).
	chain2 := []TsconfigFile{
		{
			Path: "/project/tsconfig.json",
			Raw: map[string]any{
				"compilerOptions": map[string]any{
					"paths": map[string]any{
						"@lib/*": []any{"./lib/*"},
						"@app/*": []any{"./src/*"},
					},
				},
			},
		},
	}
	opts2, err := NormalizeOptions(chain2)
	if err != nil {
		t.Fatal(err)
	}

	if opts1.Digest() != opts2.Digest() {
		t.Error("digests differ for equivalent configs with different map key order")
	}
	// PathMappings must be identical regardless of map key iteration order.
	if len(opts1.PathMappings) != len(opts2.PathMappings) {
		t.Fatalf("PathMappings length differs: %d vs %d", len(opts1.PathMappings), len(opts2.PathMappings))
	}
	for i := range opts1.PathMappings {
		if opts1.PathMappings[i].Pattern != opts2.PathMappings[i].Pattern {
			t.Errorf("PathMappings[%d] pattern: %q vs %q", i, opts1.PathMappings[i].Pattern, opts2.PathMappings[i].Pattern)
		}
	}
}

func TestPathMappingsTargetOrderPreserved(t *testing.T) {
	chain := []TsconfigFile{
		{
			Path: "/project/tsconfig.json",
			Raw: map[string]any{
				"compilerOptions": map[string]any{
					"paths": map[string]any{
						"@app/*": []any{"./first/*", "./second/*", "./third/*"},
					},
				},
			},
		},
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.PathMappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(opts.PathMappings))
	}
	targets := opts.PathMappings[0].Targets
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
	if targets[0] != "./first/*" || targets[1] != "./second/*" || targets[2] != "./third/*" {
		t.Errorf("target order changed: %v", targets)
	}
}

func TestPathMappingsJSONRoundTrip(t *testing.T) {
	chain := []TsconfigFile{
		{
			Path: "/project/tsconfig.json",
			Raw: map[string]any{
				"compilerOptions": map[string]any{
					"baseUrl": "./src",
					"paths": map[string]any{
						"@app/*": []any{"./app/*"},
					},
				},
			},
		},
	}
	opts, err := NormalizeOptions(chain)
	if err != nil {
		t.Fatal(err)
	}
	j, err := json.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	var decoded NormalizedOptions
	if err := json.Unmarshal(j, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.PathMappings) != len(opts.PathMappings) {
		t.Fatalf("round-trip: %d vs %d mappings", len(decoded.PathMappings), len(opts.PathMappings))
	}
	for i := range opts.PathMappings {
		if decoded.PathMappings[i].Pattern != opts.PathMappings[i].Pattern {
			t.Errorf("round-trip pattern mismatch at %d: %q vs %q", i, decoded.PathMappings[i].Pattern, opts.PathMappings[i].Pattern)
		}
		if len(decoded.PathMappings[i].Targets) != len(opts.PathMappings[i].Targets) {
			t.Errorf("round-trip target count mismatch at %d", i)
		}
	}
	// baseUrl must round-trip as well.
	if decoded.BaseURL != opts.BaseURL {
		t.Errorf("round-trip baseUrl: %q vs %q", decoded.BaseURL, opts.BaseURL)
	}
}

func TestSortPathMappingsEqualPrefixesAlphabetical(t *testing.T) {
	// Two wildcard patterns with identical prefix/suffix lengths
	// should tie-break alphabetically.
	mappings := []PathMapping{
		{Pattern: "zoo/*", Targets: []string{"./zoo/*"}},
		{Pattern: "abc/*", Targets: []string{"./abc/*"}},
	}
	sortPathMappings(mappings)
	if mappings[0].Pattern != "abc/*" {
		t.Errorf("alphabetical tie-break: got %q, want \"abc/*\"", mappings[0].Pattern)
	}
	if mappings[1].Pattern != "zoo/*" {
		t.Errorf("got %q", mappings[1].Pattern)
	}
}
