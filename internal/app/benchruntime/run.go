package benchruntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/node"
	rtime "github.com/mewisme/mew/internal/runtime"
	"github.com/mewisme/mew/internal/transform"
)

// Options configures a runtime benchmark run.
type Options struct {
	Cold    bool
	Warm    bool
	Samples int
	Warmup  int
	Timeout time.Duration
	Compare string // path to baseline JSON
	Fixture string // path to fixture directory or file
}

// benchRoot holds isolated benchmark-owned directories.
type benchRoot struct {
	Home     string
	CacheDir string
	StoreDir string
}

// cleanup removes the bench-owned root tree.
func (br *benchRoot) cleanup() error {
	return os.RemoveAll(br.Home)
}

// createBenchRoot creates an isolated bench-owned directory tree.
func createBenchRoot() (*benchRoot, error) {
	home, err := os.MkdirTemp("", "mew-bench-runtime-")
	if err != nil {
		return nil, fmt.Errorf("create bench home: %w", err)
	}
	br := &benchRoot{
		Home:     home,
		CacheDir: filepath.Join(home, ".cache", "mew"),
		StoreDir: filepath.Join(home, ".local", "share", "mew", "store"),
	}
	for _, d := range []string{br.CacheDir, br.StoreDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			br.cleanup()
			return nil, fmt.Errorf("create bench dir %s: %w", d, err)
		}
	}
	return br, nil
}

// benchEnv returns the environment with bench-owned paths overlaid.
func (br *benchRoot) benchEnv() []string {
	base := os.Environ()
	out := make([]string, 0, len(base)+8)
	for _, kv := range base {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		switch key {
		case "HOME", "USERPROFILE",
			"XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
			"MEW_HOME", "MEW_CACHE_DIR", "MEW_STORE_DIR", "MEW_CONFIG_DIR",
			"NO_PROXY", "no_proxy":
			continue
		}
		out = append(out, kv)
	}
	out = append(out,
		"HOME="+br.Home,
		"XDG_CACHE_HOME="+filepath.Join(br.Home, ".cache"),
		"XDG_CONFIG_HOME="+filepath.Join(br.Home, ".config"),
		"XDG_DATA_HOME="+filepath.Join(br.Home, ".local", "share"),
		"XDG_STATE_HOME="+filepath.Join(br.Home, ".local", "state"),
		"MEW_HOME="+br.Home,
		"MEW_CACHE_DIR="+br.CacheDir,
		"MEW_STORE_DIR="+br.StoreDir,
		"MEW_CONFIG_DIR="+filepath.Join(br.Home, ".config", "mew"),
		"NO_PROXY=*",
		"no_proxy=*",
	)
	return out
}

// benchConfig returns a config.Effective pointing at bench-owned directories.
func (br *benchRoot) benchConfig() *config.Effective {
	return &config.Effective{
		Values: map[string]config.Value{
			"cache.dir": {Raw: br.CacheDir},
			"store.dir": {Raw: br.StoreDir},
		},
	}
}

// transformCachePath returns the bench-owned transform cache directory.
func (br *benchRoot) transformCachePath() string {
	return filepath.Join(br.CacheDir, "transform", fmt.Sprintf("v%d", transform.CacheSchemaVersion))
}

// clearTransformCache removes the bench-owned transform cache directory.
func (br *benchRoot) clearTransformCache() error {
	p := br.transformCachePath()
	if err := os.RemoveAll(p); err != nil {
		return fmt.Errorf("clear bench transform cache: %w", err)
	}
	return nil
}

// Run executes the runtime benchmark with structured measurements.
func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Samples < 1 {
		return Result{}, apperr.New(apperr.Usage, "bench runtime", "", "samples must be >= 1")
	}
	if opts.Warmup < 0 {
		return Result{}, apperr.New(apperr.Usage, "bench runtime", "", "warmup must be >= 0")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 120 * time.Second
	}

	br, err := createBenchRoot()
	if err != nil {
		return Result{}, apperr.Wrap(apperr.Internal, "bench runtime", "", err)
	}
	defer func() { _ = br.cleanup() }()

	env := buildEnvironment(opts.Fixture)
	var measurements []Measurement

	// Always measure startup latency (runtime.Plan construction).
	sm, err := measureStartup(ctx, br, opts)
	if err != nil {
		return buildResult(env, opts, measurements), err
	}
	if sm != nil {
		measurements = append(measurements, *sm)
	}

	// Cold mode: clear bench-owned transform cache, measure cold paths.
	if opts.Cold {
		tf, err := measureTransformCold(ctx, br, opts)
		if err != nil {
			return buildResult(env, opts, measurements), err
		}
		if tf != nil {
			measurements = append(measurements, *tf)
		}
	}

	// Warm mode: prime bench-owned transform cache, measure warm paths.
	if opts.Warm {
		tf, err := measureTransformWarm(ctx, br, opts)
		if err != nil {
			return buildResult(env, opts, measurements), err
		}
		if tf != nil {
			measurements = append(measurements, *tf)
		}

		ew, err := measureExecution(ctx, br, opts)
		if err != nil {
			return buildResult(env, opts, measurements), err
		}
		if ew != nil {
			measurements = append(measurements, *ew)
		}
	}

	sortMeasurements(measurements)
	result := buildResult(env, opts, measurements)

	if opts.Compare != "" {
		cmp, err := compareBaseline(opts.Compare, result)
		if err != nil {
			return result, err
		}
		result.Compare = &cmp
	}

	if err := br.cleanup(); err != nil {
		return result, apperr.Wrap(apperr.IO, "bench runtime", br.Home, err)
	}

	return result, nil
}

func buildResult(env Environment, opts Options, measurements []Measurement) Result {
	return Result{
		SchemaVersion: SchemaVersion,
		Environment:   env,
		Cold:          opts.Cold,
		Warm:          opts.Warm,
		Samples:       opts.Samples,
		Warmup:        opts.Warmup,
		RecordedAt:    time.Now().UTC().Format(time.RFC3339),
		Measurements:  measurements,
	}
}

// measureStartup measures runtime.Plan construction latency.
// This measures the Go-level overhead of constructing a Node launch plan.
func measureStartup(ctx context.Context, br *benchRoot, opts Options) (*Measurement, error) {
	eff := br.benchConfig()
	dir := br.Home
	fixtureFile := filepath.Join(dir, "startup.js")
	if err := os.WriteFile(fixtureFile, []byte("console.log(1);"), 0o644); err != nil {
		return nil, fmt.Errorf("write startup fixture: %w", err)
	}

	// Discover Node to confirm availability.
	if _, err := node.Discover(ctx, node.Request{WorkingDir: dir}); err != nil {
		return nil, fmt.Errorf("node discover: %w", err)
	}

	totalIters := opts.Warmup + opts.Samples
	raw := make([]int64, 0, totalIters)

	for i := 0; i < totalIters; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		elapsed, err := measureDuration(func() error {
			_, e := rtime.Plan(ctx, rtime.LaunchRequest{
				Entrypoint:       fixtureFile,
				WorkingDir:       dir,
				AugmentationMode: rtime.AugmentDefault,
				Stdio: rtime.LaunchStdio{
					Stdin:  nil,
					Stdout: nil,
					Stderr: nil,
				},
			}, eff)
			return e
		})
		if err != nil {
			return nil, fmt.Errorf("startup iteration %d: %w", i, err)
		}
		raw = append(raw, elapsed)
	}

	return buildMeasurement(MetricStartupLatency, "", raw, opts.Warmup)
}

// measureTransformCold measures cold transform latency.
// Clears the bench-owned transform cache, then times engine.Transform + WriteCache.
func measureTransformCold(ctx context.Context, br *benchRoot, opts Options) (*Measurement, error) {
	if err := br.clearTransformCache(); err != nil {
		return nil, err
	}
	return measureTransform(ctx, br, opts, CategoryCold)
}

// measureTransformWarm measures warm transform (cache hit) latency.
// Primes the cache via the cold path first, then times TryReadCache.
func measureTransformWarm(ctx context.Context, br *benchRoot, opts Options) (*Measurement, error) {
	// Prime the cache using the cold path (engine transform + write).
	if _, err := measureTransform(ctx, br, Options{Samples: 1, Warmup: 0}, CategoryCold); err != nil {
		return nil, fmt.Errorf("prime transform cache: %w", err)
	}
	return measureTransform(ctx, br, opts, CategoryWarm)
}

// measureTransform runs transform benchmarks using the cache layer.
func measureTransform(ctx context.Context, br *benchRoot, opts Options, cat Category) (*Measurement, error) {
	engine := transform.NewEsbuildEngine()
	cacheDir := br.transformCachePath()

	src := `const x: string = "hello"; console.log(x);`
	req := transform.TransformRequest{
		SourcePath:    "test.ts",
		SourceBytes:   []byte(src),
		Loader:        transform.LoaderTS,
		Format:        transform.FormatESM,
		SourceMapMode: transform.SourceMapNone,
	}

	engineID := transform.EngineIdentity{Name: "esbuild", Version: "builtin"}
	key := transform.CacheKey(req, engineID)

	totalIters := opts.Warmup + opts.Samples
	raw := make([]int64, 0, totalIters)

	for i := 0; i < totalIters; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if cat == CategoryWarm {
			// Warm: measure cache read time.
			elapsed, err := measureDuration(func() error {
				result, e := transform.TryReadCache(cacheDir, key)
				if e != nil {
					return e
				}
				if result == nil {
					return fmt.Errorf("cache miss on primed cache for key %s", key)
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("transform warm iteration %d: %w", i, err)
			}
			raw = append(raw, elapsed)
		} else {
			// Cold: measure engine transform + cache write.
			elapsed, err := measureDuration(func() error {
				result, e := engine.Transform(ctx, req)
				if e != nil {
					return e
				}
				return transform.WriteCache(cacheDir, key, &result)
			})
			if err != nil {
				return nil, fmt.Errorf("transform cold iteration %d: %w", i, err)
			}
			raw = append(raw, elapsed)
		}
	}

	return buildMeasurement(MetricTransformLatency, cat, raw, opts.Warmup)
}

// measureExecution measures end-to-end "m run" wall-clock time.
func measureExecution(ctx context.Context, br *benchRoot, opts Options) (*Measurement, error) {
	repoRoot, err := findModuleRoot()
	if err != nil {
		return nil, err
	}

	// Write a minimal package.json with a start script.
	pkgJSON := filepath.Join(br.Home, "package.json")
	if err := os.WriteFile(pkgJSON, []byte(`{"name":"bench","private":true,"scripts":{"start":"node exec.js"}}`+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("write package.json: %w", err)
	}

	fixtureFile := filepath.Join(br.Home, "exec.js")
	if err := os.WriteFile(fixtureFile, []byte(`console.log("ready");`+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("write exec fixture: %w", err)
	}

	env := br.benchEnv()

	totalIters := opts.Warmup + opts.Samples
	raw := make([]int64, 0, totalIters)

	for i := 0; i < totalIters; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		iterCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		elapsed, err := measureDuration(func() error {
			cmd := exec.CommandContext(iterCtx, "go", "run", filepath.Join(repoRoot, "cmd", "m"),
				"--cwd", br.Home, "--output", "silent", "run", "start")
			cmd.Dir = repoRoot
			cmd.Env = env
			cmd.Stdout = nil
			cmd.Stderr = nil
			return cmd.Run()
		})
		cancel()
		if err != nil {
			if iterCtx.Err() == context.DeadlineExceeded {
				return nil, fmt.Errorf("execution iteration %d timed out after %v", i, opts.Timeout)
			}
			return nil, fmt.Errorf("execution iteration %d: %w", i, err)
		}
		raw = append(raw, elapsed)
	}

	return buildMeasurement(MetricExecutionWallTime, CategoryWarm, raw, opts.Warmup)
}

// buildMeasurement constructs a Measurement from raw samples.
func buildMeasurement(id MetricID, cat Category, raw []int64, warmup int) (*Measurement, error) {
	warmupSamples := raw[:warmup]
	measuredSamples := raw[warmup:]
	_ = warmupSamples

	sorted := sortSamples(measuredSamples)
	if err := validateSamples(sorted); err != nil {
		return nil, fmt.Errorf("%s %s: %w", id, cat, err)
	}

	return &Measurement{
		ID:           id,
		Unit:         UnitNanoseconds,
		Category:     cat,
		RawSamplesNs: sorted,
		Aggregate:    computeAggregate(sorted),
		SampleCount:  len(sorted),
		WarmupCount:  warmup,
	}, nil
}

// buildEnvironment creates the Environment metadata.
func buildEnvironment(fixture string) Environment {
	nodeVersion := ""
	if n, err := node.Discover(context.Background(), node.Request{}); err == nil {
		nodeVersion = n.NormalizedVersion
	}
	return Environment{
		SchemaVersion: SchemaVersion,
		GoVersion:     goruntime.Version(),
		NodeVersion:   nodeVersion,
		OS:            goruntime.GOOS,
		Arch:          goruntime.GOARCH,
		LogicalCPUs:   goruntime.NumCPU(),
		Fixture:       fixture,
	}
}

// findModuleRoot returns the repository root directory.
func findModuleRoot() (string, error) {
	cmd := exec.Command("go", "env", "GOMOD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go env GOMOD: %w", err)
	}
	modPath := strings.TrimSpace(string(out))
	if modPath == "" {
		return "", fmt.Errorf("go env GOMOD returned empty")
	}
	return filepath.Dir(modPath), nil
}
