// Package runtime_test provides black-box runtime conformance tests that
// exercise the Mew CLI against fixture projects. Every case runs through
// cli.NewMRoot + cli.ExecuteWithContext — the production CLI path.
package runtime_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mewisme/mew/internal/cli"
	"github.com/mewisme/mew/internal/testkit"
)

// newMRootMu serialises cli.NewMRoot calls (not concurrency-safe).
var newMRootMu sync.Mutex

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
// production CLI path (cli.NewMRoot → cli.ExecuteWithContext).
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
