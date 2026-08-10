package presentation

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mewisme/mew/internal/diagnostics"
)

// unicodeActivityFrames are the Braille-inspired spinner glyphs for rich output.
var unicodeActivityFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// asciiActivityFrames are the safe fallback spinner glyphs.
var asciiActivityFrames = []string{"|", "/", "-", "\\"}

// ActivityProgressRenderer is a gh-style transient single-line progress sink
// for stderr. It starts lazily, redraws in place with \r + CSI K, truncates
// to the terminal width, and never owns stdin, signals, or the alt screen.
type ActivityProgressRenderer struct {
	mu sync.Mutex
	w  io.Writer

	settings EffectiveSettings
	frames   []string
	interval time.Duration

	// Active operations keyed by ID.
	ops       map[string]*activityOp
	currentID string

	frameIdx int

	started   bool
	suspended bool
	closed    bool
	onScreen  bool  // true when a transient line is currently displayed
	writeErr  error // first write failure latches permanently

	stop chan struct{}
	done chan struct{}
}

type activityOp struct {
	Kind        string
	Completed   int64
	Total       *int64
	CurrentItem string
	Unit        string
	Detail      string
}

// NewActivityProgressRenderer prepares a single-line progress sink writing to w (stderr).
func NewActivityProgressRenderer(w io.Writer, settings EffectiveSettings) *ActivityProgressRenderer {
	if w == nil {
		w = io.Discard
	}
	frames := unicodeActivityFrames
	if !settings.UseUnicode {
		frames = asciiActivityFrames
	}
	return &ActivityProgressRenderer{
		w:        w,
		settings: settings,
		frames:   frames,
		interval: 100 * time.Millisecond,
		ops:      make(map[string]*activityOp),
	}
}

// OperationStarted registers a new operation and starts the spinner if idle.
func (r *ActivityProgressRenderer) OperationStarted(ev diagnostics.OperationStartedEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.suspended {
		return
	}
	kind := strings.TrimSpace(ev.Kind)
	if kind == "" {
		kind = strings.TrimSpace(ev.Label)
	}
	if kind == "" {
		return
	}
	r.ops[ev.ID] = &activityOp{
		Kind:  kind,
		Total: ev.Total,
		Unit:  ev.Unit,
	}
	r.currentID = ev.ID
	r.ensureStartedLocked()
	r.drawLocked()
}

// OperationProgress updates the current operation counts.
func (r *ActivityProgressRenderer) OperationProgress(ev diagnostics.OperationProgressEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.suspended {
		return
	}
	op := r.ops[ev.ID]
	if op == nil {
		return
	}
	op.Completed = ev.Completed
	op.Total = ev.Total
	op.CurrentItem = ev.CurrentItem
	op.Unit = ev.Unit
	op.Detail = ev.Detail
	r.currentID = ev.ID
	r.ensureStartedLocked()
	r.drawLocked()
}

// OperationCompleted removes the operation and picks the next active one.
func (r *ActivityProgressRenderer) OperationCompleted(ev diagnostics.OperationCompletedEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.suspended {
		return
	}
	delete(r.ops, ev.ID)
	if r.currentID == ev.ID {
		r.currentID = r.pickCurrentLocked()
	}
	if len(r.ops) == 0 {
		r.clearLineLocked()
		return
	}
	r.drawLocked()
}

// Notice writes a durable line. During an active spinner it clears the
// transient line, prints the notice, and redraws the frame below it.
func (r *ActivityProgressRenderer) Notice(ev diagnostics.NoticeEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	msg := strings.TrimSpace(ev.Message)
	if msg == "" {
		return
	}
	sym := r.settings.Symbols.Warning
	if r.settings.UseColor {
		theme := NewTheme(r.settings.ThemeMode)
		sym = theme.Warning.Sprint(sym)
	}
	if r.started && len(r.ops) > 0 {
		r.clearLineLocked()
		r.writeLocked(sym + " " + msg + "\n")
		r.drawLocked()
		return
	}
	r.writeLocked(sym + " " + msg + "\n")
}

// Suspend stops the ticker and clears the live line.
func (r *ActivityProgressRenderer) Suspend() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suspended = true
	r.stopTickerLocked()
	r.clearLineLocked()
}

// Resume restarts the ticker if operations remain.
func (r *ActivityProgressRenderer) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suspended = false
	if r.closed {
		return
	}
	if len(r.ops) > 0 {
		r.ensureStartedLocked()
		r.drawLocked()
	}
}

// Close stops the ticker, clears the line, and waits for the goroutine.
// It is idempotent and safe to call after a write error.
func (r *ActivityProgressRenderer) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	r.stopTickerLocked()
	r.clearLineLocked()
	done := r.done
	r.mu.Unlock()

	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}

// ensureStartedLocked creates the ticker goroutine on first use.
// Must be called with r.mu held.
func (r *ActivityProgressRenderer) ensureStartedLocked() {
	if r.started {
		return
	}
	r.started = true
	r.stop = make(chan struct{})
	r.done = make(chan struct{})
	go r.tickerLoop(r.stop, r.done)
}

// tickerLoop writes a frame every interval until stop is closed.
func (r *ActivityProgressRenderer) tickerLoop(stop, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			r.mu.Lock()
			if r.closed || r.suspended || len(r.ops) == 0 {
				r.mu.Unlock()
				continue
			}
			r.frameIdx++
			r.drawLocked()
			r.mu.Unlock()
		}
	}
}

// stopTickerLocked signals the goroutine and waits for it to exit.
// Must be called with r.mu held.
func (r *ActivityProgressRenderer) stopTickerLocked() {
	if r.stop == nil {
		return
	}
	select {
	case <-r.stop:
		// Already closed.
	default:
		close(r.stop)
	}
	done := r.done
	r.stop = nil
	r.done = nil
	r.started = false
	r.mu.Unlock()
	if done != nil {
		<-done
	}
	r.mu.Lock()
}

// drawLocked writes the current frame line to the writer.
// Must be called with r.mu held.
func (r *ActivityProgressRenderer) drawLocked() {
	if r.writeErr != nil {
		return
	}
	op := r.ops[r.currentID]
	if op == nil {
		// Pick the first available op as fallback.
		r.currentID = r.pickCurrentLocked()
		op = r.ops[r.currentID]
	}
	if op == nil {
		return
	}

	frame := r.frames[r.frameIdx%len(r.frames)]
	if r.settings.UseColor {
		theme := NewTheme(r.settings.ThemeMode)
		frame = theme.Info.Sprint(frame)
	}

	label := op.Kind
	if op.CurrentItem != "" {
		label += " " + op.CurrentItem
	}
	if op.Completed > 0 && op.Total != nil && *op.Total > 0 {
		label += fmt.Sprintf(" [%d/%d]", op.Completed, *op.Total)
	} else if op.Total != nil && *op.Total > 0 {
		label += fmt.Sprintf(" [0/%d]", *op.Total)
	}
	if op.CurrentItem == "" && op.Detail != "" {
		label += " " + op.Detail
	}

	line := r.buildLine(frame, label)
	r.writeLocked(line)
	r.onScreen = true
}

// buildLine assembles a single active frame with width truncation.
func (r *ActivityProgressRenderer) buildLine(frame, label string) string {
	// Reserve space for: frame + space + label
	frameWidth := CellWidth(frame)
	maxLabel := r.settings.Width - frameWidth - 1
	if maxLabel < 10 {
		maxLabel = 10
	}
	if CellWidth(label) > maxLabel {
		ellipsis := r.settings.Symbols.Ellipsis
		label = MiddleTruncate(label, maxLabel, ellipsis)
	}
	return "\r\x1b[K" + frame + " " + label
}

// clearLineLocked writes the clear-sequence to remove the transient line.
// Only writes when a line is currently displayed. Must be called with r.mu held.
func (r *ActivityProgressRenderer) clearLineLocked() {
	if r.writeErr != nil || !r.onScreen {
		return
	}
	r.writeLocked("\r\x1b[K")
	r.onScreen = false
}

// writeLocked writes bytes to the underlying writer.
// The first error latches permanently to prevent further writes.
// Must be called with r.mu held.
func (r *ActivityProgressRenderer) writeLocked(s string) {
	if r.writeErr != nil {
		return
	}
	_, err := fmt.Fprint(r.w, s)
	if err != nil {
		r.writeErr = err
	}
}

// pickCurrentLocked selects the most recently touched remaining operation.
// Must be called with r.mu held.
func (r *ActivityProgressRenderer) pickCurrentLocked() string {
	if len(r.ops) == 0 {
		return ""
	}
	// Return any remaining op (predicate: first key is fine since we
	// only display one line and the most-recently-updated is tracked via
	// currentID on each event. When the current one completes, any
	// survivor is acceptable.)
	for id := range r.ops {
		return id
	}
	return ""
}
