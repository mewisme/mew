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
	sym := RenderSemanticSymbol(r.settings.Symbols, r.theme, line.Status, r.settings.UseColor)
	text := line.Text
	detail := line.Detail
	switch line.Status {
	case StatusSuccess, StatusError:
		text = applyStyle(r.theme.Strong, text, true)
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
	// StatusNone in notices defaults to Warning semantics.
	st := n.Status
	if st == StatusNone {
		st = StatusWarning
	}
	sym := RenderSemanticSymbol(r.settings.Symbols, r.theme, st, r.settings.UseColor)
	if sym == "" {
		return n.Message
	}
	return sym + " " + n.Message
}

func (r *richRenderer) Hint(h Hint) string {
	arrow := RenderSymbolRole(r.settings.Symbols, r.theme, RoleArrow, r.settings.UseColor)
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
	sym := RenderSemanticSymbol(r.settings.Symbols, r.theme, StatusSuccess, r.settings.UseColor)
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

func (r *richRenderer) SpecifierDeltas(deltas []SpecifierDelta) string {
	return formatSpecifierDeltas(deltas, r.settings, true, r.theme)
}

func (r *richRenderer) Table(m TableModel) string {
	return formatTable(m, r.settings, true, r.theme)
}

func (r *richRenderer) Error(view ErrorView) string {
	return formatError(view, r.settings, true, r.theme)
}

func (r *richRenderer) PlainText(s string) string { return s }

func (r *richRenderer) Symbol(st Status) string {
	return RenderSemanticSymbol(r.settings.Symbols, r.theme, st, r.settings.UseColor)
}
