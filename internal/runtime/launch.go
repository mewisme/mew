package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/node"
	"github.com/mewisme/mew/internal/process"
	"github.com/mewisme/mew/internal/project"
	"github.com/mewisme/mew/internal/runtime/assets"
	"github.com/mewisme/mew/internal/trace"
)

// Plan resolves a LaunchRequest into a concrete LaunchPlan.
func Plan(ctx context.Context, req LaunchRequest, eff *config.Effective) (*LaunchPlan, error) {
	if req.Entrypoint == "" {
		return nil, apperr.New(apperr.RuntimeEntrypoint, "runtime.plan", "", "empty entrypoint")
	}

	// Early NODE_OPTIONS validation on the host environment, before any
	// attempt to invoke Node (including version detection). A malformed
	// NODE_OPTIONS value can cause node --version to fail with a cryptic
	// error; we validate first to produce a clear rejection.
	// The full final-environment validation runs in Launch() after all
	// overlays and plan changes have been composed.
	if req.AugmentationMode != AugmentNone {
		if err := ValidateNodeEnv(os.Environ()); err != nil {
			return nil, err
		}
	}

	nodeInst, err := node.Discover(ctx, node.Request{
		WorkingDir:        req.WorkingDir,
		ExplicitCandidate: "",
	})
	if err != nil {
		return nil, err
	}

	// Enforce required Node capabilities before building argv.
	if req.AugmentationMode != AugmentNone {
		if err := enforceCapabilities(nodeInst, req.Entrypoint); err != nil {
			return nil, err
		}
	}

	plan := &LaunchPlan{
		NodeExe:          nodeInst.ExePath,
		NodeVersion:      nodeInst.NormalizedVersion,
		NodeCapabilities: nodeInst.Capabilities,
		Entrypoint:       req.Entrypoint,
		AppArgs:          append([]string(nil), req.AppArgs...),
		ZeroAugmentation: req.AugmentationMode == AugmentNone,
		EnableSourceMaps: hasCap(nodeInst.Capabilities, "source-maps"),
	}

	// Resolve and validate user-specified custom ESM loaders.
	// Works in both augmented and --node modes. Loader paths are converted
	// to file:// URLs and passed via MEW_USER_LOADERS env var, consumed by
	// credential-grabber.cjs (augmented) or loader-register.mjs (--node).
	if len(req.Loaders) > 0 {
		for _, p := range req.Loaders {
			abs := p
			if !filepath.IsAbs(p) {
				abs = filepath.Join(req.WorkingDir, p)
			}
			// Validate existence before launch so the user gets a
			// deterministic bootstrap error for clearly invalid input.
			if _, err := os.Stat(abs); err != nil {
				if os.IsNotExist(err) {
					return nil, apperr.New(apperr.RuntimeEntrypoint, "runtime.plan", abs,
						fmt.Sprintf("loader not found: %s", abs))
				}
				return nil, apperr.Wrap(apperr.RuntimeEntrypoint, "runtime.plan", abs, err)
			}
			u := fileURL(abs)
			plan.CustomLoaders = append(plan.CustomLoaders, PreloadAsset{
				Path:       u,
				ModuleType: "esm",
			})
		}
		// Pass user loader URLs via env var. Consumed and deleted by the
		// registration shim (credential-grabber.cjs or loader-register.mjs).
		urls := make([]string, len(plan.CustomLoaders))
		for i, cl := range plan.CustomLoaders {
			urls[i] = cl.Path
		}
		plan.EnvChanges = append(plan.EnvChanges,
			"MEW_USER_LOADERS="+strings.Join(urls, "\n"))
	}

	// Apply launch contribution from app-level orchestrator.
	if req.Contribution != nil {
		plan.CleanupHook = req.Contribution.CleanupHook
		plan.PreloadAssets = append(plan.PreloadAssets, req.Contribution.ExtraPreloads...)
		plan.EnvChanges = append(plan.EnvChanges, req.Contribution.ExtraEnv...)
	}

	// Compute localStorage persistence path from project identity.
	// When project root is not found (standalone scripts), localStorage
	// stays in-memory-only — no cross-invocation persistence.
	if sp := storagePath(req.WorkingDir, eff); sp != "" {
		plan.EnvChanges = append(plan.EnvChanges, "MEW_LOCAL_STORAGE_PATH="+sp)
	}

	// Extract assets when augmentation is active, or when --node mode has
	// custom loaders (needs loader-register.mjs shim).
	needsAssets := req.AugmentationMode != AugmentNone || (len(req.Loaders) > 0 && req.AugmentationMode == AugmentNone)
	if needsAssets {
		// Verify cached assets before use; corrupt entries are deleted.
		// VerifyCache only returns fatal errors (permission, I/O, manifest).
		// Missing or corrupt files are deleted so EnsureAssets re-extracts them.
		if err := VerifyCache(eff); err != nil {
			return nil, err
		}
		assetPaths, err := EnsureAssets(eff)
		if err != nil {
			return nil, err
		}
		m, err := assets.LoadManifest()
		if err != nil {
			return nil, err
		}
		if req.AugmentationMode != AugmentNone {
			// Full augmentation: inject credential grabber and preloads.
			for _, entry := range m.AssetsSorted() {
				if !entry.Role.Injected() {
					continue
				}
				// Loader registration is now handled by credential-grabber.cjs,
				// which calls module.register() with credential data inline.
				// The loader-register asset is no longer injected in augmented mode.
				if entry.Role == assets.RoleLoaderRegistration {
					continue
				}
				p, ok := assetPaths[entry.Name]
				if !ok {
					continue
				}
				pa := PreloadAsset{
					Path:       p,
					ModuleType: entry.ModuleType,
				}
				if entry.Role == assets.RoleCredentialGrabber {
					plan.CredentialPreload = &pa
				} else {
					plan.PreloadAssets = append(plan.PreloadAssets, pa)
				}
			}
		} else {
			// --node mode with custom loaders: only need loader-register.mjs.
			for _, entry := range m.AssetsSorted() {
				if entry.Role == assets.RoleLoaderRegistration {
					if p, ok := assetPaths[entry.Name]; ok {
						plan.LoaderShimPath = p
					}
					break
				}
			}
		}
	}

	// Normalize inspector flags before building argv.
	inspCfg, nonInspectorV8, inspErr := ParseInspectorFlags(req.NodeV8Args, plan.ZeroAugmentation)
	if inspErr != nil {
		return nil, inspErr
	}
	plan.Inspector = inspCfg

	// Merge normalized inspector flags with remaining V8 args.
	// Inspector flags sit in the same position as other user V8 flags
	// (after credential grabber, before preloads).
	v8Args := nonInspectorV8
	if plan.Inspector != nil {
		v8Args = append(v8Args, plan.Inspector.BuildInspectorArgv()...)
	}

	plan.NodeArgv = BuildArgv(plan, v8Args)
	return plan, nil
}

// hasCap reports whether caps contains the named capability.
func hasCap(caps []string, name string) bool {
	for _, c := range caps {
		if c == name {
			return true
		}
	}
	return false
}

// enforceCapabilities verifies the Node installation supports required features.
func enforceCapabilities(inst *node.Installation, entrypoint string) error {
	capSet := make(map[string]bool, len(inst.Capabilities))
	for _, c := range inst.Capabilities {
		capSet[c] = true
	}
	required := []string{"require-preload", "import-preload"}
	// module-register is required for all entrypoints when augmentation
	// is active: the loader's resolve hook handles tsconfig paths and
	// .js→.ts extension substitution regardless of entrypoint type.
	required = append(required, "module-register")
	for _, c := range required {
		if !capSet[c] {
			return apperr.New(apperr.RuntimeNodeUnsupported, "runtime.plan", inst.NormalizedVersion,
				fmt.Sprintf("Node %s lacks required capability %q", inst.NormalizedVersion, c))
		}
	}
	return nil
}

// BuildArgv constructs the full Node argument vector.
// Order: node exe -> credential grabber --require -> user V8 flags
//
//	-> custom loader --import flags -> Mew preload flags -> entrypoint -> app args
//
// The credential grabber is placed before user V8 flags so it captures and
// strips transform credentials from process.env before any user --require,
// user --import, or NODE_OPTIONS preload runs.
func BuildArgv(plan *LaunchPlan, v8Args []string) []string {
	shimSlots := 0
	if plan.LoaderShimPath != "" {
		shimSlots = 2 // --import <loader-register.mjs>
	}
	credSlots := 0
	if !plan.ZeroAugmentation && plan.CredentialPreload != nil {
		credSlots = 2 // --require <path>
	}
	argv := make([]string, 0, 4+credSlots+len(v8Args)+shimSlots+len(plan.PreloadAssets)*2+1+len(plan.AppArgs))
	argv = append(argv, plan.NodeExe)

	// Enable Node source-map support for stack trace mapping when available
	// (Node >= 20.6). This tells Node to read sourceMappingURL from module
	// source and resolve .map files for error stack traces. It has no effect
	// when no source maps are present in the module graph.
	if plan.EnableSourceMaps {
		argv = append(argv, "--enable-source-maps")
	}

	// Credential grabber runs FIRST — before any user preload.
	// Node processes --require from left to right; this must be the
	// first preload so credentials are stripped before user code.
	if !plan.ZeroAugmentation && plan.CredentialPreload != nil {
		argv = append(argv, "--require", plan.CredentialPreload.Path)
	}

	// user V8/Node flags
	argv = append(argv, v8Args...)

	// --node mode with custom loaders: inject loader-register.mjs as --import.
	// This minimal shim reads MEW_USER_LOADERS from the env and registers
	// each user loader via module.register(). No credential handling, no
	// ts-loader — just the user's loaders on stock Node.
	if plan.LoaderShimPath != "" {
		argv = append(argv, "--import", plan.LoaderShimPath)
	}

	if !plan.ZeroAugmentation {
		// Custom loaders are now registered by credential-grabber.cjs via
		// module.register(), not injected as bare --import. The env var
		// MEW_USER_LOADERS carries the file:// URLs; credential-grabber
		// registers them in reverse order so the first --loader is the
		// outermost hook. ts-loader is registered last (innermost, fills gaps).

		for _, pa := range plan.PreloadAssets {
			assetPath := pa.Path
			switch pa.ModuleType {
			case "cjs":
				// --require works with native Windows paths; no URL conversion.
				argv = append(argv, "--require", assetPath)
			case "esm":
				// --import needs file:// URLs on Windows for ESM loader.
				if runtime.GOOS == "windows" && filepath.IsAbs(assetPath) {
					assetPath = fileURL(assetPath)
				}
				argv = append(argv, "--import", assetPath)
			}
		}
	}

	argv = append(argv, plan.Entrypoint)
	argv = append(argv, plan.AppArgs...)
	return argv
}

// fileURL converts an absolute Windows path to a file:// URL.
func fileURL(p string) string {
	// Use url.URL for standards-compliant file:// URL construction.
	// Handles drive letters, spaces, Unicode, and special chars correctly.
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	return u.String()
}

// PlanAndLaunch resolves, launches, and guarantees cleanup on every exit path.
//
// When req.Contribution is set and Plan fails, the contribution's cleanup hook
// runs before PlanAndLaunch returns. When Plan succeeds but Launch fails (or
// succeeds), plan.CleanupHook runs after Launch returns.
//
// Cleanup ownership moves from req.Contribution (on Plan failure) to
// plan.CleanupHook (on Plan success). The same underlying Session.Close is
// idempotent, so there is no risk of double-close even if both paths were
// somehow reached.
func PlanAndLaunch(ctx context.Context, req LaunchRequest, eff *config.Effective) error {
	trace.Emit(ctx, trace.CatLifecycle, trace.TypePlanStart, trace.LifecycleData{
		Entrypoint: req.Entrypoint,
	})

	planStart := time.Now()
	plan, planErr := Plan(ctx, req, eff)
	if planErr != nil {
		trace.Emit(ctx, trace.CatLifecycle, trace.TypePlanError, trace.LifecycleData{
			Entrypoint:   req.Entrypoint,
			ErrorCode:    string(apperr.CodeOf(planErr)),
			ErrorMessage: trace.RedactError(planErr),
			DurationMs:   time.Since(planStart).Milliseconds(),
		})
		// Plan failed: contribution still owns the session; clean it up.
		if req.Contribution != nil && req.Contribution.CleanupHook != nil {
			_ = req.Contribution.CleanupHook()
		}
		return planErr
	}

	planData := trace.LifecycleData{
		Entrypoint:  plan.Entrypoint,
		NodeVersion: plan.NodeVersion,
		Augmented:   !plan.ZeroAugmentation,
		DurationMs:  time.Since(planStart).Milliseconds(),
	}
	if plan.Inspector != nil {
		planData.InspectorMode = plan.Inspector.Mode.String()
		planData.InspectorHost = plan.Inspector.Host
		planData.InspectorPort = plan.Inspector.Port
	}
	trace.Emit(ctx, trace.CatLifecycle, trace.TypePlanComplete, planData)

	launchStartData := trace.LifecycleData{Entrypoint: plan.Entrypoint}
	if plan.Inspector != nil {
		launchStartData.InspectorMode = plan.Inspector.Mode.String()
		launchStartData.InspectorHost = plan.Inspector.Host
		launchStartData.InspectorPort = plan.Inspector.Port
	}
	trace.Emit(ctx, trace.CatLifecycle, trace.TypeLaunchStart, launchStartData)

	launchStart := time.Now()
	launchErr := Launch(ctx, plan, req)
	launchDur := time.Since(launchStart).Milliseconds()

	// Attach inspector fields to launch exit/error events.
	inspectorFields := func(d trace.LifecycleData) trace.LifecycleData {
		if plan.Inspector != nil {
			d.InspectorMode = plan.Inspector.Mode.String()
			d.InspectorHost = plan.Inspector.Host
			d.InspectorPort = plan.Inspector.Port
		}
		return d
	}

	if launchErr != nil {
		var exitStatus *apperr.ExitStatus
		if errors.As(launchErr, &exitStatus) && exitStatus != nil {
			code := exitStatus.Code
			trace.Emit(ctx, trace.CatLifecycle, trace.TypeLaunchExit, inspectorFields(trace.LifecycleData{
				Entrypoint: plan.Entrypoint,
				DurationMs: launchDur,
				ExitCode:   &code,
			}))
		} else {
			trace.Emit(ctx, trace.CatLifecycle, trace.TypeLaunchError, inspectorFields(trace.LifecycleData{
				Entrypoint:   plan.Entrypoint,
				DurationMs:   launchDur,
				ErrorCode:    string(apperr.CodeOf(launchErr)),
				ErrorMessage: trace.RedactError(launchErr),
			}))
		}
	} else {
		code := 0
		trace.Emit(ctx, trace.CatLifecycle, trace.TypeLaunchExit, inspectorFields(trace.LifecycleData{
			Entrypoint: plan.Entrypoint,
			DurationMs: launchDur,
			ExitCode:   &code,
		}))
	}

	trace.Emit(ctx, trace.CatLifecycle, trace.TypeCleanupStart, nil)
	var cleanupErr error
	if plan.CleanupHook != nil {
		cleanupErr = plan.CleanupHook()
	}
	if cleanupErr != nil {
		trace.Emit(ctx, trace.CatLifecycle, trace.TypeCleanupError, trace.LifecycleData{
			ErrorCode:    string(apperr.CodeOf(cleanupErr)),
			ErrorMessage: trace.RedactError(cleanupErr),
		})
	} else {
		trace.Emit(ctx, trace.CatLifecycle, trace.TypeCleanupDone, nil)
	}

	return MergeCleanupError(launchErr, cleanupErr)
}

// MergeCleanupError merges launch and cleanup errors preserving primary type.
// When launch succeeds and cleanup fails: returns cleanup error.
// When both fail: preserves launch as primary, attaches cleanup.
// Child exit codes, cancellation, and timeout classification are preserved.
func MergeCleanupError(launchErr, cleanupErr error) error {
	return apperr.JoinCleanup(launchErr, cleanupErr)
}
func Launch(ctx context.Context, plan *LaunchPlan, req LaunchRequest) error {
	if plan == nil {
		return apperr.New(apperr.RuntimeInvocation, "runtime.launch", "", "nil plan")
	}

	childEnv := buildEnv(req.EnvOverlay, plan.EnvChanges)

	// Reject unsafe NODE_OPTIONS that would execute before credential isolation.
	if !plan.ZeroAugmentation {
		if err := ValidateNodeEnv(childEnv); err != nil {
			return err
		}
	}

	supervisor := process.NewExecSupervisor()
	spec := process.Spec{
		Path:   plan.NodeArgv[0],
		Args:   plan.NodeArgv[1:],
		Dir:    req.WorkingDir,
		Env:    childEnv,
		Stdin:  req.Stdio.Stdin,
		Stdout: req.Stdio.Stdout,
		Stderr: req.Stdio.Stderr,
	}

	h, err := supervisor.Start(ctx, spec)
	if err != nil {
		return apperr.Wrap(apperr.RuntimeNodeStart, "runtime.launch", plan.Entrypoint, err)
	}

	if err := supervisor.Wait(ctx, h); err != nil {
		var exitErr *process.ExitError
		if errors.As(err, &exitErr) && exitErr != nil {
			return &apperr.ExitStatus{Code: exitErr.Code, Err: err}
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return apperr.Wrap(apperr.Cancelled, "runtime.launch", plan.Entrypoint, context.Canceled)
		}
		return apperr.Wrap(apperr.RuntimeInvocation, "runtime.launch", plan.Entrypoint, err)
	}
	return nil
}

// nodeOptionsUnsafe lists NODE_OPTIONS flags that can execute user-controlled
// code before the Node entrypoint. Any flag in this set — whether bare, with
// an attached value (--flag=value), or consuming the next token — is rejected
// in augmented launches.
//
// Review this set when Node adds new executable startup flags to NODE_OPTIONS.
var nodeOptionsUnsafe = map[string]bool{
	"--require":             true,
	"--import":              true,
	"--loader":              true,
	"--experimental-loader": true,
}

// ValidateNodeEnv rejects NODE_OPTIONS values that contain startup flags
// capable of executing user code before Mew's credential isolation boundary.
//
// Blocked flags: --require, -r, --import, --loader, --experimental-loader.
// Value forms like --flag=value and -rvalue are detected. Harmless options
// whose value happens to contain a blocked flag name (e.g.
// --title=my--require-app) are not rejected.
//
// Fail-closed: malformed or ambiguous input that could conceal an executable
// preload is rejected.
func ValidateNodeEnv(env []string) error {
	for _, kv := range env {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		// Case-insensitive key for Windows; Node only honors the exact
		// spelling on Unix, but validating both is harmless.
		if !strings.EqualFold(kv[:eq], "NODE_OPTIONS") {
			continue
		}
		val := kv[eq+1:]
		if unsafe, detail := validateNodeOptions(val); unsafe {
			return apperr.New(apperr.Usage, "runtime.launch", "",
				"NODE_OPTIONS contains "+detail+", which would execute before credential isolation; pass these flags as CLI arguments instead")
		}
	}
	return nil
}

// validateNodeOptions tokenizes a NODE_OPTIONS value and reports whether it
// contains any unsafe startup-execution flag.
//
// Tokenization matches Node's own whitespace-split semantics: no shell-style
// quoting or escaping is recognised.
func validateNodeOptions(val string) (unsafe bool, detail string) {
	toks := tokenizeNodeOptions(val)

	// Was the previous token an unsafe flag that consumes the next
	// whitespace-separated token as its value?
	var needValue bool
	var prevFlag string

	for _, tok := range toks {
		if needValue {
			return true, prevFlag + " <value>"
		}
		needValue = false

		if strings.HasPrefix(tok, "--") {
			name := tok
			if idx := strings.IndexByte(tok, '='); idx >= 0 {
				name = tok[:idx]
			}
			if nodeOptionsUnsafe[name] {
				if strings.IndexByte(tok, '=') >= 0 {
					return true, name + "=<value>"
				}
				prevFlag = name
				needValue = true
			}
		} else if strings.HasPrefix(tok, "-") && !strings.HasPrefix(tok, "--") {
			// Short flags. -r (alias for --require) takes a value.
			// -rFOO form attaches the value directly.
			if len(tok) >= 2 && tok[1] == 'r' {
				if len(tok) > 2 {
					return true, "-r <value>"
				}
				prevFlag = "-r"
				needValue = true
			}
		}
	}

	if needValue {
		return true, prevFlag + " <value>"
	}
	return false, ""
}

// tokenizeNodeOptions splits a NODE_OPTIONS value on whitespace, matching
// Node's own tokenization. No quoting or escaping is applied.
func tokenizeNodeOptions(val string) []string {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}
	var toks []string
	start := 0
	for i := 0; i < len(val); i++ {
		if val[i] == ' ' || val[i] == '\t' {
			if i > start {
				toks = append(toks, val[start:i])
			}
			start = i + 1
		}
	}
	if start < len(val) {
		toks = append(toks, val[start:])
	}
	return toks
}

func buildEnv(envOverlay []string, planEnvChanges []string) []string {
	base := os.Environ()

	// Build set of keys already in host environment.
	// Host/shell environment takes precedence over dotenv files.
	hostKeys := make(map[string]bool, len(base))
	for _, kv := range base {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				hostKeys[kv[:i]] = true
				break
			}
		}
	}

	// Apply dotenv overlay only for keys not already in host env.
	// Plan env changes (internal runtime needs) always apply.
	overlay := make(map[string]string)
	for _, kv := range envOverlay {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				key := kv[:i]
				if !hostKeys[key] {
					overlay[key] = kv[i+1:]
				}
				break
			}
		}
	}
	for _, kv := range planEnvChanges {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				overlay[kv[:i]] = kv[i+1:]
				break
			}
		}
	}

	if len(overlay) == 0 {
		return base
	}

	// Build output: host env (minus overridden keys) + overlay.
	out := make([]string, 0, len(base)+len(overlay))
	for _, kv := range base {
		key := kv
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				key = kv[:i]
				break
			}
		}
		if _, replaced := overlay[key]; replaced {
			continue
		}
		out = append(out, kv)
	}
	for k, v := range overlay {
		out = append(out, k+"="+v)
	}
	return out
}

// storagePath computes the persistent localStorage path for the project
// containing workingDir.  Returns "" when no project root exists (standalone
// script) — localStorage remains in-memory-only in that case.
//
// Namespace: first 16 hex chars of SHA-256(resolved project root).
// Path:      <cache>/webstorage/v1/<namespace>.json
//
// Symlinks in the project root are resolved before hashing so the namespace
// is stable regardless of how the directory is reached.
func storagePath(workingDir string, eff *config.Effective) string {
	root, err := project.FindRoot(workingDir)
	if err != nil {
		return ""
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	h := sha256.Sum256([]byte(realRoot))
	ns := hex.EncodeToString(h[:])[:16]
	return filepath.Join(config.CacheRoot(eff), "webstorage", "v1", ns+".json")
}
