package process

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const defaultGracePeriod = 10 * time.Second

// ExecSupervisor is the production ProcessSupervisor using os/exec.
type ExecSupervisor struct {
	// GracePeriod is how long to wait after a graceful cancellation signal
	// before force-killing the process tree. Zero means use defaultGracePeriod.
	GracePeriod time.Duration
}

func (s *ExecSupervisor) gracePeriod() time.Duration {
	if s == nil || s.GracePeriod <= 0 {
		return defaultGracePeriod
	}
	return s.GracePeriod
}

// NewExecSupervisor returns a restricted-execution process supervisor.
func NewExecSupervisor() *ExecSupervisor {
	return &ExecSupervisor{}
}

type execHandle struct {
	cmd *exec.Cmd
}

// Start launches a child process. The context is not wired into the child via
// CommandContext — cancellation is handled exclusively in Wait() with a grace period.
func (s *ExecSupervisor) Start(_ context.Context, spec Spec) (*Handle, error) {
	if s == nil {
		return nil, errors.New("nil supervisor")
	}
	path, args, err := resolveCommand(spec)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdin = spec.Stdin
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	attr := &syscall.SysProcAttr{}
	setProcessGroup(attr)
	cmd.SysProcAttr = attr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Handle{PID: cmd.Process.Pid, raw: &execHandle{cmd: cmd}}, nil
}

// Wait waits for a started process. When ctx is cancelled, it sends a graceful
// signal, waits up to GracePeriod, then force-kills the process tree and reaps.
// Returns the child ExitError for normal non-zero exits, or ctx.Err() on cancellation.
func (s *ExecSupervisor) Wait(ctx context.Context, h *Handle) error {
	if h == nil || h.raw == nil {
		return errors.New("invalid handle")
	}
	eh, ok := h.raw.(*execHandle)
	if !ok || eh.cmd == nil {
		return errors.New("invalid handle type")
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- eh.cmd.Wait()
	}()

	select {
	case err := <-waitDone:
		// Clean up orphaned grandchildren that inherited the process group.
		killProcessTree(eh.cmd)
		if err == nil {
			return nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			return &ExitError{Code: code, Err: err}
		}
		return err
	case <-ctx.Done():
		// Graceful signal first
		forwardCancelSignal(eh.cmd)

		// Wait for graceful exit or timeout
		graceTimer := time.NewTimer(s.gracePeriod())
		select {
		case err := <-waitDone:
			graceTimer.Stop()
			if err == nil {
				return nil
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return &ExitError{Code: exitErr.ExitCode(), Err: err}
			}
			return err
		case <-graceTimer.C:
			// Grace period expired — force kill
		}

		killProcessTree(eh.cmd)
		// Reap the killed process
		<-waitDone
		return ctx.Err()
	}
}

// ExitError is a non-zero child exit.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *ExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ExitCode extracts an exit code from err when available.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *ExitError
	if errors.As(err, &ee) && ee != nil {
		return ee.Code
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr != nil {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
		return exitErr.ExitCode()
	}
	return 1
}

func resolveCommand(spec Spec) (string, []string, error) {
	cmd := strings.TrimSpace(spec.Path)
	if cmd == "" {
		return "", nil, errors.New("empty command")
	}
	if len(spec.Args) > 0 {
		return spec.Path, spec.Args, nil
	}
	if runtime.GOOS == "windows" {
		shell := strings.TrimSpace(spec.Shell)
		if shell == "" {
			if s, ok := lookupEnv(spec.Env, "ComSpec"); ok && s != "" {
				shell = s
			}
		}
		if shell == "" {
			shell = "cmd.exe"
		}
		return shell, []string{"/d", "/s", "/c", cmd}, nil
	}
	return "sh", []string{"-c", cmd}, nil
}

func envKey(kv string) string {
	if i := strings.IndexByte(kv, '='); i >= 0 {
		return kv[:i]
	}
	return kv
}

func lookupEnv(env []string, key string) (string, bool) {
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			if strings.EqualFold(kv[:i], key) {
				return kv[i+1:], true
			}
		}
	}
	return "", false
}

func shouldStripEnv(key string) bool {
	u := strings.ToUpper(key)
	switch u {
	case "NPM_TOKEN", "NODE_AUTH_TOKEN", "GITHUB_TOKEN", "GH_TOKEN",
		"GITLAB_TOKEN", "NPM_CONFIG__AUTH", "SSH_AUTH_SOCK", "DOCKER_HOST":
		return true
	}
	if strings.HasPrefix(u, "AWS_") || strings.HasPrefix(u, "AZURE_") || strings.HasPrefix(u, "GOOGLE_") {
		return true
	}
	if strings.Contains(u, "SECRET") || strings.Contains(u, "PASSWORD") ||
		strings.Contains(u, "TOKEN") || strings.Contains(u, "PRIVATE_KEY") ||
		strings.Contains(u, "PRIVATE-KEY") {
		return true
	}
	return false
}

// ResolveCommandForTest exposes resolveCommand for unit tests.
func ResolveCommandForTest(spec Spec) (string, []string, error) {
	return resolveCommand(spec)
}

// BinDirForPackage returns node_modules/.bin adjacent to packageDir.
func BinDirForPackage(packageDir, nodeModules string) string {
	if nodeModules != "" {
		return filepath.Join(nodeModules, ".bin")
	}
	return filepath.Join(filepath.Dir(packageDir), ".bin")
}
