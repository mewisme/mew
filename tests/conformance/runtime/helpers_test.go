// Package runtime_test provides black-box runtime conformance tests that
// exercise the Mew CLI against fixture projects.
//
// Execution models:
//   - runM / runMCtx: production CLI path in-process (cli.NewMRoot +
//     cli.ExecuteWithContext). Appropriate for tests that verify behavior
//     through fixture output files (output.txt). The child Node process
//     still runs as a real subprocess.
//   - runMBinary: executes the pre-built m binary as a separate process.
//     Required for certifying CLI output, artifacts, and lifecycle
//     behavior (watch, trace, doctor, support-bundle, inspector).
//
// Every required conformance case must prove observable behavior, not
// merely process exit success.
package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mewisme/mew/internal/cli"
	"github.com/mewisme/mew/internal/testkit"
)

// mewBinary is the path to the pre-built m binary. Set by BuildMewBinary or
// discovered from MEW_CONFORMANCE_BINARY env.
var mewBinary string

// mewBinaryOnce guards one-time binary build.
var mewBinaryOnce sync.Once

// newMRootMu serialises cli.NewMRoot calls (not concurrency-safe).
var newMRootMu sync.Mutex

// BuildMewBinary ensures the production m binary is built and cached.
// Idempotent; safe to call from any test. Respects MEW_CONFORMANCE_BINARY
// override for CI environments. The binary is built once into a persistent
// temp directory that survives across individual test invocations.
func BuildMewBinary(t *testing.T) string {
	t.Helper()
	mewBinaryOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("MEW_CONFORMANCE_BINARY")); v != "" {
			if _, err := os.Stat(v); err != nil {
				t.Fatalf("MEW_CONFORMANCE_BINARY=%q: %v", v, err)
			}
			mewBinary = v
			return
		}
		binDir, err := os.MkdirTemp("", "mew-conformance-bin-*")
		if err != nil {
			t.Fatalf("create bin dir: %v", err)
		}
		binPath := filepath.Join(binDir, "m")
		buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/m")
		buildCmd.Dir = testkit.ModuleRoot(t)
		out, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build m binary: %v\n%s", err, out)
		}
		mewBinary = binPath
	})
	return mewBinary
}

// runMBinary executes the pre-built m binary as a separate process against
// a project directory. Returns exit code, combined stdout+stderr, and any
// execution error. Uses context for timeout/cancellation.
//
// The binary is launched with MEW_EXPERIMENTAL_RUNTIME=1 by default. Pass
// MEW_EXPERIMENTAL_RUNTIME=0 in extraEnv to suppress runtime injection
// (e.g. for --node opt-out tests).
func runMBinary(t *testing.T, ctx context.Context, projDir string, args ...string) (int, string, error) {
	t.Helper()
	bin := BuildMewBinary(t)
	full := append([]string{"--cwd", projDir}, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	cmd.Env = append(os.Environ(), "MEW_EXPERIMENTAL_RUNTIME=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	code := exitCodeFromCmd(runErr)
	out := stdout.String()
	errOut := stderr.String()
	combined := out
	if errOut != "" {
		if combined != "" {
			combined += "\n"
		}
		combined += errOut
	}
	return code, combined, runErr
}

// runMBinaryEnv is like runMBinary but allows extra environment variables
// to be appended to the child process environment.
func runMBinaryEnv(t *testing.T, ctx context.Context, projDir string, extraEnv []string, args ...string) (int, string, error) {
	t.Helper()
	bin := BuildMewBinary(t)
	full := append([]string{"--cwd", projDir}, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	code := exitCodeFromCmd(runErr)
	out := stdout.String()
	errOut := stderr.String()
	combined := out
	if errOut != "" {
		if combined != "" {
			combined += "\n"
		}
		combined += errOut
	}
	return code, combined, runErr
}

// exitCodeFromCmd extracts the exit code from a command run error.
func exitCodeFromCmd(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

// runMBinaryOrFail executes m binary; fails the test on execution error
// (process couldn't start). Returns exit code and combined output.
func runMBinaryOrFail(t *testing.T, ctx context.Context, projDir string, args ...string) (int, string) {
	t.Helper()
	code, out, err := runMBinary(t, ctx, projDir, args...)
	if err != nil {
		if ctx.Err() != nil {
			// Context cancellation is expected for lifecycle tests (watch).
			return code, out
		}
		t.Fatalf("runMBinary %v: %v", args, err)
	}
	return code, out
}

func skipWithoutNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node required for runtime conformance")
	}
}

// setupRuntimeFixture copies a runtime conformance fixture into a temp dir.
func setupRuntimeFixture(t *testing.T, rel string) string {
	t.Helper()
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	testkit.CopyFixture(t, filepath.Join("runner", rel), projDir)
	return projDir
}

// runM executes the m CLI against a project directory. It exercises the full
// production CLI path (cli.NewMRoot -> cli.ExecuteWithContext).
func runM(t *testing.T, projDir string, args ...string) (int, string) {
	t.Helper()
	return runMCtx(t, context.Background(), projDir, args...)
}

// runMCtx is like runM but accepts a context for cancellation.
func runMCtx(t *testing.T, ctx context.Context, projDir string, args ...string) (int, string) {
	t.Helper()
	newMRootMu.Lock()
	cliRoot := cli.NewMRoot(cli.BuildInfo{Version: "0.0.0-test"})
	newMRootMu.Unlock()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cliRoot.SetOut(outBuf)
	cliRoot.SetErr(errBuf)
	full := append([]string{"--cwd", projDir, "--output", "silent"}, args...)
	cliRoot.SetArgs(full)
	code := cli.ExecuteWithContext(cliRoot, ctx)
	out := outBuf.String()
	errOut := errBuf.String()
	if code != 0 {
		if trimmed := out; len(trimmed) > 0 && trimmed[0] == '{' {
			return code, out
		}
		if out != "" && errOut != "" {
			return code, out + errOut
		}
		if errOut != "" {
			return code, errOut
		}
	}
	if out != "" {
		return code, out
	}
	return code, errOut
}

// readOutput reads the fixture output file produced by scripts that write
// results to a file.
func readOutput(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatalf("read output.txt: %v", err)
	}
	return string(bytes.TrimSpace(data))
}

// writeFile writes content to a path relative to dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// fixturePath returns the absolute path to a repository fixture.
func fixturePath(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(testkit.ModuleRoot(t), "fixtures", filepath.FromSlash(rel))
}

// requireOutput asserts that output contains each wanted substring.
// Fails with the full output if any substring is missing.
func requireOutput(t *testing.T, output string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(output, w) {
			t.Fatalf("output missing %q, got:\n%s", w, output)
		}
	}
}

// requireNoOutput asserts that output does NOT contain any of the forbidden
// substrings. Fails with the matching substring if found.
func requireNoOutput(t *testing.T, output string, forbidden ...string) {
	t.Helper()
	for _, f := range forbidden {
		if strings.Contains(output, f) {
			t.Fatalf("output contains forbidden %q:\n%s", f, output)
		}
	}
}

// deadline creates a context with a timeout suitable for the test's deadline
// or a reasonable default. Uses 30s default (accounts for one-time binary
// build on first call), capped by test deadline.
func deadline(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	d := 60 * time.Second
	if dd, ok := t.Deadline(); ok {
		half := time.Until(dd) / 2
		if half < d {
			d = half
		}
	}
	if d < 5*time.Second {
		d = 5 * time.Second
	}
	return context.WithTimeout(context.Background(), d)
}

// waitForFile polls for a file to exist, returning its path when found.
// Returns empty string and an error on timeout.
func waitForFile(t *testing.T, dir, glob string, timeout time.Duration) (string, error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		matches, err := filepath.Glob(filepath.Join(dir, glob))
		if err != nil {
			return "", err
		}
		if len(matches) > 0 {
			return matches[0], nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", fmt.Errorf("file matching %q not found in %s after %v", glob, dir, timeout)
}

// hasExtension checks if the system has the given Node binary on PATH.
func hasNodeBinary(t testing.TB, name string) bool {
	t.Helper()
	_, err := exec.LookPath(name)
	return err == nil
}
