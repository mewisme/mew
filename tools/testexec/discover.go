package main

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// discoveredPackage holds the test binary path and test function names for a package.
type discoveredPackage struct {
	ImportPath string   // Go import path
	Tests      []string // Test function names
}

// discoverPackages lists all Go packages matching the given patterns and discovers
// their test functions via "go test -list".
func discoverPackages(ctx context.Context, cfg config) ([]discoveredPackage, error) {
	// List packages.
	listArgs := []string{"list"}
	if cfg.tags != "" {
		listArgs = append(listArgs, "-tags", cfg.tags)
	}
	listArgs = append(listArgs, cfg.packages...)

	cmd := exec.CommandContext(ctx, "go", listArgs...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w\n%s", err, out)
	}

	var pkgs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pkgs = append(pkgs, line)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages matched %v", cfg.packages)
	}

	// Discover tests per package.
	var discovered []discoveredPackage
	for _, pkg := range pkgs {
		dp, err := discoverTests(ctx, pkg, cfg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", pkg, err)
		}
		if len(dp.Tests) > 0 || !isTestOnlyPackage(pkg) {
			discovered = append(discovered, dp)
		}
	}
	return discovered, nil
}

// discoverTests lists test functions in a package.
func discoverTests(ctx context.Context, pkg string, cfg config) (discoveredPackage, error) {
	dp := discoveredPackage{ImportPath: pkg}

	listArgs := []string{"test", "-list", ".*"}
	if cfg.tags != "" {
		listArgs = append(listArgs, "-tags", cfg.tags)
	}
	if cfg.race {
		listArgs = append(listArgs, "-race")
	}
	listArgs = append(listArgs, pkg)

	cmd := exec.CommandContext(ctx, "go", listArgs...)
	out, err := cmd.Output()
	if err != nil {
		// Package may have no test files — that's fine.
		if strings.Contains(string(out), "no test files") || strings.Contains(string(out), "no Go files") {
			return dp, nil
		}
		return dp, fmt.Errorf("go test -list: %w\n%s", err, out)
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Test") {
			dp.Tests = append(dp.Tests, line)
		}
	}
	sort.Strings(dp.Tests)
	return dp, nil
}

// isTestOnlyPackage reports whether a package path is under tests/ (external test packages).
func isTestOnlyPackage(pkg string) bool {
	return containsPattern(pkg, "/tests/")
}

// compileTestBinary compiles the test binary for a package and returns the path.
func compileTestBinary(ctx context.Context, pkg string, workDir string, cfg config) (string, error) {
	// Use a deterministic name based on package path.
	safeName := strings.NewReplacer("/", "_", ".", "_").Replace(pkg)
	binPath := fmt.Sprintf("%s/%s.test", workDir, safeName)

	args := []string{"test", "-c", "-o", binPath}
	if cfg.tags != "" {
		args = append(args, "-tags", cfg.tags)
	}
	if cfg.race {
		args = append(args, "-race")
	}
	args = append(args, pkg)

	cmd := exec.CommandContext(ctx, "go", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go test -c %s: %w\n%s", pkg, err, out)
	}
	return binPath, nil
}

// goTestArgs builds the argument list for a standard "go test" invocation.
func goTestArgs(cfg config, pkg string) []string {
	args := []string{"test"}
	if cfg.short {
		args = append(args, "-short")
	}
	if cfg.race {
		args = append(args, "-race")
	}
	if cfg.tags != "" {
		args = append(args, "-tags", cfg.tags)
	}
	if cfg.run != "" {
		args = append(args, "-run", cfg.run)
	}
	if cfg.timeout != "" {
		args = append(args, "-timeout", cfg.timeout)
	}
	if cfg.verbose {
		args = append(args, "-v")
	}
	if cfg.jsonOut {
		args = append(args, "-json")
	}
	args = append(args, "-count=1")
	args = append(args, pkg)
	return args
}
