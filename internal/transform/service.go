package transform

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/trace"
)

// Session encapsulates a per-invocation transform service.
type Session struct {
	Token       string // random per-session bearer auth token
	Endpoint    string // "host:port" for the listener
	endpointEnv string // MEW_TRANSFORM_ENDPOINT env value
	tokenEnv    string // MEW_TRANSFORM_TOKEN env value

	listener net.Listener
	engine   Engine
	cacheDir string
	workers  chan struct{}
	active   atomic.Int32
	closed   atomic.Bool

	// activeCancels tracks per-request cancel functions keyed by cancel token.
	activeCancels   map[string]context.CancelFunc
	activeCancelsMu sync.Mutex

	// activeIDs tracks in-flight request IDs for duplicate detection.
	activeIDs   map[string]bool
	activeIDsMu sync.Mutex

	idleTimeout    time.Duration
	requestTimeout time.Duration

	// Session-scoped context and cancel, derived from the invocation context.
	// Cancel is called by Close to initiate coordinated shutdown.
	ctx    context.Context
	cancel context.CancelFunc

	// Tracked connections for coordinated shutdown.
	connsMu sync.Mutex
	conns   map[net.Conn]struct{}

	// Concurrent-close coordination.
	closeDone chan struct{} // closed when shutdown completes
	closeErr  error         // error from the winning Close call

	// WaitGroup tracks server, connection, and request goroutines.
	wg sync.WaitGroup
}

// ServiceOptions configures the transform session.
type ServiceOptions struct {
	Engine         Engine
	CacheDir       string // transform cache directory; empty disables cache
	Workers        int    // max concurrent transforms, default 4
	IdleTimeout    time.Duration
	RequestTimeout time.Duration
	// Context is the invocation context. Required for production sessions.
	// Session-scoped context is derived from this; cancellation propagates
	// to listener, connections, and active transforms.
	Context context.Context
}

// NewSession creates a transform service session bound to a random local port.
// Requires a non-nil Context in opts for production use.
func NewSession(opts ServiceOptions) (*Session, error) {
	if opts.Context == nil {
		return nil, fmt.Errorf("transform session requires a non-nil context")
	}
	if opts.Engine == nil {
		opts.Engine = NewEsbuildEngine()
	}
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = 30 * time.Second
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 60 * time.Second
	}

	token, err := generateToken(32)
	if err != nil {
		return nil, fmt.Errorf("token generation: %w", err)
	}

	ctx, cancel := context.WithCancel(opts.Context)

	s := &Session{
		Token:          token,
		engine:         opts.Engine,
		cacheDir:       opts.CacheDir,
		workers:        make(chan struct{}, opts.Workers),
		activeCancels:  make(map[string]context.CancelFunc),
		activeIDs:      make(map[string]bool),
		idleTimeout:    opts.IdleTimeout,
		requestTimeout: opts.RequestTimeout,
		ctx:            ctx,
		cancel:         cancel,
		conns:          make(map[net.Conn]struct{}),
		closeDone:      make(chan struct{}),
	}

	// Listen on localhost random port using the session context.
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("listen: %w", err)
	}
	s.listener = ln
	addr := ln.Addr().(*net.TCPAddr)
	s.Endpoint = fmt.Sprintf("127.0.0.1:%d", addr.Port)
	s.endpointEnv = s.Endpoint
	s.tokenEnv = token

	return s, nil
}

// EndpointEnv returns the environment variable for the listener endpoint.
func (s *Session) EndpointEnv() string { return s.endpointEnv }

// TokenEnv returns the environment variable for the auth token.
func (s *Session) TokenEnv() string { return s.tokenEnv }

// EnvOverlay returns key=value environment pairs for the Node child.
func (s *Session) EnvOverlay() []string {
	return []string{
		"MEW_TRANSFORM_ENDPOINT=" + s.endpointEnv,
		"MEW_TRANSFORM_TOKEN=" + s.tokenEnv,
	}
}

// Start begins accepting connections. Returns after the authenticated health
// check succeeds: it connects to the listener and performs the real protocol
// hello handshake to verify the service is reachable and authenticated.
// The health check aborts promptly when the session context is cancelled.
func (s *Session) Start() error {
	if s.listener == nil {
		return fmt.Errorf("session not initialized")
	}

	s.wg.Add(1)
	go s.serve()

	// Health check: connect and perform real auth handshake using
	// DialContext so it aborts on session context cancellation.
	var d net.Dialer
	conn, err := d.DialContext(s.ctx, "tcp", s.Endpoint)
	if err != nil {
		_ = s.listener.Close()
		return fmt.Errorf("health check dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Abort pending I/O when session context is cancelled.
	go func() {
		<-s.ctx.Done()
		_ = conn.Close()
	}()

	// Set a deadline so encode/decode don't block indefinitely.
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Perform real hello handshake.
	if err := EncodeFrame(conn, HelloRequest{V: ProtocolVersion, Token: s.Token}); err != nil {
		_ = s.listener.Close()
		return fmt.Errorf("health check hello encode: %w", err)
	}
	var helloResp HelloResponse
	if err := DecodeFrame(conn, &helloResp); err != nil {
		_ = s.listener.Close()
		return fmt.Errorf("health check hello decode: %w", err)
	}
	if !helloResp.OK {
		_ = s.listener.Close()
		return fmt.Errorf("health check auth failed: %s", helloResp.Reason)
	}

	return nil
}

// serve accepts connections on the listener. Exits when the session context is
// cancelled, the listener is closed, shutdown has begun, or an idle timeout
// expires with no active requests.
func (s *Session) serve() {
	defer s.wg.Done()
	defer func() { _ = s.listener.Close() }()

	// Close listener promptly when session context is cancelled.
	// This unblocks Accept so serve can drain and return.
	go func() {
		<-s.ctx.Done()
		_ = s.listener.Close()
	}()

	for {
		if s.closed.Load() {
			return
		}
		if s.ctx.Err() != nil {
			return
		}

		if s.idleTimeout > 0 && s.active.Load() == 0 {
			// Set accept deadline so we can check idle expiry periodically.
			if tl, ok := s.listener.(*net.TCPListener); ok {
				_ = tl.SetDeadline(time.Now().Add(s.idleTimeout))
			}
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if s.closed.Load() {
				return
			}
			if s.ctx.Err() != nil {
				return
			}
			// Idle timeout with no active requests: shut down.
			if s.active.Load() == 0 && isTimeoutErr(err) {
				return
			}
			// Retry genuine transient accept errors while session remains active.
			if isTemporaryAcceptErr(err) {
				continue
			}
			return
		}
		// Clear deadline so active connections aren't affected.
		if tl, ok := s.listener.(*net.TCPListener); ok {
			_ = tl.SetDeadline(time.Time{})
		}

		// Track connection for coordinated shutdown.
		s.connsMu.Lock()
		s.conns[conn] = struct{}{}
		s.connsMu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() {
				s.connsMu.Lock()
				delete(s.conns, conn)
				s.connsMu.Unlock()
			}()
			s.handleConn(s.ctx, conn)
		}()
	}
}

// isTimeoutErr reports whether err is a network timeout.
func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	type timeout interface{ Timeout() bool }
	if t, ok := err.(timeout); ok {
		return t.Timeout()
	}
	return false
}

// isTemporaryAcceptErr reports whether err is a transient accept error
// that should be retried.
func isTemporaryAcceptErr(err error) bool {
	if err == nil {
		return false
	}
	type temporary interface{ Temporary() bool }
	if t, ok := err.(temporary); ok {
		return t.Temporary()
	}
	return false
}

// handleConn handles a single TCP connection.
// Frame reading is decoupled from transform execution: OpTransform work is
// dispatched to a goroutine so OpCancel frames on the same connection remain
// readable while a transform is in flight. Response writes are serialized
// through a per-connection mutex to prevent frame interleaving.
func (s *Session) handleConn(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	// Step 1: authenticate via hello.
	var hello HelloRequest
	if err := DecodeFrame(conn, &hello); err != nil {
		_ = EncodeFrame(conn, HelloResponse{
			V: ProtocolVersion, OK: false,
			ErrCode: string(apperr.TransformProtocolVersion), Reason: "decode error",
		})
		return
	}
	if err := hello.Validate(); err != nil {
		_ = EncodeFrame(conn, HelloResponse{
			V: ProtocolVersion, OK: false,
			ErrCode: SanitizeErrorCode(string(apperr.TransformProtocolVersion)),
			Reason:  SanitizeErrorMessage(err.Error()),
		})
		return
	}
	if subtle.ConstantTimeCompare([]byte(hello.Token), []byte(s.Token)) != 1 {
		_ = EncodeFrame(conn, HelloResponse{
			V: ProtocolVersion, OK: false,
			ErrCode: string(apperr.TransformAuth), Reason: "unauthorized",
		})
		return
	}
	_ = EncodeFrame(conn, HelloResponse{V: ProtocolVersion, OK: true})

	// Per-connection write serialization so concurrent transform goroutines
	// don't interleave frames on the same TCP connection.
	var writeMu sync.Mutex

	// Step 2: process requests with strict per-operation decoding.
	for {
		if s.closed.Load() {
			return
		}
		if ctx.Err() != nil {
			return
		}

		// Set read deadline for idle detection.
		_ = conn.SetReadDeadline(time.Now().Add(s.idleTimeout))

		body, err := ReadFrameBody(conn)
		if err != nil {
			if err == io.EOF {
				return // clean disconnect
			}
			return // protocol error → close connection
		}

		// Clear read deadline so active transforms aren't affected by
		// slow frame processing in the read loop (deadline only guards
		// idle detection, not active work).
		_ = conn.SetReadDeadline(time.Time{})

		op, err := PeekOp(body)
		if err != nil || op == "" {
			writeResponseLocked(&writeMu, conn, TransformResponseV2{
				V: ProtocolVersion, ID: "", OK: false,
				ErrCode: string(apperr.Unsupported),
				Error:   "missing or malformed op",
			})
			continue
		}

		switch op {
		case OpHealth:
			var req HealthRequest
			if err := StrictUnmarshal(body, &req); err != nil {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: string(apperr.Usage),
					Error:   SanitizeErrorMessage(fmt.Sprintf("invalid health request: %v", err)),
				})
				continue
			}
			if err := req.Validate(); err != nil {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: SanitizeErrorCode(string(apperr.CodeOf(err))),
					Error:   SanitizeErrorMessage(err.Error()),
				})
				continue
			}
			writeResponseLocked(&writeMu, conn, TransformResponseV2{V: ProtocolVersion, ID: req.ID, OK: true})

		case OpTransform:
			var req TransformRequestV2
			if err := StrictUnmarshal(body, &req); err != nil {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: string(apperr.Usage),
					Error:   SanitizeErrorMessage(fmt.Sprintf("invalid transform request: %v", err)),
				})
				continue
			}
			if err := req.Validate(); err != nil {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: SanitizeErrorCode(string(apperr.CodeOf(err))),
					Error:   SanitizeErrorMessage(err.Error()),
				})
				continue
			}

			// Reject new requests after shutdown has begun.
			if s.closed.Load() {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: string(apperr.TransformUnavailable),
					Error:   "service shutting down",
				})
				continue
			}

			// --- Verify digests synchronously before any resource allocation.
			// This ensures invalid requests never reach cache or engine.
			if err := VerifySourceDigest(req.Source, req.SourceDigest); err != nil {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: SanitizeErrorCode(string(apperr.CodeOf(err))),
					Error:   SanitizeErrorMessage(err.Error()),
				})
				continue
			}
			if err := VerifyOptionsDigest(req.Options, req.OptsDigest); err != nil {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: SanitizeErrorCode(string(apperr.CodeOf(err))),
					Error:   SanitizeErrorMessage(err.Error()),
				})
				continue
			}

			// --- Pre-flight: register request ID and cancel token synchronously.
			// This ensures an OpCancel on the next frame can find and cancel this
			// request before the goroutine even acquires a worker slot.

			// Duplicate request ID check.
			s.activeIDsMu.Lock()
			if s.activeIDs[req.ID] {
				s.activeIDsMu.Unlock()
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: string(apperr.Usage),
					Error:   "duplicate request id",
				})
				continue
			}
			s.activeIDs[req.ID] = true
			s.activeIDsMu.Unlock()

			// Create request-scoped context with timeout derived from session ctx.
			reqCtx, reqCancel := context.WithTimeout(ctx, s.requestTimeout)

			// Register cancel token for OpCancel tracking.
			s.activeCancelsMu.Lock()
			if _, exists := s.activeCancels[req.CancelToken]; exists {
				s.activeCancelsMu.Unlock()
				reqCancel()
				s.activeIDsMu.Lock()
				delete(s.activeIDs, req.ID)
				s.activeIDsMu.Unlock()
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: string(apperr.Usage),
					Error:   "duplicate cancel token",
				})
				continue
			}
			s.activeCancels[req.CancelToken] = reqCancel
			s.activeCancelsMu.Unlock()

			// --- Dispatch work to a goroutine so the read loop stays responsive.
			// Digests were already verified above; the goroutine skips re-verification.
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer reqCancel()
				defer func() {
					s.activeIDsMu.Lock()
					delete(s.activeIDs, req.ID)
					s.activeIDsMu.Unlock()
					s.activeCancelsMu.Lock()
					delete(s.activeCancels, req.CancelToken)
					s.activeCancelsMu.Unlock()
				}()
				s.handleTransformWork(reqCtx, conn, &req, &writeMu)
			}()

		case OpCancel:
			var req CancelRequest
			if err := StrictUnmarshal(body, &req); err != nil {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: string(apperr.Usage),
					Error:   SanitizeErrorMessage(fmt.Sprintf("invalid cancel request: %v", err)),
				})
				continue
			}
			s.processCancel(conn, &req, &writeMu)

		case OpShutdown:
			var req ShutdownRequest
			if err := StrictUnmarshal(body, &req); err != nil {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: string(apperr.Usage),
					Error:   SanitizeErrorMessage(fmt.Sprintf("invalid shutdown request: %v", err)),
				})
				continue
			}
			if err := req.Validate(); err != nil {
				writeResponseLocked(&writeMu, conn, TransformResponseV2{
					V: ProtocolVersion, ID: req.ID, OK: false,
					ErrCode: SanitizeErrorCode(string(apperr.CodeOf(err))),
					Error:   SanitizeErrorMessage(err.Error()),
				})
				continue
			}
			writeResponseLocked(&writeMu, conn, TransformResponseV2{V: ProtocolVersion, ID: req.ID, OK: true})
			return

		default:
			// Extract ID for error response if available.
			var probe struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(body, &probe)
			writeResponseLocked(&writeMu, conn, TransformResponseV2{
				V: ProtocolVersion, ID: probe.ID, OK: false,
				ErrCode: string(apperr.Unsupported),
				Error:   fmt.Sprintf("unknown op %q", op),
			})
		}
	}
}

// writeResponseLocked writes a single frame to conn under mu.
// Serializes concurrent writes from transform goroutines to prevent
// frame interleaving. Returns false if the write fails (connection dead).
func writeResponseLocked(mu *sync.Mutex, conn net.Conn, resp TransformResponseV2) bool {
	mu.Lock()
	defer mu.Unlock()
	return EncodeFrame(conn, resp) == nil
}

// processCancel handles an OpCancel frame inline in the read loop.
// The lookup and cancel call are fast (mutex + function call); no I/O
// except the final acknowledgment write. Validation and strict decoding
// were already performed by the caller.
func (s *Session) processCancel(conn net.Conn, req *CancelRequest, writeMu *sync.Mutex) {
	if err := req.Validate(); err != nil {
		writeResponseLocked(writeMu, conn, TransformResponseV2{
			V: ProtocolVersion, ID: req.ID, OK: false,
			ErrCode: SanitizeErrorCode(string(apperr.CodeOf(err))),
			Error:   SanitizeErrorMessage(err.Error()),
		})
		return
	}

	// Cancel the matching active request by its cancel token.
	// Unknown or already-completed tokens: OK (idempotent cancel).
	s.activeCancelsMu.Lock()
	cancel, ok := s.activeCancels[req.CancelToken]
	if ok {
		cancel()
		delete(s.activeCancels, req.CancelToken)
	}
	s.activeCancelsMu.Unlock()

	writeResponseLocked(writeMu, conn, TransformResponseV2{V: ProtocolVersion, ID: req.ID, OK: true})
}

// handleTransformWork runs the transform pipeline for a single request.
// Pre-flight (ID registration, context creation, cancel-token registration,
// digest verification) is already done synchronously in handleConn before
// this goroutine starts. The reqCtx carries both the session-scoped
// cancellation and a per-request timeout.
func (s *Session) handleTransformWork(reqCtx context.Context, conn net.Conn, req *TransformRequestV2, writeMu *sync.Mutex) {
	trace.Emit(reqCtx, trace.CatTransform, trace.TypeTransformRequest, trace.TransformData{
		RequestID:  req.ID,
		Format:     req.Format,
		Loader:     req.Loader,
		SourceSize: int64(len(req.Source)),
	})

	// Acquire worker slot (context-aware — respects cancellation while waiting).
	select {
	case s.workers <- struct{}{}:
		defer func() { <-s.workers }()
	case <-reqCtx.Done():
		trace.Emit(reqCtx, trace.CatTransform, trace.TypeTransformCancel, trace.TransformData{
			RequestID: req.ID,
		})
		writeResponseLocked(writeMu, conn, TransformResponseV2{
			V: ProtocolVersion, ID: req.ID, OK: false,
			ErrCode: string(apperr.TransformCancelled), Error: "transform cancelled",
		})
		return
	}

	s.active.Add(1)
	defer s.active.Add(-1)

	// Parse options (digests were already verified in the read loop).
	var opts NormalizedOptions
	if req.Options != "" {
		if err := json.Unmarshal([]byte(req.Options), &opts); err != nil {
			writeResponseLocked(writeMu, conn, TransformResponseV2{
				V: ProtocolVersion, ID: req.ID, OK: false,
				ErrCode: string(apperr.TransformConfigOption),
				Error:   SanitizeErrorMessage(fmt.Sprintf("invalid options: %v", err)),
			})
			return
		}
	}

	sourceMapMode := SourceMapNone
	switch req.SourceMap {
	case "inline":
		sourceMapMode = SourceMapInline
	case "external":
		sourceMapMode = SourceMapExternal
	}

	tReq := TransformRequest{
		RequestID:       req.ID,
		SourcePath:      req.Path,
		SourceBytes:     []byte(req.Source),
		SourceDigest:    req.SourceDigest,
		Loader:          LoaderKind(req.Loader),
		Format:          mapFormatString(req.Format),
		NormalizedOpts:  opts,
		OptsDigest:      req.OptsDigest,
		TargetNodeMajor: req.NodeMajor,
		SourceMapMode:   sourceMapMode,
	}

	// Check transform cache.
	var result *TransformResult
	var resultErr error
	if s.cacheDir != "" {
		identity := s.engine.Identity()
		key := CacheKey(tReq, identity)
		trace.Emit(reqCtx, trace.CatCache, trace.TypeCacheLookup, trace.CacheData{Key: key})
		cached, cerr := TryReadCache(s.cacheDir, key)
		if cerr != nil {
			if !isCacheCorruption(cerr) {
				resultErr = cerr
			}
			trace.Emit(reqCtx, trace.CatCache, trace.TypeCacheCorrupt, trace.CacheData{
				Key:    key,
				Reason: trace.RedactError(cerr),
			})
		} else if cached != nil {
			result = cached
			trace.Emit(reqCtx, trace.CatCache, trace.TypeCacheHit, trace.CacheData{
				Key:       key,
				SchemaVer: CacheSchemaVersion,
			})
		} else {
			trace.Emit(reqCtx, trace.CatCache, trace.TypeCacheMiss, trace.CacheData{Key: key})
		}
	}

	// Cache miss, corruption, or cache disabled: run engine.
	if result == nil && resultErr == nil {
		trace.Emit(reqCtx, trace.CatTransform, trace.TypeTransformEngine, trace.TransformData{
			RequestID: req.ID,
			Format:    string(tReq.Format),
			Loader:    string(tReq.Loader),
		})
		engineResult, engineErr := s.engine.Transform(reqCtx, tReq)
		if engineErr == nil {
			result = &engineResult
			if s.cacheDir != "" {
				identity := s.engine.Identity()
				key := CacheKey(tReq, identity)
				if werr := WriteCache(s.cacheDir, key, &engineResult); werr != nil {
					resultErr = werr
					trace.Emit(reqCtx, trace.CatCache, trace.TypeCacheRejection, trace.CacheData{
						Key:    key,
						Reason: trace.RedactError(werr),
					})
				} else {
					trace.Emit(reqCtx, trace.CatCache, trace.TypeCacheWrite, trace.CacheData{Key: key})
				}
			}
		} else {
			resultErr = engineErr
		}
	}

	// ── Terminal response gate ──────────────────────────────────────
	// After engine returns, exactly one terminal response must be sent.
	// Race policy: cancel and completion compete under activeCancelsMu.
	// The lock acts as the commit point — whoever holds it first decides
	// whether this request was cancelled.
	//
	// If cancel wins: token removed from map, cancel() called.
	//   → reqCtx.Err() is non-nil, or token is absent → cancel response.
	// If transform wins: token removed from map, success response written.
	//   → cancel arrives later, sees no token → idempotent no-op.
	s.activeCancelsMu.Lock()
	_, stillActive := s.activeCancels[req.CancelToken]
	cancelled := reqCtx.Err() != nil
	if stillActive && !cancelled {
		// Commit to success: remove token so late cancel is a no-op.
		delete(s.activeCancels, req.CancelToken)
	}
	s.activeCancelsMu.Unlock()

	if !stillActive || cancelled {
		// Request was cancelled or timed out.
		code := string(apperr.TransformCancelled)
		msg := "transform cancelled"
		if reqCtx.Err() != nil && stillActive {
			// Still registered but context done → timeout won the race.
			code = string(apperr.TransformTimeout)
			msg = "transform timeout"
		}
		trace.Emit(reqCtx, trace.CatTransform, trace.TypeTransformCancel, trace.TransformData{
			RequestID: req.ID,
			ErrorCode: code,
		})
		writeResponseLocked(writeMu, conn, TransformResponseV2{
			V: ProtocolVersion, ID: req.ID, OK: false,
			ErrCode: code, Error: msg,
		})
		return
	}

	if resultErr != nil {
		trace.Emit(reqCtx, trace.CatTransform, trace.TypeTransformError, trace.TransformData{
			RequestID:    req.ID,
			ErrorCode:    SanitizeErrorCode(string(apperr.CodeOf(resultErr))),
			ErrorMessage: SanitizeErrorMessage(resultErr.Error()),
		})
		writeResponseLocked(writeMu, conn, TransformResponseV2{
			V: ProtocolVersion, ID: req.ID, OK: false,
			ErrCode: SanitizeErrorCode(string(apperr.CodeOf(resultErr))),
			Error:   SanitizeErrorMessage(resultErr.Error()),
		})
		return
	}

	cacheStr := "miss"
	switch result.CacheStatus {
	case CacheStatusHit:
		cacheStr = "hit"
	case CacheStatusBypass:
		cacheStr = "bypass"
	}

	trace.Emit(reqCtx, trace.CatTransform, trace.TypeTransformComplete, trace.TransformData{
		RequestID:   req.ID,
		CacheStatus: cacheStr,
		DurationMs:  result.Elapsed.Milliseconds(),
	})

	writeResponseLocked(writeMu, conn, TransformResponseV2{
		V:      ProtocolVersion,
		ID:     req.ID,
		OK:     true,
		Code:   string(result.Code),
		Map:    string(result.SourceMap),
		Digest: result.OutputDigest,
		Cache:  cacheStr,
	})
}

// Close initiates coordinated shutdown. It is idempotent and concurrency-safe.
// Concurrent callers block until the first caller completes shutdown, then
// all return the same effective error.
//
// Shutdown order:
//  1. Cancel session context — propagates to all derived contexts.
//  2. Close listener — stops the accept loop.
//  3. Cancel active transforms — unblocks workers.
//  4. Close tracked connections — unblocks reads/writes.
//  5. Wait for all tracked goroutines to finish.
//  6. Clean up remaining maps.
//
// Returns the listener close error, or nil. Connection close errors are
// not aggregated (they are expected side-effects of listener shutdown).
func (s *Session) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		// Another goroutine is shutting down; wait for completion
		// and return the same effective result.
		<-s.closeDone
		return s.closeErr
	}
	defer close(s.closeDone) // unblocks concurrent callers

	// 1. Cancel session context.
	s.cancel()

	// 2. Close listener (may race with serve's deferred close or
	// ctx-cancellation goroutine — "use of closed network connection"
	// is benign in that case).
	var closeErr error
	if s.listener != nil {
		closeErr = s.listener.Close()
		if closeErr != nil && isClosedNetworkErr(closeErr) {
			closeErr = nil
		}
	}

	// 3. Cancel all active transforms.
	s.activeCancelsMu.Lock()
	for _, cancel := range s.activeCancels {
		cancel()
	}
	s.activeCancelsMu.Unlock()

	// 4. Close active connections to unblock reads/writes.
	s.connsMu.Lock()
	for c := range s.conns {
		_ = c.Close()
	}
	s.connsMu.Unlock()

	// 5. Wait for server, connection, and request goroutines.
	s.wg.Wait()

	// 6. Clean up remaining active cancel/ID entries.
	s.activeCancelsMu.Lock()
	for k := range s.activeCancels {
		delete(s.activeCancels, k)
	}
	s.activeCancelsMu.Unlock()
	s.activeIDsMu.Lock()
	for k := range s.activeIDs {
		delete(s.activeIDs, k)
	}
	s.activeIDsMu.Unlock()

	s.closeErr = closeErr
	return s.closeErr
}

// ActiveRequests returns the current in-flight request count.
func (s *Session) ActiveRequests() int32 {
	return s.active.Load()
}

// mapFormatString converts a protocol format string to ModuleFormat.
func mapFormatString(f string) ModuleFormat {
	switch f {
	case "cjs":
		return FormatCJS
	default:
		return FormatESM
	}
}

// generateToken returns a random hex token with byteLen random bytes.
func generateToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// isClosedNetworkErr reports whether err is "use of closed network connection",
// which is expected when Close and serve's deferred close race.
func isClosedNetworkErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "use of closed network connection")
}

// isCacheCorruption reports whether err is a recoverable cache corruption
// (entry was cleaned up, caller should regenerate). Permission and I/O
// errors are NOT corruption — they indicate a disk problem.
func isCacheCorruption(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "cache code digest mismatch") ||
		strings.Contains(msg, "cache map digest mismatch") ||
		strings.Contains(msg, "cache code missing") ||
		strings.Contains(msg, "cache map missing") ||
		strings.Contains(msg, "corrupt cache meta") ||
		strings.Contains(msg, "cache output digest mismatch") ||
		strings.Contains(msg, "invalid cache key shape")
}
