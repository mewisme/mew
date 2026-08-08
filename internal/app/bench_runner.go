package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/conformance"
)

const runnerBenchCommandVersion = "runner-bench-v1"

// RunnerBenchProfile selects the benchmark case set.
type RunnerBenchProfile string

const (
	RunnerBenchProfileSmoke RunnerBenchProfile = "smoke"
	RunnerBenchProfileFull  RunnerBenchProfile = "full"
)

// RunnerBenchOptions configures m benchmark runner.
type RunnerBenchOptions struct {
	Profile    RunnerBenchProfile
	CaseID     string
	Compare    string
	Output     string
	Force      bool
	Samples    int
	Warmup     int
	TimeoutSec int
}

// RunnerBenchBaselineV1 is the baseline schema for runner benchmarks.
type RunnerBenchBaselineV1 struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	FixtureDigest  string                 `json:"fixtureDigest"`
	CommandVersion string                 `json:"commandVersion"`
	Environment    RunnerBenchEnvironment `json:"environment"`
	RecordedAt     string                 `json:"recordedAt"`
	Cases          []RunnerBenchCase      `json:"cases"`
	ThresholdPct   float64                `json:"thresholdPct,omitempty"`
}

// RunnerBenchEnvironment records host metadata for baselines.
type RunnerBenchEnvironment struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	MachineClass string `json:"machineClass"`
	GoVersion    string `json:"goVersion"`
	NodeVersion  string `json:"nodeVersion"`
	Commit       string `json:"commit,omitempty"`
	LogicalCPUs  int    `json:"logicalCpus"`
}

// RunnerBenchCase is one benchmark measurement set.
type RunnerBenchCase struct {
	ID           string  `json:"id"`
	CacheState   string  `json:"cacheState"`
	Samples      int     `json:"samples"`
	RawSamplesNs []int64 `json:"rawSamplesNs"`
	MedianNs     int64   `json:"medianNs"`
	P95Ns        int64   `json:"p95Ns"`
	Units        string  `json:"units"`
}

// RunnerBenchResult is the JSON output for m benchmark runner.
type RunnerBenchResult struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	Profile        string                 `json:"profile,omitempty"`
	CaseID         string                 `json:"caseId,omitempty"`
	CommandVersion string                 `json:"commandVersion"`
	Environment    RunnerBenchEnvironment `json:"environment"`
	RecordedAt     string                 `json:"recordedAt"`
	WarmupSamples  int                    `json:"warmupSamples"`
	Cases          []RunnerBenchCase      `json:"cases"`
	Compare        *RunnerBenchCompare    `json:"compare,omitempty"`
}

// RunnerBenchCompare records baseline comparison outcome.
type RunnerBenchCompare struct {
	Status    string                     `json:"status"`
	Baseline  string                     `json:"baseline,omitempty"`
	Message   string                     `json:"message,omitempty"`
	Threshold float64                    `json:"thresholdPct,omitempty"`
	Details   []RunnerBenchCompareDetail `json:"details,omitempty"`
}

type RunnerBenchCompareDetail struct {
	CaseID           string  `json:"caseId"`
	CurrentMedianNs  int64   `json:"currentMedianNs"`
	BaselineMedianNs int64   `json:"baselineMedianNs"`
	DeltaPct         float64 `json:"deltaPct"`
	Verdict          string  `json:"verdict"`
}

// BenchRunner executes runner smoke/full benchmarks without public registry access.
func BenchRunner(ctx context.Context, ac *Context, opts RunnerBenchOptions) (RunnerBenchResult, error) {
	if ac == nil {
		return RunnerBenchResult{}, apperr.New(apperr.Internal, "app.bench.runner", "", "missing app context")
	}
	if opts.Profile == "" {
		opts.Profile = RunnerBenchProfileSmoke
	}
	if opts.Profile != "" && opts.CaseID != "" {
		return RunnerBenchResult{}, apperr.New(apperr.Usage, "app.bench.runner", "", "--case and --profile are mutually exclusive")
	}
	cases := runnerBenchCases(opts)
	if len(cases) == 0 {
		return RunnerBenchResult{}, apperr.New(apperr.Usage, "app.bench.runner", "", "no benchmark cases selected")
	}
	samples := opts.Samples
	if samples < 0 {
		return RunnerBenchResult{}, apperr.New(apperr.Usage, "app.bench.runner", "", "samples must be >= 0")
	}
	if samples == 0 {
		samples = 5
	}
	warmup := opts.Warmup
	if warmup < 0 {
		return RunnerBenchResult{}, apperr.New(apperr.Usage, "app.bench.runner", "", "warmup must be >= 0")
	}
	if warmup == 0 {
		warmup = 1
	}
	timeout := opts.TimeoutSec
	if timeout <= 0 {
		timeout = 120
	}
	repoRoot, err := conformance.RepoRootFromModule("")
	if err != nil {
		return RunnerBenchResult{}, apperr.Wrap(apperr.Internal, "app.bench.runner", "", err)
	}
	srcFixture := filepath.Join(repoRoot, "fixtures", "runner", "basic-scripts")
	fixtureRoot, err := os.MkdirTemp("", "mew-runner-bench-")
	if err != nil {
		return RunnerBenchResult{}, apperr.Wrap(apperr.IO, "app.bench.runner", "", err)
	}
	defer func() { _ = os.RemoveAll(fixtureRoot) }()
	if err := copyDir(srcFixture, fixtureRoot); err != nil {
		return RunnerBenchResult{}, apperr.Wrap(apperr.IO, "app.bench.runner", fixtureRoot, err)
	}
	measured := make([]RunnerBenchCase, 0, len(cases))
	for _, c := range cases {
		raw, err := measureRunnerCase(ctx, fixtureRoot, c.ID, warmup+samples, time.Duration(timeout)*time.Second)
		if err != nil {
			return RunnerBenchResult{}, err
		}
		if len(raw) < warmup+samples {
			return RunnerBenchResult{}, apperr.New(apperr.Internal, "app.bench.runner", c.ID,
				fmt.Sprintf("expected %d samples, got %d", warmup+samples, len(raw)))
		}
		warmupRaw := raw[:warmup]
		sampleRaw := raw[warmup:]
		sort.Slice(sampleRaw, func(i, j int) bool { return sampleRaw[i] < sampleRaw[j] })
		med := sampleRaw[len(sampleRaw)/2]
		p95 := sampleRaw[int(float64(len(sampleRaw)-1)*0.95)]
		_ = warmupRaw
		measured = append(measured, RunnerBenchCase{
			ID:           c.ID,
			CacheState:   c.CacheState,
			Samples:      len(sampleRaw),
			RawSamplesNs: append([]int64(nil), sampleRaw...),
			MedianNs:     med,
			P95Ns:        p95,
			Units:        "ns",
		})
	}
	result := RunnerBenchResult{
		SchemaVersion:  1,
		Profile:        string(opts.Profile),
		CaseID:         opts.CaseID,
		CommandVersion: runnerBenchCommandVersion,
		Environment:    runnerBenchEnvironment(ac.Commit),
		RecordedAt:     time.Now().UTC().Format(time.RFC3339),
		WarmupSamples:  warmup,
		Cases:          measured,
	}
	if opts.Compare != "" {
		cmp, err := compareRunnerBaseline(opts.Compare, result)
		if err != nil {
			return RunnerBenchResult{}, err
		}
		result.Compare = &cmp
	}
	if opts.Output != "" {
		if !opts.Force {
			if _, err := os.Stat(opts.Output); err == nil {
				return RunnerBenchResult{}, apperr.New(apperr.Usage, "app.bench.runner", opts.Output, "output file exists (use --force)")
			}
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return RunnerBenchResult{}, apperr.Wrap(apperr.IO, "app.bench.runner", opts.Output, err)
		}
		tmp := opts.Output + ".tmp"
		if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
			return RunnerBenchResult{}, apperr.Wrap(apperr.IO, "app.bench.runner", opts.Output, err)
		}
		if err := os.Rename(tmp, opts.Output); err != nil {
			return RunnerBenchResult{}, apperr.Wrap(apperr.IO, "app.bench.runner", opts.Output, err)
		}
	}
	return result, nil
}

func runnerBenchCases(opts RunnerBenchOptions) []RunnerBenchCase {
	if opts.CaseID != "" {
		return []RunnerBenchCase{{ID: opts.CaseID, CacheState: "warm"}}
	}
	switch opts.Profile {
	case RunnerBenchProfileFull:
		return []RunnerBenchCase{
			{ID: "project-script", CacheState: "project"},
			{ID: "dlx-warm", CacheState: "warm"},
		}
	default:
		return []RunnerBenchCase{{ID: "project-script", CacheState: "project"}}
	}
}

func measureRunnerCase(ctx context.Context, fixtureRoot, caseID string, totalIterations int, timeout time.Duration) ([]int64, error) {
	if _, err := exec.LookPath("node"); err != nil {
		return nil, apperr.Wrap(apperr.NotFound, "app.bench.runner", "node", err)
	}
	repoRoot, err := conformance.RepoRootFromModule("")
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, totalIterations)
	for i := 0; i < totalIterations; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		iterCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		cmd := exec.CommandContext(iterCtx, "go", "run", filepath.Join(repoRoot, "cmd", "m"),
			"--cwd", fixtureRoot, "--output", "silent", "run", "dev")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		err := cmd.Run()
		cancel()
		elapsed := time.Since(start).Nanoseconds()
		if err != nil {
			if iterCtx.Err() == context.DeadlineExceeded {
				return nil, apperr.New(apperr.Internal, "app.bench.runner", caseID,
					fmt.Sprintf("iteration %d timed out after %v", i, timeout))
			}
			return nil, apperr.Wrap(apperr.Internal, "app.bench.runner", caseID, err)
		}
		if elapsed <= 0 {
			return nil, apperr.New(apperr.Internal, "app.bench.runner", caseID,
				fmt.Sprintf("invalid elapsed duration %d ns at iteration %d", elapsed, i))
		}
		out = append(out, elapsed)
	}
	return out, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer func() { _ = out.Close() }()
		_, err = io.Copy(out, in)
		return err
	})
}

func runnerBenchEnvironment(commit string) RunnerBenchEnvironment {
	nodeVer := ""
	if path, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command("node", "-v")
		cmd.Path = path
		if b, err := cmd.Output(); err == nil {
			nodeVer = strings.TrimSpace(string(b))
		}
	}
	return RunnerBenchEnvironment{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		MachineClass: runtime.GOARCH,
		GoVersion:    runtime.Version(),
		NodeVersion:  nodeVer,
		Commit:       commit,
		LogicalCPUs:  runtime.NumCPU(),
	}
}

func compareRunnerBaseline(path string, result RunnerBenchResult) (RunnerBenchCompare, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RunnerBenchCompare{}, apperr.Wrap(apperr.Manifest, "app.bench.runner", path, err)
	}
	var baseline RunnerBenchBaselineV1
	if err := json.Unmarshal(data, &baseline); err != nil {
		return RunnerBenchCompare{}, apperr.Wrap(apperr.Manifest, "app.bench.runner", path, err)
	}
	if baseline.SchemaVersion != 1 || baseline.CommandVersion != runnerBenchCommandVersion {
		return RunnerBenchCompare{}, apperr.New(apperr.Manifest, "app.bench.runner", path,
			"incompatible baseline schema or command version")
	}
	if baseline.Environment.OS != "" && !strings.EqualFold(baseline.Environment.OS, result.Environment.OS) {
		return RunnerBenchCompare{}, apperr.New(apperr.Manifest, "app.bench.runner", path,
			fmt.Sprintf("baseline OS %q differs from current %q", baseline.Environment.OS, result.Environment.OS))
	}
	if baseline.Environment.Arch != "" && !strings.EqualFold(baseline.Environment.Arch, result.Environment.Arch) {
		return RunnerBenchCompare{}, apperr.New(apperr.Manifest, "app.bench.runner", path,
			fmt.Sprintf("baseline arch %q differs from current %q", baseline.Environment.Arch, result.Environment.Arch))
	}
	threshold := baseline.ThresholdPct
	if threshold <= 0 {
		threshold = 10.0
	}
	baselineByID := make(map[string]RunnerBenchCase, len(baseline.Cases))
	for _, c := range baseline.Cases {
		baselineByID[c.ID] = c
	}
	var details []RunnerBenchCompareDetail
	allPass := true
	for _, cur := range result.Cases {
		bl, ok := baselineByID[cur.ID]
		if !ok {
			return RunnerBenchCompare{}, apperr.New(apperr.Manifest, "app.bench.runner", path,
				fmt.Sprintf("case %q not found in baseline", cur.ID))
		}
		if bl.MedianNs <= 0 {
			return RunnerBenchCompare{}, apperr.New(apperr.Manifest, "app.bench.runner", path,
				fmt.Sprintf("baseline case %q has invalid median %d", cur.ID, bl.MedianNs))
		}
		deltaPct := float64(cur.MedianNs-bl.MedianNs) / float64(bl.MedianNs) * 100.0
		verdict := "pass"
		if deltaPct > threshold {
			verdict = "regression"
			allPass = false
		} else if deltaPct < -threshold {
			verdict = "improvement"
		}
		details = append(details, RunnerBenchCompareDetail{
			CaseID:           cur.ID,
			CurrentMedianNs:  cur.MedianNs,
			BaselineMedianNs: bl.MedianNs,
			DeltaPct:         deltaPct,
			Verdict:          verdict,
		})
	}
	status := "pass"
	if !allPass {
		status = "regression"
	}
	return RunnerBenchCompare{
		Status:    status,
		Baseline:  path,
		Threshold: threshold,
		Details:   details,
	}, nil
}

// EncodeRunnerBenchResultJSON returns indented JSON for a runner benchmark result.
func EncodeRunnerBenchResultJSON(r RunnerBenchResult) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
