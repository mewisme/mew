package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
)

func newBenchCmd() *cobra.Command {
	var (
		cold        bool
		warm        bool
		asJSON      bool
		fixture     string
		baseline    bool
		warmup      int
		samples     int
		outputPath  string
		forceOut    bool
		comparePath string
		benchCase   string
		profile     string
		timeOut     int
	)
	install := &cobra.Command{
		Use:   "install",
		Short: "Benchmark install against a fixture project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "bench install", "", "missing app context")
			}
			mode := app.BenchCold
			if warm {
				mode = app.BenchWarm
			}
			if cold && warm {
				return apperr.New(apperr.Usage, "bench install", "--cold|--warm", "specify only one mode flag")
			}
			result, err := app.BenchInstall(cmd.Context(), ac, app.BenchInstallOptions{
				Fixture:  fixture,
				Mode:     mode,
				Warmup:   warmup,
				Samples:  samples,
				Baseline: baseline,
			})
			if err != nil {
				return err
			}
			if asJSON {
				data, err := app.EncodeBenchResultJSON(result)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(data)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write([]byte("\n"))
				return err
			}
			_, err = cmd.OutOrStdout().Write([]byte(formatBenchResult(result) + "\n"))
			return err
		},
	}
	install.Flags().BoolVar(&cold, "cold", false, "clear isolated cache and store before install")
	install.Flags().BoolVar(&warm, "warm", false, "reuse cache from prior bench run in bench home")
	install.Flags().BoolVar(&asJSON, "json", false, "emit BenchResult JSON")
	install.Flags().StringVar(&fixture, "fixture", "", "fixture project path (default fixtures/bench/medium-graph)")
	install.Flags().BoolVar(&baseline, "baseline", false, "update benchmarks/install-baseline.json for this case")
	install.Flags().IntVar(&warmup, "warmup", 0, "discarded warmup iterations before sampling (default 1)")
	install.Flags().IntVar(&samples, "samples", 0, "measured iterations for median/p95 (default 5)")

	runner := &cobra.Command{
		Use:   "runner",
		Short: "Benchmark runner hot paths",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "bench runner", "", "missing app context")
			}
			if benchCase != "" && cmd.Flags().Changed("profile") {
				return apperr.New(apperr.Usage, "bench runner", "", "--case and --profile are mutually exclusive")
			}
			prof := app.RunnerBenchProfileSmoke
			if benchCase == "" {
				if profile != "" {
					prof = app.RunnerBenchProfile(profile)
				}
			}
			if warmup < 0 {
				return apperr.New(apperr.Usage, "bench runner", "", "--warmup must be >= 0")
			}
			if samples < 0 {
				return apperr.New(apperr.Usage, "bench runner", "", "--samples must be >= 0")
			}
			if timeOut < 0 {
				return apperr.New(apperr.Usage, "bench runner", "", "--timeout must be >= 0")
			}
			result, err := app.BenchRunner(cmd.Context(), ac, app.RunnerBenchOptions{
				Profile:    prof,
				CaseID:     benchCase,
				Compare:    comparePath,
				Output:     outputPath,
				Force:      forceOut,
				Samples:    samples,
				Warmup:     warmup,
				TimeoutSec: timeOut,
			})
			if err != nil {
				return err
			}
			if asJSON {
				data, err := app.EncodeRunnerBenchResultJSON(result)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(data)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write([]byte("\n"))
				return err
			}
			for _, c := range result.Cases {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "case=%s medianNs=%d p95Ns=%d samples=%d\n", c.ID, c.MedianNs, c.P95Ns, c.Samples)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	runner.Flags().BoolVar(&asJSON, "json", false, "emit JSON result")
	runner.Flags().StringVar(&outputPath, "output", "", "write JSON result to path")
	runner.Flags().BoolVar(&forceOut, "force", false, "overwrite existing --output file")
	runner.Flags().StringVar(&comparePath, "compare", "", "compare against baseline JSON")
	runner.Flags().StringVar(&benchCase, "case", "", "run one benchmark case id")
	runner.Flags().StringVar(&profile, "profile", "", "benchmark profile: smoke|full (default smoke)")
	runner.Flags().IntVar(&warmup, "warmup", 1, "discarded warmup iterations before sampling")
	runner.Flags().IntVar(&samples, "samples", 5, "measured iterations for median/p95")
	runner.Flags().IntVar(&timeOut, "timeout", 120, "per-iteration timeout in seconds")

	cmd := &cobra.Command{
		Use:     "benchmark",
		Aliases: []string{"bench"},
		Short:   "Performance benchmarks",
	}
	cmd.AddCommand(install)
	cmd.AddCommand(runner)
	cmd.AddCommand(newBenchRuntimeCmd())
	return cmd
}

func formatBenchResult(r app.BenchResult) string {
	return fmt.Sprintf("case=%s mode=%s samples=%d medianMs=%d p95Ms=%d totalMs=%d",
		r.Case, r.Mode, len(r.Samples), r.MedianMs, r.P95Ms, r.TotalMs)
}
