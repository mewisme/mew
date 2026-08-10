package cli

import (
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/lifecycle"
	"github.com/mewisme/mew/internal/policy"
	"github.com/mewisme/mew/internal/presentation"
)

// ── Doctor table ──────────────────────────────────────────────────────────

func TestDoctorTableModel_StatusCell(t *testing.T) {
	rep := app.DoctorReport{
		OK: false,
		Checks: []app.DoctorCheck{
			{ID: "cache", Status: "ok", Message: "cache is healthy"},
			{ID: "lock", Status: "fail", Message: "lock file corrupt"},
			{ID: "store", Status: "warn", Message: "store is slow"},
			{ID: "network", Status: "skipped", Message: "offline"},
		},
	}

	m := doctorTableModel(rep)

	// Verify column count.
	if len(m.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(m.Columns))
	}
	// Verify RowStatuses exist and align with rows.
	if len(m.RowStatuses) != len(m.Rows) {
		t.Fatalf("RowStatuses count %d != Rows count %d", len(m.RowStatuses), len(m.Rows))
	}

	// Verify each status maps to correct presentation Status.
	wantStatus := map[string]presentation.Status{
		"ok":      presentation.StatusSuccess,
		"fail":    presentation.StatusError,
		"warn":    presentation.StatusWarning,
		"skipped": presentation.StatusSkipped,
	}
	for i, row := range m.Rows {
		sc, ok := m.RowStatuses[i]["status"]
		if !ok {
			t.Errorf("row %d (%s): missing StatusCell for 'status' column", i, row["check"])
			continue
		}
		if sc.Text != row["status"] {
			t.Errorf("row %d: StatusCell.Text=%q, row status=%q", i, sc.Text, row["status"])
		}
		if got := sc.Status; got != wantStatus[row["status"]] {
			t.Errorf("row %d (%s): Status=%d, want %d for %q", i, row["check"], got, wantStatus[row["status"]], row["status"])
		}
	}
}

func TestDoctorTableModel_RenderRich(t *testing.T) {
	rep := app.DoctorReport{
		OK: true,
		Checks: []app.DoctorCheck{
			{ID: "cache", Status: "ok", Message: "healthy"},
		},
	}
	m := doctorTableModel(rep)
	r := presentation.NewStaticRenderer(presentation.EffectiveSettings{
		UseColor:  true,
		ThemeMode: presentation.ThemeLight,
		Width:     80,
		Symbols:   presentation.UnicodeSymbols,
	})
	out := r.Table(m)
	if out == "" {
		t.Fatal("table output is empty")
	}
	// Should contain the Unicode success symbol.
	if !strings.Contains(out, presentation.UnicodeSymbols.Success) {
		t.Errorf("expected success symbol in rich table: %q", out)
	}
}

func TestDoctorTableModel_RenderPlain(t *testing.T) {
	rep := app.DoctorReport{
		OK: false,
		Checks: []app.DoctorCheck{
			{ID: "cache", Status: "ok", Message: "healthy"},
		},
	}
	m := doctorTableModel(rep)
	r := presentation.NewStaticRenderer(presentation.EffectiveSettings{
		UseColor: false,
		Width:    80,
		Symbols:  presentation.UnicodeSymbols,
	})
	out := r.Table(m)
	if out == "" {
		t.Fatal("table output is empty")
	}
	if containsAnsi(out) {
		t.Errorf("plain doctor table has ANSI: %q", out)
	}
	if !strings.Contains(out, "cache") || !strings.Contains(out, "healthy") {
		t.Errorf("plain doctor table missing content: %q", out)
	}
}

// ── Outdated table ────────────────────────────────────────────────────────

func TestOutdatedTableModel_CellStyle(t *testing.T) {
	entries := []app.OutdatedEntry{
		{Package: "my-pkg", Current: "1.0.0", Wanted: "1.1.0", Latest: "2.0.0", Importer: "."},
	}

	m := outdatedTableModel(entries)
	if len(m.Columns) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(m.Columns))
	}

	// Verify CellStyle assignments.
	wantStyles := map[string]presentation.ValueKind{
		"package":  presentation.ValuePackage,
		"current":  presentation.ValueVersion,
		"wanted":   presentation.ValueVersion,
		"latest":   presentation.ValueVersion,
		"location": presentation.ValuePath,
	}
	for _, col := range m.Columns {
		if got := col.CellStyle; got != wantStyles[col.Key] {
			t.Errorf("column %q: CellStyle=%d, want %d", col.Key, got, wantStyles[col.Key])
		}
	}
}

func TestOutdatedTableModel_Render(t *testing.T) {
	entries := []app.OutdatedEntry{
		{Package: "my-pkg", Current: "1.0.0", Wanted: "1.1.0", Latest: "2.0.0", Importer: "."},
	}
	m := outdatedTableModel(entries)
	r := presentation.NewStaticRenderer(presentation.EffectiveSettings{
		UseColor:  true,
		ThemeMode: presentation.ThemeLight,
		Width:     80,
		Symbols:   presentation.UnicodeSymbols,
	})
	out := r.Table(m)
	if out == "" {
		t.Fatal("table output is empty")
	}
	// All version values should appear.
	for _, want := range []string{"my-pkg", "1.0.0", "1.1.0", "2.0.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in outdated table: %q", want, out)
		}
	}
	// Verify no version has foreground color (only Bold at most).
	outClean := stripBold(out)
	if containsForegroundColor(outClean) {
		t.Errorf("version values should have no forced foreground: %q", out)
	}
}

// ── Workspace table ───────────────────────────────────────────────────────

func TestWorkspaceListTable_CellStyle(t *testing.T) {
	rows := []workspaceListRow{
		{Name: "ws-a", Version: "3.2.1", Path: "/home/user/ws-a"},
	}
	m := workspaceListTable(rows)

	wantStyles := map[string]presentation.ValueKind{
		"name":    presentation.ValuePackage,
		"version": presentation.ValueVersion,
		"path":    presentation.ValuePath,
	}
	for _, col := range m.Columns {
		if got := col.CellStyle; got != wantStyles[col.Key] {
			t.Errorf("column %q: CellStyle=%d, want %d", col.Key, got, wantStyles[col.Key])
		}
	}
}

// ── Policy table ──────────────────────────────────────────────────────────

func TestPolicyTableModel_StatusCell(t *testing.T) {
	result := policy.PolicyResult{
		Passed: false,
		Violations: []policy.PolicyViolation{
			{Package: "bad-pkg", Severity: policy.SeverityError, Message: "blocked license"},
			{Package: "warn-pkg", Severity: policy.SeverityWarn, Message: "unapproved"},
		},
	}

	m := policyTableModel(result)
	if len(m.RowStatuses) != len(m.Rows) {
		t.Fatalf("RowStatuses count %d != Rows count %d", len(m.RowStatuses), len(m.Rows))
	}

	wantStatus := map[string]presentation.Status{
		"error": presentation.StatusError,
		"warn":  presentation.StatusWarning,
	}
	for i, row := range m.Rows {
		sc, ok := m.RowStatuses[i]["severity"]
		if !ok {
			t.Errorf("row %d: missing StatusCell for 'severity' column", i)
			continue
		}
		if got := sc.Status; got != wantStatus[row["severity"]] {
			t.Errorf("row %d (%s): Status=%d, want %d for %q", i, row["package"], got, wantStatus[row["severity"]], row["severity"])
		}
	}

	// Verify package column has ValuePackage style.
	for _, col := range m.Columns {
		if col.Key == "package" && col.CellStyle != presentation.ValuePackage {
			t.Errorf("policy PACKAGE column CellStyle=%d, want ValuePackage", col.CellStyle)
		}
	}
}

func TestPolicyTableModel_RenderRich(t *testing.T) {
	result := policy.PolicyResult{
		Passed: false,
		Violations: []policy.PolicyViolation{
			{Package: "bad-pkg", Severity: policy.SeverityError, Message: "denied"},
		},
	}
	m := policyTableModel(result)
	r := presentation.NewStaticRenderer(presentation.EffectiveSettings{
		UseColor:  true,
		ThemeMode: presentation.ThemeLight,
		Width:     80,
		Symbols:   presentation.UnicodeSymbols,
	})
	out := r.Table(m)
	// Error severity should show the error symbol.
	if !strings.Contains(out, presentation.UnicodeSymbols.Error) {
		t.Errorf("expected error symbol in policy table: %q", out)
	}
}

// ── Builds table ──────────────────────────────────────────────────────────

func TestBuildsTableModel_CellStyle(t *testing.T) {
	entries := []lifecycle.AuditEntry{
		{Package: "my-pkg", Script: "build", ExitCode: 0, DurationMs: 1234, TS: "2024-01-01T00:00:00Z"},
	}

	m := buildsTableModel(entries)
	wantStyles := map[string]presentation.ValueKind{
		"time":     presentation.ValueMuted,
		"package":  presentation.ValuePackage,
		"script":   presentation.ValueCommand,
		"exit":     presentation.ValueNumber,
		"duration": presentation.ValueNumber,
	}
	for _, col := range m.Columns {
		if got := col.CellStyle; got != wantStyles[col.Key] {
			t.Errorf("column %q: CellStyle=%d, want %d", col.Key, got, wantStyles[col.Key])
		}
	}
}

func TestBuildsTableModel_Render(t *testing.T) {
	entries := []lifecycle.AuditEntry{
		{Package: "my-pkg", Script: "build", ExitCode: 0, DurationMs: 42, TS: "2024-01-01T00:00:00Z"},
	}
	m := buildsTableModel(entries)
	r := presentation.NewStaticRenderer(presentation.EffectiveSettings{
		UseColor:  true,
		ThemeMode: presentation.ThemeLight,
		Width:     80,
		Symbols:   presentation.UnicodeSymbols,
	})
	out := r.Table(m)
	for _, want := range []string{"my-pkg", "build", "0", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in builds table: %q", want, out)
		}
	}
}

func TestBuildsTableModel_RenderPlain(t *testing.T) {
	entries := []lifecycle.AuditEntry{
		{Package: "my-pkg", Script: "build", ExitCode: 1, DurationMs: 999, TS: ""},
	}
	m := buildsTableModel(entries)
	r := presentation.NewStaticRenderer(presentation.EffectiveSettings{
		UseColor: false,
		Width:    80,
		Symbols:  presentation.UnicodeSymbols,
	})
	out := r.Table(m)
	if containsAnsi(out) {
		t.Errorf("plain builds table has ANSI: %q", out)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────

func containsAnsi(s string) bool {
	return strings.Contains(s, "\x1b[")
}

func containsForegroundColor(s string) bool {
	for i := 30; i <= 37; i++ {
		if strings.Contains(s, string([]byte{0x1b, '[', byte('0'+i/10), byte('0'+i%10), 'm'})) {
			return true
		}
	}
	if strings.Contains(s, "\x1b[38;5;") || strings.Contains(s, "\x1b[38;2;") {
		return true
	}
	return false
}

func stripBold(s string) string {
	s = strings.ReplaceAll(s, "\x1b[1m", "")
	s = strings.ReplaceAll(s, "\x1b[22m", "")
	return s
}
