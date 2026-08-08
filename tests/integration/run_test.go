package integration_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mewisme/mew/internal/cli"
	"github.com/mewisme/mew/internal/testkit"
)

// newMRootMu protects concurrent calls to cli.NewMRoot.
// cobra.AddTemplateFunc writes to a global map without synchronization,
// and configureGroupedHelp calls it from NewMRoot.
var newMRootMu sync.Mutex

func setupRunFixture(t *testing.T, rel string) string {
	t.Helper()
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	testkit.CopyFixture(t, "runner/"+rel, projDir)
	return projDir
}

func runMProject(t *testing.T, projDir string, args ...string) (int, string) {
	t.Helper()
	return runMProjectCtx(t, context.Background(), projDir, args...)
}

func runMProjectCtx(t *testing.T, ctx context.Context, projDir string, args ...string) (int, string) {
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
		if trimmed := strings.TrimSpace(out); trimmed != "" && strings.HasPrefix(trimmed, "{") {
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

func parseEnvOutput(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i < 0 {
			continue
		}
		m[line[:i]] = line[i+1:]
	}
	return m
}

func TestRunHookOrdering(t *testing.T) {
	skipWithoutNode(t)
	projDir := setupRunFixture(t, "basic-scripts")

	code, out := runMProject(t, projDir, "run", "dev")
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	data, err := os.ReadFile(filepath.Join(projDir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	want := "predev\ndev\npostdev"
	if got != want {
		t.Fatalf("order %q, want %q", got, want)
	}
}

func TestRunEnvParityGolden(t *testing.T) {
	skipWithoutNode(t)
	projDir := setupRunFixture(t, "env-parity")

	goldenPath := filepath.Join(testkit.FixtureDir(t, "runner/env-parity"), "expected-env.golden")
	wantKeys, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}

	code, out := runMProject(t, projDir, "run", "env")
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	raw, err := os.ReadFile(filepath.Join(projDir, "env.out"))
	if err != nil {
		t.Fatal(err)
	}
	got := parseEnvOutput(string(raw))

	for _, line := range strings.Split(strings.TrimSpace(string(wantKeys)), "\n") {
		key := strings.TrimSpace(line)
		if key == "" {
			continue
		}
		if _, ok := got[key]; !ok {
			t.Fatalf("missing env key %q in output:\n%s", key, string(raw))
		}
	}

	absProj, err := filepath.Abs(projDir)
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"INIT_CWD":             absProj,
		"npm_lifecycle_event":  "env",
		"npm_lifecycle_script": "node scripts/print-env.js",
		"npm_package_name":     "runner-env-parity",
		"npm_package_version":  "2.0.0",
		"npm_package_json":     filepath.Join(absProj, "package.json"),
	}
	for key, want := range checks {
		if got[key] != want {
			t.Fatalf("%s=%q, want %q", key, got[key], want)
		}
	}
}

func TestRunArgForwarding(t *testing.T) {
	skipWithoutNode(t)
	projDir := setupRunFixture(t, "shell-quoting")

	code, out := runMProject(t, projDir, "run", "args", "--", "--mode", "production", "hello-world")
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	raw, err := os.ReadFile(filepath.Join(projDir, "args.out"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := []string{"--mode", "production", "hello-world"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines %v, want %v", len(lines), lines, want)
	}
	for i, w := range want {
		if strings.TrimSpace(lines[i]) != w {
			t.Fatalf("line %d=%q, want %q", i, lines[i], w)
		}
	}
}

func TestRunArgForwardingQuotedSpaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("quoted spaces in forwarded args are platform-gated on Windows")
	}
	skipWithoutNode(t)
	projDir := setupRunFixture(t, "shell-quoting")

	code, out := runMProject(t, projDir, "run", "args", "--", "hello world")
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	raw, err := os.ReadFile(filepath.Join(projDir, "args.out"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Fatalf("got %v, want [hello world]", lines)
	}
}

func TestRunIfPresentMissingScript(t *testing.T) {
	projDir := setupRunFixture(t, "basic-scripts")

	code, out := runMProject(t, projDir, "run", "no-such-script", "--if-present")
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
}

func TestRunChildExitCodePropagates(t *testing.T) {
	skipWithoutNode(t)
	projDir := setupRunFixture(t, "basic-scripts")

	code, out := runMProject(t, projDir, "run", "fail")
	if code != 42 {
		t.Fatalf("exit=%d out=%s, want 42", code, out)
	}
}

func TestRunRegexSelector(t *testing.T) {
	skipWithoutNode(t)
	projDir := setupRunFixture(t, "basic-scripts")

	code, out := runMProject(t, projDir, "run", "/^test:/")
	if code != 0 {
		t.Fatalf("exit=%d out=%s", code, out)
	}
	data, err := os.ReadFile(filepath.Join(projDir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	want := "test:a\ntest:b"
	if got != want {
		t.Fatalf("order %q, want %q", got, want)
	}
}

func TestRunSignalCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal cancel is best-effort on Windows")
	}
	skipWithoutNode(t)
	projDir := setupRunFixture(t, "signals")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var code int
	var out string
	go func() {
		code, out = runMProjectCtx(t, ctx, projDir, "run", "hang")
		close(done)
	}()

	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for cancelled run")
	}
	if code != 130 {
		t.Fatalf("exit=%d out=%s, want 130", code, out)
	}
}
