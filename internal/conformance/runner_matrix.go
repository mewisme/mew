package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/mewisme/mew/internal/apperr"
)

// RunnerRunOptions configures a runner certification run.
type RunnerRunOptions struct {
	RepoRoot string
	Groups   []string
	Filters  []string
	DryRun   bool
}

// RunnerReport is schema v1 runner certification output.
type RunnerReport struct {
	SchemaVersion        int                 `json:"schemaVersion"`
	Matrix               string              `json:"matrix"`
	ManifestDigest       string              `json:"manifestDigest"`
	WaiverManifestDigest string              `json:"waiverManifestDigest"`
	Revision             RunnerRevision      `json:"revision"`
	Environment          RunnerEnvironment   `json:"environment"`
	Selection            RunnerSelection     `json:"selection"`
	StartedAt            time.Time           `json:"startedAt"`
	FinishedAt           time.Time           `json:"finishedAt"`
	Overall              string              `json:"overall"`
	Suites               []RunnerSuiteResult `json:"suites"`
}

// RunnerRevision records git revision metadata.
type RunnerRevision struct {
	Available bool   `json:"available"`
	Commit    string `json:"commit,omitempty"`
	Dirty     bool   `json:"dirty"`
}

// RunnerEnvironment records host metadata.
type RunnerEnvironment struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"goVersion"`
	NodeVersion   string `json:"nodeVersion,omitempty"`
	NodeAvailable bool   `json:"nodeAvailable"`
}

// RunnerSelection records CLI filters applied.
type RunnerSelection struct {
	Groups   []string `json:"groups,omitempty"`
	SuiteIDs []string `json:"suiteIds,omitempty"`
}

// RunnerSuiteResult records one suite outcome.
type RunnerSuiteResult struct {
	ID               string    `json:"id"`
	Group            string    `json:"group"`
	Platform         string    `json:"platform"`
	Result           string    `json:"result"`
	Required         bool      `json:"required"`
	Probe            bool      `json:"probe"`
	WaiverPolicy     string    `json:"waiverPolicy"`
	AppliedWaiverIDs []string  `json:"appliedWaiverIds,omitempty"`
	ExpectedTests    []string  `json:"expectedTests"`
	MatchedTests     []string  `json:"matchedTests,omitempty"`
	SkippedTests     []string  `json:"skippedTests,omitempty"`
	StartedAt        time.Time `json:"startedAt"`
	FinishedAt       time.Time `json:"finishedAt"`
	DurationMs       int64     `json:"durationMs"`
	ExitCode         int       `json:"exitCode,omitempty"`
	StdoutTruncated  bool      `json:"stdoutTruncated,omitempty"`
	StderrTruncated  bool      `json:"stderrTruncated,omitempty"`
	Stdout           string    `json:"stdout,omitempty"`
	Stderr           string    `json:"stderr,omitempty"`
	FailureReason    string    `json:"failureReason,omitempty"`
	NetworkPolicy    string    `json:"networkPolicy"`
	Isolation        string    `json:"isolation"`
}

// RunRunner executes the runner certification matrix.
func RunRunner(ctx context.Context, opts RunnerRunOptions) (RunnerReport, error) {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		var err error
		repoRoot, err = RepoRootFromModule("")
		if err != nil {
			return RunnerReport{}, err
		}
	}
	manifest, err := LoadRunnerManifest(RunnerManifestPath(repoRoot))
	if err != nil {
		return RunnerReport{}, err
	}
	waivers, err := LoadWaiverManifest(RunnerWaiverPath(repoRoot))
	if err != nil {
		return RunnerReport{}, err
	}
	if err := validateWaiverReferences(manifest, waivers); err != nil {
		return RunnerReport{}, err
	}
	manifestDigest, err := RunnerManifestDigest(manifest)
	if err != nil {
		return RunnerReport{}, err
	}
	waiverDigest, err := RunnerWaiverManifestDigest(waivers)
	if err != nil {
		return RunnerReport{}, err
	}
	selected, err := SelectRunnerSuites(manifest.Suites, opts.Groups, opts.Filters)
	if err != nil {
		return RunnerReport{}, err
	}
	if !opts.DryRun {
		for _, suite := range selected {
			if !runnerSuiteSupportedOnPlatform(suite, runtime.GOOS) {
				continue
			}
			if err := ValidateExpectedTestsRegex(repoRoot, suite); err != nil {
				return RunnerReport{}, apperr.Wrap(apperr.Manifest, "conformance.runner.manifest", suite.ID, err)
			}
		}
	}

	goos := runtime.GOOS
	nodeVer, nodeOK := probeNodeVersion()
	report := RunnerReport{
		SchemaVersion:        RunnerReportSchemaVersion,
		Matrix:               RunnerMatrix,
		ManifestDigest:       manifestDigest,
		WaiverManifestDigest: waiverDigest,
		Revision: RunnerRevision{
			Available: ResolveCommitSHA(repoRoot) != "",
			Commit:    ResolveCommitSHA(repoRoot),
			Dirty:     gitDirty(repoRoot),
		},
		Environment: RunnerEnvironment{
			OS:            goosToPlatform(goos),
			Arch:          runtime.GOARCH,
			GoVersion:     runtime.Version(),
			NodeVersion:   nodeVer,
			NodeAvailable: nodeOK,
		},
		Selection: RunnerSelection{
			Groups:   dedupeStrings(opts.Groups),
			SuiteIDs: suiteIDs(selected),
		},
		StartedAt: time.Now().UTC(),
		Suites:    make([]RunnerSuiteResult, 0, len(selected)),
	}

	for _, suite := range selected {
		if err := ctx.Err(); err != nil {
			report.markNotRunRemaining(selected, suite.ID, goos)
			report.FinishedAt = time.Now().UTC()
			report.Overall = RunnerResultFail
			return report, err
		}
		if !runnerSuiteSupportedOnPlatform(suite, goos) {
			report.Suites = append(report.Suites, RunnerSuiteResult{
				ID:            suite.ID,
				Group:         suite.Group,
				Platform:      goosToPlatform(goos),
				Result:        RunnerResultNotApplicable,
				Required:      suite.Required,
				Probe:         suite.Probe,
				WaiverPolicy:  suite.WaiverPolicy,
				ExpectedTests: append([]string(nil), suite.ExpectedTests...),
				NetworkPolicy: suite.NetworkPolicy,
				Isolation:     suite.Isolation,
			})
			continue
		}
		if suite.Probe && os.Getenv("MEW_CONFORMANCE_TTY") != "1" {
			report.Suites = append(report.Suites, RunnerSuiteResult{
				ID:            suite.ID,
				Group:         suite.Group,
				Platform:      goosToPlatform(goos),
				Result:        RunnerResultProbeSkip,
				Required:      suite.Required,
				Probe:         suite.Probe,
				WaiverPolicy:  suite.WaiverPolicy,
				ExpectedTests: append([]string(nil), suite.ExpectedTests...),
				NetworkPolicy: suite.NetworkPolicy,
				Isolation:     suite.Isolation,
			})
			continue
		}
		if opts.DryRun {
			report.Suites = append(report.Suites, RunnerSuiteResult{
				ID:            suite.ID,
				Group:         suite.Group,
				Platform:      goosToPlatform(goos),
				Result:        RunnerResultSkip,
				Required:      suite.Required,
				Probe:         suite.Probe,
				WaiverPolicy:  suite.WaiverPolicy,
				ExpectedTests: append([]string(nil), suite.ExpectedTests...),
				NetworkPolicy: suite.NetworkPolicy,
				Isolation:     suite.Isolation,
			})
			continue
		}
		result := runOneRunnerSuite(ctx, repoRoot, suite, waivers, goos)
		report.Suites = append(report.Suites, result)
	}

	report.FinishedAt = time.Now().UTC()
	report.Overall = runnerOverallResult(report.Suites)
	if report.Overall != RunnerResultPass && report.Overall != RunnerResultProbePass {
		return report, fmt.Errorf("runner certification failed")
	}
	return report, nil
}

func runOneRunnerSuite(ctx context.Context, repoRoot string, suite RunnerSuite, waivers WaiverManifest, goos string) RunnerSuiteResult {
	started := time.Now().UTC()
	base := RunnerSuiteResult{
		ID:            suite.ID,
		Group:         suite.Group,
		Platform:      goosToPlatform(goos),
		Required:      suite.Required,
		Probe:         suite.Probe,
		WaiverPolicy:  suite.WaiverPolicy,
		ExpectedTests: append([]string(nil), suite.ExpectedTests...),
		StartedAt:     started,
		NetworkPolicy: suite.NetworkPolicy,
		Isolation:     suite.Isolation,
	}
	if matched, err := listTestsForSuite(repoRoot, suite, nil); err != nil {
		base.Result = RunnerResultFail
		base.FailureReason = err.Error()
		base.FinishedAt = time.Now().UTC()
		base.DurationMs = base.FinishedAt.Sub(started).Milliseconds()
		return base
	} else if !stringSlicesEqual(matched, suite.ExpectedTests) {
		base.Result = RunnerResultFail
		base.FailureReason = fmt.Sprintf("preflight mismatch: matched=%v expected=%v", matched, suite.ExpectedTests)
		base.MatchedTests = matched
		base.FinishedAt = time.Now().UTC()
		base.DurationMs = base.FinishedAt.Sub(started).Milliseconds()
		return base
	}
	iso, err := prepareSuiteIsolation(repoRoot, suite)
	if err != nil {
		base.Result = RunnerResultFail
		base.FailureReason = err.Error()
		base.FinishedAt = time.Now().UTC()
		base.DurationMs = base.FinishedAt.Sub(started).Milliseconds()
		return base
	}
	defer func() {
		if iso.Cleanup != nil {
			_ = iso.Cleanup()
		}
	}()
	out := runRunnerSuite(ctx, repoRoot, suite, iso)
	base.MatchedTests = out.MatchedTests
	base.SkippedTests = out.SkippedTests
	base.ExitCode = out.ExitCode
	base.Stdout = redactDiagnostics(out.Stdout)
	base.Stderr = redactDiagnostics(out.Stderr)
	base.StdoutTruncated = out.StdoutTruncated
	base.StderrTruncated = out.StderrTruncated
	base.FinishedAt = time.Now().UTC()
	base.DurationMs = base.FinishedAt.Sub(started).Milliseconds()

	if ctx.Err() != nil {
		base.Result = RunnerResultTimeout
		base.FailureReason = ctx.Err().Error()
		return base
	}
	if out.Summary.ParseError != "" {
		base.Result = runnerFailResult(suite)
		base.FailureReason = out.Summary.ParseError
		return base
	}
	if reason := out.Summary.FailReason(Suite{Required: suite.Required, Probe: suite.Probe}, out.ExitCode, false); reason != "" {
		base.Result = runnerFailResult(suite)
		base.FailureReason = reason
		return base
	}
	if suite.Probe {
		base.Result = RunnerResultProbePass
		return base
	}
	waiverIDs := activeWaiversForSuite(waivers, suite, goos)
	if len(waiverIDs) > 0 && suite.WaiverPolicy == "allowed" {
		base.AppliedWaiverIDs = waiverIDs
		base.Result = RunnerResultPassWithWaiver
		return base
	}
	base.Result = RunnerResultPass
	return base
}

func runnerFailResult(suite RunnerSuite) string {
	if suite.Probe {
		return RunnerResultProbeFail
	}
	return RunnerResultFail
}

func runnerOverallResult(suites []RunnerSuiteResult) string {
	if len(suites) == 0 {
		return RunnerResultFail
	}
	anyFail := false
	anyProbeFail := false
	allProbeOrPass := true
	hasRequired := false
	for _, s := range suites {
		if s.Required {
			hasRequired = true
		}
		switch s.Result {
		case RunnerResultFail, RunnerResultTimeout, RunnerResultProbeFail:
			anyFail = true
			if s.Probe {
				anyProbeFail = true
			}
		case RunnerResultPass, RunnerResultPassWithWaiver, RunnerResultNotApplicable, RunnerResultProbeSkip, RunnerResultSkip, RunnerResultNotRun:
		default:
			allProbeOrPass = false
		}
		if s.Required && s.Result != RunnerResultPass && s.Result != RunnerResultPassWithWaiver && s.Result != RunnerResultNotApplicable {
			if s.Probe && s.Result == RunnerResultProbeSkip {
				continue
			}
			anyFail = true
		}
	}
	if !hasRequired {
		return RunnerResultFail
	}
	if anyFail && !anyProbeFail {
		return RunnerResultFail
	}
	if anyProbeFail {
		return RunnerResultProbeFail
	}
	if allProbeOrPass {
		return RunnerResultPass
	}
	return RunnerResultPass
}

func (r *RunnerReport) markNotRunRemaining(suites []RunnerSuite, fromID, goos string) {
	found := false
	for _, suite := range suites {
		if suite.ID == fromID {
			found = true
		}
		if !found {
			continue
		}
		if suite.ID == fromID {
			continue
		}
		r.Suites = append(r.Suites, RunnerSuiteResult{
			ID:            suite.ID,
			Group:         suite.Group,
			Platform:      goosToPlatform(goos),
			Result:        RunnerResultNotRun,
			Required:      suite.Required,
			Probe:         suite.Probe,
			WaiverPolicy:  suite.WaiverPolicy,
			ExpectedTests: append([]string(nil), suite.ExpectedTests...),
			NetworkPolicy: suite.NetworkPolicy,
			Isolation:     suite.Isolation,
		})
	}
}

func suiteIDs(suites []RunnerSuite) []string {
	if len(suites) == 0 {
		return nil
	}
	out := make([]string, len(suites))
	for i, s := range suites {
		out[i] = s.ID
	}
	return out
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func probeNodeVersion() (string, bool) {
	tools := CollectTools()
	for _, tool := range tools {
		if tool.Name == "node" && tool.Version != "" {
			return tool.Version, true
		}
	}
	return "", false
}

func gitDirty(repoRoot string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "status", "--porcelain")
	out, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func redactDiagnostics(s string) string {
	replacers := []string{"NPM_TOKEN", "GH_TOKEN", "GITLAB_TOKEN", "AWS_SECRET", "PASSWORD", "SECRET"}
	for _, token := range replacers {
		if strings.Contains(strings.ToUpper(s), token) {
			return "[redacted]"
		}
	}
	return s
}

// EncodeRunnerReportJSON returns indented JSON.
func (r RunnerReport) EncodeRunnerReportJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
