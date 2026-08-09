package presentation

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mewisme/mew/internal/diagnostics"
)

// CompletionSummary is the optional post-child stderr line.
type CompletionSummary struct {
	Name      string
	Duration  time.Duration
	ExitCode  int
	Failed    bool
	Cancelled bool
}

// ShouldEmitCompletionSummary reports whether a wrapper summary is allowed.
func ShouldEmitCompletionSummary(opts ResolvedOptions, intent TerminalIntent, interactiveChild bool) bool {
	if !opts.Summary {
		return false
	}
	if opts.Structured() || opts.Output == OutputSilent {
		return false
	}
	if interactiveChild || intent == TerminalInteractive {
		return false
	}
	return true
}

// RenderCompletionSummary formats "✓ name completed in 1.2s" (or failed/cancelled).
func RenderCompletionSummary(s CompletionSummary, settings EffectiveSettings) string {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		name = "command"
	}

	prefix := settings.Symbols.Success
	verb := "completed"
	prefixColor := color.New(color.FgGreen)

	if s.Cancelled {
		prefix = settings.Symbols.Warning
		verb = "cancelled"
		prefixColor = color.New(color.FgYellow)
	} else if s.Failed || s.ExitCode != 0 {
		prefix = settings.Symbols.Error
		verb = "failed"
		prefixColor = color.New(color.FgRed)
	}

	duration := ""
	if s.Duration > 0 {
		duration = " in " + FormatDuration(s.Duration)
	}

	return fmt.Sprintf(
		"%s %s %s%s",
		prefixColor.Sprint(prefix),
		name,
		verb,
		duration,
	)
}

// WriteCompletionSummary writes the summary to stderr, inserting a leading newline
// when ensureNL is true (child output lacked a final newline).
func WriteCompletionSummary(w io.Writer, ensureNL bool, s CompletionSummary, settings EffectiveSettings) {
	if w == nil {
		return
	}
	line := RenderCompletionSummary(s, settings)
	if line == "" {
		return
	}
	if ensureNL {
		_, _ = io.WriteString(w, "\n")
	}
	_, _ = fmt.Fprintln(w, line)
}

// AttrRowsToKeyValues maps diagnostics attrs to presentation rows.
func AttrRowsToKeyValues(rows []diagnostics.Attr) []KeyValue {
	out := make([]KeyValue, 0, len(rows))
	for _, a := range rows {
		if strings.TrimSpace(a.Key) == "" {
			continue
		}
		out = append(out, KeyValue{Key: a.Key, Value: a.Value})
	}
	return out
}
