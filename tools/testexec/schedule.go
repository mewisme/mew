package main

import (
	"fmt"
	"sort"
	"strings"
)

// shard represents a subset of tests assigned to one worker.
type shard struct {
	Worker int      // worker index (0-based)
	Tests  []string // test function names to run
}

// buildRunPattern creates a -test.run regex that matches the given test names.
func buildRunPattern(tests []string) string {
	if len(tests) == 0 {
		return "^$"
	}
	if len(tests) == 1 {
		return "^" + tests[0] + "$"
	}
	parts := make([]string, len(tests))
	for i, t := range tests {
		parts[i] = "^" + t + "$"
	}
	return strings.Join(parts, "|")
}

// scheduleShards assigns tests to workers using round-robin (deterministic).
// When timing data is available, uses longest-processing-time-first (LPT)
// greedy assignment: sort by descending expected duration, assign each test
// to the currently lightest worker.
func scheduleShards(tests []string, workers int, timings map[string]float64) []shard {
	if workers < 1 {
		workers = 1
	}
	if len(tests) == 0 {
		return nil
	}

	// If timing data available, use LPT.
	if len(timings) > 0 {
		return scheduleLPT(tests, workers, timings)
	}
	return scheduleRoundRobin(tests, workers)
}

// scheduleRoundRobin assigns tests to workers deterministically.
func scheduleRoundRobin(tests []string, workers int) []shard {
	shards := make([]shard, workers)
	for i := range shards {
		shards[i].Worker = i
	}
	for i, test := range tests {
		w := i % workers
		shards[w].Tests = append(shards[w].Tests, test)
	}
	return shards
}

// scheduleLPT assigns tests using longest-processing-time-first greedy scheduling.
// Missing timing data defaults to the average of known timings (never zero).
func scheduleLPT(tests []string, workers int, timings map[string]float64) []shard {
	// Compute default duration: average of known timings.
	var sum float64
	var count int
	for _, d := range timings {
		sum += d
		count++
	}
	defaultDuration := sum / float64(count)
	if defaultDuration <= 0 {
		defaultDuration = 1.0
	}

	// Sort by descending expected duration.
	type timed struct {
		name string
		dur  float64
	}
	items := make([]timed, len(tests))
	for i, t := range tests {
		d, ok := timings[t]
		if !ok {
			d = defaultDuration
		}
		items[i] = timed{name: t, dur: d}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].dur > items[j].dur
	})

	// Greedy assignment: assign each test to the lightest worker.
	loads := make([]float64, workers)
	shards := make([]shard, workers)
	for i := range shards {
		shards[i].Worker = i
	}
	for _, item := range items {
		// Find lightest worker.
		lightest := 0
		for w := 1; w < workers; w++ {
			if loads[w] < loads[lightest] {
				lightest = w
			}
		}
		shards[lightest].Tests = append(shards[lightest].Tests, item.name)
		loads[lightest] += item.dur
	}
	return shards
}

// verifyShardCoverage checks that every test is assigned exactly once and
// shard Test slices are non-overlapping.
func verifyShardCoverage(tests []string, shards []shard) error {
	if len(shards) == 0 && len(tests) == 0 {
		return nil
	}
	seen := make(map[string]int, len(tests))
	for _, s := range shards {
		for _, t := range s.Tests {
			if prev, ok := seen[t]; ok {
				return fmt.Errorf("test %q assigned to workers %d and %d", t, prev, s.Worker)
			}
			seen[t] = s.Worker
		}
	}
	var missing []string
	for _, t := range tests {
		if _, ok := seen[t]; !ok {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("tests not assigned to any shard: %s", strings.Join(missing, ", "))
	}
	return nil
}
