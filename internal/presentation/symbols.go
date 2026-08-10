package presentation

// Symbols holds status glyphs for Unicode or ASCII output.
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

	Placeholder: "—",
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

	Placeholder: "-",
}

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
	}
	var bad []string
	for _, c := range check {
		runes := len([]rune(c.val))
		cells := CellWidth(c.val)
		if cells != runes {
			bad = append(bad, c.name)
		}
	}
	return bad
}
