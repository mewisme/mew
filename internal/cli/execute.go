package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/diagnostics"
	"github.com/mewisme/mew/internal/dotenv"
	"github.com/mewisme/mew/internal/presentation"
	presprompt "github.com/mewisme/mew/internal/presentation/prompt"
	"github.com/mewisme/mew/internal/prompt"
	"github.com/mewisme/mew/internal/runtime"
	"github.com/mewisme/mew/internal/trace"
)

// globalFlags holds persistent CLI presentation options.
type globalFlags struct {
	output        string
	logLevel      string
	debug         bool
	noColor       bool
	noProgress    bool
	ascii         bool
	noSummary     bool
	accessible    bool
	unsafe        bool
	cwd           string
	configPath    string
	offline       bool
	preferOffline bool
	filter        []string
	recursive     bool
	invokedBinary string
	headerEmitted bool
	ctrl          presentation.Controller
	// theme is the ui.theme value resolved during bootstrap ("auto" when
	// configuration is unavailable).
	theme string
	// snapshot is the single configuration snapshot for this invocation, shared
	// by presentation and every app.Context built from it.
	snapshot *app.ConfigSnapshot
	// prompter is the presentation-selected prompt adapter, set during buildAppContext.
	prompter  prompt.Prompter
	canPrompt bool
}

var flagOwners sync.Map     // *cobra.Command -> *globalFlags
var rootBuildInfos sync.Map // *cobra.Command -> BuildInfo

func storeRootBuildInfo(root *cobra.Command, info BuildInfo) {
	if root != nil {
		rootBuildInfos.Store(root, info)
	}
}

func loadRootBuildInfo(root *cobra.Command) BuildInfo {
	if v, ok := rootBuildInfos.Load(root); ok {
		return v.(BuildInfo)
	}
	return BuildInfo{}
}

func (g *globalFlags) bind(cmd *cobra.Command) {
	g.bindPresentation(cmd)
	cmd.PersistentFlags().BoolVar(&g.debug, "debug", false, "verbose diagnostics (env MEW_DEBUG or M_LOG=debug)")
	cmd.PersistentFlags().BoolVar(&g.unsafe, "unsafe-diagnostics", false, "disable secret redaction (dangerous)")
	_ = cmd.PersistentFlags().MarkHidden("unsafe-diagnostics")
	cmd.PersistentFlags().StringVar(&g.cwd, "cwd", "", "project working directory")
	cmd.PersistentFlags().StringVar(&g.configPath, "config", "", "JSONC config file overlay path")
	cmd.PersistentFlags().BoolVar(&g.offline, "offline", false, "force offline mode")
	cmd.PersistentFlags().BoolVar(&g.preferOffline, "prefer-offline", false, "prefer cached artifacts")
	cmd.PersistentFlags().StringArrayVar(&g.filter, "filter", nil, "workspace package filter (pnpm-style)")
}

func (g *globalFlags) bindRecursive(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVarP(&g.recursive, "recursive", "r", false, "workspace recursive mode (consumed by m run)")
}

func (g *globalFlags) newReporter(cmd *cobra.Command) diagnostics.Reporter {
	return g.mustReporter(cmd)
}

func (g *globalFlags) resolveDebug() bool {
	if g.debug {
		return true
	}
	if os.Getenv("MEW_DEBUG") != "" {
		return true
	}
	return strings.EqualFold(os.Getenv("M_LOG"), "debug")
}

// ensurePrompter lazily creates and caches a Prompter from the controller.
// Returns nil, false when prompting is unavailable (non-interactive or no controller).
func (g *globalFlags) ensurePrompter(cmd *cobra.Command) (prompt.Prompter, bool) {
	if g.prompter != nil {
		return g.prompter, g.canPrompt
	}
	if g.ctrl == nil {
		return nil, false
	}
	resolved := g.ctrl.Options()
	caps := g.ctrl.Capabilities()
	settings := g.ctrl.Settings()

	human := !resolved.Structured() && resolved.Output != presentation.OutputSilent
	decision := prompt.ResolveInteractive(prompt.InteractiveAuto, prompt.Caps{
		StdinTTY:     caps.StdinTTY,
		HumanMode:    human,
		CI:           caps.CI,
		Accessible:   settings.Accessible,
		AccessibleOK: true,
		RichOK:       true,
	})
	g.canPrompt = decision.CanPrompt
	if !decision.CanPrompt {
		return nil, false
	}
	useRich := !decision.UseAccessible &&
		settings.UseInteractive &&
		!settings.Accessible &&
		resolved.Output == presentation.OutputRich
	g.prompter = presprompt.New(presprompt.Options{
		Stdin:      cmd.InOrStdin(),
		Stderr:     cmd.ErrOrStderr(),
		Width:      settings.Width,
		UseColor:   settings.UseColor,
		UseUnicode: settings.UseUnicode,
		Accessible: !useRich,
		UseRich:    useRich,
		Suspend:    g.ctrl.Suspend,
		Resume:     g.ctrl.Resume,
	})
	return g.prompter, g.canPrompt
}

func attachGlobals(root *cobra.Command) *globalFlags {
	g := &globalFlags{}
	g.bind(root)
	flagOwners.Store(root, g)
	return g
}

func workspaceFilters(cmd *cobra.Command) []string {
	g := ownerFlags(cmd.Root())
	if g == nil || len(g.filter) == 0 {
		return nil
	}
	return append([]string(nil), g.filter...)
}

func workspaceRecursive(cmd *cobra.Command) bool {
	g := ownerFlags(cmd.Root())
	return g != nil && g.recursive
}

func installOptsFromGlobals(cmd *cobra.Command, base app.InstallOptions) app.InstallOptions {
	base.Filter = workspaceFilters(cmd)
	return base
}

func ownerFlags(root *cobra.Command) *globalFlags {
	if v, ok := flagOwners.Load(root); ok {
		return v.(*globalFlags)
	}
	return &globalFlags{}
}

// invocationHeader builds the command invocation header using the invoked binary and command path.
// commandPath is the Cobra CommandPath (e.g. "m install"). The binary is prepended only if the path
// does not already start with it, avoiding "m m install".
func invocationHeader(root *cobra.Command, commandPath string) string {
	g := ownerFlags(root)
	binary := g.invokedBinary
	if binary == "" {
		binary = "m"
	}
	bi := loadRootBuildInfo(root)

	// Strip the root command name prefix if commandPath already starts with the binary.
	// Cobra returns "m install" when Use is "m"; we want just "install" to avoid "m m install".
	rel := commandPath
	if root != nil {
		rootName := root.Name()
		if strings.HasPrefix(rel, rootName+" ") {
			rel = rel[len(rootName)+1:]
		} else if rel == rootName {
			rel = ""
		}
	}

	short := bi.Short()
	return presentation.FormatInvocationHeader(binary, rel, presentation.BuildInfo{
		Version:     bi.Version,
		ShortCommit: short,
		Dirty:       bi.Dirty,
	})
}

// writeInvocationHeaderOnce emits the header line once per invocation.
// The header is always emitted for human output regardless of --no-summary.
// Suppressed when stdout is not a terminal (piped/redirected), for structured output,
// or when the command has a local --json flag.
func writeInvocationHeaderOnce(cmd *cobra.Command) {
	g := ownerFlags(cmd.Root())
	if g == nil || g.headerEmitted {
		return
	}
	// Only emit header when presentation controller has been set up.
	if g.ctrl == nil {
		return
	}
	opts := g.ctrl.Options()
	if opts.Structured() || opts.Output == presentation.OutputSilent {
		return
	}
	// Suppress header for commands with local --json flag.
	if cmd.Flags().Lookup("json") != nil {
		if f, _ := cmd.Flags().GetBool("json"); f {
			return
		}
	}
	// Suppress header when stdout is not a terminal (piped/redirected output).
	caps := g.ctrl.Capabilities()
	if !caps.StdoutTTY {
		return
	}
	g.headerEmitted = true
	commandPath := cmd.CommandPath()
	hdr := invocationHeader(cmd.Root(), commandPath)
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), hdr)
}

// buildAppContext assembles the app context from invocation state bootstrap
// already established. It resolves no paths, reads no environment, loads no
// configuration, and creates no presentation of its own. A nil snapshot only
// happens when a test drives a command without going through execute; app.New
// then loads for itself.
func buildAppContext(ctx context.Context, cmd *cobra.Command, g *globalFlags, info BuildInfo) (*app.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctrl, err := g.controller(cmd)
	if err != nil {
		return nil, wrapPresentationErr(err)
	}

	ac, err := app.New(ctx, app.Options{
		CWD:           g.cwd,
		ConfigPath:    g.configPath,
		Offline:       g.offline,
		PreferOffline: g.preferOffline,
		Reporter:      ctrl.Reporter(),
		Version:       info.Version,
		Commit:        info.Commit,
		BuildDate:     info.BuildDate,
		BinaryName:    g.invokedBinary,
		Snapshot:      g.snapshot,
	})
	if err != nil {
		return nil, err
	}

	ac.SuspendUI = ctrl.Suspend
	ac.ResumeUI = ctrl.Resume
	attachPrompter(ac, cmd, ctrl)
	g.prompter = ac.Prompter
	g.canPrompt = ac.CanPrompt
	return ac, nil
}

func attachPrompter(ac *app.Context, cmd *cobra.Command, ctrl presentation.Controller) {
	if ac == nil || ctrl == nil {
		return
	}
	resolved := ctrl.Options()
	caps := ctrl.Capabilities()
	settings := ctrl.Settings()

	human := !resolved.Structured() && resolved.Output != presentation.OutputSilent
	decision := prompt.ResolveInteractive(prompt.InteractiveAuto, prompt.Caps{
		StdinTTY:     caps.StdinTTY,
		HumanMode:    human,
		CI:           caps.CI,
		Accessible:   settings.Accessible,
		AccessibleOK: true,
		RichOK:       true,
	})
	ac.CanPrompt = decision.CanPrompt
	if !decision.CanPrompt {
		return
	}
	useRich := !decision.UseAccessible &&
		settings.UseInteractive &&
		!settings.Accessible &&
		resolved.Output == presentation.OutputRich
	ac.Prompter = presprompt.New(presprompt.Options{
		Stdin:      cmd.InOrStdin(),
		Stderr:     cmd.ErrOrStderr(),
		Width:      settings.Width,
		UseColor:   settings.UseColor,
		UseUnicode: settings.UseUnicode,
		Accessible: !useRich,
		UseRich:    useRich,
		Suspend:    ctrl.Suspend,
		Resume:     ctrl.Resume,
	})
}

func execute(root *cobra.Command, info BuildInfo, argv []string) int {
	if len(argv) == 0 {
		argv = os.Args[1:]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runInvocation(ctx, root, info, argv)
}

// runInvocation is the single production invocation path: bootstrap presentation
// and configuration, dispatch, then close the controller exactly once.
func runInvocation(ctx context.Context, root *cobra.Command, info BuildInfo, argv []string) (exit int) {
	g := ownerFlags(root)
	if ctx == nil {
		ctx = context.Background()
	}
	root.SetContext(ctx)

	// Bootstrap: parse invocation flags, classify the command, load config once,
	// resolve ui.theme, then create the one controller for this invocation.
	bootErr := bootstrapInvocation(ctx, root, g, argv)
	if g.ctrl == nil {
		// Presentation itself could not be built: report on a minimal plain
		// reporter and create no controller.
		wrapped := wrapPresentationErr(bootErr)
		diagnostics.NewReporter(diagnostics.Options{
			Out: root.OutOrStdout(),
			Err: root.ErrOrStderr(),
		}).Error(wrapped)
		return apperr.ExitCode(wrapped)
	}
	rep := g.ctrl.Reporter()

	// Sole owner of controller shutdown: every return below, panics included,
	// passes through here exactly once.
	var outcomeErr error
	defer func() {
		if rec := recover(); rec != nil {
			outcomeErr = apperr.New(apperr.InternalPanic, "cli", newCrashID(), fmt.Sprintf("panic: %v", rec))
			rep.Error(outcomeErr)
			exit = apperr.ExitCode(outcomeErr)
		}
		closePresentation(root, g, presentationOutcome(outcomeErr))
	}()

	if bootErr != nil {
		outcomeErr = classifyCLIError(bootErr)
		rep.Error(outcomeErr)
		return apperr.ExitCode(outcomeErr)
	}

	if dispatchEnabledForRoot(root) {
		if code, handled := tryDirectDispatch(ctx, root, g, info, argv); handled {
			outcomeErr = dispatchOutcomeErr(code)
			return code
		}
	}
	if isMXRoot(root) {
		if code, handled := tryMXDispatch(ctx, root, g, info, argv); handled {
			outcomeErr = dispatchOutcomeErr(code)
			return code
		}
	}

	root.SetArgs(argv)
	if execErr := root.ExecuteContext(ctx); execErr != nil {
		outcomeErr = classifyCLIError(execErr)
		// A command that already emitted its own machine report suppresses the
		// reporter's rendering, so a structured consumer receives exactly one
		// document instead of the report followed by an unrelated error doc.
		if !reportSuppressed(outcomeErr) {
			rep.Error(outcomeErr)
		}
		return apperr.ExitCode(outcomeErr)
	}
	return 0
}

// suppressedError marks an error whose report the command already wrote itself.
// It changes nothing about typing or exit status: the wrapped typed error still
// resolves through errors.As, so apperr.CodeOf and apperr.ExitCode are
// unaffected. Only the reporter's duplicate rendering is skipped.
type suppressedError struct{ err error }

func (e *suppressedError) Error() string { return e.err.Error() }
func (e *suppressedError) Unwrap() error { return e.err }

// suppressReport wraps err so runInvocation does not render it a second time.
func suppressReport(err error) error {
	if err == nil {
		return nil
	}
	return &suppressedError{err: err}
}

func reportSuppressed(err error) bool {
	var s *suppressedError
	return errors.As(err, &s)
}

// dispatchOutcomeErr converts a dispatch exit code into a close outcome. The
// dispatch paths report their own errors, so only the failure signal is needed.
func dispatchOutcomeErr(code int) error {
	if code == 0 {
		return nil
	}
	return apperr.New(apperr.Internal, "cli.dispatch", "", "dispatch failed")
}

func dispatchEnabledForRoot(root *cobra.Command) bool {
	if root == nil {
		return false
	}
	switch root.Name() {
	case "m", "mew":
		return true
	default:
		return false
	}
}

func isMXRoot(root *cobra.Command) bool {
	if root == nil {
		return false
	}
	switch root.Name() {
	case "mx", "mewx":
		return true
	default:
		return false
	}
}

func tryDirectDispatch(ctx context.Context, root *cobra.Command, g *globalFlags, info BuildInfo, argv []string) (int, bool) {
	if len(argv) == 0 {
		return 0, false
	}
	if isRootMetaInvocation(argv) {
		return 0, false
	}

	phase, err := ParsePhaseA(argv)
	if err != nil {
		rep := g.newReporter(root)
		rep.Error(classifyCLIError(err))
		return apperr.ExitCode(err), true
	}

	if phase.BareM {
		applyLeadingToGlobalFlags(g, phase.Leading)
		return handleBareM(ctx, root, g, info, phase)
	}

	if kind, _ := lookupBuiltin(root, phase.Selector); kind != "" {
		return 0, false
	}

	applyLeadingToGlobalFlags(g, phase.Leading)
	ac, err := buildAppContext(ctx, root, g, info)
	if err != nil {
		rep := g.newReporter(root)
		rep.Error(classifyCLIError(err))
		return apperr.ExitCode(err), true
	}

	res := ResolveDispatch(root, phase, ac.CWD, ac.Config)
	switch res.Kind {
	case OutcomeScript:
		if res.Invocation == nil {
			err := apperr.New(apperr.Internal, "dispatch", res.Canonical, "missing script invocation")
			rep := g.newReporter(root)
			rep.Error(err)
			return apperr.ExitCode(err), true
		}
		_, err = app.Run(ctx, ac, res.Invocation.ToRunOptions())
		rep := g.newReporter(root)
		if err == nil {
			return 0, true
		}
		rep.Error(classifyCLIError(err))
		return apperr.ExitCode(err), true
	case OutcomeBin:
		if res.Bin == nil {
			err := apperr.New(apperr.Internal, "dispatch", res.Canonical, "missing bin invocation")
			rep := g.newReporter(root)
			rep.Error(err)
			return apperr.ExitCode(err), true
		}
		_, err = app.Exec(ctx, ac, res.Bin.ToExecOptions())
		rep := g.newReporter(root)
		if err == nil {
			return 0, true
		}
		rep.Error(classifyCLIError(err))
		return apperr.ExitCode(err), true
	case OutcomeSuggest, OutcomeUnknown:
		return emitDispatchFailure(root, g, res)
	case OutcomeFileRun:
		if res.FileRunPath == "" {
			err := apperr.New(apperr.Internal, "dispatch", res.Canonical, "missing file run path")
			rep := g.newReporter(root)
			rep.Error(err)
			return apperr.ExitCode(err), true
		}
		augMode := runtime.AugmentDefault
		if phase.Leading.node {
			augMode = runtime.AugmentNone
		}
		// On Windows with augmentation, use original selector as entrypoint
		// so Node resolves it relative to WorkingDir.
		entrypoint := res.FileRunPath
		if augMode != runtime.AugmentNone && goruntime.GOOS == "windows" {
			entrypoint = res.Canonical
		}
		envOverlay, buildErr := buildEnvOverlay(ac.CWD, phase.Leading)
		if buildErr != nil {
			rep := g.newReporter(root)
			rep.Error(classifyCLIError(buildErr))
			return apperr.ExitCode(buildErr), true
		}
		emitEnvTrace(ctx, phase.Leading, envOverlay, ac.CWD)
		req := runtime.LaunchRequest{
			Entrypoint:       entrypoint,
			AppArgs:          phase.ForwardedArgs,
			NodeV8Args:       append([]string(nil), phase.Leading.v8Args...),
			WorkingDir:       ac.CWD,
			AugmentationMode: augMode,
			EnvOverlay:       envOverlay,
			Loaders:          append([]string(nil), phase.Leading.loaders...),
			Stdio: runtime.LaunchStdio{
				Stdin:  os.Stdin,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
			},
		}
		// Attach transform session when augmentation is active.
		// The loader's resolve hook handles extension substitution for all
		// entrypoints; the transform service must be available for any .ts
		// files that are resolved via .js→.ts mapping.
		if augMode != runtime.AugmentNone {
			contrib, contribErr := buildTransformContribution(ctx, ac.CWD, res.FileRunPath, ac.Config)
			if contribErr != nil {
				rep := g.newReporter(root)
				rep.Error(classifyCLIError(contribErr))
				return apperr.ExitCode(contribErr), true
			}
			req.Contribution = contrib
		}

		launchErr := runtime.PlanAndLaunch(ctx, req, ac.Config)
		if launchErr == nil {
			return 0, true
		}
		rep := g.newReporter(root)
		rep.Error(classifyCLIError(launchErr))
		return apperr.ExitCode(launchErr), true
	default:
		return 0, false
	}
}

func dispatchCWD(g *globalFlags, phase PhaseAResult) string {
	if phase.Leading.cwd != "" {
		return phase.Leading.cwd
	}
	if g != nil && g.cwd != "" {
		return g.cwd
	}
	cwd, _ := os.Getwd()
	return cwd
}

func handleBareM(ctx context.Context, root *cobra.Command, g *globalFlags, info BuildInfo, phase PhaseAResult) (int, bool) {
	_ = ctx
	_ = info
	cwd := dispatchCWD(g, phase)
	msg := bareMUsageMessage(cwd, g.invokedBinary)
	err := apperr.New(apperr.Usage, "cli", "", msg)
	rep := g.newReporter(root)
	rep.Error(err)
	return apperr.ExitCode(err), true
}

func emitDispatchFailure(root *cobra.Command, g *globalFlags, res DispatchResult) (int, bool) {
	rep := g.newReporter(root)
	if res.Err != nil {
		rep.Error(classifyCLIError(res.Err))
		return apperr.ExitCode(res.Err), true
	}
	msg := res.Message
	if msg == "" {
		msg = fmt.Sprintf("unknown command %q", res.Canonical)
	}
	err := apperr.New(apperr.Usage, "dispatch", res.Canonical, msg)
	rep.Error(err)
	return apperr.ExitCode(err), true
}

func classifyCLIError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return apperr.Wrap(apperr.Cancelled, "cli", "", err)
	}
	// Child exit errors: preserve exit code, do not wrap as internal failure.
	var es *apperr.ExitStatus
	if errors.As(err, &es) {
		return err
	}
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return err
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "unknown flag") ||
		strings.Contains(msg, "invalid argument") ||
		strings.Contains(msg, "required flag") ||
		(strings.Contains(msg, "accepts") && strings.Contains(msg, "arg")) ||
		(strings.Contains(msg, "requires") && strings.Contains(msg, "arg")) {
		return apperr.Wrap(apperr.Usage, "cli", "", err)
	}
	return apperr.Wrap(apperr.Internal, "cli", "", err)
}

// RecoverPanic is exported for tests to exercise panic recovery formatting.
func RecoverPanic(rep diagnostics.Reporter, fn func()) (exit int) {
	defer func() {
		if rec := recover(); rec != nil {
			err := apperr.New(apperr.InternalPanic, "cli", newCrashID(), fmt.Sprintf("panic: %v", rec))
			rep.Error(err)
			exit = apperr.ExitCode(err)
		}
	}()
	fn()
	return 0
}

// ExecuteWithContext runs root with an explicit context (tests).
func ExecuteWithContext(root *cobra.Command, ctx context.Context) int {
	argv := cobraPendingArgs(root)
	if len(argv) == 0 {
		argv = os.Args[1:]
	}
	return runInvocation(ctx, root, loadRootBuildInfo(root), argv)
}

// ExecuteWithArgv runs root with explicit argv (integration tests).
func ExecuteWithArgv(root *cobra.Command, ctx context.Context, argv []string) int {
	return runInvocation(ctx, root, loadRootBuildInfo(root), argv)
}

func cobraPendingArgs(cmd *cobra.Command) []string {
	if cmd == nil {
		return nil
	}
	rv := reflect.ValueOf(cmd).Elem()
	f := rv.FieldByName("args")
	if !f.IsValid() || f.Kind() != reflect.Slice || f.Len() == 0 {
		return nil
	}
	out := make([]string, f.Len())
	for i := 0; i < f.Len(); i++ {
		out[i] = f.Index(i).String()
	}
	return out
}

// dispatchJSON is the __dispatch introspection schema (schemaVersion 1).
type dispatchJSON struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Kind          string                   `json:"kind"`
	Selector      string                   `json:"selector"`
	Enabled       bool                     `json:"enabled"`
	Path          string                   `json:"path,omitempty"`
	Suggestions   []dispatchSuggestionJSON `json:"suggestions"`
}

type dispatchSuggestionJSON struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Invocation string `json:"invocation"`
	Distance   int    `json:"distance"`
}

func encodeDispatchJSON(res DispatchResult, selector string) ([]byte, error) {
	doc := dispatchJSON{
		SchemaVersion: 1,
		Selector:      selector,
		Enabled:       res.DirectGateOn,
		Suggestions:   []dispatchSuggestionJSON{},
	}
	switch res.Kind {
	case OutcomeBuiltin:
		doc.Kind = "builtin"
		doc.Path = res.Canonical
	case OutcomeAlias:
		doc.Kind = "alias"
		doc.Path = res.Canonical
	case OutcomeScript:
		doc.Kind = "script"
	case OutcomeSuggest:
		doc.Kind = "suggest"
	case OutcomeUnknown:
		doc.Kind = "unknown"
	default:
		doc.Kind = string(res.Kind)
	}
	for _, s := range res.Suggestions {
		doc.Suggestions = append(doc.Suggestions, dispatchSuggestionJSON{
			Name:       s.Name,
			Kind:       string(s.Kind),
			Invocation: s.Invocation,
			Distance:   s.Distance,
		})
	}
	enc, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

// buildEnvOverlay computes env overlay from .env files per CLI flags.
// Order: auto-discovered files (if not --no-env-file), then explicit --env-file files.
// --mode <mode> appends NODE_ENV=<mode> at the end (highest precedence within the overlay).
func buildEnvOverlay(cwd string, leading leadingDispatchFlags) ([]string, error) {
	if leading.noEnvFile && len(leading.envFile) == 0 {
		if leading.mode != "" {
			return []string{"NODE_ENV=" + leading.mode}, nil
		}
		return nil, nil
	}

	explicit := len(leading.envFile) > 0
	var files []string
	if explicit {
		for _, f := range leading.envFile {
			if filepath.IsAbs(f) {
				files = append(files, f)
			} else {
				files = append(files, filepath.Join(cwd, f))
			}
		}
	} else {
		files = dotenv.Discover(cwd, leading.mode)
	}

	var envVars []string
	var err error
	if explicit {
		envVars, err = dotenv.LoadRequired(files)
	} else {
		envVars, err = dotenv.Load(files)
	}
	if err != nil {
		return nil, classifyEnvLoadError(err)
	}

	if leading.mode != "" {
		envVars = append(envVars, "NODE_ENV="+leading.mode)
	}
	return envVars, nil
}

// classifyEnvLoadError maps a dotenv load error to an apperr.Error with the
// appropriate stable code based on the underlying OS/filesystem error.
func classifyEnvLoadError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "no such file") || strings.Contains(msg, "cannot find") {
		return apperr.New(apperr.EnvFileNotFound, "env-file", "", err.Error())
	}
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "access is denied") {
		return apperr.New(apperr.EnvFileRead, "env-file", "", err.Error())
	}
	return apperr.New(apperr.EnvFileParse, "env-file", "", err.Error())
}

// emitEnvTrace emits trace events for environment source decisions.
// It reports the mode, source kind (explicit/discovered), and key names
// without values. Never emits environment variable values.
func emitEnvTrace(ctx context.Context, leading leadingDispatchFlags, overlay []string, cwd string) {
	if len(overlay) == 0 && leading.mode == "" {
		return
	}
	keys := make([]string, 0, len(overlay))
	for _, kv := range overlay {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				keys = append(keys, kv[:i])
				break
			}
		}
	}
	precedence := "discovered"
	if len(leading.envFile) > 0 {
		precedence = "explicit"
	}
	trace.Emit(ctx, trace.CatEnv, trace.TypeEnvSource, trace.EnvData{
		Mode:       leading.mode,
		Keys:       keys,
		Precedence: precedence,
	})
}
