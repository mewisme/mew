package cli

import (
	"github.com/spf13/cobra"
)

// newXCmd returns the "m x" command — an alias for mx package execution.
func newXCmd(info BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:                "x",
		Aliases:            []string{"mx"},
		Short:              "Execute a package binary",
		Long:               "Run a package binary without installing it. Alias for mx.",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// DisableFlagParsing prevents cobra from intercepting --help/-h.
			for _, a := range args {
				if a == "--help" || a == "-h" {
					return cmd.Help()
				}
			}
			root := cmd.Root()
			g := ownerFlags(root)
			_, handled, err := runMXDispatch(cmd.Context(), root, g, info, args)
			if !handled {
				return cmd.Help()
			}
			if err != nil {
				// Error already reported by runMXDispatch.
				return suppressReport(err)
			}
			return nil
		},
	}
	// Skip inherited PersistentPreRunE — runMXDispatch handles bootstrap
	// including --cwd reload.
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return nil
	}
	return cmd
}
