package trace

import "context"

// Emitter receives trace events. A nil *Session implements Emitter as a no-op.
// Implementations must be safe for concurrent use.
type Emitter interface {
	Emit(ctx context.Context, cat Category, typ EventType, data any)
}

// Emit sends an event through the session. If s is nil, this is a no-op.
func (s *Session) Emit(_ context.Context, cat Category, typ EventType, data any) {
	if s == nil {
		return
	}
	ev := s.NewEvent(cat, typ, data)
	s.deliver(ev)
}

// deliver sends the event to the configured sink. The base implementation
// is a no-op; set Session.Sink to capture events.
func (s *Session) deliver(ev Event) {
	if s == nil || s.Sink == nil {
		return
	}
	s.Sink.Send(ev)
}

// Sink is the destination for emitted events.
type Sink interface {
	Send(ev Event)
}

// Emit is a convenience function that emits through the session on ctx, or
// no-ops when no session is present. Prefer (*Session).Emit for hot paths
// where the session is already available; use this for call sites that only
// have a context.
func Emit(ctx context.Context, cat Category, typ EventType, data any) {
	if ctx == nil {
		return
	}
	s := SessionFrom(ctx)
	s.Emit(ctx, cat, typ, data)
}

// contextKey is the type used for context keys in this package.
type contextKey struct{ name string }

var sessionKey = contextKey{name: "trace-session"}

// WithSession returns a context carrying the trace session.
func WithSession(ctx context.Context, s *Session) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, sessionKey, s)
}

// SessionFrom returns the trace session on ctx, or nil.
func SessionFrom(ctx context.Context) *Session {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(sessionKey).(*Session)
	return s
}
