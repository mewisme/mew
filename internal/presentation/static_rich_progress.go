package presentation

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/fatih/color"

	"github.com/mewisme/mew/internal/diagnostics"
)

// StaticRichProgressRenderer writes append-only rich-format lines to stderr.
type StaticRichProgressRenderer struct {
	mu        sync.Mutex
	out       io.Writer
	settings  EffectiveSettings
	theme     Theme
	symbols   Symbols
	suspended bool
	closed    bool
	active    map[string]string // id -> kind
	completed []completedPhase  // accumulated completed phases for summary
}

type completedPhase struct {
	kind       string
	status     string
	durationMs int64
	metrics    []diagnostics.Metric
}

// NewStaticRichProgressRenderer builds an append-only rich progress sink.
func NewStaticRichProgressRenderer(w io.Writer, settings EffectiveSettings) *StaticRichProgressRenderer {
	if w == nil {
		w = io.Discard
	}
	return &StaticRichProgressRenderer{
		out:      w,
		settings: settings,
		theme:    NewTheme(settings.ThemeMode),
		symbols:  settings.Symbols,
		active:   map[string]string{},
	}
}

func (p *StaticRichProgressRenderer) OperationStarted(ev diagnostics.OperationStartedEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.suspended {
		return
	}
	kind := strings.TrimSpace(ev.Kind)
	if kind == "" {
		kind = strings.TrimSpace(ev.Label)
	}
	if kind == "" {
		return
	}
	p.active[ev.ID] = kind

	var b strings.Builder
	b.WriteString(p.colorSymbol(p.theme.Info, p.symbols.Running))
	b.WriteString(" ")
	b.WriteString(kind)
	if ev.Total != nil && *ev.Total > 0 {
		unit := ev.Unit
		if unit == "" {
			unit = "packages"
		}
		fmt.Fprintf(&b, " %d %s", *ev.Total, unit)
	}
	p.writeln(b.String())
}

func (p *StaticRichProgressRenderer) OperationProgress(diagnostics.OperationProgressEvent) {}

func (p *StaticRichProgressRenderer) OperationCompleted(ev diagnostics.OperationCompletedEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.suspended {
		return
	}
	kind := p.active[ev.ID]
	delete(p.active, ev.ID)
	if kind == "" {
		kind = phaseKindFromID(ev.ID)
	}
	if kind == "" {
		return
	}
	status := strings.TrimSpace(ev.Status)
	if status == "" {
		status = "ok"
	}

	var b strings.Builder
	var symStyle *color.Color
	var sym string
	switch status {
	case "ok":
		symStyle, sym = p.theme.Success, p.symbols.Success
	case "skipped":
		symStyle, sym = p.theme.Warning, p.symbols.Skipped
	case "cancelled":
		symStyle, sym = p.theme.Warning, p.symbols.Warning
	default:
		symStyle, sym = p.theme.Error, p.symbols.Error
	}
	b.WriteString(p.colorSymbol(symStyle, sym))
	b.WriteString(" ")
	b.WriteString(kind)

	if ev.DurationMs > 0 {
		b.WriteString(" ")
		b.WriteString(FormatDurationMs(ev.DurationMs))
	}
	p.writeln(b.String())

	metrics := append([]diagnostics.Metric(nil), ev.Metrics...)
	p.completed = append(p.completed, completedPhase{
		kind:       kind,
		status:     status,
		durationMs: ev.DurationMs,
		metrics:    metrics,
	})
}

// colorSymbol applies the color to the symbol when color is enabled.
func (p *StaticRichProgressRenderer) colorSymbol(c *color.Color, sym string) string {
	if !p.settings.UseColor {
		return sym
	}
	return c.Sprint(sym)
}

func (p *StaticRichProgressRenderer) Notice(ev diagnostics.NoticeEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	msg := strings.TrimSpace(ev.Message)
	if msg == "" {
		return
	}
	sym := p.symbols.Warning
	if p.settings.UseColor {
		sym = p.theme.Warning.Sprint(sym)
	}
	p.writeln(sym + " " + msg)
}

func (p *StaticRichProgressRenderer) Suspend() {
	p.mu.Lock()
	p.suspended = true
	p.mu.Unlock()
}

func (p *StaticRichProgressRenderer) Resume() {
	p.mu.Lock()
	p.suspended = false
	p.mu.Unlock()
}

func (p *StaticRichProgressRenderer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true

	// Emit summary
	if len(p.completed) > 0 {
		added, updated, removed := extractInstallCounts(p.completed)
		var totalDuration int64
		for _, cp := range p.completed {
			totalDuration += cp.durationMs
		}
		var b strings.Builder
		fmt.Fprintf(&b, "installed added=%d updated=%d removed=%d", added, updated, removed)
		if totalDuration > 0 {
			b.WriteString(" duration=")
			b.WriteString(FormatDurationMs(totalDuration))
		}
		p.writeln(b.String())
	}
	return nil
}

func extractInstallCounts(phases []completedPhase) (added, updated, removed int) {
	for _, cp := range phases {
		if cp.status != "ok" {
			continue
		}
		for _, m := range cp.metrics {
			switch strings.ToLower(m.Name) {
			case "added":
				added += int(m.Value)
			case "updated":
				updated += int(m.Value)
			case "removed":
				removed += int(m.Value)
			}
		}
	}
	return
}

func (p *StaticRichProgressRenderer) writeln(line string) {
	_, _ = fmt.Fprintln(p.out, line)
}

// FlushCompletedPhases returns the completed phases (for testing).
func (p *StaticRichProgressRenderer) FlushCompletedPhases() []completedPhase {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.completed
	p.completed = nil
	return out
}

// WriteStaticRichInstallSummary emits the final installed= key=value summary line.
func WriteStaticRichInstallSummary(w io.Writer, settings EffectiveSettings, added, updated, removed int, durationMs int64) {
	if w == nil {
		return
	}
	theme := NewTheme(settings.ThemeMode)
	sym := settings.Symbols.Success
	if settings.UseColor {
		sym = theme.Success.Sprint(sym)
	}
	var b strings.Builder
	b.WriteString(sym)
	b.WriteString(" ")
	fmt.Fprintf(&b, "installed added=%d updated=%d removed=%d", added, updated, removed)
	if durationMs > 0 {
		b.WriteString(" duration=")
		b.WriteString(FormatDurationMs(durationMs))
	}
	_, _ = fmt.Fprintln(w, b.String())
}
