package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/manifest"
	"github.com/mewisme/mew/internal/project"
	"github.com/mewisme/mew/internal/runner"
	"github.com/mewisme/mew/internal/runtime"
	"github.com/mewisme/mew/internal/workspace"
)

// ScriptInvocation is the normalized script dispatch request shared by m <script> and m run.
type ScriptInvocation struct {
	Selector         string
	ForwardedArgs    []string
	IfPresent        bool
	Recursive        bool
	Filters          []string
	Concurrency      int
	Order            runner.WorkspaceOrder
	Output           runner.WorkspaceOutputMode
	Bail             bool
	WorkspaceOnlySet bool
}

// ToRunOptions converts the invocation into app.RunOptions.
func (inv ScriptInvocation) ToRunOptions() app.RunOptions {
	return app.RunOptions{
		Selector:         inv.Selector,
		IfPresent:        inv.IfPresent,
		ForwardedArgs:    append([]string(nil), inv.ForwardedArgs...),
		Recursive:        inv.Recursive,
		Filters:          append([]string(nil), inv.Filters...),
		Concurrency:      inv.Concurrency,
		Order:            inv.Order,
		Output:           inv.Output,
		Bail:             inv.Bail,
		WorkspaceOnlySet: inv.WorkspaceOnlySet,
	}
}

// leadingDispatchFlags holds root globals and workspace runner flags parsed before the selector.
type leadingDispatchFlags struct {
	output        string
	debug         bool
	noColor       bool
	unsafe        bool
	cwd           string
	configPath    string
	offline       bool
	preferOffline bool
	filter        []string
	recursive     bool

	ifPresent     bool
	wsConcurrency int
	wsOrder       string
	wsOutput      string
	wsBail        bool
	wsBailSet     bool
	noWsBail      bool
	wsOnlyTouched bool

	node    bool
	loaders []string

	envFile   []string
	noEnvFile bool
	mode      string

	v8Args []string // collected --inspect, --inspect-brk, and other V8 flags
}

// PhaseAResult is the output of the leading-global parser (Phase A).
type PhaseAResult struct {
	Selector      string
	ForwardedArgs []string
	Leading       leadingDispatchFlags
	BareM         bool
}

// DispatchOutcomeKind is the resolved dispatch target for diagnostics and __dispatch.
type DispatchOutcomeKind string

const (
	OutcomeBuiltin DispatchOutcomeKind = "builtin"
	OutcomeAlias   DispatchOutcomeKind = "alias"
	OutcomeScript  DispatchOutcomeKind = "script"
	OutcomeFileRun DispatchOutcomeKind = "fileRun"
	OutcomeBin     DispatchOutcomeKind = "bin"
	OutcomeSuggest DispatchOutcomeKind = "suggest"
	OutcomeUnknown DispatchOutcomeKind = "unknown"
	OutcomeBareM   DispatchOutcomeKind = "bare"
)

// BinInvocation is the normalized bin dispatch request for direct m <bin>.
type BinInvocation struct {
	Selector      string
	ForwardedArgs []string
	Filters       []string
}

// ToExecOptions converts the invocation into app.ExecOptions.
func (inv BinInvocation) ToExecOptions() app.ExecOptions {
	return app.ExecOptions{
		Command:         inv.Selector,
		ForwardedArgs:   append([]string(nil), inv.ForwardedArgs...),
		Filters:         append([]string(nil), inv.Filters...),
		RequireVerified: true,
	}
}

// DispatchResult is the Phase B resolution for a selector.
type DispatchResult struct {
	Kind         DispatchOutcomeKind
	Canonical    string
	Invocation   *ScriptInvocation
	Bin          *BinInvocation
	FileRunPath  string // resolved absolute path for OutcomeFileRun
	Suggestions  []Suggestion
	DirectGateOn bool
	Message      string
	Err          error
}

// rootPersistentFlagNames returns sorted long names for root persistent flags (drift test).
func rootPersistentFlagNames(root *cobra.Command) []string {
	if root == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var names []string
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f == nil || f.Name == "" {
			return
		}
		if _, ok := seen[f.Name]; ok {
			return
		}
		seen[f.Name] = struct{}{}
		names = append(names, f.Name)
	})
	sort.Strings(names)
	return names
}

// phaseAParserFlagNames returns flags Phase A must understand (root persistent + dispatch workspace flags).
func phaseAParserFlagNames(root *cobra.Command) []string {
	base := rootPersistentFlagNames(root)
	extra := dispatchOnlyFlagNames()
	seen := map[string]struct{}{}
	var out []string
	for _, n := range base {
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	for _, n := range extra {
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// dispatchOnlyFlagNames are leading flags ParsePhaseA accepts that are not root
// persistent flags. Bootstrap registers them too so a legitimate leading flag is
// never mistaken for a dispatched child's argument.
func dispatchOnlyFlagNames() []string {
	return []string{
		"if-present",
		"workspace-concurrency",
		"workspace-order",
		"workspace-output",
		"workspace-bail",
		"no-workspace-bail",
		"node",
		"loader",
		"env-file",
		"no-env-file",
		"mode",
		"inspect",
		"inspect-brk",
	}
}

// ParsePhaseA parses leading globals and returns the selector plus verbatim post-selector args.
func ParsePhaseA(args []string) (PhaseAResult, error) {
	var leading leadingDispatchFlags
	leading.wsBail = true
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			return PhaseAResult{BareM: true, Leading: leading}, nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			selector := arg
			forwarded := stripScriptSeparator(append([]string(nil), args[i+1:]...))
			return PhaseAResult{Selector: selector, ForwardedArgs: forwarded, Leading: leading}, nil
		}
		consumed, err := consumeLeadingFlag(arg, args, i, &leading)
		if err != nil {
			return PhaseAResult{}, err
		}
		if consumed == 0 {
			return PhaseAResult{}, apperr.New(apperr.Usage, "dispatch", arg, fmt.Sprintf("unknown flag %q", arg))
		}
		i += consumed
	}
	return PhaseAResult{BareM: true, Leading: leading}, nil
}

func consumeLeadingFlag(arg string, args []string, i int, leading *leadingDispatchFlags) (int, error) {
	if arg == "-r" {
		leading.recursive = true
		return 1, nil
	}
	if arg == "-h" || arg == "--help" || arg == "-v" || arg == "--version" {
		return 1, nil
	}
	name, value, hasValue, inline := splitFlag(arg)
	if name == "" {
		return 0, apperr.New(apperr.Usage, "dispatch", arg, fmt.Sprintf("unknown flag %q", arg))
	}
	if !hasValue && needsValue(name) {
		if i+1 >= len(args) {
			return 0, apperr.New(apperr.Usage, "dispatch", name, fmt.Sprintf("flag %q requires a value", name))
		}
		value = args[i+1]
		inline = false
	}
	switch name {
	case "output":
		leading.output = value
	case "debug":
		leading.debug = parseBoolValue(value, true)
	case "no-color":
		leading.noColor = parseBoolValue(value, true)
	case "unsafe-diagnostics":
		leading.unsafe = parseBoolValue(value, true)
	case "cwd":
		leading.cwd = value
	case "config":
		leading.configPath = value
	case "offline":
		leading.offline = parseBoolValue(value, true)
	case "prefer-offline":
		leading.preferOffline = parseBoolValue(value, true)
	// Presentation-only root flags. Bootstrap parses these onto globalFlags
	// before the controller exists, so Phase A only needs to consume them with
	// the right arity rather than reject them as unknown.
	case "log-level", "no-progress", "ascii", "no-summary", "accessible":
	case "filter":
		leading.filter = append(leading.filter, value)
	case "recursive":
		leading.recursive = parseBoolValue(value, true)
	case "if-present":
		leading.ifPresent = parseBoolValue(value, true)
	case "workspace-concurrency":
		n, err := strconv.Atoi(value)
		if err != nil {
			return 0, apperr.Wrap(apperr.Usage, "dispatch", name, err)
		}
		leading.wsConcurrency = n
		leading.wsOnlyTouched = true
	case "workspace-order":
		leading.wsOrder = value
		leading.wsOnlyTouched = true
	case "workspace-output":
		leading.wsOutput = value
		leading.wsOnlyTouched = true
	case "workspace-bail":
		leading.wsBail = parseBoolValue(value, true)
		leading.wsBailSet = true
		leading.wsOnlyTouched = true
	case "no-workspace-bail":
		leading.noWsBail = true
		leading.wsOnlyTouched = true
	case "loader":
		leading.loaders = append(leading.loaders, value)
	case "node":
		leading.node = parseBoolValue(value, true)
	case "env-file":
		leading.envFile = append(leading.envFile, value)
	case "no-env-file":
		leading.noEnvFile = parseBoolValue(value, true)
	case "mode":
		leading.mode = value
	case "inspect":
		leading.v8Args = append(leading.v8Args, argToV8Flag(arg, name, value, inline, hasValue))
	case "inspect-brk":
		leading.v8Args = append(leading.v8Args, argToV8Flag(arg, name, value, inline, hasValue))
	default:
		return 0, apperr.New(apperr.Usage, "dispatch", name, fmt.Sprintf("unknown flag %q", name))
	}
	if inline || !needsValue(name) {
		return 1, nil
	}
	return 2, nil
}

func needsValue(name string) bool {
	switch name {
	case "output", "log-level", "cwd", "config", "filter", "workspace-concurrency", "workspace-order", "workspace-output", "loader", "env-file", "mode":
		return true
	default:
		return false
	}
}

// argToV8Flag reconstructs a V8 flag argument from the split parts for
// passthrough to Node. Flags like --inspect and --inspect-brk are consumed
// by Phase A but forwarded verbatim to Node's argument vector.
func argToV8Flag(arg, name string, value string, inline, hasValue bool) string {
	if hasValue {
		if inline {
			return "--" + name + "=" + value
		}
		return "--" + name + " " + value
	}
	return "--" + name
}

// parseBoolValue returns the boolean interpretation of a flag value.
// If no value is given (bare flag), defaultVal is returned.
// Otherwise "false", "0", "no", "off" map to false; everything else to true.
func parseBoolValue(value string, defaultVal bool) bool {
	if value == "" {
		return defaultVal
	}
	switch strings.ToLower(value) {
	case "false", "0", "no", "off":
		return false
	}
	return true
}

func splitFlag(arg string) (name, value string, hasValue, inline bool) {
	if !strings.HasPrefix(arg, "-") {
		return "", "", false, false
	}
	body := strings.TrimPrefix(arg, "--")
	if body == "" {
		return "", "", false, false
	}
	if idx := strings.IndexByte(body, '='); idx >= 0 {
		return body[:idx], body[idx+1:], true, true
	}
	if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) == 2 {
		return arg[1:], "", false, false
	}
	return body, "", false, false
}

func stripScriptSeparator(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return append([]string(nil), args[1:]...)
	}
	return args
}

func forwardedScriptArgs(args []string, dashIdx int) []string {
	var rest []string
	if dashIdx >= 0 {
		rest = append([]string(nil), args[dashIdx:]...)
	} else if len(args) > 1 {
		rest = append([]string(nil), args[1:]...)
	}
	return stripScriptSeparator(rest)
}

// BuildScriptInvocation assembles a ScriptInvocation from selector, args, and flags.
func BuildScriptInvocation(selector string, args []string, dashIdx int, leading leadingDispatchFlags) (ScriptInvocation, error) {
	var order runner.WorkspaceOrder
	var output runner.WorkspaceOutputMode
	if leading.wsOrder != "" {
		o, err := runner.ParseWorkspaceOrder(leading.wsOrder)
		if err != nil {
			return ScriptInvocation{}, err
		}
		order = o
	}
	if leading.wsOutput != "" {
		o, err := runner.ParseWorkspaceOutput(leading.wsOutput)
		if err != nil {
			return ScriptInvocation{}, err
		}
		output = o
	}
	if err := runner.ValidateConcurrency(leading.wsConcurrency); err != nil {
		return ScriptInvocation{}, err
	}
	bail := leading.wsBail
	if leading.noWsBail {
		bail = false
	}
	return ScriptInvocation{
		Selector:         selector,
		ForwardedArgs:    forwardedScriptArgs(args, dashIdx),
		IfPresent:        leading.ifPresent,
		Recursive:        leading.recursive,
		Filters:          append([]string(nil), leading.filter...),
		Concurrency:      leading.wsConcurrency,
		Order:            order,
		Output:           output,
		Bail:             bail,
		WorkspaceOnlySet: leading.wsOnlyTouched,
	}, nil
}

func applyLeadingToGlobalFlags(g *globalFlags, leading leadingDispatchFlags) {
	if leading.output != "" {
		g.output = leading.output
	}
	if leading.debug {
		g.debug = true
	}
	if leading.noColor {
		g.noColor = true
	}
	if leading.unsafe {
		g.unsafe = true
	}
	if leading.cwd != "" {
		g.cwd = leading.cwd
	}
	if leading.configPath != "" {
		g.configPath = leading.configPath
	}
	if leading.offline {
		g.offline = true
	}
	if leading.preferOffline {
		g.preferOffline = true
	}
	if len(leading.filter) > 0 {
		g.filter = append([]string(nil), leading.filter...)
	}
	if leading.recursive {
		g.recursive = true
	}
}

func lookupBuiltin(root *cobra.Command, name string) (DispatchOutcomeKind, string) {
	// SetHelpCommand registers help outside Commands() until Cobra Execute;
	// without this, Phase A direct dispatch treats "help" as an unknown selector.
	if name == "help" {
		return OutcomeBuiltin, "help"
	}
	for _, c := range root.Commands() {
		if c.Name() == name {
			return OutcomeBuiltin, c.Name()
		}
		for _, a := range c.Aliases {
			if a == name {
				return OutcomeAlias, c.Name()
			}
		}
	}
	return "", ""
}

func loadManifestScripts(cwd string) (map[string]string, error) {
	root, err := project.FindRoot(cwd)
	if err != nil {
		return nil, err
	}
	doc, err := manifest.LoadCached(root)
	if err != nil {
		return nil, err
	}
	if doc == nil || len(doc.Scripts) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(doc.Scripts))
	for k, v := range doc.Scripts {
		out[k] = v
	}
	return out, nil
}

func scriptNames(scripts map[string]string) []string {
	names := make([]string, 0, len(scripts))
	for name := range scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func exactScript(scripts map[string]string, selector string) bool {
	_, ok := scripts[selector]
	return ok
}

// ResolveDispatch performs Phase B resolution for a parsed selector.
func ResolveDispatch(root *cobra.Command, phase PhaseAResult, cwd string, eff *config.Effective) DispatchResult {
	directOn := DirectScriptsEnabled(eff)
	reserved := reservedSetForRoot(root)

	if phase.BareM {
		return DispatchResult{Kind: OutcomeBareM}
	}

	selector := phase.Selector
	if kind, canonical := lookupBuiltin(root, selector); kind != "" {
		return DispatchResult{Kind: kind, Canonical: canonical, DirectGateOn: directOn}
	}

	builtinNames, aliasNames := builtinAndAliasNames(root)
	builtinSug := suggestFromNames(selector, builtinNames, DispatchBuiltin, formatBuiltinInvocation)
	aliasSug := suggestFromNames(selector, aliasNames, DispatchAlias, formatBuiltinInvocation)

	scripts, manifestErr := loadManifestScripts(cwd)
	scriptSug := []Suggestion(nil)
	if manifestErr == nil {
		scriptSug = suggestFromNames(selector, scriptNames(scripts), DispatchScript, func(name string) string {
			_, reservedName := reserved[name]
			return formatScriptInvocation(name, reservedName, directOn)
		})
	}

	if directOn {
		workspaceMode := phase.Leading.recursive || len(phase.Leading.filter) > 0 || phase.Leading.wsOnlyTouched
		if _, isReserved := reserved[selector]; !isReserved && (workspaceMode || (manifestErr == nil && exactScript(scripts, selector))) {
			inv, err := BuildScriptInvocation(selector, append([]string{selector}, phase.ForwardedArgs...), -1, phase.Leading)
			if err != nil {
				return DispatchResult{Kind: OutcomeUnknown, Err: err, DirectGateOn: directOn}
			}
			if workspaceMode {
				if eff == nil || !workspace.Enabled(eff) {
					return DispatchResult{
						Kind: OutcomeUnknown,
						Err: apperr.New(apperr.Usage, "dispatch", selector,
							"workspaces not enabled; set MEW_EXPERIMENTAL_WORKSPACES=1 or workspaces.enabled in config"),
						DirectGateOn: directOn,
					}
				}
				return DispatchResult{Kind: OutcomeScript, Canonical: selector, Invocation: &inv, DirectGateOn: directOn}
			}
			return DispatchResult{Kind: OutcomeScript, Canonical: selector, Invocation: &inv, DirectGateOn: directOn}
		}
	}

	// Detect runtime file selectors for direct execution (after scripts, before bins).
	// Exact package scripts win over bare file names per documented dispatch precedence.
	if RuntimeEnabled() && runtime.IsRuntimeFile(selector) {
		// Deferred extensions (.jsx) → actionable plan-deferral message.
		if plan, ok := runtime.IsNextPlanExt(selector); ok {
			return DispatchResult{
				Kind:      OutcomeUnknown,
				Canonical: selector,
				Err: apperr.New(apperr.RuntimeEntrypoint, "dispatch", selector,
					fmt.Sprintf("%s: TypeScript JSX/TSX execution is planned for Mew plan %s; not yet available", selector, plan)),
				DirectGateOn: directOn,
			}
		}
		resolved, err := runtime.ResolveEntrypoint(cwd, selector)
		if err != nil {
			return DispatchResult{Kind: OutcomeUnknown, Canonical: selector, Err: err, DirectGateOn: directOn}
		}
		return DispatchResult{Kind: OutcomeFileRun, Canonical: selector, FileRunPath: resolved, DirectGateOn: directOn}
	}

	binDirectOn := DirectDispatchBinsEnabled(eff)
	if res, handled := tryDirectBinDispatch(phase, cwd, binDirectOn); handled {
		return res
	}

	if !directOn && manifestErr == nil && exactScript(scripts, selector) {
		if _, isReserved := reserved[selector]; !isReserved {
			return DispatchResult{
				Kind:         OutcomeSuggest,
				Canonical:    selector,
				DirectGateOn: false,
				Err:          apperr.New(apperr.Usage, "dispatch", selector, gateOffExactScriptMessage(selector)),
			}
		}
	}

	suggestions := mergeSuggestions(builtinSug, aliasSug, scriptSug)
	if len(suggestions) > 0 {
		return DispatchResult{
			Kind:         OutcomeSuggest,
			Canonical:    selector,
			Suggestions:  suggestions,
			DirectGateOn: directOn,
			Err:          apperr.New(apperr.Usage, "dispatch", selector, formatSuggestionMessage(selector, suggestions, directOn)),
		}
	}

	if manifestErr != nil {
		if apperr.CodeOf(manifestErr) == apperr.Manifest {
			return DispatchResult{Kind: OutcomeUnknown, Err: manifestErr, DirectGateOn: directOn}
		}
		workspaceMode := phase.Leading.recursive || len(phase.Leading.filter) > 0 || phase.Leading.wsOnlyTouched
		attemptedScript := directOn && (workspaceMode || exactScript(scripts, selector) || isPlausibleScriptSelector(selector))
		if apperr.CodeOf(manifestErr) == apperr.NotFound && leadingHasExplicitCWD(phase.Leading) && attemptedScript && len(suggestions) == 0 {
			return DispatchResult{Kind: OutcomeUnknown, Err: manifestErr, DirectGateOn: directOn}
		}
	}

	return DispatchResult{
		Kind:         OutcomeUnknown,
		Canonical:    selector,
		DirectGateOn: directOn,
		Err:          apperr.New(apperr.Usage, "dispatch", selector, fmt.Sprintf("unknown command %q", selector)),
	}
}

func leadingHasExplicitCWD(leading leadingDispatchFlags) bool {
	return leading.cwd != ""
}

// isPlausibleScriptSelector filters arbitrary unknown phrases from script-intent NotFound.
func isPlausibleScriptSelector(selector string) bool {
	if selector == "" || len(selector) > 128 {
		return false
	}
	if strings.Count(selector, "-") >= 2 {
		return false
	}
	return true
}

func gateOffExactScriptMessage(script string) string {
	return fmt.Sprintf("Direct script shortcuts are disabled.\n\nDid you mean the package script %q?\n\nRun it explicitly with:\n  m run %s", script, script)
}

func formatSuggestionMessage(selector string, suggestions []Suggestion, directOn bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "unknown command %q", selector)
	if !directOn {
		b.WriteString("\n\nDirect script shortcuts are disabled.")
	}
	if len(suggestions) > 0 {
		b.WriteString("\n\nDid you mean:")
		for _, s := range suggestions {
			b.WriteString("\n  ")
			b.WriteString(s.Invocation)
		}
	}
	return b.String()
}

func builtinAndAliasNames(root *cobra.Command) (builtins, aliases []string) {
	for _, c := range root.Commands() {
		if c.Name() != "" {
			builtins = append(builtins, c.Name())
		}
		aliases = append(aliases, c.Aliases...)
	}
	sort.Strings(builtins)
	sort.Strings(aliases)
	return builtins, aliases
}

func listBareMScripts(cwd string) ([]string, int, error) {
	scripts, err := loadManifestScripts(cwd)
	if err != nil {
		if apperr.CodeOf(err) == apperr.Manifest {
			return nil, 0, err
		}
		return nil, 0, nil
	}
	names := scriptNames(scripts)
	return names, len(names), nil
}

const bareMScriptListLimit = 10

func bareMUsageMessage(cwd, bin string) string {
	if bin == "" {
		bin = "m"
	}
	names, total, err := listBareMScripts(cwd)
	if err != nil || len(names) == 0 {
		return fmt.Sprintf("usage: %s <command> [args]\n\nRun %s run <script> to execute a package script.", bin, bin)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "usage: %s <command> [args]\n\nAvailable scripts:", bin)
	show := names
	if len(show) > bareMScriptListLimit {
		show = show[:bareMScriptListLimit]
	}
	for _, name := range show {
		b.WriteString("\n  ")
		b.WriteString(name)
	}
	if total > bareMScriptListLimit {
		fmt.Fprintf(&b, "\n  … and %d more", total-bareMScriptListLimit)
	}
	fmt.Fprintf(&b, "\n\nRun %s run <script> to execute a package script.", bin)
	return b.String()
}

func isRootMetaInvocation(args []string) bool {
	for _, a := range args {
		switch a {
		case "--help", "-h", "--version", "-v":
			return true
		}
	}
	return false
}
