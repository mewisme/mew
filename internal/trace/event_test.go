package trace_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mewisme/mew/internal/trace"
)

func TestSchemaVersion(t *testing.T) {
	if trace.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", trace.SchemaVersion)
	}
}

func TestNilSessionNoOp(t *testing.T) {
	var s *trace.Session
	s.Emit(context.Background(), trace.CatLifecycle, trace.TypePlanStart, trace.LifecycleData{Entrypoint: "test.js"})
	// Must not panic, must not allocate.
}

func TestSessionEmit(t *testing.T) {
	s := trace.NewSessionWithID("test-session")
	if s.ID != "test-session" {
		t.Fatalf("ID = %q", s.ID)
	}

	ch := make(chan trace.Event, 4)
	s.Sink = &chanSink{ch: ch}

	ctx := trace.WithSession(context.Background(), s)
	s.Emit(ctx, trace.CatLifecycle, trace.TypePlanStart, trace.LifecycleData{Entrypoint: "app.js"})
	s.Emit(ctx, trace.CatLifecycle, trace.TypeLaunchStart, trace.LifecycleData{Entrypoint: "app.js"})

	close(ch)

	var events []trace.Event
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	// Verify envelope fields.
	for i, ev := range events {
		if ev.V != 1 {
			t.Errorf("event[%d].V = %d", i, ev.V)
		}
		if ev.Session != "test-session" {
			t.Errorf("event[%d].Session = %q", i, ev.Session)
		}
		if ev.Seq != uint64(i+1) {
			t.Errorf("event[%d].Seq = %d, want %d", i, ev.Seq, i+1)
		}
		if ev.TS.IsZero() {
			t.Errorf("event[%d].TS is zero", i)
		}
	}

	// Verify category/type.
	if events[0].Cat != trace.CatLifecycle || events[0].Type != trace.TypePlanStart {
		t.Errorf("event[0] cat=%s type=%s", events[0].Cat, events[0].Type)
	}
	if events[1].Cat != trace.CatLifecycle || events[1].Type != trace.TypeLaunchStart {
		t.Errorf("event[1] cat=%s type=%s", events[1].Cat, events[1].Type)
	}
}

func TestEmitFromContext(t *testing.T) {
	s := trace.NewSessionWithID("ctx-test")
	ch := make(chan trace.Event, 1)
	s.Sink = &chanSink{ch: ch}

	ctx := trace.WithSession(context.Background(), s)
	trace.Emit(ctx, trace.CatEnv, trace.TypeEnvSource, trace.EnvData{Mode: "development"})

	close(ch)
	ev := <-ch
	if ev.Cat != trace.CatEnv || ev.Type != trace.TypeEnvSource {
		t.Errorf("cat=%s type=%s", ev.Cat, ev.Type)
	}
}

func TestEmitNoSession(t *testing.T) {
	// Emit on a context without a session must not panic.
	trace.Emit(context.Background(), trace.CatLifecycle, trace.TypePlanStart, nil)
	trace.Emit(context.TODO(), trace.CatLifecycle, trace.TypePlanStart, nil)
	trace.Emit(context.TODO(), trace.CatLifecycle, trace.TypePlanStart, nil)
}

func TestEventJSONRoundTrip(t *testing.T) {
	code := 0
	ev := trace.Event{
		V:       1,
		Session: "json-test",
		Seq:     1,
		Cat:     trace.CatLifecycle,
		Type:    trace.TypeLaunchExit,
		Data: trace.LifecycleData{
			Entrypoint: "app.ts",
			ExitCode:   &code,
			DurationMs: 150,
			Augmented:  true,
		},
	}

	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded["v"] != float64(1) {
		t.Errorf("v = %v", decoded["v"])
	}
	if decoded["cat"] != "lifecycle" {
		t.Errorf("cat = %v", decoded["cat"])
	}
	if decoded["type"] != "launch.exit" {
		t.Errorf("type = %v", decoded["type"])
	}

	data, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatal("data is not an object")
	}
	if data["entrypoint"] != "app.ts" {
		t.Errorf("entrypoint = %v", data["entrypoint"])
	}
	if data["augmented"] != true {
		t.Errorf("augmented = %v", data["augmented"])
	}
}

func TestDeterministicSequence(t *testing.T) {
	s := trace.NewSessionWithID("seq-test")
	ch := make(chan trace.Event, 10)
	s.Sink = &chanSink{ch: ch}
	ctx := trace.WithSession(context.Background(), s)

	for i := 0; i < 5; i++ {
		s.Emit(ctx, trace.CatLifecycle, trace.TypePlanStart, nil)
	}

	close(ch)
	var seqs []uint64
	for ev := range ch {
		seqs = append(seqs, ev.Seq)
	}
	for i, seq := range seqs {
		if seq != uint64(i+1) {
			t.Errorf("seq[%d] = %d, want %d", i, seq, i+1)
		}
	}
}

// chanSink implements trace.Sink by sending to a channel.
type chanSink struct {
	ch chan trace.Event
}

func (cs *chanSink) Send(ev trace.Event) {
	if cs == nil || cs.ch == nil {
		return
	}
	select {
	case cs.ch <- ev:
	default:
	}
}
