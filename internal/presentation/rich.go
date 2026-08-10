package presentation

import "strings"

type richRenderer struct {
	settings EffectiveSettings
	theme    Theme
}

func newRichRenderer(settings EffectiveSettings) *richRenderer {
	return &richRenderer{
		settings: settings,
		theme:    NewTheme(settings.ThemeMode),
	}
}

func (r *richRenderer) Settings() EffectiveSettings { return r.settings }

func (r *richRenderer) Status(line StatusLine) string {
	sym := statusSymbol(r.settings.Symbols, line.Status)
	text := line.Text
	detail := line.Detail
	switch line.Status {
	case StatusSuccess:
		sym = applyStyle(r.theme.Success, sym, true)
		text = applyStyle(r.theme.Strong, text, true)
	case StatusWarning:
		sym = applyStyle(r.theme.Warning, sym, true)
	case StatusError:
		sym = applyStyle(r.theme.Error, sym, true)
		text = applyStyle(r.theme.Strong, text, true)
	case StatusInfo:
		sym = applyStyle(r.theme.Info, sym, true)
	}
	out := text
	if sym != "" {
		out = sym + " " + text
	}
	if detail != "" {
		out += " " + applyStyle(r.theme.Muted, detail, true)
	}
	return out
}

func (r *richRenderer) KeyValues(rows []KeyValue) string {
	return formatKeyValues(rows, r.settings, true, r.theme)
}

func (r *richRenderer) Notice(n Notice) string {
	sym := statusSymbol(r.settings.Symbols, n.Status)
	if n.Status == StatusNone {
		sym = r.settings.Symbols.Warning
	}
	switch n.Status {
	case StatusWarning, StatusNone:
		sym = applyStyle(r.theme.Warning, sym, true)
	case StatusError:
		sym = applyStyle(r.theme.Error, sym, true)
	case StatusSuccess:
		sym = applyStyle(r.theme.Success, sym, true)
	case StatusInfo:
		sym = applyStyle(r.theme.Info, sym, true)
	}
	if sym == "" {
		return n.Message
	}
	return sym + " " + n.Message
}

func (r *richRenderer) Hint(h Hint) string {
	arrow := applyStyle(r.theme.Primary, r.settings.Symbols.Arrow, true)
	return arrow + " " + h.Message
}

func (r *richRenderer) Summary(s Summary) string {
	var noticesAndHints []string
	for _, n := range s.Notices {
		noticesAndHints = append(noticesAndHints, r.Notice(n))
	}
	for _, h := range s.Hints {
		noticesAndHints = append(noticesAndHints, r.Hint(h))
	}
	footer := r.renderCompletionFooter(s.CompletionFooter)
	return joinSummarySections(
		r.Status(StatusLine{Status: s.Status, Text: s.Title}),
		r.PackageDeltas(s.Deltas),
		r.KeyValues(s.Metrics),
		strings.Join(noticesAndHints, "\n"),
		footer,
	)
}

func (r *richRenderer) renderCompletionFooter(f *CompletionFooter) string {
	if f == nil {
		return ""
	}
	sym := applyStyle(r.theme.Success, r.settings.Symbols.Success, true)
	msg := applyStyle(r.theme.Success, f.Message, true)
	out := sym + " " + msg
	if f.Duration != "" {
		out += " " + applyStyle(r.theme.Muted, f.Duration, true)
	}
	return out
}

func (r *richRenderer) PackageDeltas(deltas []PackageDelta) string {
	return formatPackageDeltas(deltas, r.settings, true, r.theme)
}

func (r *richRenderer) Table(m TableModel) string {
	return formatTable(m, r.settings, true, r.theme)
}

func (r *richRenderer) Error(view ErrorView) string {
	return formatError(view, r.settings, true, r.theme)
}

func (r *richRenderer) PlainText(s string) string { return s }

func (r *richRenderer) Symbol(st Status) string {
	sym := statusSymbol(r.settings.Symbols, st)
	if sym == "" {
		return ""
	}
	switch st {
	case StatusSuccess:
		return applyStyle(r.theme.Success, sym, true)
	case StatusWarning:
		return applyStyle(r.theme.Warning, sym, true)
	case StatusError:
		return applyStyle(r.theme.Error, sym, true)
	case StatusInfo:
		return applyStyle(r.theme.Info, sym, true)
	default:
		return sym
	}
}
