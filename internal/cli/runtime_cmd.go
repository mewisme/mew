package cli

import (
	"github.com/spf13/cobra"
)

func newRuntimeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Inspect and trace runtime execution",
		Long:  "Commands for runtime introspection, tracing, and diagnostics.",
	}
	cmd.AddCommand(newRuntimeTraceCmd())
	cmd.AddCommand(newRuntimeSupportBundleCmd())
	return cmd
}
