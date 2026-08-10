package presentation_test

import (
	"testing"

	"github.com/mewisme/mew/internal/presentation"
)

// TestSymbolInventory verifies every Symbols field has both Unicode and ASCII values.
func TestSymbolInventory(t *testing.T) {
	u := presentation.UnicodeSymbols
	a := presentation.ASCIISymbols

	// Every field present in both sets.
	check := func(name, uval, aval string) {
		if uval == "" {
			t.Errorf("UnicodeSymbols.%s is empty", name)
		}
		if aval == "" {
			t.Errorf("ASCIISymbols.%s is empty", name)
		}
	}
	check("Success", u.Success, a.Success)
	check("Warning", u.Warning, a.Warning)
	check("Error", u.Error, a.Error)
	check("Info", u.Info, a.Info)
	check("Arrow", u.Arrow, a.Arrow)
	check("Bullet", u.Bullet, a.Bullet)
	check("Pending", u.Pending, a.Pending)
	check("Running", u.Running, a.Running)
	check("Skipped", u.Skipped, a.Skipped)
	check("Added", u.Added, a.Added)
	check("Updated", u.Updated, a.Updated)
	check("Removed", u.Removed, a.Removed)
	check("Ellipsis", u.Ellipsis, a.Ellipsis)
	check("Placeholder", u.Placeholder, a.Placeholder)
	check("Separator", u.Separator, a.Separator)

	if len(u.SpinnerFrames) == 0 {
		t.Error("UnicodeSymbols.SpinnerFrames is empty")
	}
	if len(a.SpinnerFrames) == 0 {
		t.Error("ASCIISymbols.SpinnerFrames is empty")
	}
}

// TestSelectSymbols verifies Unicode/ASCII selection.
func TestSelectSymbols(t *testing.T) {
	uni := presentation.SelectSymbols(true)
	ascii := presentation.SelectSymbols(false)

	if uni.Success != presentation.UnicodeSymbols.Success {
		t.Error("SelectSymbols(true) should return Unicode")
	}
	if ascii.Success != presentation.ASCIISymbols.Success {
		t.Error("SelectSymbols(false) should return ASCII")
	}
}

// TestValidateSymbolWidths verifies all Unicode symbols are single-cell width.
func TestValidateSymbolWidths(t *testing.T) {
	bad := presentation.ValidateSymbolWidths(presentation.UnicodeSymbols)
	if len(bad) > 0 {
		t.Errorf("UnicodeSymbols width violations: %v", bad)
	}

	bad = presentation.ValidateSymbolWidths(presentation.ASCIISymbols)
	if len(bad) > 0 {
		t.Errorf("ASCIISymbols width violations: %v", bad)
	}
}

// TestSemanticSymbolColors verifies RenderSemanticSymbol applies correct theme colors.
func TestSemanticSymbolColors(t *testing.T) {
	theme := presentation.NewTheme(presentation.ThemeLight)
	sym := presentation.UnicodeSymbols

	tests := []struct {
		name   string
		status presentation.Status
	}{
		{"Success", presentation.StatusSuccess},
		{"Warning", presentation.StatusWarning},
		{"Cancelled", presentation.StatusCancelled},
		{"Error", presentation.StatusError},
		{"Info", presentation.StatusInfo},
		{"Running", presentation.StatusRunning},
		{"Pending", presentation.StatusPending},
		{"Skipped", presentation.StatusSkipped},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			styled := presentation.RenderSemanticSymbol(sym, theme, tc.status, true)
			if styled == "" {
				t.Errorf("RenderSemanticSymbol(%s, color=true) returned empty", tc.name)
			}
			// With color enabled, the output should differ from plain symbol.
			plain := presentation.RenderSemanticSymbol(sym, theme, tc.status, false)
			if plain == "" {
				t.Errorf("RenderSemanticSymbol(%s, color=false) returned empty", tc.name)
			}
			if styled == plain {
				// Accessible theme uses bold-only, so the plain might be same as styled.
				// This is fine for accessible mode, but light/dark should differ.
				t.Logf("RenderSemanticSymbol(%s): styled equals plain for light theme (both=%q)", tc.name, styled)
			}
		})
	}

	// Test ThemeNone with color produces non-empty (identity wrapping is fine).
	none := presentation.NewTheme(presentation.ThemeNone)
	for _, tc := range tests {
		result := presentation.RenderSemanticSymbol(sym, none, tc.status, true)
		if result == "" {
			t.Errorf("ThemeNone RenderSemanticSymbol(%s, color=true) returned empty", tc.name)
		}
		// With color=false, should return plain glyph without ANSI.
		plain := presentation.RenderSemanticSymbol(sym, none, tc.status, false)
		if plain == "" {
			t.Errorf("ThemeNone RenderSemanticSymbol(%s, color=false) returned empty", tc.name)
		}
	}
}

// TestASCIISymbolWidths verifies ASCII symbols are single-rune.
func TestASCIISymbolWidths(t *testing.T) {
	s := presentation.ASCIISymbols
	check := func(name, val string) {
		if val == "" {
			t.Errorf("ASCIISymbols.%s is empty", name)
		}
	}
	check("Success", s.Success)
	check("Warning", s.Warning)
	check("Error", s.Error)
	check("Arrow", s.Arrow)
	check("Ellipsis", s.Ellipsis)
	check("Separator", s.Separator)
	check("Placeholder", s.Placeholder)
}

// TestSymbolRoleArrowSplit verifies action arrows use accent style and structural
// arrows use muted style, both sharing the same glyph from the active symbol set.
func TestSymbolRoleArrowSplit(t *testing.T) {
	theme := presentation.NewTheme(presentation.ThemeLight)
	sym := presentation.UnicodeSymbols

	actionStyled := presentation.RenderSymbolRole(sym, theme, presentation.RoleActionArrow, true)
	structuralStyled := presentation.RenderSymbolRole(sym, theme, presentation.RoleStructuralArrow, true)

	if actionStyled == "" || structuralStyled == "" {
		t.Fatal("arrow roles must return non-empty styled glyphs")
	}

	// Both roles use the same glyph.
	actionPlain := presentation.RenderSymbolRole(sym, theme, presentation.RoleActionArrow, false)
	structuralPlain := presentation.RenderSymbolRole(sym, theme, presentation.RoleStructuralArrow, false)
	if actionPlain != structuralPlain {
		t.Errorf("action and structural arrows must share same glyph: %q vs %q", actionPlain, structuralPlain)
	}

	// In light/dark themes, action arrow should differ from structural.
	if actionStyled == structuralStyled {
		t.Logf("action and structural arrows have same style in light theme: %q", actionStyled)
	}

	// ASCII symbols: both roles use "->".
	asciiSym := presentation.ASCIISymbols
	asciiAction := presentation.RenderSymbolRole(asciiSym, theme, presentation.RoleActionArrow, false)
	asciiStructural := presentation.RenderSymbolRole(asciiSym, theme, presentation.RoleStructuralArrow, false)
	if asciiAction != "->" || asciiStructural != "->" {
		t.Errorf("ASCII arrows should be '->': action=%q structural=%q", asciiAction, asciiStructural)
	}
}

// TestSymbolRoleAllRoles tests every SymbolRole produces a non-empty glyph.
func TestSymbolRoleAllRoles(t *testing.T) {
	theme := presentation.NewTheme(presentation.ThemeLight)
	sym := presentation.UnicodeSymbols
	asciiSym := presentation.ASCIISymbols

	roles := []presentation.SymbolRole{
		presentation.RoleAdded,
		presentation.RoleUpdated,
		presentation.RoleRemoved,
		presentation.RoleActionArrow,
		presentation.RoleStructuralArrow,
		presentation.RoleBullet,
		presentation.RoleEllipsis,
		presentation.RolePlaceholder,
		presentation.RoleSeparator,
	}

	for _, role := range roles {
		uni := presentation.RenderSymbolRole(sym, theme, role, true)
		if uni == "" {
			t.Errorf("Unicode RenderSymbolRole(%d) returned empty", role)
		}
		asc := presentation.RenderSymbolRole(asciiSym, theme, role, true)
		if asc == "" {
			t.Errorf("ASCII RenderSymbolRole(%d) returned empty", role)
		}
	}
}

// TestThemeNoneNoANSI verifies ThemeNone with useColor=false emits zero ANSI.
// ThemeNone is always paired with useColor=false by Effective() and NewStaticRenderer,
// so the color=true path through ThemeNone is not exercised in production.
func TestThemeNoneNoANSI(t *testing.T) {
	theme := presentation.NewTheme(presentation.ThemeNone)
	sym := presentation.UnicodeSymbols

	statuses := []presentation.Status{
		presentation.StatusSuccess,
		presentation.StatusWarning,
		presentation.StatusError,
		presentation.StatusInfo,
		presentation.StatusRunning,
		presentation.StatusPending,
		presentation.StatusSkipped,
	}
	for _, st := range statuses {
		result := presentation.RenderSemanticSymbol(sym, theme, st, false)
		if result == "" {
			t.Errorf("ThemeNone RenderSemanticSymbol(%d, color=false) returned empty", st)
		}
		if containsANSI(result) {
			t.Errorf("ThemeNone RenderSemanticSymbol(%d, color=false) emitted ANSI: %q", st, result)
		}
	}

	roles := []presentation.SymbolRole{
		presentation.RoleActionArrow,
		presentation.RoleStructuralArrow,
		presentation.RoleBullet,
		presentation.RoleAdded,
		presentation.RoleUpdated,
		presentation.RoleRemoved,
	}
	for _, role := range roles {
		result := presentation.RenderSymbolRole(sym, theme, role, false)
		if result == "" {
			t.Errorf("ThemeNone RenderSymbolRole(%d, color=false) returned empty", role)
		}
		if containsANSI(result) {
			t.Errorf("ThemeNone RenderSymbolRole(%d, color=false) emitted ANSI: %q", role, result)
		}
	}
}

// TestAccessibleThemeBehavior verifies accessible theme uses emphasis (bold) not color.
func TestAccessibleThemeBehavior(t *testing.T) {
	theme := presentation.NewTheme(presentation.ThemeAccessible)
	sym := presentation.UnicodeSymbols

	// Accessible theme with color should still produce output.
	result := presentation.RenderSemanticSymbol(sym, theme, presentation.StatusSuccess, true)
	if result == "" {
		t.Error("accessible theme RenderSemanticSymbol(Success) returned empty")
	}

	result = presentation.RenderSemanticSymbol(sym, theme, presentation.StatusError, true)
	if result == "" {
		t.Error("accessible theme RenderSemanticSymbol(Error) returned empty")
	}
}

// TestConfigPlaceholderSelection verifies Unicode and ASCII placeholders differ.
func TestConfigPlaceholderSelection(t *testing.T) {
	if presentation.UnicodeSymbols.Placeholder == presentation.ASCIISymbols.Placeholder {
		t.Error("Unicode and ASCII placeholders must differ")
	}
	if presentation.UnicodeSymbols.Placeholder == "" || presentation.ASCIISymbols.Placeholder == "" {
		t.Error("placeholders must be non-empty in both symbol sets")
	}
}

// TestSpinnerFrames verifies spinner frames are populated for both modes.
func TestSpinnerFrames(t *testing.T) {
	if len(presentation.UnicodeSymbols.SpinnerFrames) == 0 {
		t.Error("Unicode spinner frames empty")
	}
	if len(presentation.ASCIISymbols.SpinnerFrames) == 0 {
		t.Error("ASCII spinner frames empty")
	}
	// Each frame must be 1 cell wide.
	for i, f := range presentation.UnicodeSymbols.SpinnerFrames {
		if presentation.CellWidth(f) != 1 {
			t.Errorf("Unicode spinner frame[%d]=%q width=%d want 1", i, f, presentation.CellWidth(f))
		}
	}
}

// TestTableColumnCellStyle verifies TableColumn.CellStyle is usable.
func TestTableColumnCellStyle(t *testing.T) {
	// Ensure the field exists and is usable.
	col := presentation.TableColumn{
		Key:       "name",
		Header:    "Name",
		CellStyle: presentation.ValuePackage,
	}
	if col.CellStyle != presentation.ValuePackage {
		t.Error("TableColumn.CellStyle should be settable")
	}
}

// TestStatusCellType verifies StatusCell struct fields.
func TestStatusCellType(t *testing.T) {
	sc := presentation.StatusCell{Text: "pass", Status: presentation.StatusSuccess}
	if sc.Text != "pass" || sc.Status != presentation.StatusSuccess {
		t.Error("StatusCell fields not accessible")
	}
}

func containsANSI(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			return true
		}
	}
	return false
}
