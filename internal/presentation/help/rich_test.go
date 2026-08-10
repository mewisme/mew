package helpmd_test

import (
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/presentation"
	helpmd "github.com/mewisme/mew/internal/presentation/help"
)

func TestRenderRichASCIIContent(t *testing.T) {
	md := "# Errors\n\nUse `m help errors` for codes.\n\n- one\n- two\n"
	out, err := helpmd.RenderRich(md, helpmd.RenderOptions{Width: 72, ThemeMode: presentation.ThemeDark, UseColor: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Errors") {
		t.Fatalf("missing heading text:\n%s", out)
	}
	// No Unicode bullets — must use ASCII "-".
	if strings.Contains(out, "•") || strings.Contains(out, "◦") {
		t.Fatalf("Unicode bullet leaked:\n%s", out)
	}
	if strings.Contains(out, "<script>") {
		t.Fatalf("html leaked:\n%s", out)
	}
}

func TestRenderRichLightTheme(t *testing.T) {
	md := "## Light\n\nbody text\n"
	out, err := helpmd.RenderRich(md, helpmd.RenderOptions{Width: 60, ThemeMode: presentation.ThemeLight, UseColor: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Light") {
		t.Fatalf("missing heading:\n%s", out)
	}
}

func TestRenderRichStripsHTMLAndImages(t *testing.T) {
	md := "Hello <script>alert(1)</script> ![x](http://evil) world"
	out, err := helpmd.RenderRich(md, helpmd.RenderOptions{Width: 80, ThemeMode: presentation.ThemeDark, UseColor: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "script") || strings.Contains(out, "http://evil") {
		t.Fatalf("unsafe content remained:\n%s", out)
	}
}

func TestRenderPlainFlagSkipsANSI(t *testing.T) {
	md := "# Hello\n\n**bold** and `code`\n\n- item\n"
	out, err := helpmd.Render(md, helpmd.RenderOptions{Width: 80, Plain: true, ThemeMode: presentation.ThemeDark, UseColor: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("plain path emitted SGR:\n%q", out)
	}
	if strings.Contains(out, "•") {
		t.Fatalf("plain path used Unicode bullet:\n%s", out)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "* item") {
		t.Fatalf("missing plain content:\n%s", out)
	}
}

func TestRenderUseColorEmitsANSI(t *testing.T) {
	md := "# Hello\n\n- item\n"
	out, err := helpmd.Render(md, helpmd.RenderOptions{
		Width: 80, ThemeMode: presentation.ThemeDark, UseColor: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("UseColor should emit SGR:\n%q", out)
	}
}

func TestRenderRichThemeAcceptsBothPalettes(t *testing.T) {
	md := "# Theme\n\n`code` and **bold**\n"
	dark, err := helpmd.RenderRich(md, helpmd.RenderOptions{
		Width: 72, ThemeMode: presentation.ThemeDark, UseColor: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	light, err := helpmd.RenderRich(md, helpmd.RenderOptions{
		Width: 72, ThemeMode: presentation.ThemeLight, UseColor: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Both palettes use bright ANSI — works on any background.
	if !strings.Contains(dark, "\x1b[") || !strings.Contains(light, "\x1b[") {
		t.Fatalf("expected ANSI in both:\ndark=%q\nlight=%q", dark, light)
	}
	if !strings.Contains(dark, "Theme") || !strings.Contains(light, "Theme") {
		t.Fatalf("missing content")
	}
}

func TestRenderNoColorProducesPlain(t *testing.T) {
	md := "# Hello\n\n- item\n"
	out, err := helpmd.Render(md, helpmd.RenderOptions{
		Width: 80, ThemeMode: presentation.ThemeDark, UseColor: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("UseColor=false emitted SGR:\n%q", out)
	}
}

func TestRenderAccessibleProducesPlain(t *testing.T) {
	md := "# Hello\n\n- item\n"
	out, err := helpmd.Render(md, helpmd.RenderOptions{
		Width: 80, ThemeMode: presentation.ThemeDark, Accessible: true, UseColor: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("Accessible emitted SGR:\n%q", out)
	}
}

func TestRenderASCIIOnlyGlyphs(t *testing.T) {
	md := "# Heading\n\n- list item\n\n1. ordered\n\n---\n\n`code`\n\n**bold**\n\n[link](https://example.com)\n"
	out, err := helpmd.RenderRich(md, helpmd.RenderOptions{Width: 80, ThemeMode: presentation.ThemeDark, UseColor: true})
	if err != nil {
		t.Fatal(err)
	}
	// Strip ANSI escape sequences for glyph check.
	noANSI := stripANSI(out)
	for _, r := range noANSI {
		if r > 127 {
			t.Fatalf("non-ASCII rune U+%04X in:\n%q", r, noANSI)
		}
	}
}

func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if inEsc {
			if s[i] >= '@' && s[i] <= '~' {
				inEsc = false
			}
			continue
		}
		if s[i] == '\x1b' {
			inEsc = true
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}
