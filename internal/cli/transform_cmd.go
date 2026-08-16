package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/presentation"
	"github.com/mewisme/mew/internal/transform"
)

func newTransformCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transform",
		Short: "Inspect transform capabilities and compiler option support",
	}
	cmd.AddCommand(newTransformReportCmd())
	return cmd
}

func newTransformReportCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Print compiler option support matrix",
		Long: `Print the canonical compiler option capability report.

Lists every compiler option Mew recognizes, its support status
(supported, partial, or unsupported), accepted values, and any
limitations or rejection reasons.

The report is deterministic — it depends only on the compiled binary,
not on project state or network access.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := transform.CapabilityReport()
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			return writeCapabilityTable(cmd, report)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print report as JSON")
	return cmd
}

func writeCapabilityTable(cmd *cobra.Command, report []transform.Entry) error {
	g := ownerFlags(cmd.Root())
	r := g.mustStaticRenderer(cmd)
	model := capabilityTableModel(report)
	return writeStaticOut(cmd, r.Table(model))
}

func capabilityTableModel(entries []transform.Entry) presentation.TableModel {
	cols := []presentation.TableColumn{
		{Key: "option", Header: "OPTION", MinWidth: 8, Prefer: 24, Primary: true, Truncate: presentation.TruncateEnd},
		{Key: "status", Header: "STATUS", MinWidth: 6, Prefer: 14},
		{Key: "category", Header: "CATEGORY", MinWidth: 8, Prefer: 14},
		{Key: "values", Header: "VALUES", MinWidth: 6, Prefer: 24, Truncate: presentation.TruncateEnd},
		{Key: "limitation", Header: "NOTES", MinWidth: 8, Prefer: 48, Truncate: presentation.TruncateEnd},
	}
	rows := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		values := "-"
		if len(e.Values) > 0 {
			values = fmt.Sprintf("%v", e.Values)
		}
		limitation := e.Limitation
		if limitation == "" {
			limitation = "-"
		}
		rows = append(rows, map[string]string{
			"option":     e.Option,
			"status":     string(e.Status),
			"category":   string(e.Category),
			"values":     values,
			"limitation": limitation,
		})
	}
	return presentation.TableModel{Columns: cols, Rows: rows}
}
