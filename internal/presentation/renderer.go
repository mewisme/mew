package presentation

// StaticRenderer turns presentation models into deterministic strings.
type StaticRenderer interface {
	Status(StatusLine) string
	KeyValues([]KeyValue) string
	Summary(Summary) string
	Notice(Notice) string
	Hint(Hint) string
	Error(ErrorView) string
	Table(TableModel) string
	PackageDeltas([]PackageDelta) string
	SpecifierDeltas([]SpecifierDelta) string
	PlainText(string) string
	// Symbol returns a status glyph, colored when the theme supports it.
	Symbol(status Status) string
	// StyledText returns text styled according to the given ValueKind.
	StyledText(text string, kind ValueKind) string
	Settings() EffectiveSettings
}

// NewStaticRenderer returns a plain or rich renderer from settings.
// ThemeNone and !UseColor always use the plain renderer (zero ANSI).
func NewStaticRenderer(settings EffectiveSettings) StaticRenderer {
	if settings.ThemeMode == ThemeNone || !settings.UseColor {
		return newPlainRenderer(settings)
	}
	return newRichRenderer(settings)
}
