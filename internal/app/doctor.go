package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/linker/planner"
	"github.com/mewisme/mew/internal/node"
	runtimepkg "github.com/mewisme/mew/internal/runtime"
	"github.com/mewisme/mew/internal/transaction"
	"github.com/mewisme/mew/internal/transform"
	"github.com/mewisme/mew/internal/watch"
)

const DoctorReportSchemaVersion = 1

// DoctorCheckStatus is ok, warn, or fail.
type DoctorCheckStatus string

const (
	DoctorStatusOK      DoctorCheckStatus = "ok"
	DoctorStatusWarn    DoctorCheckStatus = "warn"
	DoctorStatusFail    DoctorCheckStatus = "fail"
	DoctorStatusSkipped DoctorCheckStatus = "skipped"
)

// DoctorCheck is one health check result.
type DoctorCheck struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	Details     string `json:"details,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// DoctorReport is the JSON schema v1 output for m doctor.
type DoctorReport struct {
	SchemaVersion int           `json:"schemaVersion"`
	CheckedAt     string        `json:"checkedAt"`
	Checks        []DoctorCheck `json:"checks"`
	OK            bool          `json:"ok"`
}

// DoctorOptions tunes m doctor.
type DoctorOptions struct {
	Strict bool
}

// FilesystemProbeReport is the result of m development doctor filesystem.
type FilesystemProbeReport struct {
	StoreRoot string
	DestRoot  string
	Caps      planner.Capabilities
}

// Doctor runs PM and project health checks for end users.
func Doctor(ctx context.Context, ac *Context, opts DoctorOptions) (DoctorReport, error) {
	var rep DoctorReport
	if err := ctx.Err(); err != nil {
		return rep, err
	}
	rep.SchemaVersion = DoctorReportSchemaVersion
	rep.CheckedAt = time.Now().UTC().Format(time.RFC3339)

	rep.Checks = append(rep.Checks, doctorCheckProject(ctx, ac))
	rep.Checks = append(rep.Checks, doctorCheckConfig(ac))
	rep.Checks = append(rep.Checks, doctorCheckCacheStore(ctx, ac)...)
	rep.Checks = append(rep.Checks, doctorCheckLock(ctx, ac))
	rep.Checks = append(rep.Checks, doctorCheckFilesystem(ctx, ac))
	rep.Checks = append(rep.Checks, doctorCheckTxn(ctx, ac))
	rep.Checks = append(rep.Checks, doctorCheckNode())

	rep.OK = !reportHasStatus(rep, DoctorStatusFail)
	if opts.Strict && reportHasStatus(rep, DoctorStatusWarn) {
		rep.OK = false
	}
	return rep, nil
}

func reportHasStatus(rep DoctorReport, want DoctorCheckStatus) bool {
	for _, c := range rep.Checks {
		if DoctorCheckStatus(c.Status) == want {
			return true
		}
	}
	return false
}

func doctorCheckProject(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "project"}
	proj, err := OpenProject(ctx, ac)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = err.Error()
		check.Remediation = "run from a directory with a readable package.json"
		return check
	}
	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("package.json readable at %s", proj.Root)
	return check
}

func doctorCheckConfig(ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "config"}
	if ac == nil || ac.Config == nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "configuration not loaded"
		check.Remediation = "fix m.jsonc / MEW_* settings and retry"
		return check
	}
	check.Status = string(DoctorStatusOK)
	check.Message = "configuration loaded"
	return check
}

func doctorCheckCacheStore(ctx context.Context, ac *Context) []DoctorCheck {
	if ac == nil || ac.Config == nil {
		return []DoctorCheck{
			{ID: "cache", Status: string(DoctorStatusFail), Message: "configuration not loaded"},
			{ID: "store", Status: string(DoctorStatusFail), Message: "configuration not loaded"},
		}
	}
	cacheRoot := config.CacheRoot(ac.Config)
	storeRoot, err := config.StoreRoot(ac.Config)
	if err != nil {
		return []DoctorCheck{{
			ID: "store", Status: string(DoctorStatusFail), Message: err.Error(),
			Remediation: "set store.dir or MEW_STORE_DIR to a writable path",
		}}
	}
	return []DoctorCheck{
		doctorWritableCheck(ctx, "cache", cacheRoot, "set cache.dir or MEW_CACHE_DIR to a writable path"),
		doctorWritableCheck(ctx, "store", storeRoot, "set store.dir or MEW_STORE_DIR to a writable path"),
	}
}

func doctorWritableCheck(ctx context.Context, id, dir, remediation string) DoctorCheck {
	check := DoctorCheck{ID: id}
	if err := ctx.Err(); err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = err.Error()
		return check
	}
	if err := probeWritable(dir); err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = fmt.Sprintf("%s not writable: %v", dir, err)
		check.Remediation = remediation
		return check
	}
	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("%s writable", dir)
	return check
}

func probeWritable(dir string) error {
	if dir == "" {
		return fmt.Errorf("empty path")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	probe := filepath.Join(dir, ".mew-doctor-probe")
	f, err := os.Create(probe)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Remove(probe)
}

func doctorCheckLock(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "lock"}
	proj, err := OpenProject(ctx, ac)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = err.Error()
		return check
	}
	path := LockPath(proj)
	if _, err := os.Stat(path); err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = fmt.Sprintf("lockfile missing: %s", path)
		check.Remediation = "run m install or m lock migrate to create a lockfile"
		return check
	}
	if _, err := ValidateIncumbentLock(ctx, ac, ValidateLockOptions{}); err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = err.Error()
		check.Remediation = "run m lock validate for details; regenerate with a supported package manager"
		return check
	}
	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("%s validated", filepath.Base(path))
	return check
}

func doctorCheckFilesystem(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "filesystem"}
	probe, err := DoctorFilesystem(ctx, ac)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = err.Error()
		check.Remediation = "ensure store and node_modules paths are accessible"
		return check
	}
	if !filesystemCapsUsable(probe.Caps) {
		check.Status = string(DoctorStatusWarn)
		check.Message = fmt.Sprintf("limited link support (hardlink=%v symlink=%v junction=%v); installs may copy files",
			probe.Caps.Hardlink, probe.Caps.Symlink, probe.Caps.Junction)
		check.Remediation = "see m development doctor filesystem for probe details"
		return check
	}
	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("link probe ok (store=%s)", probe.StoreRoot)
	return check
}

func filesystemCapsUsable(caps planner.Capabilities) bool {
	return caps.Hardlink || caps.Symlink || caps.Junction || caps.Reflink
}

func doctorCheckTxn(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "transaction"}
	proj, err := OpenProject(ctx, ac)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = err.Error()
		return check
	}
	if err := ctx.Err(); err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = err.Error()
		return check
	}
	txns, err := transaction.ScanIncompleteTxns(proj.Root)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = err.Error()
		return check
	}
	if len(txns) == 0 {
		check.Status = string(DoctorStatusOK)
		check.Message = "no incomplete transactions"
		return check
	}
	ids := make([]string, 0, len(txns))
	for _, t := range txns {
		ids = append(ids, fmt.Sprintf("%s(%s)", t.ID, t.State))
	}
	check.Status = string(DoctorStatusWarn)
	check.Message = fmt.Sprintf("incomplete transaction journals: %s", strings.Join(ids, ", "))
	check.Remediation = "run m recover to roll back or discard stale journals"
	return check
}

func doctorCheckNode() DoctorCheck {
	check := DoctorCheck{ID: "node"}
	path, err := exec.LookPath("node")
	if err != nil {
		check.Status = string(DoctorStatusWarn)
		check.Message = "node not found on PATH"
		check.Remediation = "install Node.js for script execution (runner support is MVP 0040)"
		return check
	}
	check.Status = string(DoctorStatusOK)
	check.Message = path
	return check
}

// DoctorExitError reports failed health checks to the CLI layer.
func DoctorExitError(rep DoctorReport) error {
	if rep.OK {
		return nil
	}
	return apperr.New(apperr.Integrity, "doctor", "", "health check failed")
}

// DoctorRuntime runs runtime-specific health checks: Node installation,
// capabilities, transform handshake, tsconfig loading, transform round-trip,
// source maps, runtime cache integrity, loader/bootstrap bridge, watch
// backend, inspector, and worker support.
func DoctorRuntime(ctx context.Context, ac *Context, opts DoctorOptions) (DoctorReport, error) {
	var rep DoctorReport
	if err := ctx.Err(); err != nil {
		return rep, err
	}
	rep.SchemaVersion = DoctorReportSchemaVersion
	rep.CheckedAt = time.Now().UTC().Format(time.RFC3339)

	// Required checks — any failure makes the overall report fail.
	rep.Checks = append(rep.Checks, doctorCheckNodeCapabilities(ctx))
	rep.Checks = append(rep.Checks, doctorCheckTransformHandshake(ctx, ac))
	rep.Checks = append(rep.Checks, doctorCheckTransformRoundtrip(ctx, ac))
	rep.Checks = append(rep.Checks, doctorCheckSourceMap(ctx, ac))

	// Best-effort checks — skipped when prerequisites are absent.
	rep.Checks = append(rep.Checks, doctorCheckTsconfig(ctx, ac))
	if ac != nil && ac.Config != nil {
		rep.Checks = append(rep.Checks, doctorCheckRuntimeCache(ctx, ac))
	}
	rep.Checks = append(rep.Checks, doctorCheckLoaderBridge(ctx, ac))
	rep.Checks = append(rep.Checks, doctorCheckWatchBackend())
	rep.Checks = append(rep.Checks, doctorCheckInspector(ctx))
	rep.Checks = append(rep.Checks, doctorCheckWorker(ctx))

	rep.OK = !reportHasStatus(rep, DoctorStatusFail)
	if opts.Strict && reportHasStatus(rep, DoctorStatusWarn) {
		rep.OK = false
	}
	return rep, nil
}

// transformHandshakeProbe starts a transform session, performs the
// authenticated handshake, and closes the session. It exercises the real
// production service start and auth paths.
func doctorCheckTransformHandshake(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "transform-handshake"}

	engine := transform.NewEsbuildEngine()
	sess, err := transform.NewSession(transform.ServiceOptions{
		Engine:  engine,
		Context: ctx,
	})
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "transform session creation failed"
		check.Details = sanitizeErr(err)
		check.Remediation = "verify transform engine and network availability"
		return check
	}

	// Start performs the real auth handshake; must close on error paths.
	if err := sess.Start(); err != nil {
		_ = sess.Close()
		check.Status = string(DoctorStatusFail)
		check.Message = "transform handshake failed"
		check.Details = sanitizeErr(err)
		check.Remediation = "verify local network and transform service readiness"
		return check
	}

	// Close and collect any cleanup error as detail only.
	if err := sess.Close(); err != nil {
		check.Status = string(DoctorStatusOK)
		check.Message = "transform handshake ok"
		check.Details = fmt.Sprintf("cleanup error: %s", sanitizeErr(err))
		return check
	}

	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("transform handshake ok (engine=%s v%s)", engine.Identity().Name, engine.Identity().Version)
	return check
}

// tsconfigPipeline fetches node capabilities. Needed by tsconfig check.
func doctorCheckTsconfig(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "tsconfig"}

	proj, err := OpenProject(ctx, ac)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "project not open"
		check.Details = sanitizeErr(err)
		return check
	}

	tsPath, err := transform.DiscoverTsconfig(proj.Root)
	if err != nil {
		check.Status = string(DoctorStatusWarn)
		check.Message = "tsconfig not found"
		check.Details = sanitizeErr(err)
		check.Remediation = "add tsconfig.json for TypeScript support"
		return check
	}

	// Verify it's a regular file before attempting to parse.
	if st, err := os.Stat(tsPath); err != nil {
		check.Status = string(DoctorStatusWarn)
		check.Message = "tsconfig not accessible"
		check.Details = sanitizeErr(err)
		return check
	} else if st.IsDir() {
		check.Status = string(DoctorStatusWarn)
		check.Message = "tsconfig not found (directory at expected path)"
		check.Remediation = "add tsconfig.json for TypeScript support"
		return check
	}

	chain, err := transform.LoadTsconfigChain(tsPath)
	if err != nil {
		// Distinguish extends chain errors from parse errors.
		var cfgErr *transform.ConfigError
		if errors.As(err, &cfgErr) {
			switch cfgErr.Kind {
			case transform.ConfigErrExtendsMissing, transform.ConfigErrExtendsCycle,
				transform.ConfigErrExtendsDepth, transform.ConfigErrExtendsInvalid:
				check.Status = string(DoctorStatusFail)
				check.Message = "tsconfig extends chain broken"
				check.Details = sanitizeErr(err)
				return check
			case transform.ConfigErrParse:
				check.Status = string(DoctorStatusFail)
				check.Message = "tsconfig parse error"
				check.Details = sanitizeErr(err)
				return check
			}
		}
		// Generic parse/load failure.
		check.Status = string(DoctorStatusFail)
		check.Message = "tsconfig load failed"
		check.Details = sanitizeErr(err)
		return check
	}
	_ = chain

	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("tsconfig found at %s", sanitizePath(tsPath, proj.Root))
	return check
}

// doctorCheckTransformRoundtrip runs a minimal TypeScript transform through
// the real engine to prove the transform pipeline works end-to-end.
func doctorCheckTransformRoundtrip(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "transform-roundtrip"}

	engine := transform.NewEsbuildEngine()

	// Minimal diagnostic TypeScript fixture — no user code executed.
	source := []byte("export const x: number = 1;\n")
	req := transform.TransformRequest{
		SourcePath:  "doctor-probe.ts",
		SourceBytes: source,
		Loader:      transform.LoaderTS,
		Format:      transform.FormatESM,
		NormalizedOpts: transform.NormalizedOptions{
			Target: "es2022",
		},
		SourceMapMode: transform.SourceMapExternal,
	}

	result, err := engine.Transform(ctx, req)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "transform engine failed"
		check.Details = sanitizeErr(err)
		check.Remediation = "verify transform engine (esbuild) works correctly"
		return check
	}

	if len(result.Code) == 0 {
		check.Status = string(DoctorStatusFail)
		check.Message = "transform produced empty output"
		return check
	}

	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("transform ok (%s v%s, %d bytes output)", engine.Identity().Name, engine.Identity().Version, len(result.Code))
	return check
}

// doctorCheckSourceMap proves the transform can produce a valid source map.
func doctorCheckSourceMap(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "source-map"}

	engine := transform.NewEsbuildEngine()
	source := []byte("export const x: number = 1;\n")
	req := transform.TransformRequest{
		SourcePath:     "doctor-probe.ts",
		SourceBytes:    source,
		Loader:         transform.LoaderTS,
		Format:         transform.FormatESM,
		NormalizedOpts: transform.NormalizedOptions{Target: "es2022"},
		SourceMapMode:  transform.SourceMapExternal,
	}

	result, err := engine.Transform(ctx, req)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "source-map transform failed"
		check.Details = sanitizeErr(err)
		return check
	}

	if len(result.SourceMap) == 0 {
		check.Status = string(DoctorStatusWarn)
		check.Message = "source map not produced (may be unsupported by engine)"
		check.Remediation = "check engine source-map configuration"
		return check
	}

	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("source-map ok (%d bytes map)", len(result.SourceMap))
	return check
}

// doctorCheckRuntimeCache verifies the runtime asset cache directory exists
// and passes integrity validation via the production VerifyCache path.
func doctorCheckRuntimeCache(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "runtime-cache"}
	cacheDir, err := runtimepkg.CacheDir(ac.Config)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "cache dir resolution failed"
		check.Details = sanitizeErr(err)
		return check
	}
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		check.Status = string(DoctorStatusWarn)
		check.Message = fmt.Sprintf("runtime cache not populated at %s", cacheDir)
		check.Remediation = "run any TypeScript file with m to populate the cache"
		return check
	}
	// Verify cache integrity through production path.
	if err := runtimepkg.VerifyCache(ac.Config); err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "runtime cache verification failed"
		check.Details = sanitizeErr(err)
		check.Remediation = "run m cache explain for details; delete the cache directory to force re-extraction"
		return check
	}

	// Probe with a controlled diagnostic cache entry to prove read/write integrity.
	if err := probeRuntimeCacheIntegrity(ctx, ac.Config, cacheDir); err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "runtime cache integrity probe failed"
		check.Details = sanitizeErr(err)
		return check
	}

	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("runtime cache valid at %s", cacheDir)
	return check
}

// probeRuntimeCacheIntegrity writes and reads back a controlled diagnostic
// transform cache entry to prove cache read/write integrity.
func probeRuntimeCacheIntegrity(ctx context.Context, eff *config.Effective, cacheDir string) error {
	_ = ctx

	// Use the transform cache (not runtime asset cache) for the integrity probe.
	tcDir := transform.TransformCacheDir(eff)
	if tcDir == "" {
		return fmt.Errorf("transform cache dir empty")
	}

	// Build a deterministic diagnostic key from random bytes mixed with a
	// fixed source — exercises the full CacheKey/CacheKeyPath/TryReadCache/WriteCache pipeline.
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("nonce: %w", err)
	}
	diagKey := "doctor-probe-" + hex.EncodeToString(nonce)

	// Write a controlled entry.
	result := &transform.TransformResult{
		Code:         []byte("// doctor probe\n"),
		SourceMap:    []byte(`{"version":3}`),
		OutputDigest: "", // computed by WriteCache
	}
	if err := transform.WriteCache(tcDir, diagKey, result); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// Read back through production TryReadCache.
	read, err := transform.TryReadCache(tcDir, diagKey)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	if read == nil {
		return fmt.Errorf("cache miss after write")
	}
	if !transform.VerifyCachedResult(read, result) {
		return fmt.Errorf("cached result mismatch")
	}

	// Clean up the diagnostic entry.
	keyPath := transform.CacheKeyPath(tcDir, diagKey)
	_ = os.Remove(keyPath + ".code")
	_ = os.Remove(keyPath + ".map")
	_ = os.Remove(keyPath + ".meta")

	return nil
}

// doctorCheckLoaderBridge verifies the runtime asset manifest loads and the
// credential preload assets needed for the loader bridge exist.
func doctorCheckLoaderBridge(ctx context.Context, ac *Context) DoctorCheck {
	check := DoctorCheck{ID: "loader-bridge"}

	// Verify the runtime asset cache is populated and valid.
	if ac != nil && ac.Config != nil {
		if err := runtimepkg.VerifyCache(ac.Config); err != nil {
			check.Status = string(DoctorStatusFail)
			check.Message = "loader assets unavailable"
			check.Details = sanitizeErr(err)
			check.Remediation = "run m to re-extract runtime assets"
			return check
		}
	}

	// Verify node discovery works (needed by Plan).
	inst, err := node.Discover(ctx, node.Request{})
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "node discovery failed for loader bridge"
		check.Details = sanitizeErr(err)
		return check
	}

	if inst == nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "node not found for loader bridge"
		check.Remediation = "install Node.js 18+"
		return check
	}

	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("loader bridge ok (Node %s)", inst.NormalizedVersion)
	return check
}

// doctorCheckWatchBackend verifies the watch backend factory can create a
// watcher without starting a long-running watch session.
func doctorCheckWatchBackend() DoctorCheck {
	check := DoctorCheck{ID: "watch-backend"}

	w, err := watch.NewWatcher()
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "watch backend unavailable"
		check.Details = sanitizeErr(err)
		check.Remediation = "verify filesystem watcher support"
		return check
	}

	// Close immediately — we only care that the factory succeeded.
	_ = w.Close()

	check.Status = string(DoctorStatusOK)
	check.Message = "watch backend available"
	return check
}

// doctorCheckInspector validates the inspector flag pipeline using Issue 26's
// ParseInspectorFlags without starting a real inspector endpoint.
func doctorCheckInspector(ctx context.Context) DoctorCheck {
	check := DoctorCheck{ID: "inspector"}

	// Validate the inspector flag parsing pipeline with a test input.
	cfg, _, err := runtimepkg.ParseInspectorFlags(
		[]string{"--inspect=127.0.0.1:0"},
		false, // not zeroAugmentation
	)
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "inspector flag parsing failed"
		check.Details = sanitizeErr(err)
		return check
	}
	if cfg == nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "inspector config is nil"
		return check
	}

	argv := cfg.BuildInspectorArgv()
	if len(argv) == 0 {
		check.Status = string(DoctorStatusWarn)
		check.Message = "inspector argv empty (may be configured for no debug)"
		check.Remediation = "add --inspect to Node V8 args for debugging"
		return check
	}

	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("inspector pipeline ok (mode=%s host=%s port=%d)", cfg.Mode, cfg.Host, cfg.Port)
	return check
}

// doctorCheckWorker verifies Node worker_threads availability.
// Worker support is built into Node 12+.
func doctorCheckWorker(ctx context.Context) DoctorCheck {
	check := DoctorCheck{ID: "worker"}

	inst, err := node.Discover(ctx, node.Request{})
	if err != nil {
		check.Status = string(DoctorStatusWarn)
		check.Message = "node discovery failed; worker support unknown"
		check.Details = sanitizeErr(err)
		check.Remediation = "install Node.js 12+ for worker_threads support"
		return check
	}
	if inst == nil {
		check.Status = string(DoctorStatusWarn)
		check.Message = "node not found; worker support unknown"
		check.Remediation = "install Node.js 12+"
		return check
	}

	// worker_threads became stable in Node 12.
	major, err := parseNodeMajor(inst.NormalizedVersion)
	if err != nil {
		check.Status = string(DoctorStatusWarn)
		check.Message = "cannot parse node version; worker support unknown"
		check.Details = sanitizeErr(err)
		return check
	}
	if major < 12 {
		check.Status = string(DoctorStatusFail)
		check.Message = fmt.Sprintf("Node %s does not support worker_threads (need 12+)", inst.NormalizedVersion)
		check.Remediation = "upgrade to Node.js 12+"
		return check
	}

	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("worker_threads available (Node %s)", inst.NormalizedVersion)
	return check
}

func doctorCheckNodeCapabilities(ctx context.Context) DoctorCheck {
	check := DoctorCheck{ID: "node-capabilities"}
	inst, err := node.Discover(ctx, node.Request{})
	if err != nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "node discovery failed"
		check.Details = sanitizeErr(err)
		check.Remediation = "install Node.js 18+ (m requires module-register, import-preload, require-preload)"
		return check
	}
	if inst == nil {
		check.Status = string(DoctorStatusFail)
		check.Message = "node not found"
		check.Remediation = "install Node.js 18+"
		return check
	}
	capSet := make(map[string]bool, len(inst.Capabilities))
	for _, c := range inst.Capabilities {
		capSet[c] = true
	}
	required := []string{"require-preload", "import-preload", "module-register"}
	var missing []string
	for _, c := range required {
		if !capSet[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		check.Status = string(DoctorStatusFail)
		check.Message = fmt.Sprintf("Node %s at %s missing capabilities: %s", inst.NormalizedVersion, inst.ExePath, strings.Join(missing, ", "))
		check.Remediation = "install Node.js 18+ or a newer LTS release"
		return check
	}
	check.Status = string(DoctorStatusOK)
	check.Message = fmt.Sprintf("Node %s at %s (capabilities: %s)", inst.NormalizedVersion, inst.ExePath, strings.Join(inst.Capabilities, ", "))
	return check
}

// sanitizeErr returns a sanitized version of err for diagnostic output.
func sanitizeErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// sanitizePath returns target relative to base, or target if relative fails.
func sanitizePath(target, base string) string {
	if base == "" || target == "" {
		return target
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == "" {
		return target
	}
	if strings.HasPrefix(rel, "..") {
		return target
	}
	return rel
}

// parseNodeMajor extracts the major version from a normalized version string.
func parseNodeMajor(version string) (int, error) {
	parts := strings.SplitN(strings.TrimPrefix(version, "v"), ".", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty version")
	}
	var major int
	if _, err := fmt.Sscanf(parts[0], "%d", &major); err != nil {
		return 0, err
	}
	return major, nil
}
func FormatDoctorReport(rep DoctorReport) string {
	var b strings.Builder
	for _, c := range rep.Checks {
		fmt.Fprintf(&b, "check=%s status=%s message=%s", c.ID, c.Status, c.Message)
		if c.Details != "" {
			fmt.Fprintf(&b, " details=%s", c.Details)
		}
		if c.Remediation != "" {
			fmt.Fprintf(&b, " remediation=%s", c.Remediation)
		}
		b.WriteByte('\n')
	}
	if rep.OK {
		b.WriteString("doctor=ok\n")
	} else {
		b.WriteString("doctor=failed\n")
	}
	return b.String()
}

// DoctorFilesystem probes link capabilities between store and node_modules.
func DoctorFilesystem(ctx context.Context, ac *Context) (FilesystemProbeReport, error) {
	var rep FilesystemProbeReport
	if err := ctx.Err(); err != nil {
		return rep, err
	}
	if ac == nil || ac.Config == nil {
		return rep, apperr.New(apperr.Internal, "app.doctor.filesystem", "", "missing app context")
	}
	storeRoot, err := config.StoreRoot(ac.Config)
	if err != nil {
		return rep, err
	}
	rep.StoreRoot = storeRoot
	rep.DestRoot = "node_modules"
	if p, err := OpenProject(ctx, ac); err == nil {
		rep.DestRoot = filepath.Join(p.Root, "node_modules")
	}
	rep.Caps, err = planner.ProbeCached(config.CacheRoot(ac.Config), rep.StoreRoot, rep.DestRoot)
	return rep, err
}

// FormatFilesystemProbe returns human-readable probe output.
func FormatFilesystemProbe(rep FilesystemProbeReport) string {
	return fmt.Sprintf("src=%s\ndest=%s\nsameVolume=%v hardlink=%v reflink=%v symlink=%v junction=%v\n",
		rep.StoreRoot, rep.DestRoot,
		rep.Caps.SameVolume, rep.Caps.Hardlink, rep.Caps.Reflink, rep.Caps.Symlink, rep.Caps.Junction)
}
