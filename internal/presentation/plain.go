package presentation

import (
	"fmt"
	"strings"

	fc "github.com/fatih/color"
)

type plainRenderer struct {
	settings EffectiveSettings
}

func newPlainRenderer(settings EffectiveSettings) *plainRenderer {
	return &plainRenderer{settings: settings}
}

func (r *plainRenderer) Settings() EffectiveSettings { return r.settings }

func (r *plainRenderer) Status(line StatusLine) string {
	sym := statusSymbol(r.settings.Symbols, line.Status)
	var b strings.Builder
	if sym != "" {
		b.WriteString(sym)
		b.WriteByte(' ')
	}
	b.WriteString(line.Text)
	if line.Detail != "" {
		b.WriteByte(' ')
		b.WriteString(line.Detail)
	}
	return b.String()
}

func (r *plainRenderer) KeyValues(rows []KeyValue) string {
	return formatKeyValues(rows, r.settings, false, Theme{})
}

func (r *plainRenderer) Notice(n Notice) string {
	sym := statusSymbol(r.settings.Symbols, n.Status)
	if n.Status == StatusNone {
		sym = r.settings.Symbols.Warning
	}
	if sym == "" {
		return n.Message
	}
	return sym + " " + n.Message
}

func (r *plainRenderer) Hint(h Hint) string {
	arrow := r.settings.Symbols.Arrow
	if arrow == "" {
		return h.Message
	}
	return arrow + " " + h.Message
}

func (r *plainRenderer) Summary(s Summary) string {
	var noticesAndHints []string
	for _, n := range s.Notices {
		noticesAndHints = append(noticesAndHints, r.Notice(n))
	}
	for _, h := range s.Hints {
		noticesAndHints = append(noticesAndHints, r.Hint(h))
	}
	footer := renderCompletionFooterPlain(s.CompletionFooter, r.settings)
	return joinSummarySections(
		r.Status(StatusLine{Status: s.Status, Text: s.Title}),
		r.PackageDeltas(s.Deltas),
		r.KeyValues(s.Metrics),
		strings.Join(noticesAndHints, "\n"),
		footer,
	)
}

func renderCompletionFooterPlain(f *CompletionFooter, settings EffectiveSettings) string {
	if f == nil {
		return ""
	}
	sym := settings.Symbols.Success
	out := sym + " " + f.Message
	if f.Duration != "" {
		out += " " + f.Duration
	}
	return out
}

func (r *plainRenderer) PackageDeltas(deltas []PackageDelta) string {
	return formatPackageDeltas(deltas, r.settings, false, Theme{})
}

func (r *plainRenderer) Table(m TableModel) string {
	return formatTable(m, r.settings, false, Theme{})
}

func (r *plainRenderer) Error(view ErrorView) string {
	return formatError(view, r.settings, false, Theme{})
}

func (r *plainRenderer) PlainText(s string) string { return s }

func statusSymbol(s Symbols, st Status) string {
	switch st {
	case StatusSuccess:
		return s.Success
	case StatusWarning:
		return s.Warning
	case StatusError:
		return s.Error
	case StatusInfo:
		return s.Info
	default:
		return ""
	}
}

func formatKeyValues(rows []KeyValue, settings EffectiveSettings, color bool, theme Theme) string {
	if len(rows) == 0 {
		return ""
	}
	stacked := settings.Width < 60 || settings.Accessible
	if stacked {
		var b strings.Builder
		for i, row := range rows {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(applyStyle(theme.Label, row.Key, color))
			b.WriteString(": ")
			b.WriteString(styleValue(row.Value, row.Style, color, theme))
		}
		return b.String()
	}
	maxKey := 0
	for _, row := range rows {
		if w := CellWidth(row.Key); w > maxKey {
			maxKey = w
		}
	}
	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("  ")
		b.WriteString(applyStyle(theme.Label, row.Key, color))
		pad := maxKey - CellWidth(row.Key)
		if pad < 0 {
			pad = 0
		}
		b.WriteString(strings.Repeat(" ", pad+2))
		b.WriteString(styleValue(row.Value, row.Style, color, theme))
	}
	return b.String()
}

func styleValue(val string, kind ValueKind, color bool, theme Theme) string {
	if !color {
		return val
	}
	switch kind {
	case ValuePackage:
		return applyStyle(theme.Package, val, true)
	case ValueVersion:
		return applyStyle(theme.Version, val, true)
	case ValuePath:
		return applyStyle(theme.Path, val, true)
	case ValueCommand:
		return applyStyle(theme.Command, val, true)
	case ValueNumber:
		return applyStyle(theme.Number, val, true)
	case ValueMuted:
		return applyStyle(theme.Muted, val, true)
	default:
		return applyStyle(theme.Value, val, true)
	}
}

func formatPackageDeltas(deltas []PackageDelta, settings EffectiveSettings, color bool, theme Theme) string {
	return formatPackageDeltasWithOptions(deltas, settings, color, theme, PackageDeltaOptions{
		GroupByKind: true,
		MaxRows:     maxSummaryPackageDeltas,
	})
}

func formatPackageDeltasWithOptions(deltas []PackageDelta, settings EffectiveSettings, color bool, theme Theme, opts PackageDeltaOptions) string {
	if len(deltas) == 0 {
		return ""
	}

	truncated := false
	omitted := 0
	if opts.MaxRows > 0 && len(deltas) > opts.MaxRows {
		truncated = true
		omitted = len(deltas) - opts.MaxRows
		deltas = deltas[:opts.MaxRows]
	}

	if !opts.GroupByKind {
		body := formatFlatPackageDeltas(deltas, settings, color, theme)
		if truncated {
			return body + "\n" + formatDeltaTruncationNotice(omitted, color, theme, settings)
		}
		return body
	}

	// Group by kind: Added, Updated, Removed.
	groups := [][]PackageDelta{
		nil, // DeltaAdded
		nil, // DeltaUpdated
		nil, // DeltaRemoved
	}
	for _, d := range deltas {
		switch d.Kind {
		case DeltaAdded:
			groups[0] = append(groups[0], d)
		case DeltaUpdated:
			groups[1] = append(groups[1], d)
		case DeltaRemoved:
			groups[2] = append(groups[2], d)
		}
	}

	kindNames := []string{"Added", "Updated", "Removed"}
	kindBoldColors := []*fc.Color{theme.AddedBold, theme.UpdatedBold, theme.RemovedBold}
	var parts []string
	for i, group := range groups {
		if len(group) == 0 {
			continue
		}
		heading := kindNames[i]
		if color {
			heading = applyStyle(kindBoldColors[i], heading, true)
		}
		body := formatFlatPackageDeltas(group, settings, color, theme)
		parts = append(parts, heading+"\n"+body)
	}

	out := strings.Join(parts, "\n\n")
	if truncated {
		out += "\n" + formatDeltaTruncationNotice(omitted, color, theme, settings)
	}
	return out
}

func formatDeltaTruncationNotice(omitted int, color bool, theme Theme, settings EffectiveSettings) string {
	arrow := settings.Symbols.Arrow
	if color {
		arrow = applyStyle(theme.Muted, arrow, true)
	}
	msg := fmt.Sprintf("%s %d additional package changes are not shown.", arrow, omitted)
	msg += "\n  Run `" + settings.BinName() + " plan` for the complete mutation plan."
	if color {
		msg = applyStyle(theme.Muted, msg, true)
	}
	return msg
}

func formatFlatPackageDeltas(deltas []PackageDelta, settings EffectiveSettings, color bool, theme Theme) string {
	if len(deltas) == 0 {
		return ""
	}
	sym := settings.Symbols

	// Compute max package name width for version alignment.
	maxName := 0
	for _, d := range deltas {
		if w := CellWidth(d.Name); w > maxName {
			maxName = w
		}
	}
	// Minimum 2-space gap between name and version.
	const nameVersionGap = 2

	var b strings.Builder
	for i, d := range deltas {
		if i > 0 {
			b.WriteByte('\n')
		}
		var mark string
		switch d.Kind {
		case DeltaAdded:
			mark = sym.Added
			if color {
				mark = applyStyle(theme.Added, mark, true)
			}
		case DeltaRemoved:
			mark = sym.Removed
			if color {
				mark = applyStyle(theme.Removed, mark, true)
			}
		default:
			mark = sym.Updated
			if color {
				mark = applyStyle(theme.Updated, mark, true)
			}
		}
		b.WriteString(mark)
		b.WriteByte(' ')

		// Package name: bright cyan when colored.
		pkgName := d.Name
		if color {
			pkgName = applyStyle(theme.Package, pkgName, true)
		}
		b.WriteString(pkgName)

		// Pad to align versions.
		pad := maxName - CellWidth(d.Name) + nameVersionGap
		if pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}

		// Version: bright white when colored.
		switch d.Kind {
		case DeltaUpdated:
			from, to := d.From, d.To
			if from == "" {
				from = d.Version
			}
			if to == "" {
				to = d.Version
			}
			if color {
				from = applyStyle(theme.Version, from, true)
				to = applyStyle(theme.Version, to, true)
			}
			b.WriteString(from)
			b.WriteByte(' ')
			arrow := sym.Arrow
			if color {
				arrow = applyStyle(theme.Muted, arrow, true)
			}
			b.WriteString(arrow)
			b.WriteByte(' ')
			b.WriteString(to)
		default:
			ver := d.Version
			if color {
				ver = applyStyle(theme.Version, ver, true)
			}
			b.WriteString(ver)
		}
	}
	return b.String()
}

// joinSummarySections joins major summary sections with one blank line, discarding empties.
func joinSummarySections(sections ...string) string {
	var filtered []string
	for _, s := range sections {
		if strings.TrimSpace(s) == "" {
			continue
		}
		filtered = append(filtered, s)
	}
	out := strings.Join(filtered, "\n\n")
	return strings.TrimSpace(out)
}
