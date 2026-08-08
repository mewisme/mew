package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/presentation"
	rt "github.com/mewisme/mew/internal/runtime"
	"github.com/mewisme/mew/internal/trace"
)

func newRuntimeTraceCmd() *cobra.Command {
	var (
		asJSON    bool
		noEnvFile bool
		envFile   []string
		mode      string
		nodeMode  bool
		loaders   []string
		bufSize   int
	)
	cmd := &cobra.Command{
		Use:   "trace <entrypoint>",
		Short: "Execute with structured runtime tracing",
		Long: `Run an entrypoint with structured runtime tracing enabled.

Collects deterministic, redacted trace events across the runtime pipeline:
plan, launch, exit, transform, cache, environment, and worker/watch
decisions. Events are versioned (schema v1), carry a session correlation
ID, and are sanitized to never expose secrets or environment values.

Output is human-readable by default. Use --json for machine-readable
NDJSON (one event per line) on stdout.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "runtime.trace", "", "missing app context")
			}

			entrypoint := args[0]
			appArgs := args[1:]
			cwd := ac.CWD

			epAbs := entrypoint
			if !filepath.IsAbs(epAbs) {
				epAbs = filepath.Join(cwd, epAbs)
			}

			// Build env overlay from CLI flags.
			envOverlay, envErr := buildEnvOverlay(cwd, leadingDispatchFlags{
				envFile:   envFile,
				noEnvFile: noEnvFile,
				mode:      mode,
			})
			if envErr != nil {
				return envErr
			}

			augMode := rt.AugmentDefault
			if nodeMode {
				augMode = rt.AugmentNone
			}
			// On Windows with augmentation, use original selector.
			ep := epAbs
			if augMode != rt.AugmentNone && runtime.GOOS == "windows" {
				ep = entrypoint
			}

			req := rt.LaunchRequest{
				Entrypoint:       ep,
				AppArgs:          appArgs,
				WorkingDir:       cwd,
				AugmentationMode: augMode,
				EnvOverlay:       envOverlay,
				Loaders:          append([]string(nil), loaders...),
				Stdio: rt.LaunchStdio{
					Stdin:  os.Stdin,
					Stdout: os.Stdout,
					Stderr: os.Stderr,
				},
			}

			// Attach transform session when augmentation is active.
			if augMode != rt.AugmentNone {
				contrib, contribErr := buildTransformContribution(cmd.Context(), cwd, epAbs, ac.Config)
				if contribErr != nil {
					return contribErr
				}
				req.Contribution = contrib
			}

			// Create trace session and sink.
			capacity := bufSize
			if capacity <= 0 {
				capacity = 256
			}
			sess := trace.NewSession()
			var sink *trace.ChannelSink

			ctx := trace.WithSession(cmd.Context(), sess)

			if asJSON {
				sink = trace.NewChannelSinkWriter(capacity, cmd.OutOrStdout())
			} else {
				sink = trace.NewChannelSink(capacity)
			}
			sess.Sink = sink

			launchErr := rt.PlanAndLaunch(ctx, req, ac.Config)

			if !asJSON {
				// Human-readable: drain events and render after execution.
				if err := sink.Close(); err != nil {
					return err
				}
				if err := renderHumanTrace(cmd, sink.Events(), sess.ID); err != nil {
					return err
				}
			} else {
				// NDJSON mode: events already streaming to stdout via writeLoop.
				// Wait briefly for final events then close.
				if err := sink.Close(); err != nil {
					return err
				}
			}

			return launchErr
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "output trace events as NDJSON")
	cmd.Flags().BoolVar(&nodeMode, "node", false, "run with stock Node (no augmentation)")
	cmd.Flags().StringArrayVar(&loaders, "loader", nil, "custom ESM loader path (repeatable)")
	cmd.Flags().StringArrayVar(&envFile, "env-file", nil, "path to .env file (repeatable)")
	cmd.Flags().BoolVar(&noEnvFile, "no-env-file", false, "skip .env file auto-discovery")
	cmd.Flags().StringVar(&mode, "mode", "", "mode for .env file discovery (sets NODE_ENV)")
	cmd.Flags().IntVar(&bufSize, "trace-buffer", 256, "trace event buffer capacity")
	_ = cmd.Flags().MarkHidden("trace-buffer")
	return cmd
}

// renderHumanTrace writes a human-readable trace summary.
func renderHumanTrace(cmd *cobra.Command, events <-chan trace.Event, sessionID string) error {
	g := ownerFlags(cmd.Root())
	r := g.mustStaticRenderer(cmd)

	lines := make([]string, 0, 2)
	lines = append(lines, r.Status(presentation.StatusLine{
		Text: fmt.Sprintf("Trace session %s", sessionID),
	}))

	counts := map[trace.Category]int{}
	for ev := range events {
		counts[ev.Cat]++
	}
	if len(counts) == 0 {
		lines = append(lines, r.Status(presentation.StatusLine{
			Text: "No trace events emitted.",
		}))
	} else {
		kvs := make([]presentation.KeyValue, 0, len(counts))
		order := []trace.Category{
			trace.CatLifecycle, trace.CatEnv, trace.CatResolution,
			trace.CatTransform, trace.CatCache, trace.CatWorker, trace.CatWatch,
		}
		for _, cat := range order {
			if n, ok := counts[cat]; ok {
				kvs = append(kvs, presentation.KeyValue{
					Key:   string(cat),
					Value: fmt.Sprintf("%d", n),
				})
			}
		}
		if len(kvs) > 0 {
			lines = append(lines, r.KeyValues(kvs))
		}
	}

	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return writeStaticOut(cmd, out)
}
