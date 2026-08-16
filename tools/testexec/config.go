package main

import (
	"runtime"
	"strconv"
)

type config struct {
	workers     string // "auto", "1", or explicit N
	short       bool
	race        bool
	tags        string
	run         string
	timeout     string
	verbose     bool
	jsonOut     bool
	cpuOverride int
	packages    []string
}

// workloadClass for adaptive worker count.
type workloadClass int

const (
	classUnit        workloadClass = iota // standard unit tests
	classIntegration                      // integration, conformance, E2E
	classCrash                            // crash-tagged tests
	classRace                             // race detector
)

// effectiveWorkers returns the resolved worker count.
func (c config) effectiveWorkers(class workloadClass) int {
	logicalCPUs := runtime.NumCPU()
	if c.cpuOverride > 0 {
		logicalCPUs = c.cpuOverride
	}

	// Explicit numeric value.
	if c.workers != "auto" {
		n, err := strconv.Atoi(c.workers)
		if err == nil && n > 0 {
			return n
		}
		return 1
	}

	// Auto: adapt to workload.
	max := logicalCPUs
	switch class {
	case classIntegration:
		if max > 4 {
			max = 4
		}
	case classCrash:
		if max > 3 {
			max = 3
		}
	case classRace:
		if max > 2 {
			max = 2
		}
	}
	if max < 1 {
		max = 1
	}
	return max
}

// perWorkerGOMAXPROCS returns GOMAXPROCS budget per worker.
func perWorkerGOMAXPROCS(logicalCPUs, workers int) int {
	if workers <= 0 {
		return logicalCPUs
	}
	budget := logicalCPUs / workers
	if budget < 1 {
		budget = 1
	}
	return budget
}

// classifyPackages determines the workload class for a set of packages.
func classifyPackages(pkgs []string) workloadClass {
	for _, p := range pkgs {
		if containsPattern(p, "tests/integration", "tests/conformance") {
			return classIntegration
		}
	}
	return classUnit
}

// isHeavyPackage reports whether a package should use process-level sharding.
func isHeavyPackage(pkg string) bool {
	heavyPatterns := []string{
		"tests/integration",
		"tests/conformance",
		"internal/app",
		"internal/transaction",
	}
	for _, h := range heavyPatterns {
		if containsPattern(pkg, h) {
			return true
		}
	}
	return false
}

func containsPattern(s string, patterns ...string) bool {
	for _, p := range patterns {
		if len(s) >= len(p) {
			for i := 0; i <= len(s)-len(p); i++ {
				if s[i:i+len(p)] == p {
					return true
				}
			}
		}
	}
	return false
}
