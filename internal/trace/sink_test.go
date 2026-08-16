package trace_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/trace"
)

func TestChannelSinkSend(t *testing.T) {
	cs := trace.NewChannelSink(4)
	ev := trace.Event{V: 1, Cat: trace.CatLifecycle, Type: trace.TypePlanStart}
	cs.Send(ev)
	cs.Send(ev)
	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}

	count := 0
	for range cs.Events() {
		count++
	}
	if count != 2 {
		t.Fatalf("got %d events, want 2", count)
	}
}

func TestChannelSinkBackpressure(t *testing.T) {
	cs := trace.NewChannelSink(2)
	ev1 := trace.Event{V: 1, Seq: 1}
	ev2 := trace.Event{V: 1, Seq: 2}
	ev3 := trace.Event{V: 1, Seq: 3}
	ev4 := trace.Event{V: 1, Seq: 4}

	cs.Send(ev1)
	cs.Send(ev2)
	// Buffer is full — ev3 and ev4 should evict oldest.
	cs.Send(ev3)
	cs.Send(ev4)

	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}

	var seqs []uint64
	for ev := range cs.Events() {
		seqs = append(seqs, ev.Seq)
	}
	// At least 2 events should be present (one was evicted for each overflow).
	if len(seqs) < 2 {
		t.Fatalf("got %d events, want at least 2", len(seqs))
	}
}

func TestChannelSinkCloseIdempotent(t *testing.T) {
	cs := trace.NewChannelSink(1)
	cs.Send(trace.Event{V: 1})
	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}
	// Second close must not panic.
	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelSinkWriter(t *testing.T) {
	var buf bytes.Buffer
	cs := trace.NewChannelSinkWriter(8, &buf)
	cs.Send(trace.Event{V: 1, Cat: trace.CatLifecycle, Type: trace.TypePlanStart, Session: "test"})
	cs.Send(trace.Event{V: 1, Cat: trace.CatLifecycle, Type: trace.TypeLaunchStart, Session: "test"})
	// Close flushes the write loop and waits for it to finish.
	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("expected output, got empty")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d NDJSON lines, want 2\noutput: %q", len(lines), out)
	}
	for _, line := range lines {
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Errorf("invalid JSON line: %s: %v", line, err)
		}
	}
}

func TestNilChannelSink(t *testing.T) {
	var cs *trace.ChannelSink
	cs.Send(trace.Event{V: 1}) // must not panic
	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}
	if evts := cs.Events(); evts != nil {
		t.Fatal("expected nil channel from nil sink")
	}
}
