package transform_test

import (
	"context"
	"testing"

	"github.com/mewisme/mew/internal/transform"
)

func BenchmarkEngineTransform(b *testing.B) {
	engine := transform.NewEsbuildEngine()
	src := `const x: string = "hello"; console.log(x);`
	req := transform.TransformRequest{
		SourcePath:    "test.ts",
		SourceBytes:   []byte(src),
		Loader:        transform.LoaderTS,
		Format:        transform.FormatESM,
		SourceMapMode: transform.SourceMapNone,
	}
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_, err := engine.Transform(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCacheWriteRead(b *testing.B) {
	engine := transform.NewEsbuildEngine()
	identity := engine.Identity()
	src := `const x: string = "bench"; console.log(x);`
	req := transform.TransformRequest{
		SourcePath:    "bench.ts",
		SourceBytes:   []byte(src),
		Loader:        transform.LoaderTS,
		Format:        transform.FormatESM,
		SourceMapMode: transform.SourceMapNone,
	}
	ctx := context.Background()
	result, err := engine.Transform(ctx, req)
	if err != nil {
		b.Fatal(err)
	}
	key := transform.CacheKey(req, identity)
	dir := b.TempDir()

	b.ResetTimer()
	for b.Loop() {
		if err := transform.WriteCache(dir, key, &result); err != nil {
			b.Fatal(err)
		}
		_, err := transform.TryReadCache(dir, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCacheKeyDeterminism(b *testing.B) {
	engine := transform.NewEsbuildEngine()
	identity := engine.Identity()
	req := transform.TransformRequest{
		SourcePath:    "det.ts",
		SourceBytes:   []byte(`const a: number = 1;`),
		Loader:        transform.LoaderTS,
		Format:        transform.FormatESM,
		SourceMapMode: transform.SourceMapNone,
	}
	for b.Loop() {
		_ = transform.CacheKey(req, identity)
	}
}

func BenchmarkEngineTransformJSX(b *testing.B) {
	engine := transform.NewEsbuildEngine()
	src := `const el = <div className="app"><h1>Hello</h1><p>World</p></div>;`
	req := transform.TransformRequest{
		SourcePath:     "component.tsx",
		SourceBytes:    []byte(src),
		Loader:         transform.LoaderTSX,
		Format:         transform.FormatESM,
		SourceMapMode:  transform.SourceMapNone,
		NormalizedOpts: transform.NormalizedOptions{JSX: "react-jsx"},
	}
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_, err := engine.Transform(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngineTransformDecorator(b *testing.B) {
	engine := transform.NewEsbuildEngine()
	src := "function sealed(target: any) {}\n@sealed\nclass MyClass {\n  method() { return 1; }\n}"
	req := transform.TransformRequest{
		SourcePath:     "decorator.ts",
		SourceBytes:    []byte(src),
		Loader:         transform.LoaderTS,
		Format:         transform.FormatESM,
		SourceMapMode:  transform.SourceMapNone,
		NormalizedOpts: transform.NormalizedOptions{ExperimentalDecorators: true},
	}
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_, err := engine.Transform(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngineTransformSourceMap(b *testing.B) {
	engine := transform.NewEsbuildEngine()
	src := "const x: number = 1;\nexport default x;\n"
	req := transform.TransformRequest{
		SourcePath:    "app.ts",
		SourceBytes:   []byte(src),
		Loader:        transform.LoaderTS,
		Format:        transform.FormatESM,
		SourceMapMode: transform.SourceMapExternal,
	}
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_, err := engine.Transform(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}
