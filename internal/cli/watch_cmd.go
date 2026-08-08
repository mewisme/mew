package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/runtime"
	"github.com/mewisme/mew/internal/watch"
)

func newWatchCmd() *cobra.Command {
	var (
		clearScreen   bool
		noClearScreen bool
		envFile       []string
		noEnvFile     bool
		mode          string
		debounceMS    int
	)
	cmd := &cobra.Command{
		Use:   "watch <entrypoint>",
		Short: "Watch files and restart on changes",
		Long: `Watch source files, configuration, and environment files for changes,
then restart the application automatically.

Watches imported modules, tsconfig extends chains, package.json files,
and mode-specific .env files. Restarts happen after a short debounce
period (200ms default) to coalesce rapid saves.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "watch", "", "missing app context")
			}

			entrypoint := args[0]
			appArgs := args[1:]
			cwd := ac.CWD

			epAbs := entrypoint
			if !filepath.IsAbs(epAbs) {
				epAbs = filepath.Join(cwd, epAbs)
			}

			// Determine clear-screen policy.
			clear := clearScreen
			if noClearScreen {
				clear = false
			} else if !clearScreen && !noClearScreen {
				if fi, err := os.Stdout.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
					clear = true
				}
			}

			// Build the dependency graph and seed with config deps.
			graph := watch.NewDependencyGraph()
			initialConfigs := collectConfigDeps(cwd, epAbs, mode, envFile, noEnvFile)
			graph.Seed([]string{epAbs}, initialConfigs, nil)

			// depTraceFile holds the path to the per-run dependency trace.
			var depTraceFile string

			// Build restart function that launches Node on each restart.
			restart := func(ctx context.Context) (int, error) {
				currentEnv, envErr := buildWatchEnvOverlay(cwd, envFile, noEnvFile, mode)
				if envErr != nil {
					return 1, envErr
				}

				req := runtime.LaunchRequest{
					Entrypoint:       epAbs,
					AppArgs:          appArgs,
					WorkingDir:       cwd,
					EnvOverlay:       currentEnv,
					AugmentationMode: runtime.AugmentDefault,
					Stdio: runtime.LaunchStdio{
						Stdin:  os.Stdin,
						Stdout: os.Stdout,
						Stderr: os.Stderr,
					},
				}

				// Attach transform session when augmentation is active.
				// The loader's resolve hook handles extension substitution for all
				// entrypoints; the transform service must be available for any .ts
				// files that are resolved via .js→.ts mapping.
				contrib, contribErr := buildTransformContribution(ctx, cwd, epAbs, ac.Config)
				if contribErr != nil {
					fmt.Fprintf(os.Stderr, "watch: transform error: %v\n", contribErr)
					return 1, contribErr
				}
				req.Contribution = contrib

				// Create per-run trace file so the JS loader reports
				// resolved module paths.
				tf, err := os.CreateTemp("", "mew-deps-*.txt")
				if err == nil {
					depTraceFile = tf.Name()
					_ = tf.Close()
					req.Contribution.ExtraEnv = append(req.Contribution.ExtraEnv,
						"MEW_TRANSFORM_DEP_TRACE_FILE="+depTraceFile,
						"MEW_TRANSFORM_DEP_TRACE_ROOT="+cwd,
					)
				}

				if err := runtime.PlanAndLaunch(ctx, req, ac.Config); err != nil {
					code := apperr.ExitCode(err)
					if apperr.CodeOf(err) == apperr.Cancelled {
						return code, nil
					}
					return code, err
				}
				return 0, nil
			}

			w, err := watch.NewWatcher()
			if err != nil {
				return apperr.Wrap(apperr.IO, "watch", entrypoint, err)
			}
			defer func() { _ = w.Close() }()

			debounce := watch.DefaultDebounceInterval
			if debounceMS > 0 {
				debounce = time.Duration(debounceMS) * time.Millisecond
			}

			sup := watch.NewSupervisor(watch.SupervisorOptions{
				Watcher:          w,
				Restart:          restart,
				ClearScreen:      clear,
				DebounceInterval: debounce,
				OnRestart: func(reason string) {
					fmt.Fprintf(os.Stderr, "\n[watch] restarting: %s\n\n", reason)
				},
				Graph: graph,
				ReconcilePaths: func(code int) (add, remove []string) {
					tf := depTraceFile
					depTraceFile = "" // next restart creates a fresh file
					defer func() {
						if tf != "" {
							_ = os.Remove(tf)
						}
					}()
					if code != 0 {
						return nil, nil // preserve known deps on failure
					}
					modules := readDepTrace(tf)
					modules = append(modules, epAbs)
					configs := collectConfigDeps(cwd, epAbs, mode, envFile, noEnvFile)
					return graph.Reconcile(modules, configs)
				},
			})

			code, err := sup.Run(cmd.Context())
			if err != nil && err != context.Canceled {
				return err
			}
			_ = code
			return nil
		},
	}

	cmd.Flags().BoolVar(&clearScreen, "clear-screen", false, "clear terminal before each restart")
	cmd.Flags().BoolVar(&noClearScreen, "no-clear-screen", false, "never clear terminal")
	cmd.Flags().StringArrayVar(&envFile, "env-file", nil, "path to .env file (repeatable)")
	cmd.Flags().BoolVar(&noEnvFile, "no-env-file", false, "skip .env file auto-discovery")
	cmd.Flags().StringVar(&mode, "mode", "", "mode for .env file discovery (sets NODE_ENV)")
	cmd.Flags().IntVar(&debounceMS, "debounce", 0, "debounce interval in milliseconds")
	return cmd
}

func buildWatchEnvOverlay(cwd string, envFile []string, noEnvFile bool, mode string) ([]string, error) {
	return buildEnvOverlay(cwd, leadingDispatchFlags{
		envFile:   envFile,
		noEnvFile: noEnvFile,
		mode:      mode,
	})
}
