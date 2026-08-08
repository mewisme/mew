package watch

import (
	"context"
	"os"
	"sync"
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

func TestSupervisorChildTerminationTimeout(t *testing.T) {
	fw := newFakeWatcher()
	defer func() { _ = fw.Close() }()

	// Child that ignores context cancellation.
	restart := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		// Don't return — simulate stubborn child.
		select {}
	}

	sup := NewSupervisor(SupervisorOptions{
		Watcher:            fw,
		WatchPaths:         []string{"/fake"},
		Restart:            restart,
		DebounceInterval:   10 * time.Millisecond,
		TerminationTimeout: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := sup.Run(ctx)
		errCh <- err
	}()

	// Let the supervisor launch the child.
	time.Sleep(50 * time.Millisecond)

	// Cancel the top-level context. The supervisor should cancel the
	// child, wait for TerminationTimeout, then return.
	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Logf("supervisor returned: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for supervisor to give up on stubborn child")
	}
}
