package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/presentation"
	"github.com/mewisme/mew/internal/transform"
)

func newCacheExplainCmd() *cobra.Command {
	var (
		asJSON bool
		key    string
	)
	cmd := &cobra.Command{
		Use:   "explain [--key <cache-key>]",
		Short: "Show transform cache status and explain entries",
		Long: `Displays the transform cache overview: directory, entry count, disk usage,
schema version, and orphan files.

With --key, explains a specific cache entry: metadata validation, digest
checks, and the final hit/miss/corrupt disposition. Uses the same canonical
cache key, schema, and digest code as production cache reads and writes.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "cache.explain", "", "missing app context")
			}
			cacheDir := transform.TransformCacheDir(ac.Config)
			result, err := transform.CacheExplain(cacheDir, transform.CacheExplainOptions{
				Key: key,
			})
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			g := ownerFlags(cmd.Root())
			r := g.mustStaticRenderer(cmd)
			if err := writeStaticOut(cmd, r.Summary(cacheExplainSummary(result))); err != nil {
				return err
			}
			if table := r.Table(cacheExplainTable(result)); table != "" {
				if err := writeStaticOut(cmd, table); err != nil {
					return err
				}
			}
			// Render per-entry explanation.
			if result.Entry != nil {
				if err := writeStaticOut(cmd, r.Summary(cacheEntrySummary(result.Entry))); err != nil {
					return err
				}
				if table := r.Table(cacheEntryTable(result.Entry)); table != "" {
					if err := writeStaticOut(cmd, table); err != nil {
						return err
					}
				}
			}
			// Render orphan warnings.
			if len(result.Orphans) > 0 {
				if table := r.Table(orphansTable(result.Orphans)); table != "" {
					if err := writeStaticOut(cmd, table); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print cache info as JSON")
	cmd.Flags().StringVar(&key, "key", "", "explain a specific cache entry by key")
	return cmd
}

func cacheExplainSummary(result *transform.CacheExplainResult) presentation.Summary {
	if result == nil {
		return presentation.Summary{Status: presentation.StatusError, Title: "Failed to gather cache info"}
	}
	st := presentation.StatusSuccess
	title := "Transform cache active"
	if result.EntryCount == 0 {
		title = "Transform cache empty"
	}
	return presentation.Summary{
		Status: st,
		Title:  title,
		Metrics: []presentation.KeyValue{
			{Key: "directory", Value: result.CacheDir, Style: presentation.ValuePath},
			{Key: "entries", Value: fmt.Sprintf("%d", result.EntryCount), Style: presentation.ValueNumber},
			{Key: "total size", Value: formatBytes(result.TotalBytes), Style: presentation.ValueNumber},
		},
	}
}

func cacheExplainTable(result *transform.CacheExplainResult) presentation.TableModel {
	if result == nil {
		return presentation.TableModel{}
	}
	cols := []presentation.TableColumn{
		{Key: "metric", Header: "METRIC", MinWidth: 8, Prefer: 16, Primary: true},
		{Key: "value", Header: "VALUE", MinWidth: 8, Prefer: 40},
	}
	rows := []map[string]string{
		{"metric": "Cache directory", "value": result.CacheDir},
		{"metric": "Schema version", "value": fmt.Sprintf("v%d", result.SchemaVer)},
		{"metric": "Entries", "value": fmt.Sprintf("%d", result.EntryCount)},
		{"metric": "Code bytes", "value": formatBytes(result.CodeBytes)},
		{"metric": "Map bytes", "value": formatBytes(result.MapBytes)},
		{"metric": "Meta bytes", "value": formatBytes(result.MetaBytes)},
		{"metric": "Total", "value": formatBytes(result.TotalBytes)},
	}
	if len(result.Orphans) > 0 {
		rows = append(rows, map[string]string{
			"metric": "Orphans", "value": fmt.Sprintf("%d", len(result.Orphans)),
		})
	}
	if len(result.Errors) > 0 {
		rows = append(rows, map[string]string{
			"metric": "Errors", "value": fmt.Sprintf("%d", len(result.Errors)),
		})
	}
	return presentation.TableModel{Columns: cols, Rows: rows}
}

func cacheEntrySummary(entry *transform.CacheEntryExplain) presentation.Summary {
	st := presentation.StatusSuccess
	title := fmt.Sprintf("Cache entry %s: %s", entry.Key[:min(12, len(entry.Key))], entry.Disposition)
	if entry.Disposition == transform.CacheDispositionCorrupt || entry.Disposition == transform.CacheDispositionUnreadable {
		st = presentation.StatusError
		title = fmt.Sprintf("Cache entry %s: %s", entry.Key[:min(12, len(entry.Key))], entry.Disposition)
	} else if entry.Disposition == transform.CacheDispositionSchemaStale || entry.Disposition == transform.CacheDispositionOrphan {
		st = presentation.StatusError
	}
	return presentation.Summary{
		Status: st,
		Title:  title,
	}
}

func cacheEntryTable(entry *transform.CacheEntryExplain) presentation.TableModel {
	cols := []presentation.TableColumn{
		{Key: "metric", Header: "METRIC", MinWidth: 8, Prefer: 16, Primary: true},
		{Key: "value", Header: "VALUE", MinWidth: 8, Prefer: 50},
	}
	rows := []map[string]string{
		{"metric": "Key", "value": entry.Key},
		{"metric": "Disposition", "value": string(entry.Disposition)},
	}
	if entry.SchemaVer > 0 {
		rows = append(rows, map[string]string{"metric": "Schema", "value": fmt.Sprintf("v%d", entry.SchemaVer)})
	}
	if entry.CodeSize > 0 {
		rows = append(rows, map[string]string{"metric": "Code", "value": formatBytes(entry.CodeSize)})
	}
	if entry.MapSize > 0 {
		rows = append(rows, map[string]string{"metric": "Map", "value": formatBytes(entry.MapSize)})
	}
	if entry.MetaSize > 0 {
		rows = append(rows, map[string]string{"metric": "Meta", "value": formatBytes(entry.MetaSize)})
	}

	// Sort reasons: errors first, then warnings, then info.
	sorted := make([]transform.CacheReason, len(entry.Reasons))
	copy(sorted, entry.Reasons)
	sort.Slice(sorted, func(i, j int) bool {
		sev := map[string]int{"error": 0, "warn": 1, "info": 2}
		return sev[sorted[i].Severity] < sev[sorted[j].Severity]
	})

	for _, reason := range sorted {
		rows = append(rows, map[string]string{
			"metric": fmt.Sprintf("[%s] %s", reason.Severity, reason.Code),
			"value":  reason.Message,
		})
	}
	return presentation.TableModel{Columns: cols, Rows: rows}
}

func orphansTable(orphans []transform.CacheReason) presentation.TableModel {
	cols := []presentation.TableColumn{
		{Key: "file", Header: "ORPHAN", MinWidth: 8, Prefer: 50, Primary: true},
		{Key: "severity", Header: "SEVERITY", MinWidth: 4, Prefer: 8},
	}
	rows := make([]map[string]string, 0, len(orphans))
	for _, o := range orphans {
		rows = append(rows, map[string]string{
			"file":     o.Message,
			"severity": o.Severity,
		})
	}
	return presentation.TableModel{Columns: cols, Rows: rows}
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
