package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// runner manages test execution across workers.
type runner struct {
	cfg         config
	workDir     string // temp dir for compiled test binaries
	logicalCPUs int
}

func newRunner(cfg config) (*runner, error) {
	dir, err := os.MkdirTemp("", "testexec-*")
	if err != nil {
		return nil, fmt.Errorf("creating work dir: %w", err)
	}
	return &runner{
		cfg:         cfg,
		workDir:     dir,
		logicalCPUs: runtime.NumCPU(),
	}, nil
}

func (r *runner) cleanup() {
	_ = os.RemoveAll(r.workDir)
}

// runResult holds the outcome of a single worker or package execution.
type runResult struct {
	Package string
	Worker  int
	Pass    bool
	Output  string // combined stdout+stderr
	Err     error
}

// runAll executes all packages. Heavy packages get process-level sharding;
// light packages run normally with package-level parallelism via -p.
func (r *runner) runAll(ctx context.Context, discovered []discoveredPackage) []runResult {
	class := classifyPackages(r.cfg.packages)
	workers := r.cfg.effectiveWorkers(class)
	gomaxprocs := perWorkerGOMAXPROCS(r.logicalCPUs, workers)

	if r.cfg.verbose {
		fmt.Fprintf(os.Stderr, "testexec: %d logical CPUs, %d workers, GOMAXPROCS=%d per worker\n",
			r.logicalCPUs, workers, gomaxprocs)
	}

	if workers == 1 {
		return r.runSerial(ctx, discovered)
	}

	var results []runResult
	var heavy, light []discoveredPackage
	for _, dp := range discovered {
		if isHeavyPackage(dp.ImportPath) {
			heavy = append(heavy, dp)
		} else {
			light = append(light, dp)
		}
	}

	// Light packages: run via go test with package-level parallelism.
	if len(light) > 0 {
		results = append(results, r.runLightPackages(ctx, light, workers, gomaxprocs)...)
	}

	// Heavy packages: process-level sharding.
	for _, dp := range heavy {
		results = append(results, r.runHeavyPackage(ctx, dp, workers, gomaxprocs)...)
	}

	return results
}

// runSerial executes all packages serially (workers=1).
func (r *runner) runSerial(ctx context.Context, discovered []discoveredPackage) []runResult {
	var results []runResult
	for _, dp := range discovered {
		args := goTestArgs(r.cfg, dp.ImportPath)
		cmd := exec.CommandContext(ctx, "go", args...)
		out, err := cmd.CombinedOutput()
		rr := runResult{
			Package: dp.ImportPath,
			Pass:    err == nil,
			Output:  string(out),
			Err:     err,
		}
		if r.cfg.verbose || !rr.Pass {
			r.printResult(rr, 0)
		}
		results = append(results, rr)
	}
	return results
}

// runLightPackages runs packages normally with -p for package-level parallelism.
func (r *runner) runLightPackages(ctx context.Context, pkgs []discoveredPackage, workers, gomaxprocs int) []runResult {
	// Collect all package paths.
	pkgPaths := make([]string, len(pkgs))
	for i, dp := range pkgs {
		pkgPaths[i] = dp.ImportPath
	}

	args := []string{"test"}
	if r.cfg.short {
		args = append(args, "-short")
	}
	if r.cfg.race {
		args = append(args, "-race")
	}
	if r.cfg.tags != "" {
		args = append(args, "-tags", r.cfg.tags)
	}
	if r.cfg.run != "" {
		args = append(args, "-run", r.cfg.run)
	}
	if r.cfg.timeout != "" {
		args = append(args, "-timeout", r.cfg.timeout)
	}
	if r.cfg.verbose {
		args = append(args, "-v")
	}
	if r.cfg.jsonOut {
		args = append(args, "-json")
	}
	args = append(args, "-count=1")
	args = append(args, fmt.Sprintf("-p=%d", workers))
	args = append(args, pkgPaths...)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOMAXPROCS=%d", gomaxprocs))
	out, err := cmd.CombinedOutput()

	// Parse multi-package output to produce per-package results.
	results := r.splitMultiPackageOutput(pkgPaths, string(out), err)
	for _, rr := range results {
		if r.cfg.verbose || !rr.Pass {
			r.printResult(rr, 0)
		}
	}
	return results
}

// runHeavyPackage shards a single heavy package across workers.
func (r *runner) runHeavyPackage(ctx context.Context, dp discoveredPackage, workers, gomaxprocs int) []runResult {
	// If few tests, no tests, serial mode, or explicit -run filter, run normally.
	if len(dp.Tests) <= 1 || workers <= 1 || r.cfg.run != "" {
		args := goTestArgs(r.cfg, dp.ImportPath)
		cmd := exec.CommandContext(ctx, "go", args...)
		out, err := cmd.CombinedOutput()
		return []runResult{{Package: dp.ImportPath, Pass: err == nil, Output: string(out), Err: err}}
	}

	// Shard tests across workers.
	shards := scheduleShards(dp.Tests, workers, nil)
	if err := verifyShardCoverage(dp.Tests, shards); err != nil {
		return []runResult{{Package: dp.ImportPath, Pass: false, Err: fmt.Errorf("shard verification: %w", err)}}
	}

	// Compile test binary once.
	binPath, err := compileTestBinary(ctx, dp.ImportPath, r.workDir, r.cfg)
	if err != nil {
		return []runResult{{Package: dp.ImportPath, Pass: false, Err: err, Output: err.Error()}}
	}

	// Run each shard as a separate process with isolated environment.
	var mu sync.Mutex
	var results []runResult
	var wg sync.WaitGroup

	for _, sh := range shards {
		wg.Add(1)
		go func(s shard) {
			defer wg.Done()
			rr := r.runShard(ctx, dp.ImportPath, binPath, s, gomaxprocs)
			mu.Lock()
			results = append(results, rr)
			mu.Unlock()
			if r.cfg.verbose || !rr.Pass {
				r.printResult(rr, s.Worker)
				if r.cfg.verbose && rr.Pass {
					fmt.Fprint(os.Stderr, rr.Output)
				}
			}
		}(sh)
	}
	wg.Wait()

	return results
}

// runShard executes a single test binary shard in an isolated process.
// pkgDir is the filesystem path to the package source directory.
func (r *runner) runShard(ctx context.Context, pkg, binPath string, s shard, gomaxprocs int) runResult {
	pkgDir := packageDir(pkg)
	pattern := buildRunPattern(s.Tests)

	// Create isolated temp home for this worker.
	homeDir, err := os.MkdirTemp("", fmt.Sprintf("testexec-worker%d-*", s.Worker))
	if err != nil {
		return runResult{Package: pkg, Worker: s.Worker, Pass: false, Err: fmt.Errorf("creating home: %w", err)}
	}
	defer func() { _ = os.RemoveAll(homeDir) }()

	cacheDir := filepath.Join(homeDir, ".cache", "mew")
	storeDir := filepath.Join(homeDir, ".local", "share", "github.com", "mewisme", "mew", "store")
	configDir := filepath.Join(homeDir, ".config", "mew")
	for _, d := range []string{cacheDir, storeDir, configDir} {
		_ = os.MkdirAll(d, 0o755)
	}

	env := os.Environ()
	// Overlay isolated home vars.
	env = setEnv(env, "HOME", homeDir)
	env = setEnv(env, "USERPROFILE", homeDir)
	env = setEnv(env, "XDG_CACHE_HOME", filepath.Join(homeDir, ".cache"))
	env = setEnv(env, "XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	env = setEnv(env, "XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))
	env = setEnv(env, "XDG_STATE_HOME", filepath.Join(homeDir, ".local", "state"))
	env = setEnv(env, "MEW_HOME", homeDir)
	env = setEnv(env, "MEW_CACHE_DIR", cacheDir)
	env = setEnv(env, "MEW_STORE_DIR", storeDir)
	env = setEnv(env, "MEW_CONFIG_DIR", configDir)
	env = setEnv(env, "GOMAXPROCS", fmt.Sprintf("%d", gomaxprocs))

	args := []string{"-test.run", pattern}
	if r.cfg.verbose {
		args = append(args, "-test.v")
	}
	if r.cfg.short {
		args = append(args, "-test.short")
	}
	// Note: cfg.run is already applied at discovery time — tests in
	// shard.Tests are already filtered. Applying it again would
	// conflict with -test.run (last flag wins).

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Env = env
	cmd.Dir = pkgDir

	out, err := cmd.CombinedOutput()
	return runResult{
		Package: fmt.Sprintf("%s [worker %d]", pkg, s.Worker),
		Worker:  s.Worker,
		Pass:    err == nil,
		Output:  string(out),
		Err:     err,
	}
}

// splitMultiPackageOutput splits combined "go test pkg1 pkg2 ..." output
// into per-package results.
func (r *runner) splitMultiPackageOutput(pkgs []string, output string, _ error) []runResult {
	var results []runResult
	for _, pkg := range pkgs {
		rr := runResult{
			Package: pkg,
			Pass:    true, // assume pass; failures are detected below
		}
		// Check for "FAIL <pkg>" in output.
		if strings.Contains(output, "FAIL\t"+pkg+"\t") || strings.Contains(output, "FAIL\t"+pkg+" ") {
			rr.Pass = false
		}
		// Check for build failure.
		if strings.Contains(output, "can't load package") && strings.Contains(output, pkg) {
			rr.Pass = false
		}
		results = append(results, rr)
	}
	// go test with multiple packages exits non-zero if any fail;
	// individual PASS/FAIL is already detected from output above.
	return results
}

func (r *runner) printResult(rr runResult, worker int) {
	status := "PASS"
	if !rr.Pass {
		status = "FAIL"
	}
	if worker >= 0 && rr.Worker >= 0 {
		fmt.Fprintf(os.Stderr, "--- %s: %s (worker %d)\n", status, rr.Package, rr.Worker)
	} else {
		fmt.Fprintf(os.Stderr, "--- %s: %s\n", status, rr.Package)
	}
	if !rr.Pass && rr.Err != nil {
		fmt.Fprintf(os.Stderr, "    error: %v\n", rr.Err)
		// Print last few lines of output for debugging.
		lines := strings.Split(strings.TrimSpace(rr.Output), "\n")
		start := len(lines) - 10
		if start < 0 {
			start = 0
		}
		for _, line := range lines[start:] {
			fmt.Fprintf(os.Stderr, "    %s\n", line)
		}
	}
}

// findModuleRoot walks up from the current working directory to find go.mod.
// packageDir converts a Go import path to a filesystem directory
// by resolving it relative to the module root.
func packageDir(importPath string) string {
	root := findModuleRoot()
	if root == "" {
		return ""
	}
	// Strip module path prefix to get relative directory.
	// We use a heuristic: the import path starts with the module path
	// from go.mod, and the remainder is the relative directory.
	// For simplicity, use "go list -m" or just resolve via GOPATH/src pattern.
	// Since we're in a module, we can use "go list -f {{.Dir}}" for the package.
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", importPath)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return root
	}
	return strings.TrimSpace(string(out))
}

// findModuleRoot walks up from the current working directory to find go.mod.
func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
