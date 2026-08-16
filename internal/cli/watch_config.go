package cli

import (
	"os"
	"path/filepath"
	"strings"

	"bufio"

	"github.com/mewisme/mew/internal/dotenv"
	"github.com/mewisme/mew/internal/transform"
	"github.com/mewisme/mew/internal/watch"
)

// collectConfigDeps returns canonical, existing config file paths to watch.
// It collects: the tsconfig extends chain rooted at the entrypoint dir,
// package.json files walking up from the entrypoint dir, env files via
// dotenv.Discover per ancestor dir (mode-specific), and explicit --env-file
// values. Respects noEnvFile: when true, env auto-discovery is skipped but
// explicit --env-file paths are still included.
func collectConfigDeps(cwd, entrypoint string, mode string, envFiles []string, noEnvFile bool) []string {
	seen := make(map[string]bool)
	var out []string

	add := func(p string) {
		if p == "" {
			return
		}
		p = watch.NormalizePath(p)
		if seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}

	entryDir := filepath.Dir(entrypoint)

	// Tsconfig chain: discover + load, walk parent chain.
	if configPath, err := transform.DiscoverTsconfig(entryDir); err == nil && configPath != "" {
		chain, err := transform.LoadTsconfigChain(configPath)
		if err == nil {
			// Walk from the root parent through to the leaf.
			for _, tsc := range chain {
				add(tsc.Path)
			}
		} else {
			// Chain load failed; still watch the discovered file.
			add(configPath)
		}
	}

	// Package.json and env files: walk up from entrypoint dir.
	for dir := entryDir; dir != "" && dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		pkg := filepath.Join(dir, "package.json")
		if _, err := os.Stat(pkg); err == nil {
			add(pkg)
		}

		if !noEnvFile {
			for _, envPath := range dotenv.Discover(dir, mode) {
				add(envPath)
			}
			// Also collect .env.*.local files in dir for new-file detection
			// (dotenv.Discover only returns existing files).
			entries, err := os.ReadDir(dir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						continue
					}
					n := e.Name()
					if strings.HasPrefix(n, ".env.") && strings.HasSuffix(n, ".local") {
						add(filepath.Join(dir, n))
					}
				}
			}
		}
	}

	// Explicit --env-file paths.
	for _, f := range envFiles {
		p := f
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		add(p)
	}

	return out
}

// readDepTrace reads a newline-delimited file of resolved module paths,
// canonicalizes each path, deduplicates, and filters out paths that no
// longer exist.
func readDepTrace(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	seen := make(map[string]bool)
	var out []string

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		p := watch.NormalizePath(line)
		if seen[p] {
			continue
		}
		seen[p] = true
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}
