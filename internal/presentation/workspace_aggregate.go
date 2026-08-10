package presentation

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mewisme/mew/internal/diagnostics"
)

// WorkspaceTaskRow is one compact workspace status row.
type WorkspaceTaskRow struct {
	Package  string
	Script   string
	Status   string
	Exit     *int
	Index    int
	Duration time.Duration
	Failed   bool
}

// WorkspaceAggregateRenderer writes append-only task status lines and a final summary.
// It never repaints prior rows (avoids cursor fights with aggregate child dumps).
type WorkspaceAggregateRenderer struct {
	mu        sync.Mutex
	out       io.Writer
	settings  EffectiveSettings
	suspended bool
	closed    bool
	disabled  bool // inherit/interactive: no live status
	order     []int
	byIndex   map[int]*WorkspaceTaskRow
	started   map[int]time.Time
	clock     func() time.Time
}

// NewWorkspaceAggregateRenderer builds an append-only workspace status sink.
func NewWorkspaceAggregateRenderer(w io.Writer, settings EffectiveSettings) *WorkspaceAggregateRenderer {
	if w == nil {
		w = io.Discard
	}
	return &WorkspaceAggregateRenderer{
		out:      w,
		settings: settings,
		byIndex:  map[int]*WorkspaceTaskRow{},
		started:  map[int]time.Time{},
		clock:    time.Now,
	}
}

// DisableLiveStatus turns off append-only status (inherit/raw TTY tasks).
func (r *WorkspaceAggregateRenderer) DisableLiveStatus() {
	r.mu.Lock()
	r.disabled = true
	r.mu.Unlock()
}

// Suspend pauses status lines while a child owns the terminal.
func (r *WorkspaceAggregateRenderer) Suspend() {
	r.mu.Lock()
	r.suspended = true
	r.mu.Unlock()
}

// Resume restores status emission after child exit.
func (r *WorkspaceAggregateRenderer) Resume() {
	r.mu.Lock()
	r.suspended = false
	r.mu.Unlock()
}

// WorkspaceTask records and optionally emits a compact status line.
func (r *WorkspaceAggregateRenderer) WorkspaceTask(ev diagnostics.WorkspaceTaskEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	row := r.byIndex[ev.Index]
	if row == nil {
		row = &WorkspaceTaskRow{Index: ev.Index}
		r.byIndex[ev.Index] = row
		r.order = append(r.order, ev.Index)
	}
	if ev.Package != "" {
		row.Package = ev.Package
	}
	if ev.Script != "" {
		row.Script = ev.Script
	}
	row.Status = normalizeTaskStatus(ev.Status)
	row.Exit = ev.Exit
	now := r.clock()
	switch row.Status {
	case "running", "start":
		row.Status = "running"
		if _, ok := r.started[ev.Index]; !ok {
			r.started[ev.Index] = now
		}
	case "done", "fail", "skip", "cancel", "not-run":
		if t0, ok := r.started[ev.Index]; ok {
			row.Duration = now.Sub(t0)
		}
		row.Failed = row.Status == "fail"
	}
	if r.disabled || r.suspended {
		return
	}
	r.writeln(formatTaskRow(row, r.settings))
}

// WorkspaceSummary emits the final failed list and counts.
func (r *WorkspaceAggregateRenderer) WorkspaceSummary(ev diagnostics.WorkspaceSummaryEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	indexes := append([]int(nil), r.order...)
	sort.Ints(indexes)
	// Prefer recorded report order (insertion), not sorted by index alone —
	// keep insertion order for failed list.
	failed := make([]*WorkspaceTaskRow, 0)
	for _, idx := range r.order {
		row := r.byIndex[idx]
		if row != nil && row.Status == "fail" {
			failed = append(failed, row)
		}
	}
	theme := NewTheme(r.settings.ThemeMode)
	var b strings.Builder
	if ev.Failed > 0 {
		b.WriteString(RenderSemanticSymbol(r.settings.Symbols, theme, StatusError, r.settings.UseColor))
		b.WriteByte(' ')
		fmt.Fprintf(&b, "%d of %d tasks failed", ev.Failed, totalTasks(ev))
		b.WriteByte('\n')
		if len(failed) > 0 {
			b.WriteString("\n  Failed\n")
			for _, row := range failed {
				b.WriteString("  ")
				b.WriteString(row.Package)
				b.WriteString("  ")
				b.WriteString(row.Script)
				if row.Exit != nil {
					fmt.Fprintf(&b, "  exit %d", *row.Exit)
				}
				b.WriteByte('\n')
			}
		}
		b.WriteByte('\n')
	} else {
		b.WriteString(RenderSemanticSymbol(r.settings.Symbols, theme, StatusSuccess, r.settings.UseColor))
		b.WriteByte(' ')
		fmt.Fprintf(&b, "%d tasks completed", ev.Completed)
		b.WriteByte('\n')
		b.WriteByte('\n')
	}
	b.WriteString(formatSummaryCounts(ev, r.settings))
	r.writeln(strings.TrimRight(b.String(), "\n"))
}

func totalTasks(ev diagnostics.WorkspaceSummaryEvent) int {
	return ev.Completed + ev.Failed + ev.Cancelled + ev.Skipped + ev.NotRun
}

func formatSummaryCounts(ev diagnostics.WorkspaceSummaryEvent, settings EffectiveSettings) string {
	rows := []KeyValue{
		{Key: "Completed", Value: fmt.Sprintf("%d", ev.Completed), Style: ValueNumber},
		{Key: "Failed", Value: fmt.Sprintf("%d", ev.Failed), Style: ValueNumber},
		{Key: "Not run", Value: fmt.Sprintf("%d", ev.NotRun), Style: ValueNumber},
	}
	if ev.Cancelled > 0 {
		rows = append(rows, KeyValue{Key: "Cancelled", Value: fmt.Sprintf("%d", ev.Cancelled), Style: ValueNumber})
	}
	if ev.Skipped > 0 {
		rows = append(rows, KeyValue{Key: "Skipped", Value: fmt.Sprintf("%d", ev.Skipped), Style: ValueNumber})
	}
	return NewStaticRenderer(settings).KeyValues(rows)
}

func formatTaskRow(row *WorkspaceTaskRow, settings EffectiveSettings) string {
	theme := NewTheme(settings.ThemeMode)
	var st Status
	label := row.Status
	switch row.Status {
	case "running", "start":
		st = StatusRunning
		label = "running"
	case "done":
		st = StatusSuccess
		label = ""
	case "fail":
		st = StatusError
		label = ""
	case "skip", "not-run":
		st = StatusSkipped
		if row.Status == "not-run" {
			label = "not run"
		} else {
			label = "skipped"
		}
	case "cancel":
		st = StatusCancelled
		label = "cancelled"
	case "pending":
		st = StatusPending
	default:
		st = StatusPending
	}
	statusSym := RenderSemanticSymbol(settings.Symbols, theme, st, settings.UseColor)
	pkg := row.Package
	if pkg == "" {
		pkg = RenderSymbolRole(settings.Symbols, theme, RolePlaceholder, settings.UseColor)
	}
	script := row.Script
	var b strings.Builder
	b.WriteString(pkg)
	b.WriteString("  ")
	b.WriteString(statusSym)
	if script != "" {
		b.WriteByte(' ')
		b.WriteString(script)
	}
	if label != "" {
		b.WriteByte(' ')
		b.WriteString(label)
	}
	if row.Duration > 0 && (row.Status == "done" || row.Status == "fail") {
		b.WriteByte(' ')
		b.WriteString(FormatDuration(row.Duration))
	}
	return b.String()
}

func normalizeTaskStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "start", "running":
		return "running"
	case "done", "completed", "ok":
		return "done"
	case "fail", "failed", "error":
		return "fail"
	case "skip", "skipped":
		return "skip"
	case "cancel", "cancelled", "canceled":
		return "cancel"
	case "not-run", "notrun", "pending", "queued":
		if s == "pending" || s == "queued" {
			return "pending"
		}
		return "not-run"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func (r *WorkspaceAggregateRenderer) writeln(line string) {
	if line == "" {
		return
	}
	_, _ = fmt.Fprintln(r.out, line)
}
