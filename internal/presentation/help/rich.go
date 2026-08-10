package helpmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"

	"github.com/mewisme/mew/internal/presentation"
)

// RenderRich renders Markdown as ASCII-only text with semantic ANSI colors.
// No Glamour — all glyphs are ASCII; styling uses the shared theme palette.
func RenderRich(md string, opts RenderOptions) (string, error) {
	md = reHTMLComment.ReplaceAllString(md, "")
	md = reImage.ReplaceAllString(md, "")
	md = reHTML.ReplaceAllString(md, "")

	theme := presentation.NewTheme(opts.ThemeMode)
	width := presentation.ClampWidth(opts.Width)

	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []string
	inFence := false
	var fence []string

	flushFence := func() {
		if len(fence) == 0 {
			return
		}
		for _, fl := range fence {
			out = append(out, theme.Muted.Sprint("  "+fl))
		}
		fence = nil
	}

	for _, line := range lines {
		if reFence.MatchString(strings.TrimSpace(line)) {
			if inFence {
				flushFence()
				inFence = false
			} else {
				inFence = true
			}
			continue
		}
		if inFence {
			fence = append(fence, line)
			continue
		}

		trim := strings.TrimSpace(line)
		if trim == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		// Horizontal rule.
		if trim == "---" || trim == "***" || trim == "___" || trim == "- - -" || trim == "* * *" {
			rule := strings.Repeat(opts.Symbols.Separator, 4)
			out = append(out, theme.Muted.Sprint(rule))
			continue
		}
		if strings.HasPrefix(trim, "|") && strings.Contains(trim, "|") {
			for _, r := range renderTableRow(trim, width) {
				out = append(out, theme.Muted.Sprint(r))
			}
			continue
		}
		if m := reHeading.FindStringSubmatch(line); m != nil {
			text := formatInline(m[2], opts)
			level := len(m[1])
			prefix := strings.Repeat("#", level) + " "
			out = append(out, wrapPrefixedStyled(prefix, text, width, theme.Header, theme.Strong)...)
			continue
		}
		if m := reUL.FindStringSubmatch(line); m != nil {
			text := formatInline(m[2], opts)
			out = append(out, wrapPrefixedStyled("  "+opts.Symbols.Bullet+" ", text, width, theme.Primary, nil)...)
			continue
		}
		if m := reOL.FindStringSubmatch(line); m != nil {
			text := formatInline(m[2], opts)
			out = append(out, wrapPrefixedStyled("  "+m[1]+". ", text, width, theme.Primary, nil)...)
			continue
		}
		text := formatInlineRich(line, opts, theme)
		out = append(out, presentation.WrapWords(text, width)...)
	}
	flushFence()
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n", nil
}

// wrapPrefixedStyled wraps text with a styled prefix.
// bodyStyle may be nil — continuation lines are then emitted unstyled.
func wrapPrefixedStyled(prefix, text string, width int, prefixStyle, bodyStyle *color.Color) []string {
	rawOut := wrapPrefixed(prefix, text, width)
	if len(rawOut) == 0 {
		return []string{prefixStyle.Sprint(prefix)}
	}
	out := make([]string, len(rawOut))
	padLen := presentation.CellWidth(prefix)
	for i, line := range rawOut {
		if i == 0 {
			if padLen < len(line) {
				out[i] = prefixStyle.Sprint(line[:padLen]) + applyMaybe(bodyStyle, line[padLen:])
			} else {
				out[i] = prefixStyle.Sprint(line)
			}
		} else {
			out[i] = applyMaybe(bodyStyle, line)
		}
	}
	return out
}

func applyMaybe(c *color.Color, s string) string {
	if c == nil {
		return s
	}
	return c.Sprint(s)
}

func formatInlineRich(s string, opts RenderOptions, theme presentation.Theme) string {
	s = reBold.ReplaceAllString(s, "$1")
	s = reItalic.ReplaceAllString(s, "$1$2$3")
	s = reInlineCode.ReplaceAllStringFunc(s, func(m string) string {
		parts := reInlineCode.FindStringSubmatch(m)
		if len(parts) != 2 {
			return m
		}
		return theme.Code.Sprint(parts[1])
	})
	s = reLink.ReplaceAllStringFunc(s, func(m string) string {
		parts := reLink.FindStringSubmatch(m)
		if len(parts) != 3 {
			return m
		}
		text, dest := parts[1], parts[2]
		if opts.Hyperlinks {
			return osc8(theme.Primary.Sprint(text), dest)
		}
		if text == dest {
			return theme.Primary.Sprint(dest)
		}
		return fmt.Sprintf("%s: %s", theme.Primary.Sprint(text), theme.Muted.Sprint(dest))
	})
	return strings.TrimSpace(s)
}
