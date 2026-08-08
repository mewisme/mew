// Package trace provides lightweight context-keyed spans and structured runtime
// trace events without OpenTelemetry.
package trace

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
	"time"
)

// SchemaVersion is the canonical trace event schema version.
// Increment on breaking event shape changes; backward-compatible additions
// (new categories, new optional fields) do not require a bump.
const SchemaVersion = 1

// Category classifies a trace event into its operational domain.
type Category string

const (
	CatLifecycle  Category = "lifecycle"
	CatTransform  Category = "transform"
	CatCache      Category = "cache"
	CatEnv        Category = "env"
	CatResolution Category = "resolution"
	CatWorker     Category = "worker"
	CatWatch      Category = "watch"
)

// EventType names a specific event within a category.
type EventType string

// Lifecycle events.
const (
	TypePlanStart    EventType = "plan.start"
	TypePlanComplete EventType = "plan.complete"
	TypePlanError    EventType = "plan.error"
	TypeLaunchStart  EventType = "launch.start"
	TypeLaunchExit   EventType = "launch.exit"
	TypeLaunchError  EventType = "launch.error"
	TypeCancel       EventType = "cancel"
	TypeCleanupStart EventType = "cleanup.start"
	TypeCleanupDone  EventType = "cleanup.done"
	TypeCleanupError EventType = "cleanup.error"
)

// Transform events.
const (
	TypeTransformRequest   EventType = "transform.request"
	TypeTransformCacheHit  EventType = "transform.cache_hit"
	TypeTransformCacheMiss EventType = "transform.cache_miss"
	TypeTransformEngine    EventType = "transform.engine"
	TypeTransformComplete  EventType = "transform.complete"
	TypeTransformError     EventType = "transform.error"
	TypeTransformCancel    EventType = "transform.cancel"
)

// Cache events.
const (
	TypeCacheLookup    EventType = "cache.lookup"
	TypeCacheHit       EventType = "cache.hit"
	TypeCacheMiss      EventType = "cache.miss"
	TypeCacheWrite     EventType = "cache.write"
	TypeCacheCorrupt   EventType = "cache.corrupt"
	TypeCacheRejection EventType = "cache.rejection"
)

// Env events.
const (
	TypeEnvSource     EventType = "env.source"
	TypeEnvModeSelect EventType = "env.mode_select"
	TypeEnvPrecedence EventType = "env.precedence"
	TypeEnvError      EventType = "env.error"
)

// Resolution events.
const (
	TypeResolveStage    EventType = "resolution.stage"
	TypeResolveComplete EventType = "resolution.complete"
	TypeResolveError    EventType = "resolution.error"
)

// Worker/child events.
const (
	TypeWorkerStart  EventType = "worker.start"
	TypeWorkerExit   EventType = "worker.exit"
	TypeWorkerError  EventType = "worker.error"
	TypeWorkerCancel EventType = "worker.cancel"
)

// Watch events.
const (
	TypeWatchStart      EventType = "watch.start"
	TypeWatchRestart    EventType = "watch.restart"
	TypeWatchInvalidate EventType = "watch.invalidate"
	TypeWatchBackend    EventType = "watch.backend"
	TypeWatchChildStart EventType = "watch.child_start"
	TypeWatchChildExit  EventType = "watch.child_exit"
	TypeWatchShutdown   EventType = "watch.shutdown"
	TypeWatchError      EventType = "watch.error"
)

// Event is a single structured trace event.
// The envelope fields (V, TS, Session, Seq, Cat, Type) are present on every
// event. Data carries category-specific typed payloads.
type Event struct {
	V       int       `json:"v"`
	TS      time.Time `json:"ts"`
	Session string    `json:"session"`
	Seq     uint64    `json:"seq"`
	Cat     Category  `json:"cat"`
	Type    EventType `json:"type"`
	Data    any       `json:"data,omitempty"`
}

// LifecycleData is the payload for lifecycle events.
type LifecycleData struct {
	Entrypoint   string `json:"entrypoint,omitempty"`
	NodeVersion  string `json:"node_version,omitempty"`
	Augmented    bool   `json:"augmented,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	// Inspector fields are populated when --inspect or --inspect-brk is active.
	InspectorMode string `json:"inspector_mode,omitempty"` // "run" or "brk"
	InspectorHost string `json:"inspector_host,omitempty"` // sanitized bind address
	InspectorPort int    `json:"inspector_port,omitempty"` // 0 when auto/unspecified
}

// TransformData is the payload for transform events.
type TransformData struct {
	RequestID   string `json:"request_id,omitempty"`
	Format      string `json:"format,omitempty"`
	Loader      string `json:"loader,omitempty"`
	Target      string `json:"target,omitempty"`
	CacheStatus string `json:"cache_status,omitempty"`
	SourceSize  int64  `json:"source_size,omitempty"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
	ErrorCode   string `json:"error_code,omitempty"`
	// ErrorMessage is sanitized before emission.
	ErrorMessage string `json:"error_message,omitempty"`
}

// CacheData is the payload for cache events.
type CacheData struct {
	Key        string `json:"key,omitempty"`
	SchemaVer  int    `json:"schema_ver,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	// Reason describes why a cache entry was rejected or corrupt.
	Reason string `json:"reason,omitempty"`
}

// EnvData is the payload for env events.
// Never includes environment variable values.
type EnvData struct {
	// SourceFile is the path to the discovered .env file (project-relative when possible).
	SourceFile string `json:"source_file,omitempty"`
	// Mode is the resolved mode (e.g. "development", "production").
	Mode string `json:"mode,omitempty"`
	// Keys is the list of env var names discovered, never values.
	Keys []string `json:"keys,omitempty"`
	// Precedence describes the resolution order (e.g. "host", "explicit", "discovered").
	Precedence string `json:"precedence,omitempty"`
	// ErrorCode when env loading fails.
	ErrorCode string `json:"error_code,omitempty"`
}

// ResolutionData is the payload for resolution events.
type ResolutionData struct {
	Stage      string `json:"stage,omitempty"`
	Package    string `json:"package,omitempty"`
	Version    string `json:"version,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Count      int    `json:"count,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
}

// WorkerData is the payload for worker/child events.
type WorkerData struct {
	// ChildPID is the OS process ID of the child (0 when not available).
	ChildPID int `json:"child_pid,omitempty"`
	// ExitCode from the child process.
	ExitCode *int `json:"exit_code,omitempty"`
	// DurationMs is wall-clock time the child ran.
	DurationMs int64 `json:"duration_ms,omitempty"`
	// ErrorCode when the child fails.
	ErrorCode string `json:"error_code,omitempty"`
}

// WatchData is the payload for watch events.
type WatchData struct {
	// Reason describes why a restart occurred.
	Reason string `json:"reason,omitempty"`
	// Backend identifies the watcher backend (e.g. "native", "polling").
	Backend string `json:"backend,omitempty"`
	// ChangedPath is the path that triggered a change (project-relative when possible).
	ChangedPath string `json:"changed_path,omitempty"`
	// Generation is a monotonic restart counter.
	Generation int `json:"generation,omitempty"`
	// AddedPaths and RemovedPaths describe watch coverage changes.
	AddedPaths   []string `json:"added_paths,omitempty"`
	RemovedPaths []string `json:"removed_paths,omitempty"`
	// ExitCode from the watched child.
	ExitCode *int `json:"exit_code,omitempty"`
	// ErrorCode when a watch operation fails.
	ErrorCode string `json:"error_code,omitempty"`
}

// newSessionID generates an opaque hex session correlation identifier.
func newSessionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback determinism: use a fixed prefix so tests can match.
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}

// Session carries the identity and sequence for a trace session.
// A nil *Session is safe: it acts as a no-op emitter.
type Session struct {
	ID   string
	Sink Sink
	seq  atomic.Uint64
}

// NewSession creates a trace session with a random correlation ID.
func NewSession() *Session {
	return &Session{ID: newSessionID()}
}

// NewSessionWithID creates a trace session with a known ID (for testing).
func NewSessionWithID(id string) *Session {
	return &Session{ID: id}
}

// nextSeq returns the next monotonic sequence number for this session.
func (s *Session) nextSeq() uint64 {
	if s == nil {
		return 0
	}
	return s.seq.Add(1)
}

// NewEvent creates an event with envelope fields populated from the session.
func (s *Session) NewEvent(cat Category, typ EventType, data any) Event {
	seq := uint64(0)
	if s != nil {
		seq = s.nextSeq()
	}
	sid := ""
	if s != nil {
		sid = s.ID
	}
	return Event{
		V:       SchemaVersion,
		TS:      time.Now(),
		Session: sid,
		Seq:     seq,
		Cat:     cat,
		Type:    typ,
		Data:    data,
	}
}
