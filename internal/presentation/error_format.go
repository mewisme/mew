package presentation

import (
	"regexp"
	"strings"

	fc "github.com/fatih/color"
)

var reErrCode = regexp.MustCompile(`ERR_M_\w+`)

// colorErrorRefs wraps ERR_M_* codes with the given style.
func colorErrorRefs(s string, c *fc.Color, useColor bool) string {
	if !useColor || c == nil {
		return s
	}
	return reErrCode.ReplaceAllStringFunc(s, func(m string) string {
		return c.Sprint(m)
	})
}

func formatError(view ErrorView, settings EffectiveSettings, color bool, theme Theme) string {
	if view.Title == "" && view.Message == "" {
		return ""
	}
	var parts []string
	title := view.Title
	if !color {
		if title != "" {
			title = "ERROR " + title
		}
	} else {
		if sym := RenderSemanticSymbol(settings.Symbols, theme, StatusError, true); sym != "" {
			title = sym + " " + title
		}
	}
	if title != "" {
		parts = append(parts, title)
	}
	if view.Message != "" {
		if len(parts) > 0 {
			parts = append(parts, "")
		}
		msg := view.Message
		if color {
			msg = colorErrorRefs(msg, theme.Error, true)
		}
		parts = append(parts, msg)
	}
	if len(view.Context) > 0 {
		if len(parts) > 0 {
			parts = append(parts, "")
		}
		parts = append(parts, formatKeyValues(view.Context, settings, color, theme))
	}
	if view.Code != "" {
		if len(parts) > 0 {
			parts = append(parts, "")
		}
		codeLine := FormatErrorCode(view.Code)
		if color {
			codeLine = applyStyle(theme.Muted, "Code: ", true) + applyStyle(theme.Error, view.Code, true)
		}
		parts = append(parts, codeLine)
	}
	for _, h := range view.Hints {
		if h.Message == "" {
			continue
		}
		parts = append(parts, formatHintLine(h, settings, color, theme))
	}
	for _, c := range view.Causes {
		if c.Message == "" {
			continue
		}
		line := c.Label + ": " + c.Message
		if color {
			line = applyStyle(theme.Error, c.Label, true) + applyStyle(theme.Muted, ": "+c.Message, true)
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "\n")
}

func formatHintLine(h Hint, settings EffectiveSettings, color bool, theme Theme) string {
	arrow := RenderSymbolRole(settings.Symbols, theme, RoleActionArrow, color)
	if arrow == "" {
		return h.Message
	}
	return arrow + " " + h.Message
}
