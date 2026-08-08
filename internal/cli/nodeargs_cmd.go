package cli

import (
	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/runtime"
)

func newNodeArgsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node-args [-- <node-flags> <entrypoint> [-- app-args]]",
		Short: "Run a JS file with explicit Node/V8 flags",
		Long: `Run a JavaScript file with full control over Node and V8 flags.

The -- separator is required before the Node flags:

  m node-args -- --trace-warnings app.js
  m node-args -- --max-old-space-size=4096 server.mjs -- --port 3000
`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "node-args", "", "no app context")
			}

			v8Args, entrypoint, appArgs, err := runtime.ParseNodeArgs(args)
			if err != nil {
				return err
			}

			resolved, err := runtime.ResolveEntrypoint(ac.CWD, entrypoint)
			if err != nil {
				return err
			}

			req := runtime.LaunchRequest{
				Entrypoint:       resolved,
				AppArgs:          appArgs,
				NodeV8Args:       v8Args,
				WorkingDir:       ac.CWD,
				AugmentationMode: runtime.AugmentDefault,
				Loaders:          nil, // node-args users pass --experimental-loader via -- separator
				Stdio: runtime.LaunchStdio{
					Stdin:  cmd.InOrStdin(),
					Stdout: cmd.OutOrStdout(),
					Stderr: cmd.ErrOrStderr(),
				},
			}

			// Attach transform session when augmentation is active.
			// The loader's resolve hook handles extension substitution for all
			// entrypoints (same as the file-run dispatch path).
			{
				contrib, contribErr := buildTransformContribution(cmd.Context(), ac.CWD, resolved, ac.Config)
				if contribErr != nil {
					return contribErr
				}
				req.Contribution = contrib
			}

			return runtime.PlanAndLaunch(cmd.Context(), req, ac.Config)
		},
	}
	return cmd
}
