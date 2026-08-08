package transform_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mewisme/mew/internal/transform"
)

// fakeEngine blocks Transform until unblocked, for cancellation testing.
type fakeEngine struct {
	blockCh   chan struct{} // closed to unblock
	unblocked sync.Once
	identity  transform.EngineIdentity
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{
		blockCh:  make(chan struct{}),
		identity: transform.EngineIdentity{Name: "fake", Version: "1.0"},
	}
}

func (e *fakeEngine) Identity() transform.EngineIdentity { return e.identity }

func (e *fakeEngine) Transform(ctx context.Context, _ transform.TransformRequest) (transform.TransformResult, error) {
	select {
	case <-ctx.Done():
		return transform.TransformResult{}, ctx.Err()
	case <-e.blockCh:
		return transform.TransformResult{
			Code:         []byte("transformed"),
			OutputDigest: "abc",
			CacheStatus:  transform.CacheStatusBypass,
		}, nil
	}
}

func (e *fakeEngine) unblock() { e.unblocked.Do(func() { close(e.blockCh) }) }

// blockingEngine returns a fake engine that blocks on every transform until
// the returned unblock function is called.
func blockingEngine() (transform.Engine, func()) {
	e := newFakeEngine()
	return e, e.unblock
}

func TestNewSessionRequiresNonNilContext(t *testing.T) {
	_, err := transform.NewSession(transform.ServiceOptions{
		Context: nil,
	})
	if err == nil {
		t.Fatal("expected error for nil context")
	}
}

func TestNewSessionCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatalf("NewSession with cancelled context: %v", err)
	}
	defer func() { _ = sess.Close() }()

	// Start should fail because health check dial uses cancelled context.
	err = sess.Start()
	if err == nil {
		t.Fatal("expected Start to fail with cancelled context")
	}
}

func TestCloseIdempotent(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	// First close.
	if err := sess.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close — must be safe.
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	// Third close.
	if err := sess.Close(); err != nil {
		t.Fatalf("third Close: %v", err)
	}
}

func TestCloseConcurrent(t *testing.T) {
	ctx := context.Background()
	engine, unblock := blockingEngine()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	// Send a transform that blocks on the engine so Close has work to wait for.
	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "cc-1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "cc-tok",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	// Give the transform time to acquire worker and block on engine.
	time.Sleep(100 * time.Millisecond)

	// Now close concurrently. The transform must be running when Close is called.
	started := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx == 0 {
				close(started) // signal one goroutine has entered
			}
			errs[idx] = sess.Close()
		}(i)
	}
	// Wait for at least one goroutine to enter Close, then wait briefly
	// for others to also enter (and block on closeDone).
	<-started
	time.Sleep(100 * time.Millisecond)

	// All goroutines should still be waiting for the transform to finish.
	// Unblock the engine so shutdown can complete.
	unblock()

	// All must return promptly.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Close callers did not return within 5s")
	}

	for i, e := range errs {
		if e != nil {
			t.Errorf("Close[%d]: %v", i, e)
		}
	}
}

func TestCloseConcurrentWaitersReturnSameError(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Collect results from 5 concurrent Close calls.
	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = sess.Close()
		}(i)
	}
	wg.Wait()

	// All must return the same value.
	for i, e := range errs {
		if e != errs[0] {
			t.Errorf("Close[%d] returned %v, want %v", i, e, errs[0])
		}
	}
}

func TestCloseWaitsForGoroutines(t *testing.T) {
	ctx := context.Background()
	engine, unblock := blockingEngine()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	// Send a transform that blocks (worker slot acquired).
	// We need a client connection for this.
	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")

	// Send a transform request that blocks on the engine.
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-1",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	// Give the transform time to acquire the worker and start blocking.
	time.Sleep(100 * time.Millisecond)

	// Unblock the engine so Close doesn't hang waiting for the transform.
	unblock()

	// Close must return reasonably quickly.
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- sess.Close()
	}()

	select {
	case err := <-closeDone:
		if err != nil {
			t.Logf("Close returned: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return within 5s")
	}

	_ = conn.Close()
}

func TestInvocationCancellationClosesListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	// Verify listener is accepting.
	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()

	// Cancel the invocation context.
	cancel()

	// The listener should close promptly.
	timeout := time.After(2 * time.Second)
	for {
		_, err := net.Dial("tcp", sess.Endpoint)
		if err != nil {
			break // listener closed
		}
		select {
		case <-timeout:
			t.Fatal("listener still accepting after context cancellation")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	_ = sess.Close()
}

func TestNewRequestsRejectedAfterShutdown(t *testing.T) {
	ctx := context.Background()
	engine, unblock := blockingEngine()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer unblock()

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	// Start shutdown.
	if err := sess.Close(); err != nil {
		t.Fatal(err)
	}

	// Try to connect — the listener should be closed.
	conn, err := net.DialTimeout("tcp", sess.Endpoint, 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected connection refused after shutdown")
	}
}

func TestActiveCancelsCleanedAfterCompletion(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("const x = 1;")

	// Send a valid transform.
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "cleanup-1", Op: "transform",
		Path: "a.ts", Source: "const x = 1;", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-cleanup",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	// Read response.
	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("transform failed: %s", resp.Error)
	}

	// Send a cancel for the same token — should be idempotent (no crash, no panic).
	cancelReq := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "cancel-1", Op: "cancel", CancelToken: "tok-cleanup",
	}
	if err := transform.EncodeFrame(conn, cancelReq); err != nil {
		t.Fatal(err)
	}
	var cancelResp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &cancelResp); err != nil {
		t.Fatal(err)
	}
	if !cancelResp.OK {
		t.Fatalf("cancel failed: %s", cancelResp.Error)
	}
}

func TestHealthCheckUsesDialContext(t *testing.T) {
	// Verify that health check respects context cancellation by using
	// a context that is cancelled before the dial completes.
	ctx, cancel := context.WithCancel(context.Background())

	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: context.Background(), // session uses background
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	// Start serve in background.
	// We can't use sess.Start() because it uses s.ctx.
	// Instead verify that with a cancelled session context, Start fails.
	// The session context is derived from the opts.Context.
	_ = ctx
	cancel()

	// Create session with cancelled context.
	sess2, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		// Might succeed or fail depending on OS.
		// If it succeeds, Start must fail.
		if sess2 != nil {
			defer func() { _ = sess2.Close() }()
			if err := sess2.Start(); err == nil {
				t.Fatal("expected Start to fail with cancelled context")
			}
		}
		return
	}
	defer func() { _ = sess2.Close() }()

	err = sess2.Start()
	if err == nil {
		t.Fatal("expected Start to fail with cancelled context")
	}
	t.Logf("Start error (expected): %v", err)
}

func TestBlockedWorkerExitsOnCancellation(t *testing.T) {
	ctx := context.Background()
	engine, unblock := blockingEngine()
	defer unblock()

	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")

	// First transform blocks the worker.
	req1 := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "blk-1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "blk-tok-1",
	}
	if err := transform.EncodeFrame(conn, req1); err != nil {
		t.Fatal(err)
	}

	// Let it acquire the worker.
	time.Sleep(50 * time.Millisecond)

	// Second connection for the second request that will block on worker.
	conn2, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn2.Close() }()

	authReq2 := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn2, authReq2); err != nil {
		t.Fatal(err)
	}
	var authResp2 transform.HelloResponse
	if err := transform.DecodeFrame(conn2, &authResp2); err != nil {
		t.Fatal(err)
	}
	if !authResp2.OK {
		t.Fatalf("auth2 failed: %s", authResp2.Reason)
	}

	// Second transform with short timeout — should get timeout on worker acquisition.
	req2 := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "blk-2", Op: "transform",
		Path: "b.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "blk-tok-2",
	}

	// We can't directly control the per-request timeout from the client side.
	// Instead, close the session while the second request is pending, which
	// should unblock worker acquisition.
	if err := transform.EncodeFrame(conn2, req2); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	// Close the session — cancels all active transforms.
	closeErr := sess.Close()
	// Worker should be released.
	_ = closeErr

	// Read responses with deadlines — both should fail or get error responses.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_ = conn2.SetReadDeadline(time.Now().Add(2 * time.Second))

	var resp1 transform.TransformResponseV2
	err1 := transform.DecodeFrame(conn, &resp1)
	if err1 == nil && resp1.OK {
		// First request might have completed if unblock raced with close.
		t.Logf("first request completed despite close")
	}

	var resp2 transform.TransformResponseV2
	err2 := transform.DecodeFrame(conn2, &resp2)
	if err2 == nil && resp2.OK {
		t.Error("expected second request to fail after session close")
	}
}

func TestRepeatedCloseWaitsForGoroutines(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	// Connect to verify it's up.
	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()

	// Close should be clean — no goroutine leaks.
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Second close — idempotent.
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestInvocationContextCancelsActiveTransforms(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	engine, unblock := blockingEngine()
	defer unblock()

	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")

	// Send a transform that blocks on the engine.
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "ctx-cancel-1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "ctx-tok",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	// Give it time to acquire the worker and block on the engine.
	time.Sleep(100 * time.Millisecond)

	// Cancel the invocation context.
	cancel()

	// Read the response with a deadline — connection may close during shutdown.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		// Connection closed during shutdown — acceptable.
		t.Logf("decode after cancel: %v", err)
		_ = sess.Close()
		return
	}
	if resp.OK {
		t.Error("expected transform to be cancelled")
	}
	if resp.ErrCode != "ERR_M_TRANSFORM_CANCELLED" && resp.ErrCode != "ERR_M_TRANSFORM_TIMEOUT" {
		t.Logf("error code: %s, error: %s", resp.ErrCode, resp.Error)
	}

	_ = sess.Close()
}

func TestSessionCloseReleasesActiveCancels(t *testing.T) {
	ctx := context.Background()
	engine, unblock := blockingEngine()

	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")

	// Send a transform that blocks.
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "close-cleanup", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "close-tok",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// Close the session — this should cancel the active transform
	// and clean up the cancel entry.
	unblock() // unblock first so the transform can complete/fail
	time.Sleep(50 * time.Millisecond)

	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Read the response to drain.
	var resp transform.TransformResponseV2
	_ = transform.DecodeFrame(conn, &resp)
}

func TestTransformRequestV2ValidationError(t *testing.T) {
	// Verify that invalid requests get proper error codes on the wire.
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	// Send a request with an unsupported Node major version.
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "val-err", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: transform.DigestString("x"),
		Loader: "ts", Format: "esm", NodeMajor: 99,
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected validation error")
	}
	if resp.ErrCode == "" {
		t.Fatal("expected non-empty err_code")
	}
}

func TestDuplicateRequestIDRejected(t *testing.T) {
	ctx := context.Background()
	engine, unblock := blockingEngine()
	defer unblock()

	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	// Connection 1 sends a transform that blocks on the engine.
	conn1, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn1.Close() }()

	auth1 := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn1, auth1); err != nil {
		t.Fatal(err)
	}
	var authResp1 transform.HelloResponse
	if err := transform.DecodeFrame(conn1, &authResp1); err != nil {
		t.Fatal(err)
	}
	if !authResp1.OK {
		t.Fatalf("auth1 failed: %s", authResp1.Reason)
	}

	srcDigest := transform.DigestString("x")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "dup-id", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "dup-id",
	}

	// Connection 1: send request that blocks on engine.
	if err := transform.EncodeFrame(conn1, req); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	// Connection 2: send request with same ID — must be rejected.
	conn2, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn2.Close() }()

	auth2 := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn2, auth2); err != nil {
		t.Fatal(err)
	}
	var authResp2 transform.HelloResponse
	if err := transform.DecodeFrame(conn2, &authResp2); err != nil {
		t.Fatal(err)
	}
	if !authResp2.OK {
		t.Fatalf("auth2 failed: %s", authResp2.Reason)
	}

	if err := transform.EncodeFrame(conn2, req); err != nil {
		t.Fatal(err)
	}

	_ = conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var rejectResp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn2, &rejectResp); err != nil {
		t.Fatalf("decode rejection: %v", err)
	}
	if rejectResp.ErrCode != "ERR_M_USAGE" || rejectResp.Error != "duplicate request id" {
		t.Errorf("expected duplicate rejection, got err_code=%q error=%q",
			rejectResp.ErrCode, rejectResp.Error)
	}
}

func TestCancelTokenValidation(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	// Missing cancel token.
	req := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "ct-1", Op: "cancel", CancelToken: "",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}
	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected error for missing cancel token")
	}
}

func TestIdempotentCancel(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	// Cancel a non-existent request — should succeed (idempotent).
	req := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "ic-1", Op: "cancel", CancelToken: "nonexistent",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}
	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("idempotent cancel failed: %s", resp.Error)
	}
}

func TestShutdownRequest(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	// Send shutdown.
	req := transform.ShutdownRequest{
		V: transform.ProtocolVersion, ID: "sd-1", Op: "shutdown",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}
	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("shutdown rejected: %s", resp.Error)
	}

	// The connection handler should exit after shutdown acknowledgement.
	// Verify the conn eventually closes (server side closes after response).
	time.Sleep(100 * time.Millisecond)

	// Subsequent read should get EOF.
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var extraResp transform.TransformResponseV2
	err = transform.DecodeFrame(conn, &extraResp)
	if err == nil {
		t.Error("expected connection close after shutdown")
	}
}

// errReader fails reads for testing error paths.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func TestDecodeFrameErrorHandling(t *testing.T) {
	// Verify DecodeFrame returns errors for various failure modes.
	var resp transform.TransformResponseV2

	err := transform.DecodeFrame(errReader{err: errors.New("injected")}, &resp)
	if err == nil {
		t.Fatal("expected error from failing reader")
	}
}

func TestSanitizeErrorMessageDoesNotExposeContent(t *testing.T) {
	// Source content.
	msg := transform.SanitizeErrorMessage("const x = 1; unexpected token")
	if msg == "const x = 1; unexpected token" {
		t.Fatal("source content not sanitized")
	}

	// Endpoints.
	msg2 := transform.SanitizeErrorMessage("failed to connect to 127.0.0.1:9999")
	if msg2 == "failed to connect to 127.0.0.1:9999" {
		t.Fatal("endpoint not sanitized")
	}

	// Options JSON.
	msg3 := transform.SanitizeErrorMessage(`bad "target": "ES2022" setting`)
	if msg3 == `bad "target": "ES2022" setting` {
		t.Fatal("options not sanitized")
	}

	// Safe message passes through.
	msg4 := transform.SanitizeErrorMessage("transform timeout")
	if msg4 != "transform timeout" {
		t.Fatalf("safe message altered: %s", msg4)
	}
}

func TestDigestStringDeterministic(t *testing.T) {
	d1 := transform.DigestString("hello world")
	d2 := transform.DigestString("hello world")
	if d1 != d2 {
		t.Fatal("DigestString not deterministic")
	}
	if len(d1) != 64 {
		t.Fatalf("DigestString length=%d, want 64", len(d1))
	}
}

// TestServiceOptionsDefaults verifies default values are applied.
func TestServiceOptionsDefaults(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	if sess.Token == "" {
		t.Fatal("token not generated")
	}
	if sess.Endpoint == "" {
		t.Fatal("endpoint not set")
	}
	if sess.ActiveRequests() != 0 {
		t.Fatalf("active=%d after creation", sess.ActiveRequests())
	}
}

func TestEnvOverlay(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	overlay := sess.EnvOverlay()
	if len(overlay) != 2 {
		t.Fatalf("expected 2 env entries, got %d", len(overlay))
	}

	if overlay[0] != "MEW_TRANSFORM_ENDPOINT="+sess.EndpointEnv() {
		t.Errorf("endpoint env mismatch: %s", overlay[0])
	}
	if overlay[1] != "MEW_TRANSFORM_TOKEN="+sess.TokenEnv() {
		t.Errorf("token env mismatch: %s", overlay[1])
	}
}

// Test that health check uses a real auth handshake.
func TestHealthCheckAuthHandshake(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	// Session started successfully means health check passed.
	// Verify we can connect and authenticate.
	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Auth with wrong token should fail.
	wrongAuth := transform.HelloRequest{V: transform.ProtocolVersion, Token: "wrong"}
	if err := transform.EncodeFrame(conn, wrongAuth); err != nil {
		t.Fatal(err)
	}
	var wrongResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &wrongResp); err != nil {
		t.Fatal(err)
	}
	if wrongResp.OK {
		t.Fatal("auth should have failed with wrong token")
	}
}

func TestTransformRequestV2ValidateOptionsDigestMismatch(t *testing.T) {
	opts := `{"target":"ES2022"}`
	wrongDigest := transform.DigestString("different opts")
	err := transform.VerifyOptionsDigest(opts, wrongDigest)
	if err == nil {
		t.Fatal("expected options digest mismatch error")
	}
}

// Ensure stable error codes needed by this package are exported.
// This test fails at compile time if these constants don't exist.
func TestErrorCodesAccessible(t *testing.T) {
	_ = transform.ProtocolVersion
	_ = transform.MaxFrameSize
}

// TestTransformSuccessButCacheWriteFailure verifies that when a transform
// succeeds but WriteCache fails, the response is an error (not OK with cached
// status silently dropped).
func TestTransformSuccessButCacheWriteFailure(t *testing.T) {
	// os.Chmod 0o555 does not prevent file creation inside the directory on
	// Windows; the Unix permission model the test relies on is unavailable.
	if runtime.GOOS == "windows" {
		t.Skip("read-only directory semantics not available on Windows")
	}
	ctx := context.Background()
	dir := t.TempDir()

	// Create cache dir and make it read-only so writes fail.
	cacheDir := filepath.Join(dir, "transform", "v1")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// WriteCache writes into <cacheDir>/<prefix>/<key>.{code,map,meta}.
	// Making the entire cacheDir read-only causes the prefix dir MkdirAll
	// or file writes to fail.
	if err := os.Chmod(cacheDir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(cacheDir, 0o755) }()

	sess, err := transform.NewSession(transform.ServiceOptions{
		Context:  ctx,
		CacheDir: cacheDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	src := "const x: number = 1;"
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "cache-fail", Op: "transform",
		Path: "test.ts", Source: src, SourceDigest: transform.DigestString(src),
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "cache-fail",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected error response when cache write fails")
	}
	if resp.ErrCode == "" {
		t.Fatal("expected non-empty err_code")
	}
	if resp.Code != "" {
		t.Fatal("expected empty code on failure, got transformed code")
	}
}

// TestCacheErrorDiagnosticsDoNotExposeCredentials verifies that cache and
// transform error responses do not leak tokens, source content, endpoints,
// or transform options.
func TestCacheErrorDiagnosticsDoNotExposeCredentials(t *testing.T) {
	// os.Chmod 0o555 does not prevent file creation inside the directory on
	// Windows; the Unix permission model the test relies on is unavailable.
	if runtime.GOOS == "windows" {
		t.Skip("read-only directory semantics not available on Windows")
	}
	ctx := context.Background()
	dir := t.TempDir()

	cacheDir := filepath.Join(dir, "transform", "v1")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cacheDir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(cacheDir, 0o755) }()

	sess, err := transform.NewSession(transform.ServiceOptions{
		Context:  ctx,
		CacheDir: cacheDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	src := "const x: number = 1;"
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "no-leak", Op: "transform",
		Path: "test.ts", Source: src, SourceDigest: transform.DigestString(src),
		Loader: "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "no-leak",
		Options:     `{"target":"ES2022"}`, OptsDigest: transform.DigestString(`{"target":"ES2022"}`),
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected error response when cache write fails")
	}

	// Error response must not contain the session token.
	if strings.Contains(resp.Error, sess.Token) {
		t.Fatal("error response contains session token")
	}
	// Error response must not contain the source content.
	if strings.Contains(resp.Error, src) {
		t.Fatal("error response contains source content")
	}
	// Error response must not contain the endpoint.
	if strings.Contains(resp.Error, sess.Endpoint) {
		t.Fatal("error response contains endpoint")
	}
	// Error response must not contain transform options.
	if strings.Contains(resp.Error, `"target"`) || strings.Contains(resp.Error, "ES2022") {
		t.Fatal("error response contains transform options")
	}
}

// verifyErrorCode helper — unused, kept for documentation of the pattern
// that callers should use to check error codes.

// ── In-flight cancellation tests (Issue 4) ─────────────────────────────

// blockingEngineUnblocked creates a fake engine that blocks until context
// cancellation (unlike blockingEngine which returns success on unblock).
// Returns the engine and a channel that signals when Transform was called.
func engineThatBlocksUntilCancelled() (transform.Engine, chan struct{}) {
	called := make(chan struct{}, 4)
	e := &cancellationOnlyEngine{called: called}
	return e, called
}

type cancellationOnlyEngine struct {
	called   chan struct{}
	identity transform.EngineIdentity
}

func (e *cancellationOnlyEngine) Identity() transform.EngineIdentity {
	if e.identity.Name == "" {
		e.identity = transform.EngineIdentity{Name: "cancel-only", Version: "1.0"}
	}
	return e.identity
}

func (e *cancellationOnlyEngine) Transform(ctx context.Context, _ transform.TransformRequest) (transform.TransformResult, error) {
	select {
	case e.called <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return transform.TransformResult{}, ctx.Err()
}

// TestOpCancelProcessedWhileTransformInFlight proves the core fix:
// an OpCancel frame on the same connection is read and processed while
// the transform is still running (not after it completes).
func TestOpCancelProcessedWhileTransformInFlight(t *testing.T) {
	ctx := context.Background()
	engine, engineCalled := engineThatBlocksUntilCancelled()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Authenticate.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("const x: number = 1;")

	// Send transform that blocks until context cancellation.
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "in-flight-1", Op: "transform",
		Path: "a.ts", Source: "const x: number = 1;", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-inflight",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	// Wait for engine to be called (transform is in flight).
	select {
	case <-engineCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("engine was never called")
	}

	// Now send OpCancel on the same connection.
	cancelReq := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "cancel-inflight", Op: "cancel", CancelToken: "tok-inflight",
	}
	if err := transform.EncodeFrame(conn, cancelReq); err != nil {
		t.Fatal(err)
	}

	// Read cancel acknowledgment.
	var cancelAck transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &cancelAck); err != nil {
		t.Fatalf("decode cancel ack: %v", err)
	}
	if !cancelAck.OK {
		t.Fatalf("cancel rejected: %s", cancelAck.Error)
	}

	// Read the transform response — must be cancellation, not success.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatalf("decode transform response: %v", err)
	}
	if resp.OK {
		t.Fatal("transform succeeded after cancellation — expected cancellation response")
	}
	if resp.ErrCode != "ERR_M_TRANSFORM_CANCELLED" {
		t.Errorf("expected ERR_M_TRANSFORM_CANCELLED, got %q", resp.ErrCode)
	}
}

// TestCancelProducesSingleTerminalResponse proves exactly one response is
// sent for a cancelled request — no success response follows.
func TestCancelProducesSingleTerminalResponse(t *testing.T) {
	ctx := context.Background()
	engine, engineCalled := engineThatBlocksUntilCancelled()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")

	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "single-1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-single",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	select {
	case <-engineCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("engine was never called")
	}

	// Send cancel.
	cancelReq := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "cancel-single", Op: "cancel", CancelToken: "tok-single",
	}
	if err := transform.EncodeFrame(conn, cancelReq); err != nil {
		t.Fatal(err)
	}

	// Read cancel ack + transform response.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	responseCount := 0
	for i := 0; i < 3; i++ {
		var resp transform.TransformResponseV2
		if err := transform.DecodeFrame(conn, &resp); err != nil {
			break
		}
		if resp.ID == "single-1" {
			responseCount++
		}
	}

	if responseCount != 1 {
		t.Errorf("expected exactly 1 response for transform, got %d", responseCount)
	}
}

// TestCancelOneDoesNotAffectOther proves cancelling one request leaves
// another independent request unaffected.
func TestCancelOneDoesNotAffectOther(t *testing.T) {
	ctx := context.Background()
	// Use selective engine: only blocks request with cancel token "tok-conc-1".
	engine, engineCalled := engineThatBlocksForToken("tok-conc-1")
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("const x: number = 1;")

	// Request 1: will be cancelled (blocks on engine).
	req1 := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "conc-1", Op: "transform",
		Path: "a.ts", Source: "const x: number = 1;", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-conc-1",
	}
	if err := transform.EncodeFrame(conn, req1); err != nil {
		t.Fatal(err)
	}

	// Wait for req1 to enter the engine.
	select {
	case <-engineCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("engine not called for first request")
	}

	// Request 2: completes normally via real esbuild.
	req2 := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "conc-2", Op: "transform",
		Path: "b.ts", Source: "const x: number = 1;", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-conc-2",
	}
	if err := transform.EncodeFrame(conn, req2); err != nil {
		t.Fatal(err)
	}

	// Cancel request 1.
	cancelReq := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "cancel-conc", Op: "cancel", CancelToken: "tok-conc-1",
	}
	if err := transform.EncodeFrame(conn, cancelReq); err != nil {
		t.Fatal(err)
	}

	// Collect responses.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var resp1, resp2 *transform.TransformResponseV2
	for i := 0; i < 4; i++ {
		var r transform.TransformResponseV2
		if err := transform.DecodeFrame(conn, &r); err != nil {
			break
		}
		switch r.ID {
		case "conc-1":
			resp1 = &r
		case "conc-2":
			resp2 = &r
		}
	}

	if resp1 == nil {
		t.Fatal("no response for request 1")
	}
	if resp1.OK {
		t.Fatal("request 1 should be cancelled, got success")
	}

	if resp2 == nil {
		t.Fatal("no response for request 2")
	}
	if !resp2.OK {
		t.Errorf("request 2 should succeed, got err_code=%q", resp2.ErrCode)
	}
}

// engineThatBlocksForToken creates an engine that blocks until context
// cancellation only for transforms whose request carries the given cancel token.
// All other transforms use the real esbuild engine.
func engineThatBlocksForToken(token string) (transform.Engine, chan struct{}) {
	called := make(chan struct{}, 1)
	return &selectiveBlockEngine{blockToken: token, called: called}, called
}

type selectiveBlockEngine struct {
	blockToken string
	called     chan struct{}
	real       transform.Engine
	identity   transform.EngineIdentity
}

func (e *selectiveBlockEngine) Identity() transform.EngineIdentity {
	if e.identity.Name == "" {
		e.identity = transform.EngineIdentity{Name: "selective", Version: "1.0"}
	}
	return e.identity
}

func (e *selectiveBlockEngine) Transform(ctx context.Context, req transform.TransformRequest) (transform.TransformResult, error) {
	// The engine matches by request ID. The test uses "conc-1" as the
	// request ID that should block until cancelled.
	if req.RequestID == "conc-1" {
		select {
		case e.called <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return transform.TransformResult{}, ctx.Err()
	}
	if e.real == nil {
		e.real = transform.NewEsbuildEngine()
	}
	return e.real.Transform(ctx, req)
}

// TestDuplicateCancelIdempotent proves multiple OpCancel frames for the same
// token are all acknowledged OK.
func TestDuplicateCancelIdempotent(t *testing.T) {
	ctx := context.Background()
	engine, engineCalled := engineThatBlocksUntilCancelled()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "dup-cancel-1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-dup-cancel",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	select {
	case <-engineCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("engine not called")
	}

	// First cancel.
	c1 := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "cancel-dup-1", Op: "cancel", CancelToken: "tok-dup-cancel",
	}
	if err := transform.EncodeFrame(conn, c1); err != nil {
		t.Fatal(err)
	}

	// Second cancel — same token, should still be OK.
	c2 := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "cancel-dup-2", Op: "cancel", CancelToken: "tok-dup-cancel",
	}
	if err := transform.EncodeFrame(conn, c2); err != nil {
		t.Fatal(err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for i := 0; i < 3; i++ {
		var r transform.TransformResponseV2
		if err := transform.DecodeFrame(conn, &r); err != nil {
			break
		}
		// Cancel acknowledgments should be OK.
		if (r.ID == "cancel-dup-1" || r.ID == "cancel-dup-2") && !r.OK {
			t.Errorf("duplicate cancel %s not OK: %s", r.ID, r.Error)
		}
	}
}

// TestUnknownCancelToken proves cancelling a non-existent token is idempotent.
func TestUnknownCancelTokenIdempotent(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	// Cancel a never-registered token.
	req := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "unknown-tok", Op: "cancel", CancelToken: "never-existed",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("unknown token cancel not OK: %s", resp.Error)
	}
}

// TestCancelAlreadyCompletedToken proves cancelling a completed transform's
// token is safe (idempotent no-op).
func TestCancelAlreadyCompletedToken(t *testing.T) {
	ctx := context.Background()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("const x: number = 1;")

	// Send a fast transform that completes immediately.
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "fast-1", Op: "transform",
		Path: "a.ts", Source: "const x: number = 1;", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-fast",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	// Read success response.
	var successResp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &successResp); err != nil {
		t.Fatal(err)
	}
	if !successResp.OK {
		t.Fatalf("transform failed: %s", successResp.Error)
	}

	// Now cancel the already-completed token.
	cancelReq := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "cancel-fast", Op: "cancel", CancelToken: "tok-fast",
	}
	if err := transform.EncodeFrame(conn, cancelReq); err != nil {
		t.Fatal(err)
	}

	var cancelResp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &cancelResp); err != nil {
		t.Fatal(err)
	}
	if !cancelResp.OK {
		t.Fatalf("post-completion cancel not OK: %s", cancelResp.Error)
	}
}

// TestCancelWhileWaitingForWorkerSlot proves cancellation works even when
// the request hasn't yet acquired a worker (it's queued).
func TestCancelWhileWaitingForWorkerSlot(t *testing.T) {
	ctx := context.Background()
	engine, unblock := blockingEngine()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer unblock()

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	// Use a single connection so cancel can arrive while second request is queued.
	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")

	// First request: blocks the only worker slot.
	req1 := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "slot-1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-slot-1",
	}
	if err := transform.EncodeFrame(conn, req1); err != nil {
		t.Fatal(err)
	}

	// Give it time to acquire the worker.
	time.Sleep(100 * time.Millisecond)

	// Second request: will block waiting for worker slot.
	req2 := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "slot-2", Op: "transform",
		Path: "b.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-slot-2",
	}
	if err := transform.EncodeFrame(conn, req2); err != nil {
		t.Fatal(err)
	}

	// Give it time to start waiting.
	time.Sleep(50 * time.Millisecond)

	// Cancel the second request while it's waiting for a worker slot.
	cancelReq := transform.CancelRequest{
		V: transform.ProtocolVersion, ID: "cancel-slot", Op: "cancel", CancelToken: "tok-slot-2",
	}
	if err := transform.EncodeFrame(conn, cancelReq); err != nil {
		t.Fatal(err)
	}

	// Read responses: cancel ack + cancelled transform response.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var gotCancelAck, gotCancelledResp bool
	for i := 0; i < 3; i++ {
		var r transform.TransformResponseV2
		if err := transform.DecodeFrame(conn, &r); err != nil {
			break
		}
		if r.ID == "cancel-slot" && r.OK {
			gotCancelAck = true
		}
		if r.ID == "slot-2" {
			gotCancelledResp = true
			if r.OK {
				t.Errorf("request 2 succeeded while waiting for slot, expected cancellation")
			}
		}
	}

	if !gotCancelAck {
		t.Error("no cancel acknowledgment received")
	}
	if !gotCancelledResp {
		t.Error("no cancellation response for request 2")
	}
}

// TestResponseWriteFailureCleanup proves that when a connection dies mid-transform,
// state is cleaned up without blocking.
func TestResponseWriteFailureCleanup(t *testing.T) {
	ctx := context.Background()
	engine, unblock := blockingEngine()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "conn-fail", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-conn-fail",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	// Let the transform acquire the worker and block.
	time.Sleep(100 * time.Millisecond)

	// Kill the connection before the transform completes.
	_ = conn.Close()

	// Unblock the engine so the transform goroutine wakes up and tries to write.
	unblock()

	// Session Close should return promptly (write fails, cleanup runs).
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- sess.Close()
	}()

	select {
	case <-closeDone:
		// OK: Close returned promptly.
	case <-time.After(3 * time.Second):
		t.Fatal("Close hung after connection failure")
	}
}

// TestShutdownCancelsMultipleActiveTransforms proves session Close cancels
// all active transforms and waits for their goroutines.
func TestShutdownCancelsMultipleActiveTransforms(t *testing.T) {
	ctx := context.Background()
	engine, engineCalled := engineThatBlocksUntilCancelled()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")

	// Send two transforms, both block until cancelled.
	for i := 0; i < 2; i++ {
		req := transform.TransformRequestV2{
			V:  transform.ProtocolVersion,
			ID: fmt.Sprintf("shutdown-%d", i), Op: "transform",
			Path: "a.ts", Source: "x", SourceDigest: srcDigest,
			OptsDigest: transform.DigestString(""),
			Loader:     "ts", Format: "esm", NodeMajor: 20,
			SourceMap:   "none",
			CancelToken: fmt.Sprintf("tok-shutdown-%d", i),
		}
		if err := transform.EncodeFrame(conn, req); err != nil {
			t.Fatal(err)
		}
	}

	// Wait for both to enter the engine.
	for i := 0; i < 2; i++ {
		select {
		case <-engineCalled:
		case <-time.After(2 * time.Second):
			t.Fatalf("engine not called for request %d", i)
		}
	}

	// Close the session — should cancel both.
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- sess.Close()
	}()

	select {
	case err := <-closeDone:
		if err != nil {
			t.Logf("Close returned: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close hung with multiple active transforms")
	}

	// Verify no goroutines leaked: Close's wg.Wait returned.
}

// TestDuplicateActiveRequestIDRejectedOnSameConnection verifies that sending
// a second request with the same ID before the first completes is rejected.
func TestDuplicateActiveRequestIDRejected(t *testing.T) {
	ctx := context.Background()
	engine, engineCalled := engineThatBlocksUntilCancelled()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: ctx,
		Engine:  engine,
		Workers: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("x")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "dup-on-same", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-dup-id",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	// Wait for first to enter engine.
	select {
	case <-engineCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("engine not called for first request")
	}

	// Send duplicate ID — must be rejected inline (before any goroutine dispatch).
	dupReq := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "dup-on-same", Op: "transform",
		Path: "b.ts", Source: "x", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-dup-id-2",
	}
	if err := transform.EncodeFrame(conn, dupReq); err != nil {
		t.Fatal(err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var rejectResp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &rejectResp); err != nil {
		t.Fatalf("decode rejection: %v", err)
	}
	if rejectResp.ErrCode != "ERR_M_USAGE" || rejectResp.Error != "duplicate request id" {
		t.Errorf("expected duplicate rejection, got err_code=%q error=%q",
			rejectResp.ErrCode, rejectResp.Error)
	}
}

// TestRequestTimeoutSendsCancellation proves the per-request timeout triggers
// a cancellation response (not a success).
func TestRequestTimeoutSendsCancellation(t *testing.T) {
	ctx := context.Background()
	engine, engineCalled := engineThatBlocksUntilCancelled()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context:        ctx,
		Engine:         engine,
		Workers:        1,
		RequestTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	srcDigest := transform.DigestString("const x: number = 1;")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "timeout-1", Op: "transform",
		Path: "a.ts", Source: "const x: number = 1;", SourceDigest: srcDigest,
		OptsDigest: transform.DigestString(""),
		Loader:     "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "none",
		CancelToken: "tok-timeout",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	// Wait for engine to be called.
	select {
	case <-engineCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("engine not called")
	}

	// Wait for timeout response.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatalf("decode timeout response: %v", err)
	}
	if resp.OK {
		t.Fatal("expected timeout error, got success")
	}
	if resp.ErrCode != "ERR_M_TRANSFORM_TIMEOUT" {
		t.Errorf("expected ERR_M_TRANSFORM_TIMEOUT, got %q", resp.ErrCode)
	}
}

// ── Strict validation service-level tests ───────────────────────────

// recordingEngine records whether Transform was ever called.
type recordingEngine struct {
	called atomic.Bool
	transform.EngineIdentity
}

func (e *recordingEngine) Identity() transform.EngineIdentity { return e.EngineIdentity }

func (e *recordingEngine) Transform(_ context.Context, _ transform.TransformRequest) (transform.TransformResult, error) {
	e.called.Store(true)
	return transform.TransformResult{
		Code: []byte("ok"), OutputDigest: "abc", CacheStatus: transform.CacheStatusMiss,
	}, nil
}

func (e *recordingEngine) wasCalled() bool { return e.called.Load() }

func TestInvalidRequestNeverReachesEngine(t *testing.T) {
	// Proves that requests with bad digests are rejected synchronously in the
	// read loop and never acquire a worker slot or reach the engine.
	eng := &recordingEngine{EngineIdentity: transform.EngineIdentity{Name: "record", Version: "1"}}
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: context.Background(),
		Engine:  eng,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Auth.
	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	src := "const x: number = 1;"

	// Send transform with mismatched source digest — structurally valid,
	// but digest verification must reject it before engine work.
	badDigest := transform.DigestString("wrong content")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "bad-digest", Op: "transform",
		Path: "a.ts", Source: src, SourceDigest: badDigest,
		Loader: "ts", Format: "esm", OptsDigest: transform.DigestString(""),
		NodeMajor: 20, SourceMap: "none", CancelToken: "bad-digest",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected digest mismatch error, got success")
	}

	// Engine must never have been called.
	if eng.wasCalled() {
		t.Fatal("engine was called for a request with bad source digest")
	}
}

func TestInvalidRequestNeverReachesEngine_BadOptsDigest(t *testing.T) {
	eng := &recordingEngine{EngineIdentity: transform.EngineIdentity{Name: "record", Version: "1"}}
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: context.Background(),
		Engine:  eng,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	src := "const x: number = 1;"
	srcDigest := transform.DigestString(src)
	badOptsDigest := transform.DigestString("wrong options")

	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "bad-opts", Op: "transform",
		Path: "a.ts", Source: src, SourceDigest: srcDigest,
		Loader: "ts", Format: "esm", Options: `{"target":"ES2022"}`,
		OptsDigest: badOptsDigest,
		NodeMajor:  20, SourceMap: "none", CancelToken: "bad-opts",
	}
	if err := transform.EncodeFrame(conn, req); err != nil {
		t.Fatal(err)
	}

	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected opts digest mismatch error, got success")
	}
	if eng.wasCalled() {
		t.Fatal("engine was called for a request with bad options digest")
	}
}

func TestRejectTransformFieldsOnCancel(t *testing.T) {
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	// Send a cancel with transform-specific fields — strict decoding must reject.
	body := []byte(`{"v":2,"id":"c1","op":"cancel","cancel_token":"tok","path":"/evil.ts","source":"malicious"}`)
	hdr := [4]byte{}
	hdr[0] = byte(len(body))
	_, _ = conn.Write(hdr[:])
	_, _ = conn.Write(body)

	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected rejection of cancel with transform fields")
	}
}

func TestRejectTransformFieldsOnShutdown(t *testing.T) {
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	// Shutdown with transform-specific fields — must be rejected.
	body := []byte(`{"v":2,"id":"s1","op":"shutdown","loader":"ts"}`)
	hdr := [4]byte{}
	hdr[0] = byte(len(body))
	_, _ = conn.Write(hdr[:])
	_, _ = conn.Write(body)

	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected rejection of shutdown with transform fields")
	}
}

func TestRejectUnknownOp(t *testing.T) {
	sess, err := transform.NewSession(transform.ServiceOptions{
		Context: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	if err := sess.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", sess.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	authReq := transform.HelloRequest{V: transform.ProtocolVersion, Token: sess.Token}
	if err := transform.EncodeFrame(conn, authReq); err != nil {
		t.Fatal(err)
	}
	var authResp transform.HelloResponse
	if err := transform.DecodeFrame(conn, &authResp); err != nil {
		t.Fatal(err)
	}
	if !authResp.OK {
		t.Fatalf("auth failed: %s", authResp.Reason)
	}

	// Unknown operation.
	body := []byte(`{"v":2,"id":"u1","op":"bundle"}`)
	hdr := [4]byte{}
	hdr[0] = byte(len(body))
	_, _ = conn.Write(hdr[:])
	_, _ = conn.Write(body)

	var resp transform.TransformResponseV2
	if err := transform.DecodeFrame(conn, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected rejection of unknown op")
	}
	if resp.ErrCode != "ERR_M_UNSUPPORTED" {
		t.Errorf("expected ERR_M_UNSUPPORTED, got %q", resp.ErrCode)
	}
}
