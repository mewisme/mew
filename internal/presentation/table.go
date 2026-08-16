package presentation

import (
	"sort"
	"strings"

	"github.com/fatih/color"
)

const stackedTableThreshold = 60

func formatTable(m TableModel, settings EffectiveSettings, color bool, theme Theme) string {
	if len(m.Columns) == 0 {
		return ""
	}
	rows := append([]map[string]string(nil), m.Rows...)
	statuses := append([]map[string]StatusCell(nil), m.RowStatuses...)

	// Sort rows; if RowStatuses present, permute them in tandem.
	if len(statuses) > 0 {
		type indexed struct {
			row    map[string]string
			status map[string]StatusCell
		}
		items := make([]indexed, len(rows))
		for i := range rows {
			st := map[string]StatusCell(nil)
			if i < len(statuses) {
				st = statuses[i]
			}
			items[i] = indexed{row: rows[i], status: st}
		}
		sort.SliceStable(items, func(i, j int) bool {
			a := rowSortKey(items[i].row, m.Columns)
			b := rowSortKey(items[j].row, m.Columns)
			return a < b
		})
		for i := range items {
			rows[i] = items[i].row
			if i < len(statuses) {
				statuses[i] = items[i].status
			}
		}
	} else {
		sort.SliceStable(rows, func(i, j int) bool {
			a := rowSortKey(rows[i], m.Columns)
			b := rowSortKey(rows[j], m.Columns)
			return a < b
		})
	}

	// Build a model copy with sorted rows/statuses for cell resolution.
	sorted := TableModel{Columns: m.Columns, Rows: rows, RowStatuses: statuses}
	if settings.Width < stackedTableThreshold || settings.Accessible {
		return formatStackedTable(sorted, settings, color, theme)
	}
	return formatWideTable(sorted, settings, color, theme)
}

func rowSortKey(row map[string]string, cols []TableColumn) string {
	var parts []string
	for _, c := range cols {
		parts = append(parts, row[c.Key])
	}
	return strings.Join(parts, "\x00")
}

func formatStackedTable(m TableModel, settings EffectiveSettings, color bool, theme Theme) string {
	cols := m.Columns
	rows := m.Rows
	primary := cols[0]
	for _, c := range cols {
		if c.Primary {
			primary = c
			break
		}
	}
	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteByte('\n')
			b.WriteByte('\n')
		}
		title := row[primary.Key]
		b.WriteString(applyStyle(theme.Header, title, color))
		for _, c := range cols {
			if c.Key == primary.Key {
				continue
			}
			b.WriteByte('\n')
			b.WriteString("  ")
			b.WriteString(applyStyle(theme.Label, strings.ToLower(c.Header), color))
			b.WriteString("  ")
			b.WriteString(tableCellValue(row[c.Key], c.CellStyle, i, c.Key, m, color, theme, settings.Symbols))
		}
	}
	return b.String()
}

func formatWideTable(m TableModel, settings EffectiveSettings, color bool, theme Theme) string {
	cols := m.Columns
	rows := m.Rows
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = CellWidth(c.Header)
		if c.MinWidth > widths[i] {
			widths[i] = c.MinWidth
		}
	}
	for _, row := range rows {
		for i, c := range cols {
			w := CellWidth(row[c.Key])
			if w > widths[i] {
				widths[i] = w
			}
		}
	}
	for i, c := range cols {
		if c.Prefer > 0 && widths[i] > c.Prefer {
			widths[i] = c.Prefer
		}
	}
	widths = fitTableWidths(widths, cols, settings.Width)

	var b strings.Builder
	for i, c := range cols {
		if i > 0 {
			b.WriteString("  ")
		}
		h := applyStyle(theme.Header, c.Header, color)
		b.WriteString(padCell(h, c.Header, widths[i], c.Align))
	}
	b.WriteByte('\n')
	for ri, row := range rows {
		if ri > 0 {
			b.WriteByte('\n')
		}
		for i, c := range cols {
			if i > 0 {
				b.WriteString("  ")
			}
			raw := row[c.Key]
			cell := fitCell(raw, widths[i], c.Truncate, settings.Symbols.Ellipsis)
			styled := tableCellValue(cell, c.CellStyle, ri, c.Key, m, color, theme, settings.Symbols)
			b.WriteString(padCell(styled, cell, widths[i], c.Align))
		}
	}
	return b.String()
}

func fitTableWidths(widths []int, cols []TableColumn, termWidth int) []int {
	sep := 2 * (len(widths) - 1)
	if sep < 0 {
		sep = 0
	}
	total := sep
	for _, w := range widths {
		total += w
	}
	if total <= termWidth || termWidth <= 0 {
		return widths
	}
	overflow := total - termWidth
	out := append([]int(nil), widths...)
	for overflow > 0 {
		idx := -1
		best := 0
		for i, w := range out {
			min := cols[i].MinWidth
			if min <= 0 {
				min = 4
			}
			slack := w - min
			if slack > best {
				best = slack
				idx = i
			}
		}
		if idx < 0 || best <= 0 {
			break
		}
		out[idx]--
		overflow--
	}
	return out
}

func fitCell(s string, width int, policy TruncatePolicy, ellipsis string) string {
	if width <= 0 {
		return ""
	}
	if CellWidth(s) <= width {
		return s
	}
	switch policy {
	case TruncateMiddle:
		return MiddleTruncate(s, width, ellipsis)
	case TruncateWrap:
		lines := WrapWords(s, width)
		if len(lines) == 0 {
			return ""
		}
		return lines[0]
	default:
		return MiddleTruncate(s, width, ellipsis)
	}
}

// tableCellValue resolves the styled form of a table cell, checking for
// row-level StatusCell metadata first, falling back to column-level ValueKind.
func tableCellValue(val string, colKind ValueKind, rowIdx int, colKey string, m TableModel, color bool, theme Theme, sym Symbols) string {
	if rowIdx < len(m.RowStatuses) {
		if sc, ok := m.RowStatuses[rowIdx][colKey]; ok {
			statusSym := RenderSemanticSymbol(sym, theme, sc.Status, color)
			statusText := applyStyle(semanticStatusStyle(theme, sc.Status), sc.Text, color)
			return statusSym + " " + statusText
		}
	}
	return styleValue(val, colKind, color, theme)
}

// semanticStatusStyle returns the theme style for a status, so StatusCell
// text receives the same semantic color as its symbol (not generic Value).
func semanticStatusStyle(theme Theme, st Status) *color.Color {
	switch st {
	case StatusSuccess:
		return theme.Success
	case StatusWarning, StatusCancelled:
		return theme.Warning
	case StatusError:
		return theme.Error
	case StatusInfo, StatusRunning:
		return theme.Info
	case StatusPending, StatusSkipped:
		return theme.Muted
	default:
		return theme.Value
	}
}

func padCell(styled, plain string, width int, align ColumnAlign) string {
	pad := width - CellWidth(plain)
	if pad < 0 {
		pad = 0
	}
	if align == AlignRight {
		return strings.Repeat(" ", pad) + styled
	}
	return styled + strings.Repeat(" ", pad)
}
