package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/snapshot"
)

func newHistoryCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show install snapshot timeline",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "history", "", "missing app context")
			}
			proj, err := app.OpenProject(cmd.Context(), ac)
			if err != nil {
				return err
			}
			list, err := snapshot.NewStore(proj.Root).List()
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				return enc.Encode(snapshotEntriesWithDelta(list))
			}
			if len(list) == 0 {
				g := ownerFlags(cmd.Root())
				r := g.mustStaticRenderer(cmd)
				return writeStaticOut(cmd, r.Notice(emptyNotice("no snapshots")))
			}
			g := ownerFlags(cmd.Root())
			r := g.mustStaticRenderer(cmd)
			return writeStaticOut(cmd, r.Table(snapshotTableModel(list, r.Settings().Symbols)))
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print timeline as JSON")
	return cmd
}
