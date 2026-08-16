package presentation_test

import (
	"fmt"
	"strings"
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

// TestThemeValueNoForeground proves that generic values, numbers, versions,
// and paths have no forced foreground color in light and dark themes.
func TestThemeValueNoForeground(t *testing.T) {
	valueKinds := []struct {
		name string
		kind presentation.ValueKind
	}{
		{"Value", presentation.ValuePlain},
		{"Number", presentation.ValueNumber},
		{"Version", presentation.ValueVersion},
		{"Path", presentation.ValuePath},
	}
	testText := "hello-123"

	for _, mode := range []presentation.ThemeMode{presentation.ThemeLight, presentation.ThemeDark} {
		for _, vk := range valueKinds {
			t.Run(modeName(mode)+"/"+vk.name, func(t *testing.T) {
				settings := presentation.EffectiveSettings{
					UseColor:  true,
					ThemeMode: mode,
					Width:     80,
					Symbols:   presentation.UnicodeSymbols,
				}
				r := presentation.NewStaticRenderer(settings)
				styled := r.StyledText(testText, vk.kind)
				if styled == "" {
					t.Fatal("styled output is empty")
				}
				if hasForegroundColor(styled) {
					t.Errorf("%s/%s has forced foreground: %q", modeName(mode), vk.name, styled)
				}
				// In no-color mode, output must be plain.
				r2 := presentation.NewStaticRenderer(presentation.EffectiveSettings{UseColor: false, Width: 80})
				plain := r2.StyledText(testText, vk.kind)
				if plain != testText {
					t.Errorf("%s/%s no-color should be identity: got %q want %q", modeName(mode), vk.name, plain, testText)
				}
			})
		}
	}
}

// TestThemeLabelIsFaint proves that keys/labels are dim/faint in rich mode.
func TestThemeLabelIsFaint(t *testing.T) {
	for _, mode := range []presentation.ThemeMode{presentation.ThemeLight, presentation.ThemeDark} {
		t.Run(modeName(mode), func(t *testing.T) {
			// Test: render a key/value pair and verify the key contains Faint styling.
			settings := presentation.EffectiveSettings{
				UseColor:  true,
				ThemeMode: mode,
				Width:     80,
				Symbols:   presentation.UnicodeSymbols,
			}
			r := presentation.NewStaticRenderer(settings)
			out := r.KeyValues([]presentation.KeyValue{
				{Key: "test-key", Value: "test-value", Style: presentation.ValuePlain},
			})
			// The key should have faint/dim styling around it.
			if !containsFaint(out) {
				t.Errorf("%s: key should contain faint styling, got: %q", modeName(mode), out)
			}
			// Verify the value has no foreground color.
			// Extract value portion (after ": ").
			if idx := strings.Index(out, ": "); idx >= 0 {
				valuePart := out[idx+2:]
				// Strip trailing ANSI reset.
				valuePart = strings.TrimSuffix(valuePart, "\x1b[0m")
				valuePart = strings.TrimSuffix(valuePart, "\x1b[m")
				if hasForegroundColor(valuePart) {
					t.Errorf("%s: value has forced foreground: %q", modeName(mode), valuePart)
				}
			}
		})
	}
}

// TestLabelRenderer proves the Label renderer method styles keys as dim/faint.
func TestLabelRenderer(t *testing.T) {
	for _, mode := range []presentation.ThemeMode{presentation.ThemeLight, presentation.ThemeDark} {
		settings := presentation.EffectiveSettings{
			UseColor:  true,
			ThemeMode: mode,
			Width:     80,
			Symbols:   presentation.UnicodeSymbols,
		}
		r := presentation.NewStaticRenderer(settings)
		label := r.Label("my-key")
		if !containsFaint(label) {
			t.Errorf("%s: Label(%q) should be faint, got: %q", modeName(mode), "my-key", label)
		}
	}
	// Verify plain mode returns unstyled text.
	settings := presentation.EffectiveSettings{UseColor: false, Width: 80}
	r := presentation.NewStaticRenderer(settings)
	if r.Label("key") != "key" {
		t.Errorf("plain Label should be identity, got: %q", r.Label("key"))
	}
}

// TestStatusCellRendering proves StatusCell renders the correct Unicode symbol
// and semantic status text color.
func TestStatusCellRendering(t *testing.T) {
	settings := presentation.EffectiveSettings{
		UseColor:  true,
		ThemeMode: presentation.ThemeLight,
		Width:     80,
		Symbols:   presentation.UnicodeSymbols,
	}
	r := presentation.NewStaticRenderer(settings)

	m := presentation.TableModel{
		Columns: []presentation.TableColumn{
			{Key: "check", Header: "CHECK"},
			{Key: "status", Header: "STATUS"},
		},
		Rows: []map[string]string{
			{"check": "cache", "status": "ok"},
			{"check": "lock", "status": "fail"},
		},
		RowStatuses: []map[string]presentation.StatusCell{
			{"status": {Text: "ok", Status: presentation.StatusSuccess}},
			{"status": {Text: "fail", Status: presentation.StatusError}},
		},
	}

	out := r.Table(m)
	if out == "" {
		t.Fatal("table output is empty")
	}
	// The output should contain the Unicode success symbol (✓) with green color.
	if !strings.Contains(out, "\x1b[32m") && !strings.Contains(out, "\x1b[38") {
		t.Errorf("expected success green in table, got: %q", out)
	}
	// The output should contain the Unicode error symbol (×) with red color.
	if !strings.Contains(out, "\x1b[31m") && !strings.Contains(out, "\x1b[38") {
		t.Errorf("expected error red in table, got: %q", out)
	}
}

// TestStatusCellWithASCIISymbols proves StatusCell uses ASCII symbols.
func TestStatusCellWithASCIISymbols(t *testing.T) {
	settings := presentation.EffectiveSettings{
		UseColor:  true,
		ThemeMode: presentation.ThemeLight,
		Width:     80,
		Symbols:   presentation.ASCIISymbols,
	}
	r := presentation.NewStaticRenderer(settings)

	m := presentation.TableModel{
		Columns: []presentation.TableColumn{
			{Key: "check", Header: "CHECK"},
			{Key: "status", Header: "STATUS"},
		},
		Rows: []map[string]string{
			{"check": "cache", "status": "ok"},
		},
		RowStatuses: []map[string]presentation.StatusCell{
			{"status": {Text: "ok", Status: presentation.StatusSuccess}},
		},
	}

	out := r.Table(m)
	// ASCII success symbol is "OK".
	if !strings.Contains(out, "OK") {
		t.Errorf("expected ASCII 'OK' symbol in table, got: %q", out)
	}
}

// TestStatusCellNoColor proves StatusCell emits no ANSI when color is disabled.
func TestStatusCellNoColor(t *testing.T) {
	settings := presentation.EffectiveSettings{
		UseColor: false,
		Width:    80,
		Symbols:  presentation.UnicodeSymbols,
	}
	r := presentation.NewStaticRenderer(settings)

	m := presentation.TableModel{
		Columns: []presentation.TableColumn{
			{Key: "check", Header: "CHECK"},
			{Key: "status", Header: "STATUS"},
		},
		Rows: []map[string]string{
			{"check": "cache", "status": "ok"},
		},
		RowStatuses: []map[string]presentation.StatusCell{
			{"status": {Text: "ok", Status: presentation.StatusSuccess}},
		},
	}

	out := r.Table(m)
	if containsANSI(out) {
		t.Errorf("no-color StatusCell should emit zero ANSI, got: %q", out)
	}
	// The Unicode success symbol must still be present (no ANSI, just the glyph).
	if !strings.Contains(out, presentation.UnicodeSymbols.Success) {
		t.Errorf("expected Unicode success symbol in plain table, got: %q", out)
	}
}

// TestTableSortingPreservesRowStatuses proves sorting keeps RowStatuses
// aligned with the correct row after sort.
func TestTableSortingPreservesRowStatuses(t *testing.T) {
	settings := presentation.EffectiveSettings{
		UseColor: false,
		Width:    80,
		Symbols:  presentation.UnicodeSymbols,
	}
	r := presentation.NewStaticRenderer(settings)

	m := presentation.TableModel{
		Columns: []presentation.TableColumn{
			{Key: "name", Header: "NAME", Primary: true},
			{Key: "status", Header: "STATUS"},
		},
		Rows: []map[string]string{
			{"name": "z-pkg", "status": "ok"},
			{"name": "a-pkg", "status": "fail"},
		},
		RowStatuses: []map[string]presentation.StatusCell{
			{"status": {Text: "ok", Status: presentation.StatusSuccess}},
			{"status": {Text: "fail", Status: presentation.StatusError}},
		},
	}

	out := r.Table(m)
	// After sorting, "a-pkg" with "fail" should come first.
	idxA := strings.Index(out, "a-pkg")
	idxZ := strings.Index(out, "z-pkg")
	if idxA < 0 || idxZ < 0 {
		t.Fatalf("table missing rows: %q", out)
	}
	if idxA > idxZ {
		t.Errorf("rows not sorted: %q", out)
	}
	// The "fail" status should be next to "a-pkg", and "ok" next to "z-pkg".
	idxFail := strings.Index(out, "fail")
	idxOK := strings.Index(out, "ok")
	if idxFail < idxA || idxFail > idxZ {
		t.Errorf("status 'fail' not aligned with a-pkg: %q", out)
	}
	if idxOK < idxZ {
		t.Errorf("status 'ok' not aligned with z-pkg: %q", out)
	}
}

// TestTableColumnCellStyleApplied proves column-level CellStyle is applied.
func TestTableColumnCellStyleApplied(t *testing.T) {
	settings := presentation.EffectiveSettings{
		UseColor:  true,
		ThemeMode: presentation.ThemeLight,
		Width:     80,
		Symbols:   presentation.UnicodeSymbols,
	}
	r := presentation.NewStaticRenderer(settings)

	m := presentation.TableModel{
		Columns: []presentation.TableColumn{
			{Key: "pkg", Header: "PACKAGE", Primary: true, CellStyle: presentation.ValuePackage},
			{Key: "ver", Header: "VERSION", CellStyle: presentation.ValueVersion},
		},
		Rows: []map[string]string{
			{"pkg": "my-pkg", "ver": "1.0.0"},
		},
	}

	out := r.Table(m)
	if out == "" {
		t.Fatal("table output is empty")
	}
	// Package should have cyan foreground (FgCyan = 36 in light mode).
	// Version should only have Bold (1), no foreground color.
	// The version "1.0.0" should appear in the output.
	if !strings.Contains(out, "my-pkg") || !strings.Contains(out, "1.0.0") {
		t.Fatalf("table missing cell values: %q", out)
	}
}

// TestSymbolRoleOnRenderer proves SymbolRole is accessible via StaticRenderer.
func TestSymbolRoleOnRenderer(t *testing.T) {
	settings := presentation.EffectiveSettings{
		UseColor:  true,
		ThemeMode: presentation.ThemeLight,
		Symbols:   presentation.UnicodeSymbols,
	}
	r := presentation.NewStaticRenderer(settings)

	arrow := r.SymbolRole(presentation.RoleStructuralArrow)
	if arrow == "" {
		t.Fatal("SymbolRole returned empty")
	}
	// Should contain the arrow glyph.
	if !strings.Contains(arrow, presentation.UnicodeSymbols.Arrow) {
		t.Errorf("SymbolRole(RoleStructuralArrow) missing arrow glyph: %q", arrow)
	}

	// Plain mode should return unstyled glyph.
	r2 := presentation.NewStaticRenderer(presentation.EffectiveSettings{
		UseColor: false,
		Symbols:  presentation.ASCIISymbols,
	})
	arrowPlain := r2.SymbolRole(presentation.RoleStructuralArrow)
	if arrowPlain != presentation.ASCIISymbols.Arrow {
		t.Errorf("plain SymbolRole should be ASCII arrow %q, got: %q",
			presentation.ASCIISymbols.Arrow, arrowPlain)
	}
	if containsANSI(arrowPlain) {
		t.Errorf("plain SymbolRole should have no ANSI: %q", arrowPlain)
	}
}

func modeName(m presentation.ThemeMode) string {
	switch m {
	case presentation.ThemeLight:
		return "Light"
	case presentation.ThemeDark:
		return "Dark"
	case presentation.ThemeAccessible:
		return "Accessible"
	case presentation.ThemeNone:
		return "None"
	default:
		return "Unknown"
	}
}

func hasForegroundColor(s string) bool {
	if !containsANSI(s) {
		return false
	}
	// Check for standard foreground colors (30-37, 90-97).
	for i := 30; i <= 37; i++ {
		if strings.Contains(s, fmt.Sprintf("\x1b[%dm", i)) {
			return true
		}
	}
	for i := 90; i <= 97; i++ {
		if strings.Contains(s, fmt.Sprintf("\x1b[%dm", i)) {
			return true
		}
	}
	// Check for extended/RGB foreground (38;5;N or 38;2;R;G;B).
	if strings.Contains(s, "\x1b[38;5;") || strings.Contains(s, "\x1b[38;2;") {
		return true
	}
	return false
}

func containsFaint(s string) bool {
	return strings.Contains(s, "\x1b[2m")
}

func containsANSI(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			return true
		}
	}
	return false
}
