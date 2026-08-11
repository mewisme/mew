package runtime_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// runtimeSoakCyclesDefault is the default number of cycles for soak tests.
// Tests can override via MEW_SOAK_CYCLES env for long-duration manual runs.
const runtimeSoakCyclesDefault = 20

// runtimeSoakGoroutineDeltaMax is the maximum goroutine growth after all soak
// cycles complete and GC runs.
const runtimeSoakGoroutineDeltaMax = 15

func soakCycles() int {
	if v := strings.TrimSpace(os.Getenv("MEW_SOAK_CYCLES")); v != "" {
		n := 0
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return runtimeSoakCyclesDefault
}

func TestConformanceSoakWatchCycles(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	bin := BuildMewBinary(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ensurePackageJSON(t, proj)
	entry := filepath.Join(proj, "hello.ts")
	if _, err := os.Stat(entry); os.IsNotExist(err) {
		entry = filepath.Join(proj, "hello.mjs")
	}

	cycles := soakCycles()
	if testing.Short() {
		cycles = 5
	}
	t.Logf("watch soak: %d cycles", cycles)

	startGoroutines := runtime.NumGoroutine()
	// ponytail: simple error buffer, formal error collector if soak grows
	errBuf := make([]string, 0, cycles)

	for i := 0; i < cycles; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)

		cmd := exec.CommandContext(ctx, bin, "--cwd", proj,
			"--watch", entry,
			"--", fmt.Sprintf("--soak-iter=%d", i))
		cmd.Env = append(os.Environ(),
			"MEW_EXPERIMENTAL_RUNTIME=1",
		)

		if err := cmd.Start(); err != nil {
			cancel()
			errBuf = append(errBuf, fmt.Sprintf("cycle %d start: %v", i, err))
			continue
		}

		// Let watch settle briefly.
		time.Sleep(300 * time.Millisecond)

		// Signal shutdown.
		if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
		}

		// Wait for exit or timeout.
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			errBuf = append(errBuf, fmt.Sprintf("cycle %d timed out", i))
		}
		cancel()
	}

	if len(errBuf) > 0 {
		t.Fatalf("soak watch cycles had failures:\n%s", strings.Join(errBuf, "\n"))
	}

	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	endGoroutines := runtime.NumGoroutine()
	delta := endGoroutines - startGoroutines
	if delta > runtimeSoakGoroutineDeltaMax {
		t.Errorf("goroutine delta %d exceeds max %d (start=%d, end=%d)",
			delta, runtimeSoakGoroutineDeltaMax, startGoroutines, endGoroutines)
	} else {
		t.Logf("goroutine delta: %d (start=%d, end=%d, max=%d)",
			delta, startGoroutines, endGoroutines, runtimeSoakGoroutineDeltaMax)
	}
}

func TestConformanceSoakWorkerCycles(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	bin := BuildMewBinary(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ensurePackageJSON(t, proj)

	workerEntry := filepath.Join(proj, "worker-ts.mjs")
	if _, err := os.Stat(workerEntry); os.IsNotExist(err) {
		workerEntry = filepath.Join(proj, "worker-env.mjs")
		if _, err := os.Stat(workerEntry); os.IsNotExist(err) {
			t.Skip("no worker fixture entry available")
		}
	}

	cycles := soakCycles()
	if testing.Short() {
		cycles = 5
	}
	t.Logf("worker soak: %d cycles", cycles)

	startGoroutines := runtime.NumGoroutine()
	errBuf := make([]string, 0, cycles)

	// Run sequentially to avoid swamping the system with parallel Go+Node builds.
	for i := 0; i < cycles; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		cmd := exec.CommandContext(ctx, bin,
			"--cwd", proj, workerEntry)
		cmd.Env = append(os.Environ(),
			"MEW_EXPERIMENTAL_RUNTIME=1",
		)

		out, err := cmd.CombinedOutput()
		if err != nil {
			errBuf = append(errBuf, fmt.Sprintf("worker cycle %d: %v (%s)", i, err, strings.TrimSpace(string(out))))
		}
		cancel()
	}

	if len(errBuf) > 0 {
		t.Fatalf("soak worker cycles had failures:\n%s", strings.Join(errBuf, "\n"))
	}

	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	endGoroutines := runtime.NumGoroutine()
	delta := endGoroutines - startGoroutines
	if delta > runtimeSoakGoroutineDeltaMax {
		t.Errorf("goroutine delta %d exceeds max %d (start=%d, end=%d)",
			delta, runtimeSoakGoroutineDeltaMax, startGoroutines, endGoroutines)
	} else {
		t.Logf("goroutine delta: %d (start=%d, end=%d, max=%d)",
			delta, startGoroutines, endGoroutines, runtimeSoakGoroutineDeltaMax)
	}
}

func TestConformanceSoakTransformRecovery(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	bin := BuildMewBinary(t)
	proj := setupRuntimeFixture(t, "runtime-e2e")
	ensurePackageJSON(t, proj)

	tsEntry := filepath.Join(proj, "hello.ts")
	if _, err := os.Stat(tsEntry); os.IsNotExist(err) {
		tsEntry = filepath.Join(proj, "import-esm.ts")
		if _, err := os.Stat(tsEntry); os.IsNotExist(err) {
			t.Skip("no TS fixture entry available")
		}
	}

	cycles := soakCycles()
	if testing.Short() {
		cycles = 5
	}
	t.Logf("transform recovery soak: %d cycles", cycles)

	startGoroutines := runtime.NumGoroutine()
	errBuf := make([]string, 0, cycles)

	for i := 0; i < cycles; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		cmd := exec.CommandContext(ctx, bin,
			"--cwd", proj, tsEntry)
		cmd.Env = append(os.Environ(),
			"MEW_EXPERIMENTAL_RUNTIME=1",
		)

		out, err := cmd.CombinedOutput()
		if err != nil {
			errBuf = append(errBuf, fmt.Sprintf("transform cycle %d: %v (%s)", i, err, strings.TrimSpace(string(out))))
		}
		cancel()
	}

	if len(errBuf) > 0 {
		t.Fatalf("soak transform recovery had failures:\n%s", strings.Join(errBuf, "\n"))
	}

	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	endGoroutines := runtime.NumGoroutine()
	delta := endGoroutines - startGoroutines
	if delta > runtimeSoakGoroutineDeltaMax {
		t.Errorf("goroutine delta %d exceeds max %d (start=%d, end=%d)",
			delta, runtimeSoakGoroutineDeltaMax, startGoroutines, endGoroutines)
	} else {
		t.Logf("goroutine delta: %d (start=%d, end=%d, max=%d)",
			delta, startGoroutines, endGoroutines, runtimeSoakGoroutineDeltaMax)
	}
}

// repoRoot returns the repository root by walking up from cwd.
func repoRoot() string {
	cwd, _ := os.Getwd()
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
	}
	return cwd
}

// ensurePackageJSON writes a minimal package.json to dir if one doesn't exist.
func ensurePackageJSON(t *testing.T, dir string) {
	t.Helper()
	pj := filepath.Join(dir, "package.json")
	if _, err := os.Stat(pj); os.IsNotExist(err) {
		if err := os.WriteFile(pj, []byte(`{"name":"soak-test","private":true}`+"\n"), 0o644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}
	}
}

