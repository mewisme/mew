package presentation

import "github.com/fatih/color"

// Theme holds semantic *color.Color styles for one palette.
type Theme struct {
	Success *color.Color
	Warning *color.Color
	Error   *color.Color
	Info    *color.Color

	Primary   *color.Color
	Secondary *color.Color
	Muted     *color.Color
	Strong    *color.Color

	Command *color.Color
	Package *color.Color
	Version *color.Color
	Path    *color.Color
	Code    *color.Color
	Number  *color.Color

	Added       *color.Color
	AddedBold   *color.Color
	Updated     *color.Color
	UpdatedBold *color.Color
	Removed     *color.Color
	RemovedBold *color.Color
	Reused      *color.Color

	Header *color.Color
	Label  *color.Color
	Value  *color.Color
}

// NewTheme builds a palette for mode. ThemeNone returns identity colors.
func NewTheme(mode ThemeMode) Theme {
	switch mode {
	case ThemeDark:
		return darkTheme()
	case ThemeAccessible:
		return accessibleTheme()
	case ThemeNone:
		return noneTheme()
	default:
		return lightTheme()
	}
}

func noneTheme() Theme {
	id := color.New()
	return Theme{
		Success: id, Warning: id, Error: id, Info: id,
		Primary: id, Secondary: id, Muted: id, Strong: id,
		Command: id, Package: id, Version: id, Path: id, Code: id, Number: id,
		Added: id, AddedBold: id, Updated: id, UpdatedBold: id, Removed: id, RemovedBold: id, Reused: id,
		Header: id, Label: id, Value: id,
	}
}

func lightTheme() Theme {
	return Theme{
		Success:     color.New(color.FgGreen),
		Warning:     color.New(color.FgYellow),
		Error:       color.New(color.FgRed),
		Info:        color.New(color.FgCyan),
		Primary:     color.New(color.FgCyan),
		Muted:       color.New(color.FgHiBlack),
		Strong:      color.New(color.FgBlack, color.Bold),
		Command:     color.New(color.FgCyan),
		Package:     color.New(color.FgCyan),
		Version:     color.New(color.FgBlack),
		Path:        color.New(color.FgHiBlack),
		Code:        color.New(color.FgMagenta),
		Number:      color.New(color.FgBlack),
		Added:       color.New(color.FgGreen),
		AddedBold:   color.New(color.FgGreen, color.Bold),
		Updated:     color.New(color.FgYellow),
		UpdatedBold: color.New(color.FgYellow, color.Bold),
		Removed:     color.New(color.FgRed),
		RemovedBold: color.New(color.FgRed, color.Bold),
		Reused:      color.New(color.FgHiBlack),
		Header:      color.New(color.FgBlack, color.Bold),
		Label:       color.New(color.FgBlack, color.Bold),
		Value:       color.New(color.FgBlack),
	}
}

func darkTheme() Theme {
	return Theme{
		Success:     color.New(color.FgHiGreen),
		Warning:     color.New(color.FgHiYellow),
		Error:       color.New(color.FgHiRed),
		Info:        color.New(color.FgHiCyan),
		Primary:     color.New(color.FgHiCyan),
		Muted:       color.New(color.FgHiBlack),
		Strong:      color.New(color.FgHiWhite, color.Bold),
		Command:     color.New(color.FgHiCyan),
		Package:     color.New(color.FgHiMagenta),
		Version:     color.New(color.FgHiGreen),
		Path:        color.New(color.FgHiBlue),
		Code:        color.New(color.FgHiMagenta),
		Number:      color.New(color.FgHiYellow),
		Added:       color.New(color.FgHiGreen),
		AddedBold:   color.New(color.FgHiGreen, color.Bold),
		Updated:     color.New(color.FgHiYellow),
		UpdatedBold: color.New(color.FgHiYellow, color.Bold),
		Removed:     color.New(color.FgHiRed),
		RemovedBold: color.New(color.FgHiRed, color.Bold),
		Reused:      color.New(color.FgHiBlack),
		Header:      color.New(color.FgHiWhite, color.Bold),
		Label:       color.New(color.FgHiWhite, color.Bold),
		Value:       color.New(color.FgHiWhite),
	}
}

func accessibleTheme() Theme {
	bold := color.New(color.Bold)
	plain := color.New()
	return Theme{
		Success:     bold,
		Warning:     bold,
		Error:       bold,
		Info:        plain,
		Primary:     bold,
		Muted:       plain,
		Strong:      bold,
		Command:     bold,
		Package:     bold,
		Version:     plain,
		Path:        plain,
		Code:        plain,
		Number:      plain,
		Added:       bold,
		AddedBold:   bold,
		Updated:     bold,
		UpdatedBold: bold,
		Removed:     bold,
		RemovedBold: bold,
		Reused:      plain,
		Header:      bold,
		Label:       bold,
		Value:       plain,
	}
}

// applyStyle applies c to text when color is enabled and text is non-empty.
// A nil *color.Color is treated as identity (no styling).
func applyStyle(c *color.Color, text string, useColor bool) string {
	if !useColor || text == "" || c == nil {
		return text
	}
	return c.Sprint(text)
}
