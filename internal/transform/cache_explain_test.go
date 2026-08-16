package transform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheExplainEmptyDir(t *testing.T) {
	dir := t.TempDir()
	result, err := CacheExplain(dir, CacheExplainOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.EntryCount != 0 {
		t.Errorf("expected 0 entries, got %d", result.EntryCount)
	}
	if result.TotalBytes != 0 {
		t.Errorf("expected 0 bytes, got %d", result.TotalBytes)
	}
}

func TestCacheExplainNonExistentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	result, err := CacheExplain(dir, CacheExplainOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.EntryCount != 0 {
		t.Errorf("expected 0 entries for nonexistent dir, got %d", result.EntryCount)
	}
}

func TestCacheExplainHit(t *testing.T) {
	dir := t.TempDir()
	engine := NewEsbuildEngine()
	req := TransformRequest{
		SourcePath:  "test.ts",
		SourceBytes: []byte("const x: number = 1;\n"),
		Loader:      LoaderTS,
		Format:      FormatESM,
		NormalizedOpts: NormalizedOptions{
			Target: "es2022",
		},
		SourceMapMode: SourceMapExternal,
	}
	key := CacheKey(req, engine.Identity())

	// Write a real cache entry through the production path.
	ref := engine.Identity()
	_ = ref
	result, err := engine.Transform(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCache(dir, key, &result); err != nil {
		t.Fatal(err)
	}

	// Explain the entry.
	explain, err := CacheExplain(dir, CacheExplainOptions{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	if explain.Entry == nil {
		t.Fatal("expected entry explanation")
	}
	if explain.Entry.Disposition != CacheDispositionHit {
		t.Errorf("expected hit, got %s", explain.Entry.Disposition)
	}
	if len(explain.Entry.Reasons) == 0 {
		t.Error("expected at least one reason")
	}
	found := false
	for _, r := range explain.Entry.Reasons {
		if r.Code == ReasonHit {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a hit reason")
	}
}

func TestCacheExplainMiss(t *testing.T) {
	dir := t.TempDir()
	explain, err := CacheExplain(dir, CacheExplainOptions{
		Key: "nonexistent-key-0000000000000000000000000000000000000000000000000000000000000000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if explain.Entry == nil {
		t.Fatal("expected entry explanation even for miss")
	}
	if explain.Entry.Disposition != CacheDispositionMiss {
		t.Errorf("expected miss, got %s", explain.Entry.Disposition)
	}
}

func TestCacheExplainSchemaMismatch(t *testing.T) {
	dir := t.TempDir()
	engine := NewEsbuildEngine()
	req := TransformRequest{
		SourcePath:  "test.ts",
		SourceBytes: []byte("const x: number = 1;\n"),
		Loader:      LoaderTS,
		Format:      FormatESM,
		NormalizedOpts: NormalizedOptions{
			Target: "es2022",
		},
		SourceMapMode: SourceMapNone,
	}
	key := CacheKey(req, engine.Identity())

	// Write through production, then corrupt the schema version.
	result, err := engine.Transform(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCache(dir, key, &result); err != nil {
		t.Fatal(err)
	}

	// Read the metadata, bump the schema version to a future version.
	metaPath := CacheKeyPath(dir, key) + ".meta"
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	var entry cacheEntry
	if err := json.Unmarshal(metaData, &entry); err != nil {
		t.Fatal(err)
	}
	entry.SchemaVersion = 999
	corrupted, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, corrupted, 0o644); err != nil {
		t.Fatal(err)
	}

	// Explain should detect schema mismatch.
	explain, err := CacheExplain(dir, CacheExplainOptions{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	if explain.Entry.Disposition != CacheDispositionSchemaStale {
		t.Errorf("expected schema-stale, got %s", explain.Entry.Disposition)
	}
}

func TestCacheExplainCorruptDigest(t *testing.T) {
	dir := t.TempDir()
	engine := NewEsbuildEngine()
	req := TransformRequest{
		SourcePath:  "test.ts",
		SourceBytes: []byte("const x: number = 1;\n"),
		Loader:      LoaderTS,
		Format:      FormatESM,
		NormalizedOpts: NormalizedOptions{
			Target: "es2022",
		},
		SourceMapMode: SourceMapNone,
	}
	key := CacheKey(req, engine.Identity())

	result, err := engine.Transform(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCache(dir, key, &result); err != nil {
		t.Fatal(err)
	}

	// Corrupt the code file.
	codePath := CacheKeyPath(dir, key) + ".code"
	if err := os.WriteFile(codePath, []byte("corrupted"), 0o644); err != nil {
		t.Fatal(err)
	}

	explain, err := CacheExplain(dir, CacheExplainOptions{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	if explain.Entry.Disposition != CacheDispositionCorrupt {
		t.Errorf("expected corrupt, got %s", explain.Entry.Disposition)
	}
	hasDigest := false
	for _, r := range explain.Entry.Reasons {
		if r.Code == ReasonDigestMismatch {
			hasDigest = true
			break
		}
	}
	if !hasDigest {
		t.Error("expected digest mismatch reason")
	}
}

func TestCacheExplainOrphanDetection(t *testing.T) {
	dir := t.TempDir()
	engine := NewEsbuildEngine()
	req := TransformRequest{
		SourcePath:  "test.ts",
		SourceBytes: []byte("const x: number = 1;\n"),
		Loader:      LoaderTS,
		Format:      FormatESM,
		NormalizedOpts: NormalizedOptions{
			Target: "es2022",
		},
		SourceMapMode: SourceMapNone,
	}
	key := CacheKey(req, engine.Identity())
	result, err := engine.Transform(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCache(dir, key, &result); err != nil {
		t.Fatal(err)
	}

	// Write an orphan file (not associated with any .meta entry).
	keyPath := CacheKeyPath(dir, key)
	orphanPath := keyPath + ".orphan"
	if err := os.WriteFile(orphanPath, []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}

	explain, err := CacheExplain(dir, CacheExplainOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(explain.Orphans) == 0 {
		t.Error("expected orphan detection")
	}
}

func TestCacheExplainMetaMalformed(t *testing.T) {
	dir := t.TempDir()
	engine := NewEsbuildEngine()
	req := TransformRequest{
		SourcePath:  "test.ts",
		SourceBytes: []byte("const x: number = 1;\n"),
		Loader:      LoaderTS,
		Format:      FormatESM,
		NormalizedOpts: NormalizedOptions{
			Target: "es2022",
		},
		SourceMapMode: SourceMapNone,
	}
	key := CacheKey(req, engine.Identity())

	result, err := engine.Transform(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCache(dir, key, &result); err != nil {
		t.Fatal(err)
	}

	// Corrupt the metadata.
	metaPath := CacheKeyPath(dir, key) + ".meta"
	if err := os.WriteFile(metaPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	explain, err := CacheExplain(dir, CacheExplainOptions{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	if explain.Entry.Disposition != CacheDispositionCorrupt {
		t.Errorf("expected corrupt, got %s", explain.Entry.Disposition)
	}
	hasMalformed := false
	for _, r := range explain.Entry.Reasons {
		if r.Code == ReasonMetaMalformed {
			hasMalformed = true
			break
		}
	}
	if !hasMalformed {
		t.Error("expected meta-malformed reason")
	}
}
