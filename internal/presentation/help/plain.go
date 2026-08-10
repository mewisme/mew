// Package helpmd renders trusted terminal-help Markdown for presentation.
package helpmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mewisme/mew/internal/presentation"
)

// RenderOptions controls Markdown rendering for one topic.
type RenderOptions struct {
	Width      int
	Plain      bool
	Accessible bool
	Hyperlinks bool
	// ThemeMode is the resolved palette (ThemeLight, ThemeDark, ThemeAccessible, ThemeNone).
	ThemeMode presentation.ThemeMode
	// UseColor enables semantic ANSI colors for rich TTY output.
	UseColor bool
	// Symbols provides the active glyph set (Unicode or ASCII).
	Symbols presentation.Symbols
}

// Render selects plain or rich Markdown rendering.
func Render(md string, opts RenderOptions) (string, error) {
	opts.Width = presentation.ClampWidth(opts.Width)
	// Default to ASCII symbols when none provided (zero-value guard).
	if opts.Symbols.Bullet == "" {
		opts.Symbols = presentation.ASCIISymbols
	}
	if opts.Plain || opts.Accessible || !opts.UseColor {
		return RenderPlain(md, opts), nil
	}
	return RenderRich(md, opts)
}

var (
	reHeading     = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	reUL          = regexp.MustCompile(`^([-*+])\s+(.*)$`)
	reOL          = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)
	reFence       = regexp.MustCompile("^```")
	reInlineCode  = regexp.MustCompile("`([^`]+)`")
	reLink        = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reBold        = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalic      = regexp.MustCompile(`(^|[^*])\*([^*\n]+)\*([^*]|$)`)
	reHTML        = regexp.MustCompile(`(?s)<[^>]*>`)
	reImage       = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// RenderPlain converts Markdown to width-aware plain text (no ANSI).
// All visible characters are ASCII.
func RenderPlain(md string, opts RenderOptions) string {
	width := presentation.ClampWidth(opts.Width)
	md = reHTMLComment.ReplaceAllString(md, "")
	md = reImage.ReplaceAllString(md, "")
	md = reHTML.ReplaceAllString(md, "")

	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []string
	inFence := false
	var fence []string

	flushFence := func() {
		if len(fence) == 0 {
			return
		}
		for _, fl := range fence {
			out = append(out, hardPrefix(fl, "  ", width)...)
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
			out = append(out, strings.Repeat(opts.Symbols.Separator, 4))
			continue
		}
		if strings.HasPrefix(trim, "|") && strings.Contains(trim, "|") {
			out = append(out, renderTableRow(trim, width)...)
			continue
		}
		if m := reHeading.FindStringSubmatch(line); m != nil {
			text := formatInline(m[2], opts)
			level := len(m[1])
			prefix := strings.Repeat("#", level) + " "
			out = append(out, wrapPrefixed(prefix, text, width)...)
			continue
		}
		if m := reUL.FindStringSubmatch(line); m != nil {
			text := formatInline(m[2], opts)
			out = append(out, wrapPrefixed("  "+opts.Symbols.Bullet+" ", text, width)...)
			continue
		}
		if m := reOL.FindStringSubmatch(line); m != nil {
			text := formatInline(m[2], opts)
			out = append(out, wrapPrefixed("  "+m[1]+". ", text, width)...)
			continue
		}
		text := formatInline(line, opts)
		out = append(out, presentation.WrapWords(text, width)...)
	}
	flushFence()
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

func formatInline(s string, opts RenderOptions) string {
	s = reBold.ReplaceAllString(s, "$1")
	s = reItalic.ReplaceAllString(s, "$1$2$3")
	s = reInlineCode.ReplaceAllString(s, "$1")
	s = reLink.ReplaceAllStringFunc(s, func(m string) string {
		parts := reLink.FindStringSubmatch(m)
		if len(parts) != 3 {
			return m
		}
		text, dest := parts[1], parts[2]
		if opts.Hyperlinks && !opts.Plain && !opts.Accessible && opts.UseColor {
			return osc8(text, dest)
		}
		if text == dest {
			return dest
		}
		return fmt.Sprintf("%s: %s", text, dest)
	})
	return strings.TrimSpace(s)
}

func osc8(text, url string) string {
	return "\x1b]8;;" + url + "\x07" + text + "\x1b]8;;\x07"
}

func wrapPrefixed(prefix, text string, width int) []string {
	remain := width - presentation.CellWidth(prefix)
	if remain < 20 {
		return presentation.WrapWords(prefix+text, width)
	}
	wrapped := presentation.WrapWords(text, remain)
	if len(wrapped) == 0 {
		return []string{prefix}
	}
	out := make([]string, 0, len(wrapped))
	pad := strings.Repeat(" ", presentation.CellWidth(prefix))
	for i, w := range wrapped {
		if i == 0 {
			out = append(out, prefix+w)
			continue
		}
		out = append(out, pad+w)
	}
	return out
}

func hardPrefix(line, prefix string, width int) []string {
	remain := width - presentation.CellWidth(prefix)
	if remain < 8 {
		return presentation.WrapWords(prefix+line, width)
	}
	if presentation.CellWidth(line) <= remain {
		return []string{prefix + line}
	}
	parts := presentation.WrapWords(line, remain)
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = prefix + p
	}
	return out
}

func renderTableRow(line string, width int) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	// Skip markdown separator rows.
	sep := true
	for _, c := range cells {
		if c == "" {
			continue
		}
		for _, r := range c {
			if r != '-' && r != ':' && r != ' ' {
				sep = false
				break
			}
		}
		if !sep {
			break
		}
	}
	if sep {
		return nil
	}
	joined := strings.Join(cells, " | ")
	return presentation.WrapWords(joined, width)
}
