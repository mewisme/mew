package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app/benchruntime"
	"github.com/mewisme/mew/internal/apperr"
)

func newBenchRuntimeCmd() *cobra.Command {
	var (
		cold    bool
		warm    bool
		asJSON  bool
		samples int
		warmup  int
		timeOut int
		compare string
	)
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Benchmark runtime hot paths (transform, cache, execution)",
		Long: `Measure runtime performance with structured, reproducible benchmarks.

Metrics:
  runtime.startup.latency    Plan construction (ns)
  runtime.transform.latency  TypeScript transform (ns, cold or warm)
  runtime.execution.walltime m run process wall-clock (ns, warm)

All state (cache, store, config, temp) is isolated in a bench-owned
temporary directory. The user's real Mew cache is never touched.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cold && !warm {
				return apperr.New(apperr.Usage, "bench runtime", "", "specify --cold, --warm, or both")
			}
			if samples < 1 {
				return apperr.New(apperr.Usage, "bench runtime", "", "--samples must be >= 1")
			}
			if warmup < 0 {
				return apperr.New(apperr.Usage, "bench runtime", "", "--warmup must be >= 0")
			}
			if timeOut < 1 {
				return apperr.New(apperr.Usage, "bench runtime", "", "--timeout must be >= 1")
			}

			result, err := benchruntime.Run(cmd.Context(), benchruntime.Options{
				Cold:    cold,
				Warm:    warm,
				Samples: samples,
				Warmup:  warmup,
				Timeout: time.Duration(timeOut) * time.Second,
				Compare: compare,
			})
			if err != nil {
				return err
			}

			if asJSON {
				data, err := benchruntime.EncodeResultJSON(result)
				if err != nil {
					return apperr.Wrap(apperr.Internal, "bench runtime", "", err)
				}
				return writeStaticOut(cmd, string(data)+"\n")
			}

			return writeStaticOut(cmd, benchruntime.FormatResultHuman(result))
		},
	}

	cmd.Flags().BoolVar(&cold, "cold", false, "clear bench-owned cache before measuring")
	cmd.Flags().BoolVar(&warm, "warm", false, "prime bench-owned cache before measuring")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON result on stdout")
	cmd.Flags().IntVar(&samples, "samples", 5, "measured samples per metric")
	cmd.Flags().IntVar(&warmup, "warmup", 1, "discarded warmup iterations per metric")
	cmd.Flags().IntVar(&timeOut, "timeout", 120, "per-iteration timeout in seconds")
	cmd.Flags().StringVar(&compare, "compare", "", "compare against baseline JSON at path")

	return cmd
}
