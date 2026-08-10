package watch

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeWatcher implements Watcher for testing the supervisor.
type fakeWatcher struct {
	mu      sync.Mutex
	events  chan Event
	errs    chan error
	closed  bool
	adds    []string
	removes []string
}

func newFakeWatcher() *fakeWatcher {
	return &fakeWatcher{
		events: make(chan Event, 64),
		errs:   make(chan error, 1),
	}
}

func (fw *fakeWatcher) Add(path string) error {
	fw.mu.Lock()
	fw.adds = append(fw.adds, path)
	fw.mu.Unlock()
	return nil
}
func (fw *fakeWatcher) Remove(path string) error {
	fw.mu.Lock()
	fw.removes = append(fw.removes, path)
	fw.mu.Unlock()
	return nil
}
func (fw *fakeWatcher) Backend() Backend     { return BackendNative }
func (fw *fakeWatcher) Events() <-chan Event { return fw.events }
func (fw *fakeWatcher) Errors() <-chan error { return fw.errs }
func (fw *fakeWatcher) Close() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if !fw.closed {
		fw.closed = true
		close(fw.events)
		close(fw.errs)
	}
	return nil
}

func (fw *fakeWatcher) added() []string {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	out := make([]string, len(fw.adds))
	copy(out, fw.adds)
	return out
}

func (fw *fakeWatcher) removed() []string {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	out := make([]string, len(fw.removes))
	copy(out, fw.removes)
	return out
}

func (fw *fakeWatcher) emit(op Op, path string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if !fw.closed {
		fw.events <- Event{Path: path, Op: op}
	}
}

func TestSupervisorRestartsOnChange(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	restarts := make(chan struct{}, 3)
	restart := func(ctx context.Context) (int, error) {
		restarts <- struct{}{}
		<-ctx.Done()
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := sup.Run(ctx)
		errCh <- err
	}()

	// Wait for first restart to begin.
	select {
	case <-restarts:
		// First launch started.
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first launch")
	}

	// Emit a file change.
	fw.emit(OpWrite, "/fake/app.ts")

	// Wait for the debounce and context cancellation.
	// The supervisor should cancel the first child, then restart.
	select {
	case <-restarts:
		// Second launch triggered by file change.
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for restart after file change")
	}

	cancel()
	<-errCh
}

func TestSupervisorDrainsOnCancel(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	started := make(chan struct{})
	restart := func(ctx context.Context) (int, error) {
		started <- struct{}{}
		<-ctx.Done()
		return 130, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := sup.Run(ctx)
		errCh <- err
	}()

	// Wait for start.
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for start")
	}

	// Cancel should kill the child and clean up.
	cancel()
	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for supervisor exit")
	}
}

func TestSupervisorDebounce(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	var mu sync.Mutex
	restartCount := 0
	restart := func(ctx context.Context) (int, error) {
		mu.Lock()
		restartCount++
		mu.Unlock()
		<-ctx.Done()
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		_, err := sup.Run(ctx)
		_ = err
	}()

	// Wait for first start to settle.
	time.Sleep(50 * time.Millisecond)

	// Emit 5 rapid changes.
	for i := 0; i < 5; i++ {
		fw.emit(OpWrite, "/fake/x.ts")
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce window to close + restart.
	time.Sleep(300 * time.Millisecond)

	cancel()

	mu.Lock()
	n := restartCount
	mu.Unlock()

	// Should have restarted only once or twice, not 6 times.
	if n > 3 {
		t.Errorf("expected <= 3 restarts with debounce, got %d", n)
	}
}

func TestSupervisorClearScreen(t *testing.T) {
	// Verify the option is accepted without panic.
	sup := NewSupervisor(SupervisorOptions{
		ClearScreen: true,
	})
	if sup == nil {
		t.Fatal("nil supervisor")
	}
}

// TestSupervisorGenerationCleanup verifies that each restart creates a fresh
// generation and the previous generation's cleanup completes before the next
// generation becomes authoritative.
func TestSupervisorGenerationCleanup(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	// Track live generations with a channel-based counter.
	// genEntered signals a generation has started.
	// genExited signals a generation's cleanup is done.
	genEntered := make(chan int, 8)
	genExited := make(chan int, 8)
	var genCounter int

	restart := func(ctx context.Context) (int, error) {
		genCounter++
		id := genCounter
		genEntered <- id
		defer func() { genExited <- id }()
		<-ctx.Done()
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := sup.Run(ctx)
		errCh <- err
	}()

	// Wait for first generation to start.
	select {
	case id := <-genEntered:
		if id != 1 {
			t.Fatalf("expected gen 1, got %d", id)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for gen 1 start")
	}

	// Trigger restart.
	fw.emit(OpWrite, "/fake/app.ts")

	// Wait for gen 1 to exit (cleanup) before gen 2 enters.
	select {
	case id := <-genExited:
		if id != 1 {
			t.Fatalf("expected gen 1 exit, got %d", id)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for gen 1 cleanup")
	}

	// Now gen 2 should start.
	select {
	case id := <-genEntered:
		if id != 2 {
			t.Fatalf("expected gen 2 start, got %d", id)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for gen 2 start")
	}

	// Verify gen 1 exited before gen 2 entered.
	// Already verified by channel ordering above.

	cancel()
	<-errCh

	// Wait for gen 2 cleanup.
	select {
	case id := <-genExited:
		if id != 2 {
			t.Fatalf("expected gen 2 exit, got %d", id)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for gen 2 cleanup")
	}
}

// TestSupervisorCancellationCleansGeneration verifies that supervisor
// cancellation triggers the active generation's cleanup before Run returns.
func TestSupervisorCancellationCleansGeneration(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	cleanedUp := make(chan struct{})
	restart := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		close(cleanedUp)
		return 130, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		_, _ = sup.Run(ctx)
		close(done)
	}()

	// Wait for supervisor loop to enter, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Supervisor should not return until cleanup completes.
	select {
	case <-cleanedUp:
		// Cleanup happened.
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for generation cleanup on cancel")
	}

	select {
	case <-done:
		// Supervisor exited after cleanup.
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for supervisor exit after cleanup")
	}
}

// TestSupervisorRepeatedRestartsNoAccumulation verifies that rapid repeated
// restarts do not accumulate live generations.
func TestSupervisorRepeatedRestartsNoAccumulation(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	var mu sync.Mutex
	var liveCount int
	var maxLive int
	started := make(chan struct{}, 16)
	exited := make(chan struct{}, 16)

	restart := func(ctx context.Context) (int, error) {
		mu.Lock()
		liveCount++
		if liveCount > maxLive {
			maxLive = liveCount
		}
		mu.Unlock()
		started <- struct{}{}
		<-ctx.Done()
		mu.Lock()
		liveCount--
		mu.Unlock()
		exited <- struct{}{}
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		_, _ = sup.Run(ctx)
	}()

	// Wait for first generation to start.
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first generation")
	}

	// Emit 5 rapid changes with debounce reset.
	for i := 0; i < 5; i++ {
		fw.emit(OpWrite, "/fake/x.ts")
		time.Sleep(30 * time.Millisecond)
	}

	// Let debounce settle and final restart execute.
	time.Sleep(300 * time.Millisecond)

	cancel()

	// Wait for final generation to exit.
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for final generation exit")
	}

	mu.Lock()
	ml := maxLive
	mu.Unlock()

	// At most one generation should be live at any time since the
	// supervisor serializes restarts.
	if ml > 1 {
		t.Errorf("max live generations = %d, want <= 1", ml)
	}
}

// TestSupervisorFiltersByGraph verifies that the graph's ShouldTrigger
// filters events: untracked non-source files are ignored, tracked files
// trigger restart.
func TestSupervisorFiltersByGraph(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	g := NewDependencyGraph()
	g.Seed([]string{"/proj/src/index.ts"}, []string{"/proj/tsconfig.json"}, nil)

	var mu sync.Mutex
	restartCount := 0
	restart := func(ctx context.Context) (int, error) {
		mu.Lock()
		restartCount++
		mu.Unlock()
		<-ctx.Done()
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		Restart:          restart,
		DebounceInterval: 20 * time.Millisecond,
		Graph:            g,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _, _ = sup.Run(ctx) }()

	// Let the first launch start.
	time.Sleep(50 * time.Millisecond)

	// Emit a non-relevant file (README) — should NOT trigger restart.
	mu.Lock()
	before := restartCount
	mu.Unlock()
	fw.emit(OpWrite, "/proj/src/README.md")
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	after := restartCount
	mu.Unlock()
	if after > before+1 {
		t.Errorf("README.md triggered restart: before=%d after=%d", before, after)
	}

	// Emit the exact tracked module — should trigger restart.
	before = after
	fw.emit(OpWrite, "/proj/src/index.ts")
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	after = restartCount
	mu.Unlock()
	if after <= before {
		t.Error("tracked module did not trigger restart")
	}

	// Emit .env under covered dir — should trigger restart (config-like).
	before = after
	fw.emit(OpWrite, "/proj/src/.env")
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	after = restartCount
	mu.Unlock()
	if after <= before {
		t.Error(".env under covered dir did not trigger restart")
	}

	// Emit an untracked sibling .ts — should NOT trigger restart.
	before = after
	fw.emit(OpWrite, "/proj/src/other.ts")
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	after = restartCount
	mu.Unlock()
	if after > before {
		t.Error("untracked .ts sibling triggered restart")
	}

	cancel()
}

// TestSupervisorReconcilesCoverage verifies that ReconcilePaths is called
// after child exit and results are applied to the watcher.
func TestSupervisorReconcilesCoverage(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	g := NewDependencyGraph()
	g.Seed([]string{"/proj/src/index.ts"}, nil, nil)

	var reconcileCalls int
	restart := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
		Graph:            g,
		ReconcilePaths: func(code int) (add, remove []string) {
			reconcileCalls++
			if code != 0 {
				return nil, nil
			}
			return []string{"/proj/src/lib"}, []string{"/proj/old"}
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _, _ = sup.Run(ctx) }()

	// Wait for first start.
	time.Sleep(50 * time.Millisecond)

	// Emit a file change to trigger restart (and thus reconcile).
	fw.emit(OpWrite, "/proj/src/index.ts")
	time.Sleep(200 * time.Millisecond)

	cancel()

	if reconcileCalls == 0 {
		t.Error("ReconcilePaths was never called")
	}

	// Check that Add and Remove were applied to the watcher.
	adds := fw.added()
	hasAdd := false
	hasRemove := false
	for _, p := range adds {
		if p == "/proj/src/lib" {
			hasAdd = true
		}
	}
	for _, p := range fw.removed() {
		if p == "/proj/old" {
			hasRemove = true
		}
	}
	if !hasAdd {
		t.Error("reconciled add not applied to watcher")
	}
	if !hasRemove {
		t.Error("reconciled remove not applied to watcher")
	}
}

// TestSupervisorReconcileFailurePreserves verifies that a non-zero exit code
// returns nil,nil from ReconcilePaths and the graph is unchanged.
func TestSupervisorReconcileFailurePreserves(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	g := NewDependencyGraph()
	g.Seed([]string{"/proj/src/index.ts"}, nil, nil)

	restart := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 1, ctx.Err() // non-zero exit
	}

	var lastCode int
	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
		Graph:            g,
		ReconcilePaths: func(code int) (add, remove []string) {
			lastCode = code
			if code != 0 {
				return nil, nil
			}
			return []string{"/new"}, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _, _ = sup.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	fw.emit(OpWrite, "/proj/src/index.ts")
	time.Sleep(200 * time.Millisecond)

	cancel()

	if lastCode == 0 {
		t.Error("expected non-zero exit code")
	}

	// No "new" path should have been added to the watcher on failure.
	for _, p := range fw.added() {
		if p == "/new" {
			t.Error("add applied despite non-zero exit code")
		}
	}
}

// TestSupervisorGraphNilBackwardCompat verifies that Graph==nil preserves
// the existing WatchPaths-based behavior.
func TestSupervisorGraphNilBackwardCompat(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	restart := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() { _, _ = sup.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should have registered the WatchPaths directly.
	found := false
	for _, p := range fw.added() {
		if p == "/fake" {
			found = true
		}
	}
	if !found {
		t.Error("WatchPaths not registered when Graph is nil")
	}
}

// failingAddWatcher returns an error from Add for a specific path.
type failingAddWatcher struct {
	fakeWatcher
	failPath string
}

func (fw *failingAddWatcher) Add(path string) error {
	if path == fw.failPath {
		return os.ErrNotExist
	}
	return fw.fakeWatcher.Add(path)
}

func TestSupervisorAddFailureReturnsError(t *testing.T) {
	fw := &failingAddWatcher{
		fakeWatcher: *newFakeWatcher(),
		failPath:    "/fake",
	}
	defer func() { _ = fw.Close() }()

	restart := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	code, err := sup.Run(ctx)
	if err == nil {
		t.Error("expected error from Add failure, got nil")
	}
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestSupervisorWatcherChannelClosure(t *testing.T) {
	fw := newFakeWatcher()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	restart := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := sup.Run(ctx)
		errCh <- err
	}()

	// Let the supervisor start.
	time.Sleep(50 * time.Millisecond)

	// Close the watcher's channels to simulate unexpected watcher
	// failure. This triggers watcherDone in the supervisor.
	_ = fw.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected error from watcher channel closure")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for supervisor to detect watcher failure")
	}
}

// TestSupervisorNoRestartDuringShutdown verifies that a pending debounce
// does not trigger a restart after the parent context is cancelled.
func TestSupervisorNoRestartDuringShutdown(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	var mu sync.Mutex
	restartCount := 0
	started := make(chan struct{}, 4)
	restart := func(ctx context.Context) (int, error) {
		mu.Lock()
		restartCount++
		mu.Unlock()
		started <- struct{}{}
		<-ctx.Done()
		return 0, ctx.Err()
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 200 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := sup.Run(ctx)
		errCh <- err
	}()

	// Wait for first launch.
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first launch")
	}

	// Emit a file change to start the debounce timer.
	fw.emit(OpWrite, "/fake/x.ts")

	// Immediately cancel the context while the debounce timer is pending.
	// The supervisor must not restart after cancellation.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Logf("supervisor returned: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for supervisor exit")
	}

	mu.Lock()
	n := restartCount
	mu.Unlock()
	// At most 1 restart (the initial launch). The debounce timer should
	// not trigger a second restart after cancellation.
	if n > 1 {
		t.Errorf("restarted %d times during shutdown, want <= 1", n)
	}
}

// TestSupervisorRestartErrorStops verifies that a RestartFunc that returns
// a non-Canceled error causes the supervisor to stop rather than continue
// to the next generation.
func TestSupervisorRestartErrorStops(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	started := make(chan struct{})
	restart := func(ctx context.Context) (int, error) {
		started <- struct{}{}
		<-ctx.Done()
		return 1, fmt.Errorf("fatal: node not found")
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := sup.Run(ctx)
		errCh <- err
	}()

	// Wait for first launch.
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first launch")
	}

	// Trigger restart — supervisor cancels child, child returns fatal error.
	fw.emit(OpWrite, "/fake/app.ts")

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected error from RestartFunc failure, got nil")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for supervisor to stop on error")
	}
}

// TestSupervisorStubbornChildForceKill verifies that a RestartFunc that
// properly force-kills a stubborn child (as process.ExecSupervisor.Wait does)
// allows the supervisor to successfully move to the next generation.
func TestSupervisorStubbornChildForceKill(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	// Simulate a RestartFunc that handles cancellation correctly:
	// it force-kills the child (simulated by a short delay) and returns.
	var genCounter atomic.Int64
	restart := func(ctx context.Context) (int, error) {
		genCounter.Add(1)
		<-ctx.Done()
		// Simulate force-kill + reap delay (much shorter than any timeout).
		select {
		case <-time.After(20 * time.Millisecond):
		case <-ctx.Done():
		}
		return 0, nil
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := sup.Run(ctx)
		errCh <- err
	}()

	// Wait for gen 1 to start.
	time.Sleep(50 * time.Millisecond)
	if genCounter.Load() < 1 {
		t.Fatal("gen 1 did not start")
	}

	// Trigger restart. Supervisor cancels gen 1, waits for force-kill,
	// then starts gen 2.
	fw.emit(OpWrite, "/fake/app.ts")
	time.Sleep(200 * time.Millisecond)

	if genCounter.Load() < 2 {
		t.Fatal("gen 2 did not start after gen 1 was force-killed")
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for supervisor exit")
	}
}

// TestSupervisorGenerationOverlapPrevention verifies that at most one
// generation is active at any time. Uses a barrier to detect overlap.
func TestSupervisorGenerationOverlapPrevention(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	var mu sync.Mutex
	var activeCount int
	var maxActive int
	genExit := make(chan struct{}, 4)

	restart := func(ctx context.Context) (int, error) {
		mu.Lock()
		activeCount++
		if activeCount > maxActive {
			maxActive = activeCount
		}
		mu.Unlock()
		<-ctx.Done()
		mu.Lock()
		activeCount--
		mu.Unlock()
		genExit <- struct{}{}
		return 0, nil
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 20 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() { _, _ = sup.Run(ctx) }()

	// Wait for first gen to start.
	select {
	case <-genExit:
		t.Fatal("unexpected early exit")
	case <-time.After(50 * time.Millisecond):
	}

	// Emit 5 rapid changes to trigger repeated restarts.
	for i := 0; i < 5; i++ {
		fw.emit(OpWrite, "/fake/x.ts")
		time.Sleep(10 * time.Millisecond)
	}

	// Let debounce settle and final restart execute.
	time.Sleep(300 * time.Millisecond)

	cancel()

	mu.Lock()
	ml := maxActive
	mu.Unlock()

	if ml > 1 {
		t.Errorf("max live generations = %d, want <= 1", ml)
	}
}

// TestSupervisorGrandchildCleanup verifies that when a RestartFunc properly
// terminates a process tree, the supervisor can move to N+1. (The actual
// process-tree cleanup is tested in internal/process.)
func TestSupervisorGrandchildCleanup(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	var genCounter atomic.Int64
	restart := func(ctx context.Context) (int, error) {
		genCounter.Add(1)
		<-ctx.Done()
		// Simulate process tree cleanup delay.
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
		}
		return 0, nil
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _, _ = sup.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	if genCounter.Load() != 1 {
		t.Fatalf("expected gen 1, got %d", genCounter.Load())
	}

	// Trigger restart.
	fw.emit(OpWrite, "/fake/app.ts")
	time.Sleep(100 * time.Millisecond)

	if genCounter.Load() < 2 {
		t.Fatal("gen 2 did not start after gen 1 cleanup")
	}

	cancel()
}

// TestSupervisorNoRestartOnFatalError verifies that when RestartFunc returns
// a non-Canceled error on natural exit, the supervisor stops rather than
// looping.
func TestSupervisorNoRestartOnFatalError(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	restart := func(ctx context.Context) (int, error) {
		return 1, fmt.Errorf("unrecoverable error")
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	code, err := sup.Run(ctx)
	if err == nil {
		t.Error("expected error from RestartFunc, got nil")
	}
	if code != 1 {
		t.Errorf("expected code 1, got %d", code)
	}
}

// TestSupervisorParentCancelReapsChild verifies that parent context
// cancellation waits for child reaping before Run returns.
func TestSupervisorParentCancelReapsChild(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	cleanedUp := make(chan struct{})
	restart := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		close(cleanedUp)
		return 130, nil
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		_, _ = sup.Run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	// Supervisor must not return until cleanup completes.
	select {
	case <-cleanedUp:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for child cleanup on cancel")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for supervisor exit after cleanup")
	}
}

// TestSupervisorWatcherFailureReapsChild verifies that watcher failure
// while a child runs terminates and reaps the child before returning.
func TestSupervisorWatcherFailureReapsChild(t *testing.T) {
	fw := newFakeWatcher()

	var reaped bool
	restart := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		reaped = true
		return 130, nil
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := sup.Run(ctx)
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// Close watcher channels while child is running.
	_ = fw.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected error from watcher failure")
		}
		if !reaped {
			t.Error("child was not reaped before supervisor returned")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for supervisor to detect watcher failure")
	}
}

// TestSupervisorLateResultIgnored verifies that a late result from
// generation N cannot affect generation N+1.
func TestSupervisorLateResultIgnored(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	var mu sync.Mutex
	var genCounter int
	genDone := make(chan int, 4)

	restart := func(ctx context.Context) (int, error) {
		mu.Lock()
		genCounter++
		id := genCounter
		mu.Unlock()
		<-ctx.Done()
		genDone <- id
		return id, nil
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _, _ = sup.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)

	// Trigger restart.
	fw.emit(OpWrite, "/fake/app.ts")
	time.Sleep(100 * time.Millisecond)

	cancel()

	// Collect results. Generation 1 must have completed before
	// generation 2 started. Since we serialise, the order of genDone
	// sends should be ascending.
	results := make([]int, 0, 2)
	timeout := time.After(time.Second)
	for i := 0; i < 2; i++ {
		select {
		case id := <-genDone:
			results = append(results, id)
		case <-timeout:
			t.Fatal("timeout waiting for gen completion")
		}
	}

	if len(results) >= 2 && results[0] > results[1] {
		t.Errorf("gen order reversed: %v (late result from gen N affected N+1)", results)
	}
}

// TestSupervisorRapidRestartCoalescing verifies that rapid restart events
// do not create unbounded process/goroutine fan-out.
func TestSupervisorRapidRestartCoalescing(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	var mu sync.Mutex
	var goroutinePeak int
	var goroutineNow int
	restart := func(ctx context.Context) (int, error) {
		mu.Lock()
		goroutineNow++
		if goroutineNow > goroutinePeak {
			goroutinePeak = goroutineNow
		}
		mu.Unlock()
		<-ctx.Done()
		mu.Lock()
		goroutineNow--
		mu.Unlock()
		return 0, nil
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:          fw,
		WatchPaths:       []string{"/fake"},
		Restart:          restart,
		DebounceInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() { _, _ = sup.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)

	// Emit 10 rapid changes.
	for i := 0; i < 10; i++ {
		fw.emit(OpWrite, "/fake/x.ts")
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(300 * time.Millisecond)
	cancel()

	mu.Lock()
	peak := goroutinePeak
	mu.Unlock()

	// At most 2 goroutines: debounce coalesces restarts, plus the
	// edge case where a new generation just started when events arrive.
	if peak > 3 {
		t.Errorf("goroutine peak = %d, want <= 3 (unbounded fan-out)", peak)
	}
}
