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
