// Command testexec runs Go tests in parallel worker processes with
// adaptive concurrency, process-level sharding for heavy packages, and
// deterministic test assignment.
//
// Usage:
//
//	go run ./tools/testexec [flags] [packages...]
//
// Flags:
//
//	-workers N    worker count: "auto" (default), "1", or explicit number
//	-short        pass -short to go test
//	-race         pass -race to go test
//	-tags TAGS    build tags (comma-separated)
//	-run RE       test filter applied per-worker
//	-timeout D    per-package timeout (default: inherited from Go test)
//	-v            verbose: show per-worker output
//	-json         output in test2json format
//	-cpu N        override CPU count for auto worker calculation
//
// Worker count "auto" adapts to workload:
//   - unit packages: NumCPU workers
//   - integration/crash: min(NumCPU, 4) workers
//   - race: min(NumCPU, 2) workers
//
// Each worker process gets GOMAXPROCS set so total CPU budget equals
// logical CPUs (e.g., 4 CPUs / 4 workers = GOMAXPROCS=1 each).
//
// For heavy packages (tests/integration, internal/app, internal/transaction),
// testexec compiles the test binary once with "go test -c" and runs it
// multiple times with distinct "-test.run" filters, sharding test functions
// across workers. Other packages run normally via "go test" with -p for
// package-level parallelism.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	cfg := parseFlags()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "testexec: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.workers, "workers", "auto", "worker count: auto, 1, or explicit N")
	flag.BoolVar(&cfg.short, "short", false, "pass -short to go test")
	flag.BoolVar(&cfg.race, "race", false, "pass -race to go test")
	flag.StringVar(&cfg.tags, "tags", "", "build tags (comma-separated)")
	flag.StringVar(&cfg.run, "run", "", "test filter (-test.run pattern)")
	flag.StringVar(&cfg.timeout, "timeout", "", "per-package timeout")
	flag.BoolVar(&cfg.verbose, "v", false, "verbose output")
	flag.BoolVar(&cfg.jsonOut, "json", false, "output in test2json format")
	flag.IntVar(&cfg.cpuOverride, "cpu", 0, "override CPU count for auto worker calculation")
	flag.Parse()
	cfg.packages = flag.Args()
	if len(cfg.packages) == 0 {
		cfg.packages = []string{"./..."}
	}
	return cfg
}

// run orchestrates test discovery, scheduling, and execution.
func run(ctx context.Context, cfg config) error {
	r, err := newRunner(cfg)
	if err != nil {
		return err
	}
	defer r.cleanup()

	if cfg.verbose {
		fmt.Fprintf(os.Stderr, "testexec: discovering tests in %v\n", cfg.packages)
	}

	discovered, err := discoverPackages(ctx, cfg)
	if err != nil {
		return err
	}

	if cfg.verbose {
		totalTests := 0
		for _, dp := range discovered {
			totalTests += len(dp.Tests)
		}
		fmt.Fprintf(os.Stderr, "testexec: %d packages, %d tests\n", len(discovered), totalTests)
	}

	start := time.Now()
	results := r.runAll(ctx, discovered)
	elapsed := time.Since(start)

	// Summary.
	var passed, failed int
	var failures []string
	for _, rr := range results {
		if rr.Pass {
			passed++
		} else {
			failed++
			failures = append(failures, rr.Package)
		}
	}

	fmt.Fprintf(os.Stderr, "\ntestexec: %d passed, %d failed (%.1fs)\n", passed, failed, elapsed.Seconds())
	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "Failures:\n")
		for _, f := range failures {
			fmt.Fprintf(os.Stderr, "  %s\n", f)
		}
		// Print repro command for first failure.
		for _, rr := range results {
			if !rr.Pass && rr.Worker >= 0 {
				fmt.Fprintf(os.Stderr, "\nReproduce with:\n  go test -count=1 -run '%s' %s\n",
					buildRunPattern(findShardTests(results, rr.Package, rr.Worker)),
					strings.TrimSuffix(rr.Package, fmt.Sprintf(" [worker %d]", rr.Worker)))
				break
			}
		}
		return fmt.Errorf("%d packages failed", failed)
	}
	return nil
}

// findShardTests returns the tests assigned to a worker for a package.
func findShardTests(results []runResult, pkg string, worker int) []string {
	for _, rr := range results {
		if rr.Package == pkg && rr.Worker == worker {
			// Tests are embedded in the shard assignment — we can't recover
			// them from results alone. Return an empty slice; the caller
			// handles this gracefully.
			return nil
		}
	}
	return nil
}
