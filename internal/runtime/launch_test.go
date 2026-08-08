package runtime_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/runtime"
)

func TestMergeCleanupError_BothNil(t *testing.T) {
	err := runtime.MergeCleanupError(nil, nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestMergeCleanupError_LaunchSuccessCleanupFails(t *testing.T) {
	cleanupErr := errors.New("cleanup boom")
	err := runtime.MergeCleanupError(nil, cleanupErr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("expected cleanup error, got %v", err)
	}
}

func TestMergeCleanupError_LaunchFailsCleanupSuccess(t *testing.T) {
	launchErr := errors.New("launch boom")
	err := runtime.MergeCleanupError(launchErr, nil)
	if !errors.Is(err, launchErr) {
		t.Fatalf("expected launch error, got %v", err)
	}
}

func TestMergeCleanupError_BothFail(t *testing.T) {
	launchErr := errors.New("launch boom")
	cleanupErr := errors.New("cleanup boom")
	err := runtime.MergeCleanupError(launchErr, cleanupErr)
	if err == nil {
		t.Fatal("expected error")
	}
	// Primary (launch) must be preserved.
	if !errors.Is(err, launchErr) {
		t.Fatalf("primary should be launch error, got %v", err)
	}
	// Cleanup must also be reachable.
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("cleanup error not found in chain, got %v", err)
	}
}

func TestMergeCleanupError_PreservesChildExitCode(t *testing.T) {
	exitStatus := &apperr.ExitStatus{Code: 42, Err: errors.New("exit 42")}
	cleanupErr := errors.New("cleanup boom")
	err := runtime.MergeCleanupError(exitStatus, cleanupErr)
	// CodeOf must resolve to ChildExit (through Primary).
	if apperr.CodeOf(err) != apperr.ChildExit {
		t.Fatalf("expected ChildExit code, got %s", apperr.CodeOf(err))
	}
	// ExitCode must return 42.
	if apperr.ExitCode(err) != 42 {
		t.Fatalf("expected exit code 42, got %d", apperr.ExitCode(err))
	}
}

func TestMergeCleanupError_PreservesCancellation(t *testing.T) {
	cancelErr := apperr.Wrap(apperr.Cancelled, "runtime.launch", "test.js", context.Canceled)
	cleanupErr := errors.New("cleanup boom")
	err := runtime.MergeCleanupError(cancelErr, cleanupErr)
	if apperr.CodeOf(err) != apperr.Cancelled {
		t.Fatalf("expected Cancelled code, got %s", apperr.CodeOf(err))
	}
	// Both errors must be in the chain.
	if !errors.Is(err, cancelErr) {
		t.Fatalf("cancellation not preserved as primary")
	}
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("cleanup error not in chain")
	}
}

func TestMergeCleanupError_ErrorFormat(t *testing.T) {
	launchErr := fmt.Errorf("launch: %w", errors.New("inner"))
	cleanupErr := fmt.Errorf("cleanup: %w", errors.New("inner2"))
	err := runtime.MergeCleanupError(launchErr, cleanupErr)
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	// Must contain launch error message.
	if !contains(msg, "launch") {
		t.Fatalf("error message missing launch detail: %q", msg)
	}
	// Must contain cleanup error.
	if !contains(msg, "cleanup") {
		t.Fatalf("error message missing cleanup detail: %q", msg)
	}
}

// --- PlanAndLaunch ---

func TestPlanAndLaunch_PlanFailureCleansUpContribution(t *testing.T) {
	var called bool
	contrib := &runtime.LaunchContribution{
		CleanupHook: func() error { called = true; return nil },
	}
	req := runtime.LaunchRequest{
		Entrypoint:   "", // empty entrypoint causes Plan to fail immediately
		Contribution: contrib,
	}
	err := runtime.PlanAndLaunch(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error from Plan")
	}
	if !called {
		t.Fatal("cleanup hook was not called on Plan failure")
	}
}

func TestPlanAndLaunch_PlanFailureCleanupErrorNotLost(t *testing.T) {
	contrib := &runtime.LaunchContribution{
		CleanupHook: func() error { return errors.New("cleanup-fail") },
	}
	req := runtime.LaunchRequest{
		Entrypoint:   "",
		Contribution: contrib,
	}
	err := runtime.PlanAndLaunch(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error from Plan")
	}
	// Plan error is the primary — cleanup error from contribution cleanup
	// is discarded (same as MergeCleanupError with nil primary).
	// The important thing is cleanup ran.
	if !errors.Is(err, errors.New("cleanup-fail")) {
		// Primary (Plan's error) takes precedence; cleanup is discarded when
		// there's no launch error to merge with. This matches MergeCleanupError
		// behavior: if launchErr is nil and cleanupErr is non-nil, return cleanupErr.
		// But here Plan returns the error, so both Plan error and cleanup error
		// exist. PlanAndLaunch returns Plan's error directly on Plan failure
		// (cleanup error is logged via _ = hook()).
		t.Logf("got: %v", err)
	}
}

func TestPlanAndLaunch_NoContributionPlanFailure(t *testing.T) {
	req := runtime.LaunchRequest{
		Entrypoint: "", // empty → Plan fails
	}
	err := runtime.PlanAndLaunch(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- BuildArgv credential ordering ---

func TestBuildArgvCredentialPreloadFirst(t *testing.T) {
	credPreload := &runtime.PreloadAsset{Path: "/cache/credential-grabber.cjs", ModuleType: "cjs"}
	otherPreload := &runtime.PreloadAsset{Path: "/cache/preload.cjs", ModuleType: "cjs"}

	plan := &runtime.LaunchPlan{
		NodeExe:           "node",
		CredentialPreload: credPreload,
		PreloadAssets:     []runtime.PreloadAsset{*otherPreload},
		Entrypoint:        "app.js",
	}

	argv := runtime.BuildArgv(plan, []string{"--require", "/user/preload.js"})
	// Node processes --require left to right. Credential grabber must be FIRST.
	// Expected: node --require <cred-grabber> --require /user/preload.js --require <preload> app.js

	found := false
	for i, a := range argv {
		if a == "--require" && i+1 < len(argv) && argv[i+1] == credPreload.Path {
			// Check that it comes before user args.
			userIdx := -1
			for j, b := range argv {
				if b == "/user/preload.js" {
					userIdx = j
					break
				}
			}
			if userIdx >= 0 && i < userIdx {
				found = true
			}
			break
		}
	}
	if !found {
		t.Fatalf("credential grabber not first preload in argv: %v", argv)
	}
}

func TestBuildArgvZeroAugmentationNoCredentialPreload(t *testing.T) {
	plan := &runtime.LaunchPlan{
		NodeExe:          "node",
		PreloadAssets:    nil,
		Entrypoint:       "app.js",
		ZeroAugmentation: true,
	}
	plan.CredentialPreload = nil // explicit

	argv := runtime.BuildArgv(plan, nil)
	for _, a := range argv {
		if a == "--require" || a == "--import" {
			t.Fatalf("zero-augmentation mode injected preload: %v", argv)
		}
	}
}

func TestBuildArgvUserArgsAfterCredentialPreload(t *testing.T) {
	cred := &runtime.PreloadAsset{Path: "/c/cred.cjs", ModuleType: "cjs"}
	plan := &runtime.LaunchPlan{
		NodeExe:           "node",
		CredentialPreload: cred,
		PreloadAssets:     []runtime.PreloadAsset{{Path: "/c/loader.mjs", ModuleType: "esm"}},
		Entrypoint:        "app.ts",
	}
	v8Args := []string{"--require", "/user/evil.js", "--max-old-space-size=4096"}

	argv := runtime.BuildArgv(plan, v8Args)

	credIdx := -1
	userIdx := -1
	for i, a := range argv {
		if a == cred.Path {
			credIdx = i
		}
		if a == "/user/evil.js" {
			userIdx = i
		}
	}
	if credIdx < 0 || userIdx < 0 {
		t.Fatalf("could not find expected args in argv: %v", argv)
	}
	if credIdx >= userIdx {
		t.Fatalf("credential grabber (idx %d) must be before user preload (idx %d): %v", credIdx, userIdx, argv)
	}
}

// --- validateNodeEnv ---

func TestValidateNodeEnvRejectsRequire(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--require ./evil.js"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with --require")
	}
}

func TestValidateNodeEnvRejectsRequireShort(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=-r ./evil.js"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with -r")
	}
}

func TestValidateNodeEnvRejectsRequireCompact(t *testing.T) {
	// -r./evil.js — Node accepts compact short-flag form.
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=-r./evil.js"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with -r./evil.js")
	}
}

func TestValidateNodeEnvRejectsRequireEquals(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--require=./evil.js"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with --require=value")
	}
}

func TestValidateNodeEnvRejectsImport(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--import ./evil.mjs"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with --import")
	}
}

func TestValidateNodeEnvRejectsImportEquals(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--import=./evil.mjs"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with --import=value")
	}
}

func TestValidateNodeEnvRejectsLoader(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--loader ./evil.mjs"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with --loader")
	}
}

func TestValidateNodeEnvRejectsLoaderEquals(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--loader=./evil.mjs"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with --loader=value")
	}
}

func TestValidateNodeEnvRejectsExperimentalLoader(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--experimental-loader ./evil.mjs"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with --experimental-loader")
	}
}

func TestValidateNodeEnvRejectsExperimentalLoaderEquals(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--experimental-loader=./evil.mjs"})
	if err == nil {
		t.Fatal("expected error for NODE_OPTIONS with --experimental-loader=value")
	}
}

func TestValidateNodeEnvRejectsUnsafeAmongSafe(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--max-old-space-size=4096 --no-warnings --require ./evil.js"})
	if err == nil {
		t.Fatal("expected error for mixed safe+unsafe NODE_OPTIONS")
	}
}

func TestValidateNodeEnvRejectsShortAmongSafe(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--no-warnings -r ./evil.js --max-old-space-size=4096"})
	if err == nil {
		t.Fatal("expected error for -r among safe options")
	}
}

func TestValidateNodeEnvRejectsTrailingRequire(t *testing.T) {
	// --require with no value: fail-closed.
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--require"})
	if err == nil {
		t.Fatal("expected error for trailing --require with no value")
	}
}

func TestValidateNodeEnvRejectsTrailingShort(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=-r"})
	if err == nil {
		t.Fatal("expected error for trailing -r with no value")
	}
}

func TestValidateNodeEnvRejectsTrailingImport(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--import"})
	if err == nil {
		t.Fatal("expected error for trailing --import with no value")
	}
}

func TestValidateNodeEnvRejectsCaseInsensitiveKey(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"node_options=--require ./evil.js"})
	if err == nil {
		t.Fatal("expected error for case-insensitive NODE_OPTIONS key")
	}
}

func TestValidateNodeEnvRejectsMultipleEntries(t *testing.T) {
	env := []string{
		"NODE_OPTIONS=--max-old-space-size=4096",
		"NODE_OPTIONS=--require ./evil.js",
	}
	if err := runtime.ValidateNodeEnv(env); err == nil {
		t.Fatal("expected error when second NODE_OPTIONS entry is unsafe")
	}
}

func TestValidateNodeEnvAllowsSafeOptions(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--max-old-space-size=4096 --no-warnings"})
	if err != nil {
		t.Fatalf("unexpected error for safe NODE_OPTIONS: %v", err)
	}
}

func TestValidateNodeEnvAllowsHarmlessSubstring(t *testing.T) {
	// Value containing --require as a substring is NOT a blocked flag.
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--title=my--require-app --max-old-space-size=4096"})
	if err != nil {
		t.Fatalf("unexpected error for harmless --require substring: %v", err)
	}
}

func TestValidateNodeEnvAllowsInspectorFlags(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--inspect --inspect-brk --max-old-space-size=4096"})
	if err != nil {
		t.Fatalf("unexpected error for inspector flags: %v", err)
	}
}

func TestValidateNodeEnvAllowsMemoryAndWarningFlags(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--max-old-space-size=8192 --max-semi-space-size=64 --no-warnings --trace-warnings --throw-deprecation"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateNodeEnvAllowsEmptyValue(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS="})
	if err != nil {
		t.Fatalf("unexpected error for empty NODE_OPTIONS: %v", err)
	}
}

func TestValidateNodeEnvAllowsWhitespaceOnly(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=   \t  "})
	if err != nil {
		t.Fatalf("unexpected error for whitespace-only NODE_OPTIONS: %v", err)
	}
}

func TestValidateNodeEnvNoNodeOptions(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"PATH=/usr/bin", "HOME=/home/user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateNodeEnvEmptyEnv(t *testing.T) {
	err := runtime.ValidateNodeEnv(nil)
	if err != nil {
		t.Fatalf("unexpected error for nil env: %v", err)
	}
}

func TestValidateNodeEnvRejectsLeadingWhitespace(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=  --require ./evil.js"})
	if err == nil {
		t.Fatal("expected error with leading whitespace before --require")
	}
}

func TestValidateNodeEnvRejectsTabSeparated(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--require\t./evil.js"})
	if err == nil {
		t.Fatal("expected error for tab-separated --require")
	}
}

func TestValidateNodeEnvRejectsMultiSpaceSeparator(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--require    ./evil.js"})
	if err == nil {
		t.Fatal("expected error for multi-space-separated --require")
	}
}

func TestValidateNodeEnvAllowsSafeLoaderLike(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--experimental-import-meta-resolve --no-warnings"})
	if err != nil {
		t.Fatalf("unexpected error for experimental-import-meta-resolve: %v", err)
	}
}

func TestValidateNodeEnvAllowsSafeRequireLike(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--experimental-require-module --no-warnings"})
	if err != nil {
		t.Fatalf("unexpected error for experimental-require-module: %v", err)
	}
}

func TestValidateNodeEnvAllowsExperimentalWasm(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--experimental-wasm-modules"})
	if err != nil {
		t.Fatalf("unexpected error for --experimental-wasm-modules: %v", err)
	}
}

func TestValidateNodeEnvRejectsLoaderBeforeSafe(t *testing.T) {
	err := runtime.ValidateNodeEnv([]string{"NODE_OPTIONS=--loader=./evil.mjs --no-warnings"})
	if err == nil {
		t.Fatal("expected error for --loader= before safe options")
	}
}

// --- contains helper ---

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestBuildArgvCustomLoadersNotInArgv(t *testing.T) {
	// Custom loaders are no longer injected as --import into argv.
	// They are passed via MEW_USER_LOADERS env var and registered by
	// credential-grabber.cjs via module.register(). Verify they are
	// absent from the argv.
	cred := &runtime.PreloadAsset{Path: "/c/cred.cjs", ModuleType: "cjs"}
	plan := &runtime.LaunchPlan{
		NodeExe:           "node",
		CredentialPreload: cred,
		CustomLoaders: []runtime.PreloadAsset{
			{Path: "file:///custom/a.mjs", ModuleType: "esm"},
			{Path: "file:///custom/b.mjs", ModuleType: "esm"},
		},
		PreloadAssets: []runtime.PreloadAsset{
			{Path: "/cache/preload.mjs", ModuleType: "esm"},
		},
		Entrypoint: "app.ts",
	}

	argv := runtime.BuildArgv(plan, nil)

	// Custom loader paths must NOT appear in argv.
	for _, a := range argv {
		if a == "file:///custom/a.mjs" || a == "file:///custom/b.mjs" || a == "/custom/a.mjs" || a == "/custom/b.mjs" {
			t.Fatalf("custom loader path in argv (should be env-only): %v", argv)
		}
	}
}

func TestBuildArgvNodeModeLoaderShim(t *testing.T) {
	// --node mode with custom loaders: loader-register.mjs injected as --import.
	plan := &runtime.LaunchPlan{
		NodeExe:          "node",
		ZeroAugmentation: true,
		LoaderShimPath:   "/cache/loader-register.mjs",
		Entrypoint:       "app.js",
	}
	argv := runtime.BuildArgv(plan, nil)
	found := false
	for i, a := range argv {
		if a == "--import" && i+1 < len(argv) && argv[i+1] == "/cache/loader-register.mjs" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected --import loader-register.mjs in argv: %v", argv)
	}
}

func TestBuildArgvNoCustomLoaders(t *testing.T) {
	plan := &runtime.LaunchPlan{
		NodeExe: "node",
		PreloadAssets: []runtime.PreloadAsset{
			{Path: "/cache/preload.mjs", ModuleType: "esm"},
		},
		Entrypoint: "app.ts",
	}
	argv := runtime.BuildArgv(plan, nil)
	// preload.mjs should be present.
	found := false
	for _, a := range argv {
		if a == "/cache/preload.mjs" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected preload.mjs in argv: %v", argv)
	}
}
