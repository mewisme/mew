package presentation_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoHardcodedPresentationGlyphs scans production Go files outside the
// presentation package for raw terminal glyph literals that should come from
// the centralized Symbols inventory.
func TestNoHardcodedPresentationGlyphs(t *testing.T) {
	glyphs := map[string]string{
		// Unicode presentation glyphs whose presence in feature code is suspicious.
		`"✓"`: "Unicode check mark (use Symbols.Success via RenderSemanticSymbol)",
		`"×"`: "Unicode cross mark (use Symbols.Error via RenderSemanticSymbol)",
		`"→"`: "Unicode arrow (use Symbols.Arrow via RenderSymbolRole)",
		`"—"`: "Unicode em dash (use Symbols.Placeholder or Symbols.Separator)",
		`"…"`: "Unicode ellipsis (use Symbols.Ellipsis via RenderSymbolRole)",
		`"○"`: "Unicode pending circle (use Symbols.Pending via RenderSemanticSymbol)",
		`"●"`: "Unicode running circle (use Symbols.Running via RenderSemanticSymbol)",
		`"–"`: "Unicode en dash (use Symbols.Skipped via RenderSemanticSymbol)",
	}

	// Raw color construction patterns.
	colorPatterns := []string{
		`"github.com/fatih/color"`,
		`color.New(`,
	}

	root, err := findRepoRoot()
	if err != nil {
		t.Skipf("cannot find repo root: %v", err)
	}

	internalDir := filepath.Join(root, "internal")
	err = filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip testdata, fixtures, and generated directories.
			base := filepath.Base(path)
			if base == "testdata" || base == "fixtures" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Allowlist: files that are allowed to define or use glyphs/colors directly.
		rel, _ := filepath.Rel(root, path)
		if isPresentationFile(rel) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()

			// Skip comments and strings that are part of tests/docs.
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}

			for glyph, explanation := range glyphs {
				if strings.Contains(line, glyph) {
					t.Errorf("%s:%d: hardcoded glyph %s — %s", rel, lineNo, glyph, explanation)
				}
			}
			for _, pat := range colorPatterns {
				if strings.Contains(line, pat) {
					t.Errorf("%s:%d: direct color usage %q — use presentation Theme or RenderSymbolRole instead", rel, lineNo, pat)
				}
			}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
}

// isPresentationFile returns true for files inside the presentation package
// (which are allowed to define glyphs and colors).
func isPresentationFile(rel string) bool {
	return strings.HasPrefix(rel, "internal/presentation/")
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}
