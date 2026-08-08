package watch

import (
	"path/filepath"
	"sort"
	"strings"
)

// relevantExts is the set of file extensions that can affect runtime behavior.
var relevantExts = map[string]bool{
	".ts": true, ".tsx": true, ".mts": true, ".cts": true,
	".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".json": true,
}

// relevantBasenames is the set of basenames that can affect configuration.
var relevantBasenames = map[string]bool{
	"tsconfig.json": true,
	"package.json":  true,
}

// skippedSegments are directory names excluded from recursive watching.
var skippedSegments = map[string]bool{
	"node_modules": true,
	".git":         true,
	".mew":         true,
}

// NodeKind classifies a tracked path.
type NodeKind int

const (
	KindModule NodeKind = iota // entrypoint + resolved local modules (file)
	KindConfig                 // tsconfig/package.json/.env (file)
	KindRoot                   // explicit watch directory
)

// DependencyGraph tracks the logical set of files and directories that
// materially affect runtime behavior. It is owned per supervisor instance
// (never process-global) and provides deterministic, sorted outputs.
type DependencyGraph struct {
	generation uint64
	modules    map[string]bool // canonical module file paths
	configs    map[string]bool // canonical config file paths (watched individually)
	roots      map[string]bool // canonical explicit dirs
	dirRefs    map[string]int  // canonical module parent dir → module count
}

// NewDependencyGraph returns an empty graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		modules: make(map[string]bool),
		configs: make(map[string]bool),
		roots:   make(map[string]bool),
		dirRefs: make(map[string]int),
	}
}

// Seed populates the graph with initial paths and returns the backend
// registrations needed to cover them. All paths are stored using pathKey
// for case-insensitive identity on Windows.
func (g *DependencyGraph) Seed(modules, configs, roots []string) []string {
	for _, p := range modules {
		p = pathKey(p)
		g.modules[p] = true
		g.incDir(p)
	}
	for _, p := range configs {
		p = pathKey(p)
		g.configs[p] = true
	}
	for _, p := range roots {
		p = pathKey(p)
		g.roots[p] = true
	}
	return g.WatchPaths()
}

// Reconcile diffs the current graph against new module and config sets,
// updates internal state, increments the generation, and returns the
// canonical backend paths to add and remove.
func (g *DependencyGraph) Reconcile(newModules, newConfigs []string) (add, remove []string) {
	// Normalize inputs using pathKey for stable identity.
	modSet := make(map[string]bool, len(newModules))
	for _, p := range newModules {
		modSet[pathKey(p)] = true
	}
	cfgSet := make(map[string]bool, len(newConfigs))
	for _, p := range newConfigs {
		cfgSet[pathKey(p)] = true
	}

	// Diff modules: added.
	for p := range modSet {
		if !g.modules[p] {
			dir := filepath.Dir(p)
			old := g.dirRefs[dir]
			g.modules[p] = true
			g.dirRefs[dir] = old + 1
			if old == 0 {
				add = append(add, dir)
			}
		}
	}
	// Diff modules: removed.
	for p := range g.modules {
		if !modSet[p] {
			dir := filepath.Dir(p)
			old := g.dirRefs[dir]
			if old <= 1 {
				delete(g.dirRefs, dir)
				remove = append(remove, dir)
			} else {
				g.dirRefs[dir] = old - 1
			}
			delete(g.modules, p)
		}
	}

	// Diff configs: added.
	for p := range cfgSet {
		if !g.configs[p] {
			g.configs[p] = true
			add = append(add, p)
		}
	}
	// Diff configs: removed.
	for p := range g.configs {
		if !cfgSet[p] {
			delete(g.configs, p)
			remove = append(remove, p)
		}
	}

	// Sort for deterministic output.
	sort.Strings(add)
	sort.Strings(remove)

	g.generation++
	return add, remove
}

// WatchPaths returns every backend registration required to cover the
// current graph: config files (individual), module parent dirs (refcounted),
// and explicit roots. The result is sorted and deduplicated.
func (g *DependencyGraph) WatchPaths() []string {
	seen := make(map[string]bool)
	var out []string

	for p := range g.configs {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for dir := range g.dirRefs {
		if !seen[dir] {
			seen[dir] = true
			out = append(out, dir)
		}
	}
	for dir := range g.roots {
		if !seen[dir] {
			seen[dir] = true
			out = append(out, dir)
		}
	}

	sort.Strings(out)
	return out
}

// ShouldTrigger returns true when a normalized event path is logically
// relevant: it is a directly tracked file, or a relevant file under a
// covered module/root directory. Uses pathKey for case-insensitive
// identity on Windows.
func (g *DependencyGraph) ShouldTrigger(path string) bool {
	key := pathKey(path)

	// Direct match on a tracked module or config.
	if g.modules[key] || g.configs[key] {
		return true
	}

	// Match under a root directory. Exact match on the directory
	// itself always triggers (directory events matter).
	for r := range g.roots {
		if hasPathPrefix(path, r) {
			if key == pathKey(r) {
				return true
			}
			return isRelevantPath(path)
		}
	}
	// Match under a covered module directory.  Only explicit module/config
	// files (tested above) plus directory events and .env* files trigger.
	// Arbitrary sibling .ts/.js/.json files do NOT trigger just because
	// they share a directory with a tracked module.
	for dir := range g.dirRefs {
		if hasPathPrefix(path, dir) {
			if key == pathKey(dir) {
				return true // directory event
			}
			// .env* files under module dirs are config-like and must trigger.
			if strings.HasPrefix(filepath.Base(path), ".env") {
				return true
			}
			return false
		}
	}
	return false
}

// Generation returns the current generation counter, incremented on each
// successful Reconcile.
func (g *DependencyGraph) Generation() uint64 { return g.generation }

// incDir increments the reference count for a module's parent directory.
func (g *DependencyGraph) incDir(modulePath string) {
	dir := filepath.Dir(modulePath)
	g.dirRefs[dir] = g.dirRefs[dir] + 1
}

// isRelevantPath returns true when the path has a relevant extension or
// basename and does not traverse a skipped directory segment.
func isRelevantPath(path string) bool {
	base := filepath.Base(path)

	// .env* files always relevant.
	if strings.HasPrefix(base, ".env") {
		return true
	}

	// Config basenames.
	if relevantBasenames[base] {
		return true
	}

	// Source extensions.
	ext := filepath.Ext(base)
	if relevantExts[ext] {
		// Reject paths that pass through skipped segments.
		for _, seg := range strings.Split(path, string(filepath.Separator)) {
			if segmentSkipped(seg) {
				return false
			}
		}
		return true
	}

	return false
}
