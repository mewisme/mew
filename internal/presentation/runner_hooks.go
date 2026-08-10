package presentation

import (
	"io"
	"strings"
	"time"

	"github.com/mewisme/mew/internal/diagnostics"
)

// RunnerPresentation owns human runner/workspace stderr rendering.
type RunnerPresentation struct {
	settings EffectiveSettings
	summary  bool
	errW     io.Writer
	ws       *WorkspaceAggregateRenderer
	lastCmd  string
	debug    bool
	intent   TerminalIntent
}

func newRunnerPresentation(settings EffectiveSettings, errW io.Writer, debug, summary bool) *RunnerPresentation {
	if errW == nil {
		errW = io.Discard
	}
	return &RunnerPresentation{
		settings: settings,
		summary:  summary,
		errW:     errW,
		ws:       NewWorkspaceAggregateRenderer(errW, settings),
		debug:    debug,
		intent:   TerminalAuto,
	}
}

func (r *RunnerPresentation) EnvironmentPrepared(ev diagnostics.EnvironmentPreparedEvent) {
	view := MapEnvironmentPrepared(ev, r.lastCmd, r.debug, r.settings.Symbols.Ellipsis)
	WriteExecutionPrep(r.errW, view, r.settings)
}

func (r *RunnerPresentation) PrepStage(label string) {
	label = strings.TrimSpace(label)
	if label == "" {
		return
	}
	_, _ = io.WriteString(r.errW, "  "+label+"\n")
}

func (r *RunnerPresentation) ExecPrep(title string, rows []diagnostics.Attr) {
	view := ExecutionPrepView{Title: title, Rows: AttrRowsToKeyValues(rows)}
	WriteExecutionPrep(r.errW, view, r.settings)
}

func (r *RunnerPresentation) ExecSummary(name string, durationMs int64, exit int, status string) {
	if !r.summary {
		return
	}
	if r.intent == TerminalInteractive {
		return
	}
	sum := CompletionSummary{
		Name:      name,
		Duration:  time.Duration(durationMs) * time.Millisecond,
		ExitCode:  exit,
		Failed:    status == "fail",
		Cancelled: status == "cancelled",
	}
	WriteCompletionSummary(r.errW, false, sum, r.settings)
}

func (r *RunnerPresentation) WorkspaceTask(ev diagnostics.WorkspaceTaskEvent) {
	r.ws.WorkspaceTask(ev)
}

func (r *RunnerPresentation) WorkspaceSummary(ev diagnostics.WorkspaceSummaryEvent) {
	r.ws.WorkspaceSummary(ev)
}

func (r *RunnerPresentation) Suspend() {
	r.ws.Suspend()
}

func (r *RunnerPresentation) Resume() {
	r.ws.Resume()
}

func (r *RunnerPresentation) DisableWorkspaceLiveStatus() {
	r.ws.DisableLiveStatus()
}

func (r *RunnerPresentation) SetCommand(cmd string) {
	r.lastCmd = cmd
}

func (r *RunnerPresentation) SetIntent(intent TerminalIntent) {
	r.intent = intent
}

// attachRunnerHooks wires human runner presentation into diagnostics Options.
func attachRunnerHooks(opts *diagnostics.Options, rp *RunnerPresentation) {
	if opts == nil || rp == nil {
		return
	}
	opts.OnEnvironmentPrepared = rp.EnvironmentPrepared
	opts.OnWorkspaceTask = rp.WorkspaceTask
	opts.OnWorkspaceSummary = rp.WorkspaceSummary
	opts.OnPrepStage = rp.PrepStage
	opts.OnExecPrep = rp.ExecPrep
	opts.OnExecSummary = rp.ExecSummary
}
