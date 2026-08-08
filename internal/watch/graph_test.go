package watch

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDependencyGraphSeed(t *testing.T) {
	g := NewDependencyGraph()

	modules := []string{"/proj/src/index.ts", "/proj/src/lib/foo.ts"}
	configs := []string{"/proj/tsconfig.json", "/proj/package.json"}

	add := g.Seed(modules, configs, nil)

	// Verify modules are tracked.
	for _, m := range modules {
		if !g.modules[NormalizePath(m)] {
			t.Errorf("module not tracked: %s", m)
		}
	}
	// Verify configs are tracked.
	for _, c := range configs {
		if !g.configs[NormalizePath(c)] {
			t.Errorf("config not tracked: %s", c)
		}
	}

	// WatchPaths should return dirs for modules + individual configs.
	paths := g.WatchPaths()
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	// Module parent dirs should be covered.
	for _, m := range modules {
		dir := filepath.Dir(NormalizePath(m))
		if !pathSet[dir] {
			t.Errorf("module dir not in WatchPaths: %s", dir)
		}
	}
	// Config files should be in watch paths individually.
	for _, c := range configs {
		if !pathSet[NormalizePath(c)] {
			t.Errorf("config not in WatchPaths: %s", c)
		}
	}

	// add should match WatchPaths (initial seed returns all registrations).
	sort.Strings(add)
	sort.Strings(paths)
	if len(add) != len(paths) {
		t.Fatalf("Seed add len=%d, WatchPaths len=%d", len(add), len(paths))
	}
	for i := range add {
		if add[i] != paths[i] {
			t.Errorf("add[%d] = %s, paths[%d] = %s", i, add[i], i, paths[i])
		}
	}

	// Generation should be 0 (not incremented on Seed).
	if g.Generation() != 0 {
		t.Errorf("generation after Seed = %d, want 0", g.Generation())
	}
}

func TestDependencyGraphReconcileAddRemove(t *testing.T) {
	g := NewDependencyGraph()

	modA := "/proj/src/index.ts"
	modB := "/proj/src/foo.ts"
	g.Seed([]string{modA}, []string{"/proj/tsconfig.json"}, nil)

	gen0 := g.Generation()

	// Add modB — same dir as modA, no new backend registration needed.
	add, remove := g.Reconcile([]string{modA, modB}, []string{"/proj/tsconfig.json"})
	if len(add) != 0 {
		t.Errorf("unexpected add: %v", add)
	}
	if len(remove) != 0 {
		t.Errorf("unexpected remove: %v", remove)
	}
	if g.Generation() != gen0+1 {
		t.Errorf("generation did not increment on reconcile")
	}

	// Remove modB — dir still covered by modA.
	add, remove = g.Reconcile([]string{modA}, []string{"/proj/tsconfig.json"})
	if len(add) != 0 {
		t.Errorf("unexpected add: %v", add)
	}
	if len(remove) != 0 {
		t.Errorf("unexpected remove: %v", remove)
	}

	// Remove modA — dir refcount should hit 0, dir in remove.
	add, remove = g.Reconcile(nil, nil)
	if len(remove) == 0 {
		t.Error("expected dir removal when last module removed")
	}
	_ = add

	// Adding a module in a new dir should add that dir.
	modC := "/proj/src/lib/bar.ts"
	add, _ = g.Reconcile([]string{modC}, nil)
	if len(add) == 0 {
		t.Error("expected dir add for module in new directory")
	}
}

func TestDependencyGraphDirRefcount(t *testing.T) {
	g := NewDependencyGraph()

	dir := "/proj/src"
	mod1 := filepath.Join(dir, "a.ts")
	mod2 := filepath.Join(dir, "b.ts")

	g.Seed([]string{mod1, mod2}, nil, nil)

	// Two modules, same dir — one dir entry.
	if g.dirRefs[dir] != 2 {
		t.Errorf("dirRefs[%s] = %d, want 2", dir, g.dirRefs[dir])
	}

	paths := g.WatchPaths()
	count := 0
	for _, p := range paths {
		if p == dir {
			count++
		}
	}
	if count != 1 {
		t.Errorf("dir %s appears %d times in WatchPaths, want 1", dir, count)
	}

	// Remove one module — dir still covered.
	add, remove := g.Reconcile([]string{mod1}, nil)
	if len(remove) != 0 {
		t.Errorf("unexpected remove when one module remains: %v", remove)
	}
	_ = add

	// Remove both — dir released.
	add, remove = g.Reconcile(nil, nil)
	found := false
	for _, p := range remove {
		if p == dir {
			found = true
		}
	}
	if !found {
		t.Error("dir not in remove when all modules removed")
	}
	_ = add
}

func TestDependencyGraphConfigChanges(t *testing.T) {
	g := NewDependencyGraph()

	cfg1 := "/proj/tsconfig.json"
	cfg2 := "/proj/tsconfig.base.json"

	add, _ := g.Reconcile([]string{"/proj/src/index.ts"}, []string{cfg1, cfg2})

	// Both configs should be in add.
	hasCfg := func(s string) bool {
		for _, p := range add {
			if p == s {
				return true
			}
		}
		return false
	}
	if !hasCfg(cfg1) || !hasCfg(cfg2) {
		t.Errorf("configs missing from add: %v", add)
	}

	// Remove cfg2.
	_, remove := g.Reconcile([]string{"/proj/src/index.ts"}, []string{cfg1})
	if !hasCfg(cfg2) { // cfg2 should be in remove
		found := false
		for _, p := range remove {
			if p == cfg2 {
				found = true
			}
		}
		if !found {
			t.Errorf("removed config %s not in remove: %v", cfg2, remove)
		}
	}
}

func TestDependencyGraphShouldTrigger(t *testing.T) {
	g := NewDependencyGraph()

	mod := "/proj/src/index.ts"
	cfg := "/proj/tsconfig.json"
	g.Seed([]string{mod}, []string{cfg}, nil)

	// Direct match on module.
	if !g.ShouldTrigger(NormalizePath(mod)) {
		t.Error("ShouldTrigger false for tracked module")
	}

	// Direct match on config.
	if !g.ShouldTrigger(NormalizePath(cfg)) {
		t.Error("ShouldTrigger false for tracked config")
	}

	// Arbitrary .ts sibling under covered dir — must NOT trigger.
	// Only exact tracked modules trigger under module-directory coverage.
	sibling := NormalizePath("/proj/src/other.ts")
	if g.ShouldTrigger(sibling) {
		t.Error("ShouldTrigger true for untracked .ts sibling under covered dir")
	}

	// Exact tracked module under covered dir must trigger.
	if !g.ShouldTrigger(NormalizePath(mod)) {
		t.Error("ShouldTrigger false for exact tracked module")
	}

	// .env file under covered dir triggers (config-like).
	envFile := NormalizePath("/proj/src/.env")
	if !g.ShouldTrigger(envFile) {
		t.Error("ShouldTrigger false for .env file under covered dir")
	}

	// Non-relevant file under covered dir.
	readme := NormalizePath("/proj/src/README.md")
	if g.ShouldTrigger(readme) {
		t.Error("ShouldTrigger true for README.md under covered dir")
	}

	// node_modules under covered dir.
	nmFile := NormalizePath("/proj/src/node_modules/pkg/index.js")
	if g.ShouldTrigger(nmFile) {
		t.Error("ShouldTrigger true for node_modules file")
	}

	// Outside coverage.
	outside := NormalizePath("/other/dir/file.ts")
	if g.ShouldTrigger(outside) {
		t.Error("ShouldTrigger true for file outside coverage")
	}
}

func TestDependencyGraphRootTrigger(t *testing.T) {
	g := NewDependencyGraph()
	root := "/proj"
	g.Seed(nil, nil, []string{root})

	// Root dir itself triggers.
	if !g.ShouldTrigger(root) {
		t.Error("ShouldTrigger false for root dir itself")
	}

	// Relevant file under root triggers.
	tsFile := filepath.Join(root, "src/app.ts")
	if !g.ShouldTrigger(tsFile) {
		t.Error("ShouldTrigger false for .ts file under root")
	}

	// Non-relevant file under root skipped.
	readme := filepath.Join(root, "README.md")
	if g.ShouldTrigger(readme) {
		t.Error("ShouldTrigger true for README under root")
	}

	// node_modules under root skipped.
	nmFile := filepath.Join(root, "node_modules/pkg/index.js")
	if g.ShouldTrigger(nmFile) {
		t.Error("ShouldTrigger true for node_modules under root")
	}
}

func TestDependencyGraphGeneration(t *testing.T) {
	g := NewDependencyGraph()

	if g.Generation() != 0 {
		t.Errorf("initial generation = %d, want 0", g.Generation())
	}

	g.Reconcile([]string{"/proj/src/index.ts"}, nil)
	if g.Generation() != 1 {
		t.Errorf("generation after first reconcile = %d, want 1", g.Generation())
	}

	// Same set — still increments (reconcile always bumps gen).
	g.Reconcile([]string{"/proj/src/index.ts"}, nil)
	if g.Generation() != 2 {
		t.Errorf("generation after no-op reconcile = %d, want 2", g.Generation())
	}
}

func TestDependencyGraphStableWatchPaths(t *testing.T) {
	g := NewDependencyGraph()
	g.Seed([]string{"/proj/src/index.ts", "/proj/src/lib/foo.ts"}, []string{"/proj/tsconfig.json"}, nil)

	// WatchPaths must be deterministic.
	first := g.WatchPaths()
	second := g.WatchPaths()

	if len(first) != len(second) {
		t.Fatalf("WatchPaths not stable: len %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("WatchPaths[%d] = %s vs %s", i, first[i], second[i])
		}
	}
}

func TestDependencyGraphEmptyReconcile(t *testing.T) {
	g := NewDependencyGraph()

	add, remove := g.Reconcile(nil, nil)
	if len(add) != 0 || len(remove) != 0 {
		t.Errorf("empty reconcile produced add=%v remove=%v", add, remove)
	}
}

func TestRelevantPath(t *testing.T) {
	// Source extensions.
	for _, ext := range []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs", ".json"} {
		if !isRelevantPath(filepath.Join("/proj/src", "file"+ext)) {
			t.Errorf("isRelevantPath false for %s", ext)
		}
	}

	// .env files.
	for _, name := range []string{".env", ".env.local", ".env.development", ".env.production", ".env.staging"} {
		if !isRelevantPath(filepath.Join("/proj", name)) {
			t.Errorf("isRelevantPath false for %s", name)
		}
	}

	// Config basenames.
	for _, name := range []string{"tsconfig.json", "package.json"} {
		if !isRelevantPath(filepath.Join("/proj", name)) {
			t.Errorf("isRelevantPath false for %s", name)
		}
	}

	// Non-relevant.
	for _, name := range []string{"README.md", "Dockerfile", "Makefile", "notes.txt"} {
		if isRelevantPath(filepath.Join("/proj", name)) {
			t.Errorf("isRelevantPath true for %s", name)
		}
	}

	// node_modules skip.
	if isRelevantPath("/proj/node_modules/pkg/index.js") {
		t.Error("isRelevantPath true for node_modules path")
	}
}

func TestNormalizePath(t *testing.T) {
	cwd, _ := os.Getwd()
	rel := "test.txt"
	norm := NormalizePath(rel)
	if !filepath.IsAbs(norm) {
		t.Errorf("NormalizePath should return absolute path, got %s", norm)
	}
	_ = cwd
}
