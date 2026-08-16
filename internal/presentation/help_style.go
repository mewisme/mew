package presentation

import (
	"regexp"
)

var (
	reMewParens  = regexp.MustCompile(`\([^)]+\)`)
	reMewANSISGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	reMewName    = regexp.MustCompile(`\b(Mew|mew|mewx|mx|m)\b`)
)

// DimParentheses wraps every (...) group with faint/dim styling.
// Strips any nested ANSI inside the parens so content uses dim only.
// When useColor is false, returns s unchanged.
func DimParentheses(s string, settings EffectiveSettings) string {
	if !settings.UseColor {
		return s
	}
	theme := NewTheme(settings.ThemeMode)
	return reMewParens.ReplaceAllStringFunc(s, func(m string) string {
		clean := reMewANSISGR.ReplaceAllString(m, "")
		return applyStyle(theme.Faint, clean, true)
	})
}

// StyleMewName wraps "Mew" and binary names (m, mx, mew, mewx) with the
// brand style from the active theme. When useColor is false, returns s
// unchanged.
func StyleMewName(s string, settings EffectiveSettings) string {
	if !settings.UseColor {
		return s
	}
	theme := NewTheme(settings.ThemeMode)
	return reMewName.ReplaceAllStringFunc(s, func(m string) string {
		return applyStyle(theme.Brand, m, true)
	})
}

// DimParenthesesFunc returns a func(string) string suitable for Cobra template
// FuncMap registration.
func DimParenthesesFunc(settings EffectiveSettings) func(string) string {
	return func(s string) string {
		return DimParentheses(s, settings)
	}
}

// StyleMewNameFunc returns a func(string) string suitable for Cobra template
// FuncMap registration.
func StyleMewNameFunc(settings EffectiveSettings) func(string) string {
	return func(s string) string {
		return StyleMewName(s, settings)
	}
}
