package conformance

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// RunOptions configures a go-test certification matrix run (core or cli-ux).
type RunOptions struct {
	RepoRoot string
	Filter   string
	DryRun   bool
}

// RunCore executes the core certification matrix and returns a report.
func RunCore(ctx context.Context, opts RunOptions) (Report, error) {
	return runGoTestMatrix(ctx, opts, CoreManifestPath, "core certification failed")
}

// RunCLIUX executes the CLI UX certification matrix and returns a report.
func RunCLIUX(ctx context.Context, opts RunOptions) (Report, error) {
	return runGoTestMatrix(ctx, opts, CLIUXManifestPath, "cli-ux certification failed")
}

// RunRuntime executes the runtime stabilization certification matrix and returns a report.
func RunRuntime(ctx context.Context, opts RunOptions) (Report, error) {
	return runGoTestMatrix(ctx, opts, RuntimeManifestPath, "runtime certification failed")
}

func runGoTestMatrix(ctx context.Context, opts RunOptions, manifestPath func(string) string, failMsg string) (Report, error) {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		var err error
		repoRoot, err = RepoRootFromModule("")
		if err != nil {
			return Report{}, err
		}
	}

	manifest, err := LoadManifest(manifestPath(repoRoot))
	if err != nil {
		return Report{}, err
	}

	suites := FilterSuites(manifest.Suites, opts.Filter)
	suites = excludeProbeSuitesUnlessFiltered(suites, opts.Filter)
	if len(suites) == 0 {
		if opts.Filter != "" {
			return Report{}, fmt.Errorf("no suites match filter %q", opts.Filter)
		}
		return Report{}, fmt.Errorf("no suites selected for matrix %q", manifest.Matrix)
	}

	report := Report{
		SchemaVersion: ReportSchemaVersion,
		Matrix:        manifest.Matrix,
		CommitSHA:     ResolveCommitSHA(repoRoot),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		GoVersion:     runtime.Version(),
		ToolVersion:   toolVersion(),
		StartedAt:     time.Now().UTC(),
		Filter:        opts.Filter,
		DryRun:        opts.DryRun,
		Suites:        make([]SuiteResult, 0, len(suites)),
	}
	if !opts.DryRun {
		report.Tools = CollectTools()
	}

	for _, suite := range suites {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if !suiteSupportedOnPlatform(suite) {
			report.Suites = append(report.Suites, SuiteResult{
				ID:         suite.ID,
				Title:      suite.Title,
				Package:    suite.Package,
				Run:        suite.Run,
				Required:   suite.Required,
				Status:     StatusNotApplicable,
				SkipReason: "unsupported platform",
			})
			continue
		}
		if opts.DryRun {
			report.Suites = append(report.Suites, SuiteResult{
				ID:       suite.ID,
				Title:    suite.Title,
				Package:  suite.Package,
				Run:      suite.Run,
				Required: suite.Required,
				Status:   StatusPlanned,
			})
			continue
		}

		started := time.Now()
		exitCode, summary, output, runErr := RunTest(ctx, repoRoot, suite)
		report.Suites = append(report.Suites, suiteResultFromRun(suite, started, exitCode, summary, output, runErr))
	}

	report.FinishedAt = time.Now().UTC()
	report.Passed = reportPassed(report.Suites, opts.DryRun)
	if !report.Passed && !opts.DryRun {
		return report, fmt.Errorf("%s", failMsg)
	}
	return report, nil
}

func reportPassed(suites []SuiteResult, dryRun bool) bool {
	if len(suites) == 0 {
		return false
	}
	hasRequired := false
	for _, s := range suites {
		if s.Required {
			hasRequired = true
			if dryRun {
				if s.Status != StatusPlanned && s.Status != StatusNotApplicable {
					return false
				}
				continue
			}
			if s.Status != StatusPassed && s.Status != StatusNotApplicable {
				return false
			}
		}
	}
	return hasRequired
}

func excludeProbeSuitesUnlessFiltered(suites []Suite, filter string) []Suite {
	if strings.TrimSpace(filter) != "" {
		return suites
	}
	var out []Suite
	for _, s := range suites {
		if !s.Probe {
			out = append(out, s)
		}
	}
	return out
}

// ListCore returns suite definitions from the core manifest, optionally filtered.
func ListCore(repoRoot, filter string) ([]Suite, error) {
	return listGoTestMatrix(repoRoot, filter, CoreManifestPath)
}

// ListCLIUX returns suite definitions from the cli-ux manifest, optionally filtered.
func ListCLIUX(repoRoot, filter string) ([]Suite, error) {
	return listGoTestMatrix(repoRoot, filter, CLIUXManifestPath)
}

// ListRuntime returns suite definitions from the runtime manifest, optionally filtered.
func ListRuntime(repoRoot, filter string) ([]Suite, error) {
	return listGoTestMatrix(repoRoot, filter, RuntimeManifestPath)
}

func listGoTestMatrix(repoRoot, filter string, manifestPath func(string) string) ([]Suite, error) {
	if repoRoot == "" {
		var err error
		repoRoot, err = RepoRootFromModule("")
		if err != nil {
			return nil, err
		}
	}
	manifest, err := LoadManifest(manifestPath(repoRoot))
	if err != nil {
		return nil, err
	}
	suites := FilterSuites(manifest.Suites, filter)
	suites = excludeProbeSuitesUnlessFiltered(suites, filter)
	if len(suites) == 0 {
		if filter != "" {
			return nil, fmt.Errorf("no suites match filter %q", filter)
		}
		return nil, fmt.Errorf("no suites in manifest %q", manifest.Matrix)
	}
	return suites, nil
}
