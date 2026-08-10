package presentation_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mewisme/mew/internal/diagnostics"
	"github.com/mewisme/mew/internal/presentation"
)

func TestMapEnvironmentPreparedWarmDLX(t *testing.T) {
	view := presentation.MapEnvironmentPrepared(diagnostics.EnvironmentPreparedEvent{
		Source:     "dlx",
		CacheState: "warm-hit",
	}, "eslint 9.12.0", false, presentation.UnicodeSymbols.Ellipsis)
	if !strings.Contains(view.Title, "eslint") {
		t.Fatalf("title=%q", view.Title)
	}
	got := map[string]string{}
	for _, r := range view.Rows {
		got[r.Key] = r.Value
	}
	if got["Environment"] != "warm cache" {
		t.Fatalf("env=%q rows=%v", got["Environment"], view.Rows)
	}
	if got["Source"] != "dlx" {
		t.Fatalf("source=%q", got["Source"])
	}
}

func TestMapEnvironmentPreparedSnapshotSafeLabels(t *testing.T) {
	view := presentation.MapEnvironmentPrepared(diagnostics.EnvironmentPreparedEvent{
		Source:         "snapshot",
		CacheState:     "warm-hit",
		NetworkUsed:    false,
		IdentityDigest: "abcdef0123456789",
	}, "eslint", false, presentation.UnicodeSymbols.Ellipsis)
	got := map[string]string{}
	for _, r := range view.Rows {
		got[r.Key] = r.Value
	}
	if !strings.HasPrefix(got["Source"], "snapshot ") {
		t.Fatalf("source=%q", got["Source"])
	}
	if strings.Contains(got["Source"], "/") || strings.Contains(got["Source"], `\`) {
		t.Fatalf("absolute path leaked: %q", got["Source"])
	}
	if got["Network"] != "disabled" {
		t.Fatalf("network=%q", got["Network"])
	}
	if got["Environment"] != "verified" {
		t.Fatalf("env=%q", got["Environment"])
	}
	for _, r := range view.Rows {
		if strings.EqualFold(r.Key, "Identity") {
			t.Fatal("digest must be debug-only")
		}
	}
}

func TestMapEnvironmentPreparedCapsule(t *testing.T) {
	view := presentation.MapEnvironmentPrepared(diagnostics.EnvironmentPreparedEvent{
		Source:      "capsule",
		NetworkUsed: false,
	}, "tool", false, presentation.UnicodeSymbols.Ellipsis)
	got := map[string]string{}
	for _, r := range view.Rows {
		got[r.Key] = r.Value
	}
	if got["Source"] != "capsule" {
		t.Fatalf("source=%q", got["Source"])
	}
	if got["Network"] != "disabled" {
		t.Fatalf("network=%q", got["Network"])
	}
}

func TestProjectExecPrep(t *testing.T) {
	view := presentation.ProjectExecPrep("lint", "eslint")
	out := presentation.RenderExecutionPrep(view, presentation.EffectiveSettings{
		Symbols: presentation.ASCIISymbols,
		Width:   80,
	})
	if !strings.Contains(out, "Running lint") {
		t.Fatalf("out=%q", out)
	}
	if !strings.Contains(out, "project") {
		t.Fatalf("missing source: %q", out)
	}
}

func TestTerminalIntentAutoSuspend(t *testing.T) {
	caps := presentation.Capabilities{StdinTTY: true, StdoutTTY: true, StderrTTY: true}
	if !presentation.ShouldSuspendRichUI(presentation.TerminalAuto, caps) {
		t.Fatal("auto should suspend when all TTYs")
	}
	caps.StdoutTTY = false
	if presentation.ShouldSuspendRichUI(presentation.TerminalAuto, caps) {
		t.Fatal("auto must not suspend when stdout is not a TTY")
	}
	if !presentation.ShouldSuspendRichUI(presentation.TerminalInteractive, caps) {
		t.Fatal("interactive always suspends")
	}
	if presentation.ShouldSuspendRichUI(presentation.TerminalStream, presentation.Capabilities{
		StdinTTY: true, StdoutTTY: true, StderrTTY: true,
	}) {
		t.Fatal("stream must not suspend")
	}
}

func TestNoSummaryPolicy(t *testing.T) {
	opts := presentation.ResolvedOptions{Summary: false, Output: presentation.OutputPlain}
	if presentation.ShouldEmitCompletionSummary(opts, presentation.TerminalAuto, false) {
		t.Fatal("--no-summary must suppress")
	}
	opts.Summary = true
	if !presentation.ShouldEmitCompletionSummary(opts, presentation.TerminalAuto, false) {
		t.Fatal("summary allowed")
	}
	if presentation.ShouldEmitCompletionSummary(opts, presentation.TerminalInteractive, false) {
		t.Fatal("interactive suppresses summary")
	}
	opts.Output = presentation.OutputJSON
	if presentation.ShouldEmitCompletionSummary(opts, presentation.TerminalAuto, false) {
		t.Fatal("structured suppresses summary")
	}
}

func TestCompletionSummaryRender(t *testing.T) {
	line := presentation.RenderCompletionSummary(presentation.CompletionSummary{
		Name:     "eslint",
		Duration: 1200 * time.Millisecond,
	}, presentation.EffectiveSettings{Symbols: presentation.UnicodeSymbols})
	if !strings.Contains(line, "eslint") || !strings.Contains(line, "completed") {
		t.Fatalf("line=%q", line)
	}
	if !strings.Contains(line, "1.2s") {
		t.Fatalf("duration missing: %q", line)
	}
}

func TestWorkspaceAggregateOrderingAndFailedList(t *testing.T) {
	var buf bytes.Buffer
	r := presentation.NewWorkspaceAggregateRenderer(&buf, presentation.EffectiveSettings{
		Symbols: presentation.ASCIISymbols,
		Width:   80,
	})
	r.WorkspaceTask(diagnostics.WorkspaceTaskEvent{Package: "apps/web", Script: "test", Status: "start", Index: 1})
	r.WorkspaceTask(diagnostics.WorkspaceTaskEvent{Package: "apps/api", Script: "test", Status: "start", Index: 0})
	r.WorkspaceTask(diagnostics.WorkspaceTaskEvent{Package: "apps/api", Script: "test", Status: "done", Index: 0, Exit: intPtr(0)})
	exit1 := 1
	r.WorkspaceTask(diagnostics.WorkspaceTaskEvent{Package: "apps/web", Script: "test", Status: "fail", Index: 1, Exit: &exit1})
	r.WorkspaceSummary(diagnostics.WorkspaceSummaryEvent{
		Completed: 1, Failed: 1, NotRun: 0,
	})
	out := buf.String()
	webIdx := strings.Index(out, "apps/web")
	apiIdx := strings.Index(out, "apps/api")
	if webIdx < 0 || apiIdx < 0 {
		t.Fatalf("missing rows: %q", out)
	}
	// First status line should be apps/web (report order), then apps/api.
	firstLine := strings.Split(out, "\n")[0]
	if !strings.Contains(firstLine, "apps/web") {
		t.Fatalf("expected web first in append-only order, got %q", firstLine)
	}
	if !strings.Contains(out, "Failed") || !strings.Contains(out, "apps/web") {
		t.Fatalf("missing failed list: %q", out)
	}
	if !strings.Contains(out, "Completed") {
		t.Fatalf("missing counts: %q", out)
	}
}

func TestWorkspaceAggregateAppendOnly(t *testing.T) {
	var buf bytes.Buffer
	r := presentation.NewWorkspaceAggregateRenderer(&buf, presentation.EffectiveSettings{
		Symbols: presentation.ASCIISymbols,
		Width:   80,
	})
	r.WorkspaceTask(diagnostics.WorkspaceTaskEvent{Package: "pkg", Script: "test", Status: "start", Index: 0})
	r.WorkspaceTask(diagnostics.WorkspaceTaskEvent{Package: "pkg", Script: "test", Status: "done", Index: 0, Exit: intPtr(0)})
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected append-only lines, got %q", buf.String())
	}
}

func intPtr(v int) *int { return &v }
