package presentation

// RenderSemanticSymbol is the canonical symbol-color pairing for callers that do
// not have access to a [StaticRenderer]. It returns the symbol for status colored
// according to theme when useColor is true.
func RenderSemanticSymbol(symbols Symbols, theme Theme, st Status, useColor bool) string {
	sym := statusSymbol(symbols, st)
	if sym == "" {
		return ""
	}
	if !useColor {
		return sym
	}
	switch st {
	case StatusSuccess:
		return applyStyle(theme.Success, sym, true)
	case StatusWarning, StatusCancelled:
		return applyStyle(theme.Warning, sym, true)
	case StatusError:
		return applyStyle(theme.Error, sym, true)
	case StatusInfo:
		return applyStyle(theme.Info, sym, true)
	case StatusRunning:
		return applyStyle(theme.Running, sym, true)
	case StatusPending:
		return applyStyle(theme.Pending, sym, true)
	case StatusSkipped:
		return applyStyle(theme.Skipped, sym, true)
	default:
		return sym
	}
}
