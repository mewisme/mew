package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/conformance"
)

func newConformanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conformance",
		Short: "Run certification conformance suites",
		Long:  "List and execute certification matrices (core, cli-ux, runner).",
	}
	cmd.AddCommand(newConformanceListCmd())
	cmd.AddCommand(newConformanceRunCmd())
	cmd.AddCommand(newConformanceVerifyCmd())
	return cmd
}

func newConformanceListCmd() *cobra.Command {
	var (
		filter string
		matrix string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List certification suites",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := findModuleRoot()
			if err != nil {
				return apperr.Wrap(apperr.Internal, "conformance list", "", err)
			}
			var suites []conformance.Suite
			switch matrix {
			case "", "core":
				suites, err = conformance.ListCore(repoRoot, filter)
			case "cli-ux":
				suites, err = conformance.ListCLIUX(repoRoot, filter)
			case "runtime":
				suites, err = conformance.ListRuntime(repoRoot, filter)
			default:
				return apperr.New(apperr.Usage, "conformance list", matrix, "matrix must be core, cli-ux, or runtime")
			}
			if err != nil {
				return apperr.Wrap(apperr.Usage, "conformance list", "", err)
			}
			for _, s := range suites {
				req := "optional"
				if s.Required {
					req = "required"
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", s.ID, req, s.Package, s.Run)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "suite id prefix filter")
	cmd.Flags().StringVar(&matrix, "matrix", "core", "matrix to list: core|cli-ux")
	return cmd
}

func newConformanceRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a certification matrix",
	}
	cmd.AddCommand(newConformanceRunGoTestMatrixCmd(
		"core",
		"Run the PM core certification matrix",
		conformance.RunCore,
	))
	cmd.AddCommand(newConformanceRunGoTestMatrixCmd(
		"cli-ux",
		"Run the CLI UX certification matrix",
		conformance.RunCLIUX,
	))
	cmd.AddCommand(newConformanceRunGoTestMatrixCmd(
		"runtime",
		"Run the runtime stabilization certification matrix",
		conformance.RunRuntime,
	))
	cmd.AddCommand(newConformanceRunRunnerCmd())
	return cmd
}

type goTestMatrixRunner func(ctx context.Context, opts conformance.RunOptions) (conformance.Report, error)

func newConformanceRunGoTestMatrixCmd(name, short string, run goTestMatrixRunner) *cobra.Command {
	var (
		asJSON bool
		filter string
	)
	op := "conformance run " + name
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := findModuleRoot()
			if err != nil {
				return apperr.Wrap(apperr.Internal, op, "", err)
			}
			report, err := run(cmd.Context(), conformance.RunOptions{
				RepoRoot: repoRoot,
				Filter:   filter,
			})
			if asJSON {
				data, encErr := report.EncodeJSON()
				if encErr != nil {
					return apperr.Wrap(apperr.Internal, op, "", encErr)
				}
				if _, writeErr := cmd.OutOrStdout().Write(data); writeErr != nil {
					return writeErr
				}
				_, writeErr := cmd.OutOrStdout().Write([]byte("\n"))
				if writeErr != nil {
					return writeErr
				}
			} else {
				for _, s := range report.Suites {
					line := fmt.Sprintf("%s\t%s", s.ID, s.Status)
					if s.Error != "" {
						line += "\t" + s.Error
					}
					if _, writeErr := fmt.Fprintln(cmd.OutOrStdout(), line); writeErr != nil {
						return writeErr
					}
				}
				label := name + " certification"
				if !report.Passed {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: failed (%d suites)\n", label, len(report.Suites))
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: passed (%d suites)\n", label, len(report.Suites))
				}
			}
			if err != nil {
				return apperr.Wrap(apperr.Internal, op, "", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit certification report as JSON")
	cmd.Flags().StringVar(&filter, "filter", "", "run only suites matching id prefix")
	return cmd
}

func newConformanceVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify aggregated certification reports",
	}
	cmd.AddCommand(newConformanceVerifyRunnerCmd())
	return cmd
}
