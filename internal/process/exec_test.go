package process_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mewisme/mew/internal/process"
)

func TestExecSupervisorEcho(t *testing.T) {
	sup := process.NewExecSupervisor()
	dir := t.TempDir()
	spec := process.Spec{
		Path: "echo hello",
		Dir:  dir,
		Env:  process.RestrictedEnv(process.EnvSource{Vars: os.Environ(), Explicit: true}, dir),
	}
	h, err := sup.Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := sup.Wait(context.Background(), h); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictedEnvExplicitEmpty(t *testing.T) {
	t.Setenv("NPM_TOKEN", "leaked")
	env := process.RestrictedEnv(process.EnvSource{Vars: []string{}, Explicit: true}, "/nm/.bin")
	if len(env) != 1 {
		t.Fatalf("want exactly PATH entry, got %v", env)
	}
	if !strings.HasPrefix(env[0], "PATH=") && !strings.HasPrefix(env[0], "Path=") {
		t.Fatalf("want PATH prefix, got %q", env[0])
	}
	for _, kv := range env {
		if strings.Contains(kv, "leaked") {
			t.Fatal("host env leaked into explicit-empty")
		}
	}
}

func TestRestrictedEnvUnsetFallsBack(t *testing.T) {
	t.Setenv("MEW_TEST_ENV_MARKER", "present")
	env := process.RestrictedEnv(process.EnvSource{Explicit: false}, "/bin")
	found := false
	for _, kv := range env {
		if kv == "MEW_TEST_ENV_MARKER=present" {
			found = true
		}
	}
	if !found {
		t.Fatal("unset EnvSource should inherit host env")
	}
}

func TestRestrictedEnvSecretStripping(t *testing.T) {
	cases := []struct {
		key   string
		strip bool
		keep  string
	}{
		{"NPM_TOKEN", true, ""},
		{"GH_TOKEN", true, ""},
		{"GITLAB_TOKEN", true, ""},
		{"SSH_AUTH_SOCK", true, ""},
		{"DOCKER_HOST", true, ""},
		{"AWS_SECRET_KEY", true, ""},
		{"AZURE_CLIENT_SECRET", true, ""},
		{"GOOGLE_APPLICATION_CREDENTIALS", true, ""},
		{"MY_PRIVATE_KEY", true, ""},
		{"HOME", false, "/tmp"},
		{"NODE_OPTIONS", false, "--max-old-space-size=4096"},
		{"FOO", false, "bar=baz"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			val := "secret"
			if tc.keep != "" {
				val = tc.keep
			}
			env := process.RestrictedEnv(process.EnvSource{
				Vars:     []string{tc.key + "=" + val, "PATH=/bin"},
				Explicit: true,
			}, "/nm/.bin")
			want := tc.key + "=" + val
			got := false
			for _, kv := range env {
				if kv == want {
					got = true
				}
			}
			if tc.strip && got {
				t.Fatalf("%s should be stripped", tc.key)
			}
			if !tc.strip && !got {
				t.Fatalf("%s should be preserved, env=%v", tc.key, env)
			}
		})
	}
}

func TestRestrictedEnvPATHPrefix(t *testing.T) {
	env := process.RestrictedEnv(process.EnvSource{
		Vars:     []string{"PATH=/usr/bin"},
		Explicit: true,
	}, "/nm/.bin")
	pathKey := "PATH"
	if runtime.GOOS == "windows" {
		pathKey = "Path"
	}
	var pathCount int
	for _, kv := range env {
		if strings.HasPrefix(kv, pathKey+"=") {
			pathCount++
			if !strings.Contains(kv, "/nm/.bin") && !strings.Contains(kv, `\nm\.bin`) {
				t.Fatalf("PATH missing binDir prefix: %q", kv)
			}
		}
	}
	if pathCount != 1 {
		t.Fatalf("want exactly one PATH entry, got %d", pathCount)
	}
}

func TestResolveCommandComSpecFromEnv(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	path, args, err := process.ResolveCommandForTest(process.Spec{
		Path: "echo ok",
		Env:  []string{"ComSpec=C:\\custom\\cmd.exe"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != `C:\custom\cmd.exe` {
		t.Fatalf("got shell %q", path)
	}
	if len(args) != 4 || args[0] != "/d" || args[1] != "/s" || args[2] != "/c" {
		t.Fatalf("args=%v", args)
	}
}

func TestExecSupervisorStdioPassthrough(t *testing.T) {
	sup := process.NewExecSupervisor()
	dir := t.TempDir()
	env := process.RestrictedEnv(process.EnvSource{Vars: os.Environ(), Explicit: true}, dir)

	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("stdin-marker\n")
	spec := process.Spec{
		Dir:    dir,
		Env:    env,
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	}
	if runtime.GOOS == "windows" {
		spec.Path = "cmd"
		spec.Args = []string{"/c", "more & echo stderr-marker 1>&2"}
	} else {
		spec.Path = "sh"
		spec.Args = []string{"-c", `read line; printf '%s\n' "$line"; echo stderr-marker 1>&2`}
	}
	h, err := sup.Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := sup.Wait(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "stdin-marker" {
		t.Fatalf("stdout=%q want stdin-marker", got)
	}
	if got := strings.TrimSpace(stderr.String()); got != "stderr-marker" {
		t.Fatalf("stderr=%q want stderr-marker", got)
	}
}

func TestExecSupervisorNilStdioPreservesDefaults(t *testing.T) {
	sup := process.NewExecSupervisor()
	dir := t.TempDir()
	spec := process.Spec{
		Path: "echo hello",
		Dir:  dir,
		Env:  process.RestrictedEnv(process.EnvSource{Vars: os.Environ(), Explicit: true}, dir),
	}
	h, err := sup.Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := sup.Wait(context.Background(), h); err != nil {
		t.Fatal(err)
	}
}

func TestExecSupervisorExitCodePropagation(t *testing.T) {
	sup := process.NewExecSupervisor()
	dir := t.TempDir()
	env := process.RestrictedEnv(process.EnvSource{Vars: os.Environ(), Explicit: true}, dir)
	spec := process.Spec{
		Dir:    dir,
		Env:    env,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	if runtime.GOOS == "windows" {
		spec.Path = "cmd"
		spec.Args = []string{"/c", "exit /b 7"}
	} else {
		spec.Path = "sh"
		spec.Args = []string{"-c", "exit 7"}
	}
	h, err := sup.Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	err = sup.Wait(context.Background(), h)
	if err == nil {
		t.Fatal("want non-zero exit error")
	}
	if process.ExitCode(err) != 7 {
		t.Fatalf("ExitCode=%d want 7", process.ExitCode(err))
	}
	var exitErr *process.ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 7 {
		t.Fatalf("want *process.ExitError code 7, got %T %v", err, err)
	}
}

func TestExecSupervisorCancelForceKills(t *testing.T) {
	sup := process.NewExecSupervisor()
	sup.GracePeriod = 50 * time.Millisecond
	sup.ForceKillTimeout = 2 * time.Second

	dir := t.TempDir()
	env := process.RestrictedEnv(process.EnvSource{Vars: os.Environ(), Explicit: true}, dir)
	spec := process.Spec{
		Dir:    dir,
		Env:    env,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	if runtime.GOOS == "windows" {
		// Windows: cmd /c "timeout /t 60 /nobreak" — sleeps, ignores Ctrl+C.
		spec.Path = "cmd"
		spec.Args = []string{"/c", "timeout /t 60 /nobreak"}
	} else {
		// Unix: trap '' TERM INT; sleep 60 — ignores SIGTERM/SIGINT.
		spec.Path = "sh"
		spec.Args = []string{"-c", "trap '' TERM INT; sleep 60"}
	}

	ctx, cancel := context.WithCancel(context.Background())

	h, err := sup.Start(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}

	// Cancel context — should trigger graceful signal, then force-kill.
	cancel()

	err = sup.Wait(ctx, h)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled after force-kill, got %v", err)
	}
}

func TestExecSupervisorForceKillReapTimeout(t *testing.T) {
	// Start a real process, force-kill it, then verify that Wait returns
	// promptly (not hanging on the reap).
	sup := process.NewExecSupervisor()
	sup.GracePeriod = 10 * time.Millisecond
	sup.ForceKillTimeout = 5 * time.Second

	dir := t.TempDir()
	env := process.RestrictedEnv(process.EnvSource{Vars: os.Environ(), Explicit: true}, dir)
	spec := process.Spec{
		Dir:    dir,
		Env:    env,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	if runtime.GOOS == "windows" {
		spec.Path = "cmd"
		spec.Args = []string{"/c", "timeout /t 60 /nobreak"}
	} else {
		spec.Path = "sh"
		spec.Args = []string{"-c", "trap '' TERM INT; sleep 60"}
	}

	ctx, cancel := context.WithCancel(context.Background())

	h, err := sup.Start(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	cancel()
	err = sup.Wait(ctx, h)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Should complete well within ForceKillTimeout (5s).
	if elapsed > 4*time.Second {
		t.Fatalf("Wait took %v after force-kill, expected prompt reap", elapsed)
	}
}

func TestExecSupervisorErrForceKillFailed(t *testing.T) {
	// Verify ErrForceKillFailed is a distinct sentinel.
	if process.ErrForceKillFailed == nil {
		t.Fatal("ErrForceKillFailed is nil")
	}
	if process.ErrForceKillFailed.Error() == "" {
		t.Fatal("ErrForceKillFailed has no message")
	}
}

func TestExecSupervisorProcessTreeKillUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process-group test")
	}

	sup := process.NewExecSupervisor()
	sup.GracePeriod = 100 * time.Millisecond
	sup.ForceKillTimeout = 5 * time.Second

	dir := t.TempDir()
	env := process.RestrictedEnv(process.EnvSource{Vars: os.Environ(), Explicit: true}, dir)
	// Spawn a child that spawns a grandchild in the same process group.
	// Both must be killed.
	spec := process.Spec{
		Dir:    dir,
		Env:    env,
		Stdout: io.Discard,
		Stderr: io.Discard,
		Path:   "sh",
		Args: []string{"-c", `
			trap '' TERM INT
			sleep 60 &
			wait
		`},
	}

	ctx, cancel := context.WithCancel(context.Background())

	h, err := sup.Start(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	cancel()
	err = sup.Wait(ctx, h)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Both parent and grandchild should be killed promptly.
	if elapsed > 4*time.Second {
		t.Fatalf("process tree kill took %v, expected prompt", elapsed)
	}
}
