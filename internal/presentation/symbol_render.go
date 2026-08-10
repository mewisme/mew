package presentation

// RenderSemanticSymbol is the canonical symbol-color pairing for Status symbols.
// It returns the symbol for status colored according to theme when useColor is true.
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

// RenderSymbolRole is the canonical symbol-color pairing for non-status semantic
// symbols (Added, Updated, Removed, Arrow, Bullet, Ellipsis, Placeholder,
// Separator). It returns the glyph for role colored according to theme when
// useColor is true.
func RenderSymbolRole(symbols Symbols, theme Theme, role SymbolRole, useColor bool) string {
	glyph := symbolRoleGlyph(symbols, role)
	if glyph == "" {
		return ""
	}
	if !useColor {
		return glyph
	}
	switch role {
	case RoleAdded:
		return applyStyle(theme.Added, glyph, true)
	case RoleUpdated:
		return applyStyle(theme.Updated, glyph, true)
	case RoleRemoved:
		return applyStyle(theme.Removed, glyph, true)
	case RoleArrow:
		return applyStyle(theme.Arrow, glyph, true)
	case RoleBullet:
		return applyStyle(theme.Primary, glyph, true)
	case RoleEllipsis, RolePlaceholder, RoleSeparator:
		return applyStyle(theme.Muted, glyph, true)
	default:
		return glyph
	}
}

// RenderSpinnerFrame returns a single spinner frame colored with the Running
// semantic style. idx selects the frame within the active symbol set.
func RenderSpinnerFrame(symbols Symbols, theme Theme, idx int, useColor bool) string {
	frames := symbols.SpinnerFrames
	if len(frames) == 0 {
		return ""
	}
	frame := frames[idx%len(frames)]
	if !useColor {
		return frame
	}
	return applyStyle(theme.Running, frame, true)
}

func symbolRoleGlyph(s Symbols, role SymbolRole) string {
	switch role {
	case RoleAdded:
		return s.Added
	case RoleUpdated:
		return s.Updated
	case RoleRemoved:
		return s.Removed
	case RoleArrow:
		return s.Arrow
	case RoleBullet:
		return s.Bullet
	case RoleEllipsis:
		return s.Ellipsis
	case RolePlaceholder:
		return s.Placeholder
	case RoleSeparator:
		return s.Separator
	default:
		return ""
	}
}
