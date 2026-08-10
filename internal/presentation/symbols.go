package presentation

import "fmt"

// Symbols holds status glyphs for Unicode or ASCII output.
// These are the canonical reusable UI glyph definitions for the terminal.
// Each field is semantic; callers request rendering via [RenderSemanticSymbol] or
// [StaticRenderer.Symbol] rather than hardcoding glyphs.
type Symbols struct {
	Success  string
	Warning  string
	Error    string
	Info     string
	Arrow    string
	Bullet   string
	Pending  string
	Running  string
	Skipped  string
	Added    string
	Updated  string
	Removed  string
	Ellipsis string

	// Placeholder for nil or empty values in human output.
	Placeholder string

	// Separator is a horizontal/detail separator glyph for terminal output.
	// E.g. detail separators in explain output, horizontal rules in help.
	Separator string

	// SpinnerFrames are the animation frames for activity progress spinners.
	SpinnerFrames []string
}

// UnicodeSymbols is the default rich glyph set.
var UnicodeSymbols = Symbols{
	Success:  "✓",
	Warning:  "!",
	Error:    "×",
	Info:     "•",
	Arrow:    "→",
	Bullet:   "•",
	Pending:  "○",
	Running:  "●",
	Skipped:  "–",
	Added:    "+",
	Updated:  "~",
	Removed:  "-",
	Ellipsis: "…",

	Placeholder:   "—",
	Separator:     "—",
	SpinnerFrames: unicodeActivityFrames,
}

// ASCIISymbols is the plain-safe fallback set.
var ASCIISymbols = Symbols{
	Success:  "OK",
	Warning:  "WARN",
	Error:    "ERROR",
	Info:     "*",
	Arrow:    "->",
	Bullet:   "*",
	Pending:  ".",
	Running:  "*",
	Skipped:  "-",
	Added:    "+",
	Updated:  "~",
	Removed:  "-",
	Ellipsis: "...",

	Placeholder:   "-",
	Separator:     "-",
	SpinnerFrames: asciiActivityFrames,
}

// Spinner frame glyphs: these are the canonical sets for activity progress.
var (
	unicodeActivityFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	asciiActivityFrames   = []string{"|", "/", "-", "\\"}
)

// SelectSymbols returns Unicode or ASCII glyphs.
func SelectSymbols(useUnicode bool) Symbols {
	if useUnicode {
		return UnicodeSymbols
	}
	return ASCIISymbols
}

// ValidateSymbolWidths reports symbols whose display width differs from rune count
// in a way that would break naive padding (multi-cell or multi-rune glyphs).
func ValidateSymbolWidths(s Symbols) []string {
	check := []struct {
		name string
		val  string
	}{
		{"Success", s.Success},
		{"Warning", s.Warning},
		{"Error", s.Error},
		{"Info", s.Info},
		{"Arrow", s.Arrow},
		{"Bullet", s.Bullet},
		{"Pending", s.Pending},
		{"Running", s.Running},
		{"Skipped", s.Skipped},
		{"Added", s.Added},
		{"Updated", s.Updated},
		{"Removed", s.Removed},
		{"Ellipsis", s.Ellipsis},
		{"Placeholder", s.Placeholder},
		{"Separator", s.Separator},
	}
	var bad []string
	for _, c := range check {
		runes := len([]rune(c.val))
		cells := CellWidth(c.val)
		if cells != runes {
			bad = append(bad, c.name)
		}
	}
	// Validate spinner frames: each must be exactly 1 cell wide.
	for i, frame := range s.SpinnerFrames {
		if CellWidth(frame) != 1 {
			bad = append(bad, fmt.Sprintf("SpinnerFrame[%d] width=%d", i, CellWidth(frame)))
		}
	}
	return bad
}
