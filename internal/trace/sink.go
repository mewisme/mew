package trace

import (
	"encoding/json"
	"io"
	"sync"
)

// ChannelSink is a bounded buffered Sink backed by a channel.
// When the buffer is full, the oldest event is dropped (ring-buffer
// behavior) to prevent deadlocking runtime execution.
// A nil *ChannelSink is a no-op Sink.
type ChannelSink struct {
	mu   sync.Mutex
	ch   chan Event
	done chan struct{}
	w    io.Writer // when non-nil, events are serialized as NDJSON (one per line)
	wg   sync.WaitGroup
}

// NewChannelSink creates a buffered sink with the given capacity.
// capacity of 0 means unbuffered (blocks until consumed).
func NewChannelSink(capacity int) *ChannelSink {
	return &ChannelSink{
		ch:   make(chan Event, capacity),
		done: make(chan struct{}),
	}
}

// NewChannelSinkWriter creates a buffered sink that serializes events as
// NDJSON to w in a background goroutine. Close must be called to flush
// and wait for the write loop to finish.
func NewChannelSinkWriter(capacity int, w io.Writer) *ChannelSink {
	cs := &ChannelSink{
		ch:   make(chan Event, capacity),
		done: make(chan struct{}),
		w:    w,
	}
	cs.wg.Add(1)
	go cs.writeLoop()
	return cs
}

// Send enqueues an event. If the buffer is full, the oldest buffered event
// is dropped to make room (ring-buffer semantics). Never blocks.
func (cs *ChannelSink) Send(ev Event) {
	if cs == nil {
		return
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	select {
	case <-cs.done:
		return
	default:
	}
	for {
		select {
		case cs.ch <- ev:
			return
		default:
			// Buffer full — drain oldest event.
			select {
			case <-cs.ch:
			default:
			}
		}
	}
}

// Events returns the receive side of the event channel. Close the sink
// before ranging to get a clean close.
func (cs *ChannelSink) Events() <-chan Event {
	if cs == nil {
		return nil
	}
	return cs.ch
}

// Close signals the sink to stop accepting events and, for writer-backed
// sinks, waits for the write loop to flush and exit. Safe to call multiple
// times.
func (cs *ChannelSink) Close() error {
	if cs == nil {
		return nil
	}
	cs.mu.Lock()
	select {
	case <-cs.done:
		cs.mu.Unlock()
		return nil
	default:
		close(cs.done)
		close(cs.ch)
		cs.mu.Unlock()
	}
	cs.wg.Wait()
	return nil
}

// writeLoop serializes events as NDJSON lines.
func (cs *ChannelSink) writeLoop() {
	defer cs.wg.Done()
	enc := json.NewEncoder(cs.w)
	enc.SetEscapeHTML(false)
	for ev := range cs.ch {
		if err := enc.Encode(ev); err != nil {
			return
		}
	}
}
