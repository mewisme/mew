package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/config"
)

// loadConfigFn is the configuration loader seam. Tests replace it to assert an
// invocation resolves effective configuration exactly once.
var loadConfigFn = app.LoadConfig

// newControllerFn is the presentation controller seam. Tests replace it to
// observe how many controllers an invocation creates.
var newControllerFn = presentationNewController

// bootstrapInvocation establishes invocation state before any command runs:
// parse invocation flags, classify the command, load configuration once,
// resolve ui.theme, then create the single presentation controller.
//
// A non-nil error while g.ctrl is still nil means presentation itself could not
// be built and the caller must fall back to a minimal plain reporter. A non-nil
// error with g.ctrl set is a configuration failure that normal commands fail
// closed on; repair and informational commands never produce one.
func bootstrapInvocation(ctx context.Context, root *cobra.Command, g *globalFlags, argv []string) error {
	selector, parseErr := g.preParseGlobals(root, argv)
	if parseErr != nil {
		return wrapPresentationErr(parseErr)
	}

	// Configuration is loaded for every invocation because ui.theme lives in
	// it, but only normal commands treat a load failure as fatal.
	g.theme = "auto"
	snap, cfgErr := loadConfigFn(ctx, app.Options{
		CWD:           g.cwd,
		ConfigPath:    g.configPath,
		Offline:       g.offline,
		PreferOffline: g.preferOffline,
	})
	if cfgErr == nil {
		g.snapshot = &snap
		// app.LoadConfig resolves CWD to an absolute path; adopt it so nothing
		// downstream resolves it a second time.
		g.cwd = snap.CWD
		g.theme = config.String(snap.Config, "ui.theme", "auto")
	}

	if _, err := g.controller(root); err != nil {
		return wrapPresentationErr(err)
	}
	if cfgErr != nil && configRequired(root, argv, selector) {
		return cfgErr
	}
	return nil
}

// reloadSnapshotForCWD reloads the invocation snapshot after a dispatcher
// discovers its own working directory. mx parses --cwd out of its child argv,
// which bootstrap cannot see, so the one snapshot is replaced rather than
// supplemented. Presentation keeps the theme it already resolved.
func reloadSnapshotForCWD(ctx context.Context, g *globalFlags, cwd string) error {
	if cwd == "" || cwd == g.cwd {
		return nil
	}
	snap, err := loadConfigFn(ctx, app.Options{
		CWD:           cwd,
		ConfigPath:    g.configPath,
		Offline:       g.offline,
		PreferOffline: g.preferOffline,
	})
	if err != nil {
		return err
	}
	g.snapshot = &snap
	g.cwd = snap.CWD
	return nil
}

// configRequired reports whether the invocation must have valid effective
// configuration. Config-repair and informational commands stay usable with a
// malformed config file.
func configRequired(root *cobra.Command, argv []string, selector string) bool {
	if isRootMetaInvocation(argv) {
		return false
	}
	// Bare invocation, or a pseudo-command Cobra does not register on the tree
	// until Execute, so it can only be classified by selector.
	if selector == "" || isInfoCommandName(selector) {
		return false
	}
	if target, _, err := root.Find(argv); err == nil && target != nil && target != root {
		return !isConfigRepairCommand(target) && !isInfoCommand(target)
	}
	return true
}

// preParseGlobals fills g's persistent-flag fields before Cobra parses them, so
// the presentation controller is built from real flag values. It returns the
// first positional token of the invocation.
func (g *globalFlags) preParseGlobals(root *cobra.Command, argv []string) (string, error) {
	target := resolveTargetCommand(root, argv)
	leading, selector := splitLeadingGlobals(root, target, argv)
	// Binds onto g through the same bind* functions the real root uses, so
	// there is no second flag table or default set to keep in sync.
	if err := newGlobalFlagSet(g, root, target).Parse(leading); err != nil {
		return selector, err
	}
	return selector, nil
}

// resolveTargetCommand returns the command Cobra would run for argv, or root.
func resolveTargetCommand(root *cobra.Command, argv []string) *cobra.Command {
	if root == nil {
		return nil
	}
	target, _, err := root.Find(argv)
	if err != nil || target == nil {
		return root
	}
	return target
}

// splitLeadingGlobals returns the argv prefix carrying Mew's own flags plus the
// first positional token. Flags after a non-builtin selector belong to the
// dispatched script, executable, or mx child and are never read as Mew flags;
// flags after a builtin command name are, matching Cobra. Nothing past "--" is
// scanned.
//
// The split point is found by pflag itself rather than a hand-written scanner,
// so it cannot disagree with the real parse in preParseGlobals about which
// tokens are flags or which of them take a value.
func splitLeadingGlobals(root, target *cobra.Command, argv []string) ([]string, string) {
	scan := argv
	for i, a := range argv {
		if a == "--" {
			scan = argv[:i]
			break
		}
	}

	fs := newGlobalFlagSet(&globalFlags{}, root, target)
	fs.SetInterspersed(false)
	// Parse failures surface from the real parse in preParseGlobals; here they
	// only mean the split falls back to the whole prefix.
	_ = fs.Parse(scan)
	rest := fs.Args()
	if len(rest) == 0 {
		return scan, ""
	}
	// Non-interspersed parsing appends the first positional and everything after
	// it verbatim, so the tail length locates the selector exactly.
	selector := rest[0]
	if kind, _ := lookupBuiltin(root, selector); kind != "" {
		return scan, selector
	}
	return scan[:len(scan)-len(rest)], selector
}

// newGlobalFlagSet returns a flag set that parses this invocation's Mew flags
// into dst. Unknown flags are tolerated because not every subcommand flag is
// registered here.
//
// Flags local to target shadow the persistent flag of the same name, exactly as
// Cobra resolves them, so `m capsule create --output <path>` sets the command's
// own flag and leaves the global output mode alone. Shadowed names are
// registered first as placeholders that discard their value, which keeps arity
// right while keeping the value away from dst.
func newGlobalFlagSet(dst *globalFlags, root, target *cobra.Command) *pflag.FlagSet {
	// Persistent flags come from the same bind* functions the real root uses, so
	// there is no second flag table or default set to keep in sync.
	tmp := &cobra.Command{Use: "bootstrap"}
	dst.bind(tmp)
	if root != nil && root.PersistentFlags().Lookup("recursive") != nil {
		dst.bindRecursive(tmp)
	}

	fs := pflag.NewFlagSet("bootstrap", pflag.ContinueOnError)
	// -h and -v are swallowed rather than reported as unknown flags.
	addPlaceholderFlag(fs, "help", "h", true)
	addPlaceholderFlag(fs, "version", "v", true)
	if target != nil && target != root {
		target.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f == nil || f.Name == "" {
				return
			}
			addPlaceholderFlag(fs, f.Name, f.Shorthand, f.NoOptDefVal != "")
		})
	}
	// Leading dispatch flags are not root persistent flags but still belong to
	// Mew. Their values are consumed by ParsePhaseA; only arity matters here, and
	// it comes from the same list the drift test checks.
	for _, name := range dispatchOnlyFlagNames() {
		switch name {
		case "workspace-concurrency", "workspace-order", "workspace-output", "loader":
			addPlaceholderFlag(fs, name, "", false)
		default:
			addPlaceholderFlag(fs, name, "", true)
		}
	}
	// AddFlagSet skips names already registered, so placeholders win.
	fs.AddFlagSet(tmp.PersistentFlags())

	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.Usage = func() {}
	fs.SetOutput(discardWriter{})
	return fs
}

// addPlaceholderFlag registers a flag whose value is discarded, preserving only
// whether it consumes the following argument. Names already present are left
// alone, and a shorthand already taken is dropped rather than colliding.
func addPlaceholderFlag(fs *pflag.FlagSet, name, shorthand string, valueOptional bool) {
	if name == "" || fs.Lookup(name) != nil {
		return
	}
	if shorthand != "" && fs.ShorthandLookup(shorthand) != nil {
		shorthand = ""
	}
	var sink string
	fs.StringVarP(&sink, name, shorthand, "", "")
	if valueOptional {
		// Matches how pflag marks a flag usable without a value (bools).
		fs.Lookup(name).NoOptDefVal = "true"
	}
}

// discardWriter swallows pflag's own error output; failures are reported through
// the diagnostics reporter instead.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
