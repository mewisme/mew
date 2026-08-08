package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
)

func newPMDoctorCmd() *cobra.Command {
	var (
		asJSON bool
		strict bool
	)
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check project and package-manager health",
		Long:  "Validates project discovery, lockfile, cache/store paths, filesystem link support, transaction journals, and configuration. Node on PATH is a warning only.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "doctor", "", "missing app context")
			}
			report, err := app.Doctor(cmd.Context(), ac, app.DoctorOptions{Strict: strict})
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
				return app.DoctorExitError(report)
			}
			g := ownerFlags(cmd.Root())
			r := g.mustStaticRenderer(cmd)
			if err := writeStaticOut(cmd, r.Summary(doctorSummary(report))); err != nil {
				return err
			}
			if table := r.Table(doctorTableModel(report)); table != "" {
				if err := writeStaticOut(cmd, table); err != nil {
					return err
				}
			}
			return app.DoctorExitError(report)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print doctor report as JSON")
	cmd.Flags().BoolVar(&strict, "strict", false, "treat warnings as failures")
	cmd.AddCommand(newDoctorRuntimeCmd())
	return cmd
}

func newDoctorRuntimeCmd() *cobra.Command {
	var (
		asJSON bool
		strict bool
	)
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Check runtime health: Node, capabilities, transform cache",
		Long:  "Validates Node installation, required capabilities (module-register, import-preload, require-preload), and runtime asset cache integrity.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "doctor", "", "missing app context")
			}
			report, err := app.DoctorRuntime(cmd.Context(), ac, app.DoctorOptions{Strict: strict})
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
				return app.DoctorExitError(report)
			}
			g := ownerFlags(cmd.Root())
			r := g.mustStaticRenderer(cmd)
			if err := writeStaticOut(cmd, r.Summary(doctorSummary(report))); err != nil {
				return err
			}
			if table := r.Table(doctorTableModel(report)); table != "" {
				if err := writeStaticOut(cmd, table); err != nil {
					return err
				}
			}
			return app.DoctorExitError(report)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print doctor report as JSON")
	cmd.Flags().BoolVar(&strict, "strict", false, "treat warnings as failures")
	return cmd
}
