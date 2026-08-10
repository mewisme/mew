package watch

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mewisme/mew/internal/trace"
)

// DefaultDebounceInterval is the quiet period before a restart after file changes.
const DefaultDebounceInterval = 200 * time.Millisecond

// RestartFunc starts a child process and blocks until it exits.
// ctx is cancelled to request graceful termination. The function must
// guarantee that the child process tree is terminated and reaped before
// returning. Callers rely on this contract to sequence generations.
type RestartFunc func(ctx context.Context) (int, error)

// SupervisorOptions configures the watch-restart loop.
type SupervisorOptions struct {
	Watcher          Watcher
	WatchPaths       []string // used when Graph is nil (backward compat)
	Restart          RestartFunc
	ClearScreen      bool
	DebounceInterval time.Duration
	OnRestart        func(reason string)

	// Graph is the logical dependency graph. When non-nil the supervisor
	// registers Graph.WatchPaths(), filters events through
	// Graph.ShouldTrigger, and reconciles coverage after each child
	// exit via ReconcilePaths.
	Graph *DependencyGraph
	// ReconcilePaths returns canonical paths to add/remove after a
	// child exits. code is the child's exit code; return nil,nil to
	// keep current coverage (e.g. preserve dependencies on failure).
	ReconcilePaths func(code int) (add, remove []string)
}

// Supervisor runs the watch-restart loop.
type Supervisor struct {
	opts SupervisorOptions
}

// NewSupervisor creates a new supervisor.
func NewSupervisor(opts SupervisorOptions) *Supervisor {
	if opts.DebounceInterval <= 0 {
		opts.DebounceInterval = DefaultDebounceInterval
	}
	return &Supervisor{opts: opts}
}

// Run starts the watch-restart loop. Blocks until ctx is cancelled or
// an unrecoverable error occurs.
//
// Shutdown guarantee: the supervisor never starts generation N+1 until
// generation N's RestartFunc has returned, confirming the child process
// tree is terminated and reaped. A timeout is never treated as
// successful child shutdown.
func (s *Supervisor) Run(ctx context.Context) (int, error) {
	w := s.opts.Watcher
	if w == nil {
		return 1, fmt.Errorf("watch: nil watcher")
	}

	trace.Emit(ctx, trace.CatWatch, trace.TypeWatchStart, trace.WatchData{
		Backend: string(w.Backend()),
	})

	// Register initial watch paths. Fail if any required path cannot be
	// watched rather than silently continuing with partial coverage.
	g := s.opts.Graph
	var paths []string
	if g != nil {
		paths = g.WatchPaths()
	} else {
		paths = s.opts.WatchPaths
	}
	var addErrs []error
	for _, p := range paths {
		if err := w.Add(p); err != nil {
			addErrs = append(addErrs, fmt.Errorf("cannot watch %s: %w", p, err))
		}
	}
	if len(addErrs) > 0 {
		// Report all failures before returning.
		for _, e := range addErrs {
			fmt.Fprintf(os.Stderr, "watch: %v\n", e)
		}
		return 1, fmt.Errorf("watch: %d path registration failures", len(addErrs))
	}

	// watcherDone signals that the watcher's event or error channels
	// have closed unexpectedly (watcher failure).
	watcherDone := make(chan struct{})
	var closeWatcherOnce sync.Once
	closeWatcherDone := func() { closeWatcherOnce.Do(func() { close(watcherDone) }) }
	go func() {
		for err := range w.Errors() {
			if err != nil {
				fmt.Fprintf(os.Stderr, "watch: %v\n", err)
			}
		}
		// Error channel closed; signal watcher failure.
		closeWatcherDone()
	}()

	eventCh := w.Events()
	debounce := s.opts.DebounceInterval

	var mu sync.Mutex
	var debounceTimer *time.Timer
	triggerRestart := make(chan struct{}, 1)

	notifyChange := func() {
		select {
		case triggerRestart <- struct{}{}:
		default:
		}
	}

	// Consume events in background, feeding debounce.
	// When a graph is active, filter events through ShouldTrigger.
	// When the event channel closes unexpectedly, signal watcherDone.
	go func() {
		for evt := range eventCh {
			if g != nil && !g.ShouldTrigger(evt.Path) {
				continue
			}
			trace.Emit(ctx, trace.CatWatch, trace.TypeWatchInvalidate, trace.WatchData{
				ChangedPath: evt.Path,
			})
			mu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounce, notifyChange)
			mu.Unlock()
		}
		closeWatcherDone()
	}()

	type childResult struct {
		code int
		err  error
	}

	lastCode := 0
	generation := 0
	for {
		if s.opts.ClearScreen {
			fmt.Fprint(os.Stderr, "\033[2J\033[H")
		}
		if s.opts.OnRestart != nil {
			s.opts.OnRestart("starting")
		}

		trace.Emit(ctx, trace.CatWatch, trace.TypeWatchChildStart, trace.WatchData{
			Generation: generation,
		})

		childCtx, cancelChild := context.WithCancel(ctx)

		childDone := make(chan childResult, 1)
		go func() {
			code, err := s.opts.Restart(childCtx)
			childDone <- childResult{code, err}
		}()

		select {
		case <-triggerRestart:
			// Don't restart if the parent context is already done.
			select {
			case <-ctx.Done():
				cancelChild()
				<-childDone
				return lastCode, ctx.Err()
			default:
			}
			if s.opts.OnRestart != nil {
				s.opts.OnRestart("file changed")
			}
			trace.Emit(ctx, trace.CatWatch, trace.TypeWatchRestart, trace.WatchData{
				Reason:     "file changed",
				Generation: generation,
			})
			cancelChild()
			result := <-childDone
			lastCode = result.code
			if result.err != nil && result.err != context.Canceled {
				fmt.Fprintf(os.Stderr, "watch: %v\n", result.err)
				return lastCode, result.err
			}
			s.reconcile(g, lastCode, w)
			generation++

		case result := <-childDone:
			cancelChild()
			lastCode = result.code
			exitCode := lastCode
			trace.Emit(ctx, trace.CatWatch, trace.TypeWatchChildExit, trace.WatchData{
				ExitCode:   &exitCode,
				Generation: generation,
			})
			if result.err != nil && result.err != context.Canceled {
				fmt.Fprintf(os.Stderr, "watch: %v\n", result.err)
				return lastCode, result.err
			}
			s.reconcile(g, lastCode, w)
			if s.opts.OnRestart != nil {
				s.opts.OnRestart(fmt.Sprintf("child exited (code %d)", result.code))
			}
			select {
			case <-triggerRestart:
			case <-watcherDone:
				trace.Emit(ctx, trace.CatWatch, trace.TypeWatchShutdown, trace.WatchData{
					Reason: "watcher channel closed",
				})
				return lastCode, fmt.Errorf("watch: watcher event channel closed")
			case <-ctx.Done():
				trace.Emit(ctx, trace.CatWatch, trace.TypeWatchShutdown, trace.WatchData{
					Reason: "context cancelled",
				})
				return lastCode, ctx.Err()
			}
			generation++

		case <-watcherDone:
			cancelChild()
			<-childDone
			trace.Emit(ctx, trace.CatWatch, trace.TypeWatchShutdown, trace.WatchData{
				Reason: "watcher channel closed unexpectedly",
			})
			return lastCode, fmt.Errorf("watch: watcher channel closed unexpectedly")

		case <-ctx.Done():
			cancelChild()
			<-childDone
			trace.Emit(ctx, trace.CatWatch, trace.TypeWatchShutdown, trace.WatchData{
				Reason: "context cancelled",
			})
			return lastCode, ctx.Err()
		}

		// Drain any queued restart triggers before next iteration.
		select {
		case <-triggerRestart:
		default:
		}
	}
}

// reconcile applies graph coverage updates after a child exits.
func (s *Supervisor) reconcile(g *DependencyGraph, code int, w Watcher) {
	if g == nil || s.opts.ReconcilePaths == nil {
		return
	}
	add, remove := s.opts.ReconcilePaths(code)
	for _, p := range remove {
		if err := w.Remove(p); err != nil {
			fmt.Fprintf(os.Stderr, "watch: cannot unwatch %s: %v\n", p, err)
		}
	}
	for _, p := range add {
		if err := w.Add(p); err != nil {
			fmt.Fprintf(os.Stderr, "watch: cannot watch %s: %v\n", p, err)
		}
	}
}
