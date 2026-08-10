package presentation

import (
	"fmt"
	"io"
	"strings"

	"github.com/mewisme/mew/internal/diagnostics"
)

// ExecutionPrepView is the human banner shown before child launch.
type ExecutionPrepView struct {
	Title  string
	Rows   []KeyValue
	Stages []string // cold mx only: Resolving, Consent, Fetching, Preparing
}

// MapEnvironmentPrepared builds a safe prep view from the frozen event + command label.
// Digests are omitted unless debug is true. Absolute paths are never included.
func MapEnvironmentPrepared(
	ev diagnostics.EnvironmentPreparedEvent,
	command string,
	debug bool,
	ellipsis string,
) ExecutionPrepView {
	view := ExecutionPrepView{Title: runningTitle(command)}
	source, env, network, integrity := mapPreparedLabels(ev)

	addRow := func(key, value string) {
		if value != "" {
			view.Rows = append(view.Rows, KeyValue{
				Key:   key,
				Value: value,
			})
		}
	}

	addRow("Source", source)

	if command != "" && source != "project" {
		addRow("Package", command)
	}

	addRow("Environment", env)
	addRow("Network", network)
	addRow("Integrity", integrity)

	if debug {
		addRow("Identity", shortDigest(ev.IdentityDigest, ellipsis))
		addRow("Graph", shortDigest(ev.GraphDigest, ellipsis))
	}

	return view
}

// ProjectExecPrep builds a thin local run/exec banner without inventing EnvironmentPrepared.
func ProjectExecPrep(command, packageName string) ExecutionPrepView {
	if packageName == "" {
		packageName = command
	}

	view := ExecutionPrepView{
		Title: runningTitle(command),
		Rows: []KeyValue{
			{Key: "Source", Value: "project"},
		},
	}

	if packageName != "" {
		view.Rows = append(view.Rows, KeyValue{
			Key:   "Package",
			Value: packageName,
		})
	}

	return view
}

func runningTitle(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "Running"
	}
	return "Running " + command
}

func mapPreparedLabels(
	ev diagnostics.EnvironmentPreparedEvent,
) (source, env, network, integrity string) {
	sourceType := strings.ToLower(strings.TrimSpace(ev.Source))

	switch sourceType {
	case "project", "dlx", "capsule":
		source = sourceType
	case "snapshot":
		source = "snapshot " + shortID(ev.IdentityDigest)
	default:
		source = ev.Source
	}

	switch strings.ToLower(strings.TrimSpace(ev.CacheState)) {
	case "warm-hit", "warm":
		env = "warm cache"
	case "project":
		if source == "" {
			source = "project"
		}
	case "ephemeral":
		env = "ephemeral"
	}

	switch sourceType {
	case "snapshot", "capsule":
		// Authoritative LockedProviderPolicy:
		// NetworkForbidden + VerificationRequired.
		if !ev.NetworkUsed {
			network = "disabled"
		}
		env = "verified"
	}

	return
}

func shortID(digest string) string {
	digest = strings.TrimSpace(digest)

	switch {
	case digest == "":
		return "unknown"
	case len(digest) >= 6:
		return digest[:6]
	default:
		return digest
	}
}

func shortDigest(digest, ellipsis string) string {
	digest = strings.TrimSpace(digest)
	if len(digest) > 12 {
		return digest[:12] + ellipsis
	}
	return digest
}

// RenderExecutionPrep formats a prep view with arrow title, optional stages, and KV rows.
func RenderExecutionPrep(view ExecutionPrepView, settings EffectiveSettings) string {
	theme := NewTheme(settings.ThemeMode)
	lines := make([]string, 0, len(view.Stages)+len(view.Rows)+1)

	for _, stage := range view.Stages {
		if stage = strings.TrimSpace(stage); stage != "" {
			if settings.UseColor {
				stage = applyStyle(theme.Faint, stage, true)
			}
			lines = append(lines, "  "+stage)
		}
	}

	arrow := settings.Symbols.Arrow
	if arrow == "" {
		arrow = UnicodeSymbols.Arrow
	}

	title := arrow + " " + strings.TrimSpace(view.Title)
	if settings.UseColor {
		title = applyStyle(theme.Primary, title, true)
	}
	lines = append(lines, title)

	if len(view.Rows) > 0 {
		lines = append(lines, renderPrepRows(view.Rows, settings, theme)...)
	}

	return strings.Join(lines, "\n")
}

func renderPrepRows(rows []KeyValue, settings EffectiveSettings, theme Theme) []string {
	width := 0
	for _, row := range rows {
		width = max(width, len(row.Key))
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		key := fmt.Sprintf("%-*s", width, row.Key)
		if settings.UseColor {
			key = applyStyle(theme.Faint, key, true)
		}

		lines = append(lines,
			fmt.Sprintf(" %s  %s", key, row.Value),
		)
	}

	return lines
}

// WriteExecutionPrep writes the prep banner to stderr (w).
func WriteExecutionPrep(
	w io.Writer,
	view ExecutionPrepView,
	settings EffectiveSettings,
) {
	if w == nil {
		return
	}

	if text := RenderExecutionPrep(view, settings); text != "" {
		_, _ = fmt.Fprintln(w, text)
	}
}
