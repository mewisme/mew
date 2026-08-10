// Package diagnostics provides redaction, progress events, and reporters.
package diagnostics

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/fatih/color"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/fsx"
)

// Attr is a debug key/value pair.
type Attr struct {
	Key   string
	Value string
}

// Event is a progress notification independent of terminal rendering.
type Event struct {
	V                    int     `json:"v"`
	Type                 string  `json:"type"`
	Phase                string  `json:"phase"`
	Package              string  `json:"package,omitempty"`
	Script               string  `json:"script,omitempty"`
	Status               string  `json:"status,omitempty"`
	Index                int     `json:"index,omitempty"`
	Exit                 int     `json:"exit,omitempty"`
	Stream               string  `json:"stream,omitempty"`
	Message              string  `json:"message,omitempty"`
	Partial              bool    `json:"partial,omitempty"`
	Seq                  int     `json:"seq,omitempty"`
	Completed            int     `json:"completed,omitempty"`
	Failed               int     `json:"failed,omitempty"`
	Cancelled            int     `json:"cancelled,omitempty"`
	Skipped              int     `json:"skipped,omitempty"`
	NotRun               int     `json:"not_run,omitempty"`
	EffectiveConcurrency int     `json:"effective_concurrency,omitempty"`
	Bytes                int64   `json:"bytes,omitempty"`
	TotalBytes           *int64  `json:"total_bytes"`
	OpID                 string  `json:"op_id,omitempty"`
	TxID                 *string `json:"tx_id"`
}

// MakeWorkspaceTaskEvent builds a control-plane workspace task progress event.
func MakeWorkspaceTaskEvent(pkg, script, status string, index, exit int) Event {
	return Event{
		V:       1,
		Type:    "workspace-task",
		Phase:   "workspace-run",
		Package: pkg,
		Script:  script,
		Status:  status,
		Index:   index,
		Exit:    exit,
	}
}

// MakeWorkspaceSummaryEvent builds the final workspace run summary progress event.
func MakeWorkspaceSummaryEvent(completed, failed, cancelled, skipped, notRun, effConc int) Event {
	return Event{
		V:                    1,
		Type:                 "workspace-summary",
		Phase:                "workspace-run",
		Completed:            completed,
		Failed:               failed,
		Cancelled:            cancelled,
		Skipped:              skipped,
		NotRun:               notRun,
		EffectiveConcurrency: effConc,
	}
}

// MakeChildOutputEvent builds a structured child-output progress event.
func MakeChildOutputEvent(pkg, script, stream, message string, partial bool, seq int) Event {
	return Event{
		V:       1,
		Type:    "child-output",
		Package: pkg,
		Script:  script,
		Stream:  stream,
		Message: message,
		Partial: partial,
		Seq:     seq,
	}
}

// IsStructured reports whether the reporter emits JSON/NDJSON events only.
func IsStructured(rep Reporter) bool {
	if rep == nil {
		return false
	}
	type formatReporter interface {
		Format() string
	}
	if fr, ok := rep.(formatReporter); ok {
		f := fr.Format()
		return f == "json" || f == "ndjson"
	}
	return false
}

// ErrorDocument is the JSON reporter error payload.
type ErrorDocument struct {
	V           int    `json:"v"`
	Type        string `json:"type"`
	Code        string `json:"code"`
	Op          string `json:"op,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Message     string `json:"message"`
	Operation   string `json:"operation,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Cause       string `json:"cause,omitempty"`
	Exit        int    `json:"exit"`
}

// Reporter emits progress, errors, debug lines, and workspace runner events.
type Reporter interface {
	Progress(Event)
	Error(err error)
	Debug(msg string, attrs ...Attr)
	WorkspaceTask(WorkspaceTaskEvent)
	ChildOutput(ChildOutputEvent, WorkspaceOutputMode)
	WorkspaceSummary(WorkspaceSummaryEvent)
	EnvironmentPrepared(EnvironmentPreparedEvent) error
	OperationStarted(OperationStartedEvent)
	OperationProgress(OperationProgressEvent)
	OperationCompleted(OperationCompletedEvent)
	Notice(NoticeEvent)
}

// WorkspaceOutputMode routes workspace child output in reporters.
type WorkspaceOutputMode string

const (
	WorkspaceOutputStream    WorkspaceOutputMode = "stream"
	WorkspaceOutputAggregate WorkspaceOutputMode = "aggregate"
)

// WorkspaceTaskEvent is a control-plane workspace task notification.
type WorkspaceTaskEvent struct {
	V       int    `json:"v"`
	Type    string `json:"type"`
	Phase   string `json:"phase"`
	Package string `json:"package"`
	Script  string `json:"script"`
	Status  string `json:"status"`
	Index   int    `json:"index"`
	Exit    *int   `json:"exit,omitempty"`
}

// ChildOutputEvent is structured child stdout/stderr payload.
type ChildOutputEvent struct {
	V       int    `json:"v"`
	Type    string `json:"type"`
	Package string `json:"package"`
	Script  string `json:"script"`
	Stream  string `json:"stream"`
	Message string `json:"message"`
	Partial bool   `json:"partial,omitempty"`
	Seq     *int   `json:"seq,omitempty"`
}

// WorkspaceSummaryEvent is the final workspace run summary.
type WorkspaceSummaryEvent struct {
	V                    int    `json:"v"`
	Type                 string `json:"type"`
	Phase                string `json:"phase"`
	Completed            int    `json:"completed"`
	Failed               int    `json:"failed"`
	Cancelled            int    `json:"cancelled"`
	Skipped              int    `json:"skipped"`
	NotRun               int    `json:"not-run"`
	EffectiveConcurrency int    `json:"effective_concurrency"`
}

// EnvironmentPreparedEvent is the v1 environment-prepared runner event.
type EnvironmentPreparedEvent struct {
	V                 int    `json:"v"`
	Type              string `json:"type"`
	Source            string `json:"source"`
	IdentityDigest    string `json:"identityDigest"`
	GraphDigest       string `json:"graphDigest"`
	CacheState        string `json:"cacheState"`
	NetworkUsed       bool   `json:"networkUsed"`
	PrepareDurationMs int64  `json:"prepareDurationMs"`
}

// Options configures a reporter.
type Options struct {
	Out       io.Writer // progress / json stdout (default os.Stdout)
	Err       io.Writer // errors (default os.Stderr)
	Format    string    // default | ndjson | json | silent
	Color     ColorMode
	Debug     bool
	Unsafe    bool // skip redaction (requires explicit flag)
	TermWidth int  // 0 = default 80
	// HumanErrorRender renders human errors; nil keeps legacy formatHumanError.
	HumanErrorRender func(error) string
	// Optional presentation-owned progress/notice hooks (called after reporter handling).
	OnOperationStarted   func(OperationStartedEvent)
	OnOperationProgress  func(OperationProgressEvent)
	OnOperationCompleted func(OperationCompletedEvent)
	OnNotice             func(NoticeEvent)
	// Optional presentation-owned runner hooks (human path).
	OnEnvironmentPrepared func(EnvironmentPreparedEvent)
	OnWorkspaceTask       func(WorkspaceTaskEvent)
	OnWorkspaceSummary    func(WorkspaceSummaryEvent)
	OnPrepStage           func(label string)
	OnExecPrep            func(title string, rows []Attr)
	OnExecSummary         func(name string, durationMs int64, exit int, status string)
}

// ColorMode controls ANSI color.
type ColorMode int

const (
	ColorAuto ColorMode = iota
	ColorAlways
	ColorNever
)

// NewReporter builds a reporter for the given format.
func NewReporter(opts Options) Reporter {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Err == nil {
		opts.Err = os.Stderr
	}
	if opts.TermWidth <= 0 {
		opts.TermWidth = 80
	}
	format := strings.ToLower(strings.TrimSpace(opts.Format))
	if format == "" || format == "human" {
		format = "default"
	}
	base := &baseReporter{opts: opts}
	switch format {
	case "ndjson":
		return &ndjsonReporter{base: base}
	case "json":
		return &jsonReporter{base: base}
	case "silent":
		return &silentReporter{base: base}
	default:
		return &humanReporter{base: base}
	}
}

type baseReporter struct {
	opts Options
	mu   sync.Mutex
}

// Format returns the normalized reporter format name.
func (b *baseReporter) Format() string {
	format := strings.ToLower(strings.TrimSpace(b.opts.Format))
	if format == "" || format == "human" {
		return "default"
	}
	return format
}

func (b *baseReporter) redact(s string) string {
	if b.opts.Unsafe {
		return s
	}
	return Redact(s)
}

func (b *baseReporter) colorEnabled(w io.Writer) bool {
	switch b.opts.Color {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	default:
		if os.Getenv("NO_COLOR") != "" {
			return false
		}
		return isTTY(w)
	}
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

type humanReporter struct{ base *baseReporter }

func (r *humanReporter) Progress(ev Event) {
	if ev.Type == "prep-stage" {
		hook := r.base.opts.OnPrepStage
		if hook != nil {
			hook(ev.Phase)
			return
		}
		r.base.mu.Lock()
		defer r.base.mu.Unlock()
		fmt.Fprintln(r.base.opts.Err, truncate(r.base.redact("  "+ev.Phase), r.base.opts.TermWidth))
		return
	}
	if ev.Type == "exec-prep" {
		hook := r.base.opts.OnExecPrep
		if hook != nil {
			rows := []Attr{}
			if ev.Message != "" {
				rows = append(rows, Attr{Key: "Source", Value: ev.Message})
			}
			if ev.Package != "" {
				rows = append(rows, Attr{Key: "Package", Value: ev.Package})
			}
			hook(ev.Phase, rows)
			return
		}
	}
	if ev.Type == "exec-summary" {
		hook := r.base.opts.OnExecSummary
		if hook != nil {
			hook(ev.Phase, ev.Bytes, ev.Exit, ev.Status)
			return
		}
		return
	}
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	switch ev.Type {
	case "workspace-task":
		line := fmt.Sprintf("%s %s", ev.Status, ev.Package)
		if ev.Script != "" {
			line += " " + ev.Script
		}
		fmt.Fprintln(r.base.opts.Err, truncate(r.base.redact(line), r.base.opts.TermWidth))
		return
	case "workspace-summary":
		line := fmt.Sprintf("workspace: completed=%d failed=%d cancelled=%d skipped=%d not-run=%d concurrency=%d",
			ev.Completed, ev.Failed, ev.Cancelled, ev.Skipped, ev.NotRun, ev.EffectiveConcurrency)
		fmt.Fprintln(r.base.opts.Err, truncate(r.base.redact(line), r.base.opts.TermWidth))
		return
	case "child-output":
		prefix := "[" + ev.Package + "] "
		out := r.base.opts.Out
		if ev.Stream == "stderr" {
			out = r.base.opts.Err
		}
		_, _ = fmt.Fprint(out, prefix+r.base.redact(ev.Message))
		if !ev.Partial {
			fmt.Fprintln(out)
		}
		return
	case "exec-prep":
		line := ev.Phase
		if ev.Package != "" {
			line += " " + ev.Package
		}
		fmt.Fprintln(r.base.opts.Err, truncate(r.base.redact(line), r.base.opts.TermWidth))
		return
	}
	line := ev.Phase
	if ev.Package != "" {
		line += " " + ev.Package
	}
	if ev.Bytes > 0 {
		line += fmt.Sprintf(" %dB", ev.Bytes)
	}
	line = truncate(r.base.redact(line), r.base.opts.TermWidth)
	fmt.Fprintln(r.base.opts.Err, line)
}

func (r *humanReporter) Error(err error) {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	if r.base.opts.HumanErrorRender != nil {
		msg := r.base.redact(r.base.opts.HumanErrorRender(err))
		fmt.Fprintln(r.base.opts.Err, msg)
		return
	}
	msg := r.base.redact(formatHumanError(err))
	if r.base.colorEnabled(r.base.opts.Err) {
		fmt.Fprintln(r.base.opts.Err, color.New(color.FgRed).Sprint(msg))
		return
	}
	fmt.Fprintln(r.base.opts.Err, msg)
}

func (r *humanReporter) Debug(msg string, attrs ...Attr) {
	if !r.base.opts.Debug {
		return
	}
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	line := "debug: " + r.base.redact(msg)
	for _, a := range attrs {
		line += fmt.Sprintf(" %s=%s", a.Key, r.base.redact(a.Value))
	}
	fmt.Fprintln(r.base.opts.Err, truncate(line, r.base.opts.TermWidth))
}

func (r *humanReporter) WorkspaceTask(ev WorkspaceTaskEvent) {
	hook := r.base.opts.OnWorkspaceTask
	if hook != nil {
		hook(ev)
		return
	}
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	line := fmt.Sprintf("%s %s %s [%d]", ev.Phase, ev.Package, ev.Status, ev.Index)
	if ev.Exit != nil {
		line += fmt.Sprintf(" exit=%d", *ev.Exit)
	}
	fmt.Fprintln(r.base.opts.Err, truncate(r.base.redact(line), r.base.opts.TermWidth))
}

func (r *humanReporter) ChildOutput(ev ChildOutputEvent, mode WorkspaceOutputMode) {
	if mode == WorkspaceOutputAggregate {
		return
	}
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	prefix := fmt.Sprintf("[%s] ", r.base.redact(ev.Package))
	target := r.base.opts.Out
	if ev.Stream == "stderr" {
		target = r.base.opts.Err
	}
	line := prefix + r.base.redact(ev.Message)
	if ev.Partial {
		_, _ = fmt.Fprint(target, line)
	} else {
		fmt.Fprintln(target, line)
	}
}

func (r *humanReporter) WorkspaceSummary(ev WorkspaceSummaryEvent) {
	hook := r.base.opts.OnWorkspaceSummary
	if hook != nil {
		hook(ev)
		return
	}
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	line := fmt.Sprintf("%s completed=%d failed=%d cancelled=%d skipped=%d not-run=%d concurrency=%d",
		ev.Phase, ev.Completed, ev.Failed, ev.Cancelled, ev.Skipped, ev.NotRun, ev.EffectiveConcurrency)
	fmt.Fprintln(r.base.opts.Err, truncate(r.base.redact(line), r.base.opts.TermWidth))
}

func (r *humanReporter) EnvironmentPrepared(ev EnvironmentPreparedEvent) error {
	hook := r.base.opts.OnEnvironmentPrepared
	if hook != nil {
		hook(ev)
	}
	return nil
}

func (r *humanReporter) OperationStarted(ev OperationStartedEvent) {
	// Progress is presentation-owned. Call hooks outside base.mu to avoid deadlock
	// if a ProgressSink ever touches the reporter (it must not hold locks either).
	hook := r.base.opts.OnOperationStarted
	if hook != nil {
		hook(ev)
	}
}

func (r *humanReporter) OperationProgress(ev OperationProgressEvent) {
	hook := r.base.opts.OnOperationProgress
	if hook != nil {
		hook(ev)
	}
}

func (r *humanReporter) OperationCompleted(ev OperationCompletedEvent) {
	hook := r.base.opts.OnOperationCompleted
	if hook != nil {
		hook(ev)
	}
}

func (r *humanReporter) Notice(ev NoticeEvent) {
	hook := r.base.opts.OnNotice
	if hook != nil {
		hook(ev)
		return
	}
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	line := ev.Severity + ": " + ev.Message
	if ev.Hint != "" {
		line += " (" + ev.Hint + ")"
	}
	fmt.Fprintln(r.base.opts.Err, truncate(r.base.redact(line), r.base.opts.TermWidth))
}

type ndjsonReporter struct{ base *baseReporter }

func (r *ndjsonReporter) Progress(ev Event) {
	if ev.Type == "prep-stage" || ev.Type == "exec-prep" || ev.Type == "exec-summary" {
		return
	}
	if ev.V == 0 {
		ev.V = 1
	}
	if ev.Type == "" {
		ev.Type = "progress"
	}
	ev.Phase = r.base.redact(ev.Phase)
	ev.Package = r.base.redact(ev.Package)
	ev.Script = r.base.redact(ev.Script)
	ev.Message = r.base.redact(ev.Message)
	ev.OpID = r.base.redact(ev.OpID)
	r.writeLine(ev)
}

func (r *ndjsonReporter) Error(err error) {
	r.writeLine(errorDoc(err, r.base.redact))
}

func (r *ndjsonReporter) Debug(msg string, attrs ...Attr) {
	if !r.base.opts.Debug {
		return
	}
	m := map[string]any{"v": 1, "type": "debug", "message": r.base.redact(msg)}
	if len(attrs) > 0 {
		am := map[string]string{}
		for _, a := range attrs {
			am[a.Key] = r.base.redact(a.Value)
		}
		m["attrs"] = am
	}
	r.writeLine(m)
}

func (r *ndjsonReporter) WorkspaceTask(ev WorkspaceTaskEvent) {
	ev.Package = r.base.redact(ev.Package)
	ev.Script = r.base.redact(ev.Script)
	r.writeLine(ev)
}

func (r *ndjsonReporter) ChildOutput(ev ChildOutputEvent, _ WorkspaceOutputMode) {
	ev.Package = r.base.redact(ev.Package)
	ev.Script = r.base.redact(ev.Script)
	ev.Message = r.base.redact(ev.Message)
	r.writeLine(ev)
}

func (r *ndjsonReporter) WorkspaceSummary(ev WorkspaceSummaryEvent) {
	r.writeLine(ev)
}

func (r *ndjsonReporter) EnvironmentPrepared(ev EnvironmentPreparedEvent) error {
	r.writeLine(ev)
	return nil
}

func (r *ndjsonReporter) OperationStarted(ev OperationStartedEvent) {
	if ev.V == 0 {
		ev.V = 1
	}
	if ev.Type == "" {
		ev.Type = "operation-started"
	}
	r.writeLine(ev)
	if r.base.opts.OnOperationStarted != nil {
		r.base.opts.OnOperationStarted(ev)
	}
}

func (r *ndjsonReporter) OperationProgress(ev OperationProgressEvent) {
	if ev.V == 0 {
		ev.V = 1
	}
	if ev.Type == "" {
		ev.Type = "operation-progress"
	}
	ev.Detail = r.base.redact(ev.Detail)
	r.writeLine(ev)
	if r.base.opts.OnOperationProgress != nil {
		r.base.opts.OnOperationProgress(ev)
	}
}

func (r *ndjsonReporter) OperationCompleted(ev OperationCompletedEvent) {
	if ev.V == 0 {
		ev.V = 1
	}
	if ev.Type == "" {
		ev.Type = "operation-completed"
	}
	r.writeLine(ev)
	if r.base.opts.OnOperationCompleted != nil {
		r.base.opts.OnOperationCompleted(ev)
	}
}

func (r *ndjsonReporter) Notice(ev NoticeEvent) {
	if ev.V == 0 {
		ev.V = 1
	}
	if ev.Type == "" {
		ev.Type = "notice"
	}
	ev.Message = r.base.redact(ev.Message)
	ev.Hint = r.base.redact(ev.Hint)
	r.writeLine(ev)
	if r.base.opts.OnNotice != nil {
		r.base.opts.OnNotice(ev)
	}
}

func (r *ndjsonReporter) writeLine(v any) {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(r.base.opts.Out, "{\"v\":1,\"type\":\"error\",\"code\":\"ERR_M_INTERNAL\",\"message\":\"marshal\",\"exit\":1}\n")
		return
	}
	b = append(b, '\n')
	_, _ = r.base.opts.Out.Write(b)
}

type jsonReporter struct{ base *baseReporter }

func (r *jsonReporter) Progress(Event) {}

func (r *jsonReporter) Error(err error) {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	doc := errorDoc(err, r.base.redact)
	b, err2 := json.MarshalIndent(doc, "", "  ")
	if err2 != nil {
		fmt.Fprintln(r.base.opts.Out, `{"v":1,"type":"error","code":"ERR_M_INTERNAL","message":"marshal","exit":1}`)
		return
	}
	fmt.Fprintln(r.base.opts.Out, string(b))
}

func (r *jsonReporter) Debug(msg string, attrs ...Attr) {
	if !r.base.opts.Debug {
		return
	}
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	fmt.Fprintf(r.base.opts.Err, "debug: %s\n", r.base.redact(msg))
}

func (r *jsonReporter) WorkspaceTask(ev WorkspaceTaskEvent) {
	ev.Package = r.base.redact(ev.Package)
	ev.Script = r.base.redact(ev.Script)
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	b, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return
	}
	fmt.Fprintln(r.base.opts.Out, string(b))
}

func (r *jsonReporter) ChildOutput(ev ChildOutputEvent, _ WorkspaceOutputMode) {
	ev.Package = r.base.redact(ev.Package)
	ev.Script = r.base.redact(ev.Script)
	ev.Message = r.base.redact(ev.Message)
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	b, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return
	}
	fmt.Fprintln(r.base.opts.Out, string(b))
}

func (r *jsonReporter) WorkspaceSummary(ev WorkspaceSummaryEvent) {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	b, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return
	}
	fmt.Fprintln(r.base.opts.Out, string(b))
}

func (r *jsonReporter) EnvironmentPrepared(ev EnvironmentPreparedEvent) error {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	b, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(r.base.opts.Out, string(b))
	return err
}

func (r *jsonReporter) OperationStarted(OperationStartedEvent)     {}
func (r *jsonReporter) OperationProgress(OperationProgressEvent)   {}
func (r *jsonReporter) OperationCompleted(OperationCompletedEvent) {}
func (r *jsonReporter) Notice(NoticeEvent)                         {}

type silentReporter struct{ base *baseReporter }

func (r *silentReporter) Progress(Event)                                     {}
func (r *silentReporter) Debug(string, ...Attr)                              {}
func (r *silentReporter) WorkspaceTask(WorkspaceTaskEvent)                   {}
func (r *silentReporter) ChildOutput(ChildOutputEvent, WorkspaceOutputMode)  {}
func (r *silentReporter) WorkspaceSummary(WorkspaceSummaryEvent)             {}
func (r *silentReporter) EnvironmentPrepared(EnvironmentPreparedEvent) error { return nil }
func (r *silentReporter) OperationStarted(OperationStartedEvent)             {}
func (r *silentReporter) OperationProgress(OperationProgressEvent)           {}
func (r *silentReporter) OperationCompleted(OperationCompletedEvent)         {}
func (r *silentReporter) Notice(NoticeEvent)                                 {}
func (r *silentReporter) Error(err error) {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	fmt.Fprintln(r.base.opts.Err, r.base.redact(formatHumanError(err)))
}

func formatHumanError(err error) string {
	if err == nil {
		return ""
	}
	var ae *apperr.Error
	if errors.As(err, &ae) && ae != nil {
		return formatAppError(ae)
	}
	return err.Error()
}

func formatAppError(ae *apperr.Error) string {
	if ae == nil {
		return ""
	}
	var renameErr *fsx.RenameError
	if errors.As(ae.Cause, &renameErr) && renameErr != nil {
		return fmt.Sprintf("%s: %s failed\noperation: %s\nsource: %s\ndestination: %s\ncause: %s",
			ae.Code, ae.Op, renameErr.Op, renameErr.Src, renameErr.Dst, renameErr.Cause)
	}
	if strings.Contains(ae.Message, "\n") {
		return fmt.Sprintf("%s: %s: %s:\n%s", ae.Code, ae.Op, ae.Subject, ae.Message)
	}
	return ae.Error()
}

func errorDoc(err error, redact func(string) string) ErrorDocument {
	doc := ErrorDocument{V: 1, Type: "error", Code: string(apperr.Internal), Message: "unknown", Exit: 1}
	if err == nil {
		doc.Code = string(apperr.OK)
		doc.Message = ""
		doc.Exit = 0
		return doc
	}
	var ae *apperr.Error
	if errors.As(err, &ae) && ae != nil {
		doc.Code = string(ae.Code)
		doc.Op = redact(ae.Op)
		doc.Subject = redact(ae.Subject)
		doc.Message = redact(ae.Message)
		if doc.Message == "" && ae.Cause != nil {
			doc.Message = redact(ae.Cause.Error())
		}
		var renameErr *fsx.RenameError
		if errors.As(ae.Cause, &renameErr) && renameErr != nil {
			doc.Operation = renameErr.Op
			doc.Source = redact(renameErr.Src)
			doc.Destination = redact(renameErr.Dst)
			if renameErr.Cause != nil {
				doc.Cause = redact(renameErr.Cause.Error())
			}
			doc.Message = redact(formatAppError(ae))
		}
		doc.Exit = apperr.ExitCode(ae)
		return doc
	}
	doc.Message = redact(err.Error())
	doc.Exit = apperr.ExitCode(err)
	doc.Code = string(apperr.CodeOf(err))
	return doc
}

func truncate(s string, width int) string {
	if width <= 0 || utf8.RuneCountInString(s) <= width {
		return s
	}
	runes := []rune(s)
	if width < 4 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

var (
	reBearer    = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`)
	reQueryTok  = regexp.MustCompile(`(?i)([?&](?:access_token|authToken|token|password|secret)=)([^&\s]+)`)
	reEnvSecret = regexp.MustCompile(`(?i)([A-Z0-9_]*(?:TOKEN|PASSWORD|SECRET|API_KEY)=)([^\s&]+)`)
)

// Redact removes credentials and common secret shapes from s (fail-closed).
func Redact(s string) string {
	if s == "" {
		return s
	}
	s = redactURLs(s)
	s = reBearer.ReplaceAllString(s, "Bearer ***")
	s = reQueryTok.ReplaceAllString(s, "${1}***")
	s = reEnvSecret.ReplaceAllString(s, "${1}***")
	return s
}

func redactURLs(s string) string {
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" && u.User != nil {
		u.User = url.UserPassword("***", "***")
		return u.String()
	}
	parts := strings.Fields(s)
	changed := false
	for i, p := range parts {
		u, err := url.Parse(p)
		if err != nil || u.Scheme == "" || u.Host == "" || u.User == nil {
			continue
		}
		u.User = url.UserPassword("***", "***")
		parts[i] = u.String()
		changed = true
	}
	if changed {
		return strings.Join(parts, " ")
	}
	return s
}

// FormatErrorDocument builds the JSON error document (exported for tests).
func FormatErrorDocument(err error, unsafe bool) ErrorDocument {
	redact := Redact
	if unsafe {
		redact = func(s string) string { return s }
	}
	return errorDoc(err, redact)
}
