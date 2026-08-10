package presentation

import (
	"regexp"

	"github.com/fatih/color"
)

var (
	reMewParens = regexp.MustCompile(`\([^)]+\)`)
	reMewANSISGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	reMewName  = regexp.MustCompile(`\b(Mew|mew|mewx|mx|m)\b`)

	dimParenStyle  = color.New(color.Faint)
	mewNameStyle   = color.New(color.FgHiMagenta, color.Bold)
)

// DimParentheses wraps every (...) group with faint/dim styling.
// Strips any nested ANSI inside the parens so content uses dim only.
func DimParentheses(s string) string {
	return reMewParens.ReplaceAllStringFunc(s, func(m string) string {
		clean := reMewANSISGR.ReplaceAllString(m, "")
		return dimParenStyle.Sprint(clean)
	})
}

// StyleMewName wraps "Mew" and binary names (m, mx, mew, mewx) in bright magenta.
func StyleMewName(s string) string {
	return reMewName.ReplaceAllStringFunc(s, func(m string) string {
		return mewNameStyle.Sprint(m)
	})
}
