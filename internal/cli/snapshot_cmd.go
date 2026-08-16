package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/presentation"
	"github.com/mewisme/mew/internal/snapshot"
)

func newSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage install snapshots",
	}
	cmd.AddCommand(newSnapshotListCmd())
	cmd.AddCommand(newSnapshotRestoreCmd())
	return cmd
}

func newSnapshotListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List install snapshots",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "snapshot.list", "", "missing app context")
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
				return enc.Encode(list)
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
	cmd.Flags().BoolVar(&asJSON, "json", false, "print snapshots as JSON")
	return cmd
}

func newSnapshotRestoreCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "snapshot.restore", "", "missing app context")
			}
			result, err := app.RestoreSnapshot(cmd.Context(), ac, args[0])
			outErr := writeInstallResult(cmd, result, asJSON, false)
			if err != nil {
				if outErr != nil {
					return outErr
				}
				return err
			}
			return outErr
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print result as JSON")
	return cmd
}

func newRecoverCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover from an interrupted install transaction",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "recover", "", "missing app context")
			}
			result, err := app.Recover(cmd.Context(), ac)
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
			return writeStaticOut(cmd, r.Status(presentation.StatusLine{
				Status: presentation.StatusSuccess,
				Text:   "recover: " + result.Action,
			}))
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print result as JSON")
	return cmd
}

func newRollbackCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Restore the previous install snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "rollback", "", "missing app context")
			}
			result, err := app.Rollback(cmd.Context(), ac)
			outErr := writeInstallResult(cmd, result, asJSON, false)
			if err != nil {
				if outErr != nil {
					return outErr
				}
				return err
			}
			return outErr
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print result as JSON")
	return cmd
}
