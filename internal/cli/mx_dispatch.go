package cli

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/runner/dlx"
)

func tryMXDispatch(ctx context.Context, root *cobra.Command, g *globalFlags, info BuildInfo, argv []string) (int, bool) {
	if root == nil || (root.Name() != "mx" && root.Name() != "mewx") {
		return 0, false
	}
	code, handled, _ := runMXDispatch(ctx, root, g, info, argv)
	return code, handled
}

// runMXDispatch executes mx DLX dispatch. Callers must ensure the command is
// mx or intends to behave as mx. Returns the exit code, whether the argv was
// handled as an mx invocation, and the classified error (already reported).
func runMXDispatch(ctx context.Context, root *cobra.Command, g *globalFlags, info BuildInfo, argv []string) (int, bool, error) {
	if len(argv) == 0 {
		return 0, false, nil
	}
	selector := mxSelectorAfterFlags(argv)
	if selector != "" && (isRootMetaInvocation([]string{selector}) || isMXBuiltin(root, selector)) {
		return 0, false, nil
	}
	if selector == "" && !mxArgvLooksLikeDLX(argv) {
		return 0, false, nil
	}
	inv, err := ParseMXInvocation(argv)
	if err != nil {
		cerr := classifyCLIError(err)
		rep := g.newReporter(root)
		rep.Error(cerr)
		return apperr.ExitCode(cerr), true, cerr
	}
	if inv.Offline {
		g.offline = true
	}
	// mx parses --cwd out of child argv that bootstrap could not classify, so
	// the invocation snapshot is reloaded for that directory.
	if err := reloadSnapshotForCWD(ctx, g, inv.CWD); err != nil {
		cerr := classifyCLIError(err)
		rep := g.newReporter(root)
		rep.Error(cerr)
		return apperr.ExitCode(cerr), true, cerr
	}
	ac, err := buildAppContext(ctx, root, g, info)
	if err != nil {
		cerr := classifyCLIError(err)
		rep := g.newReporter(root)
		rep.Error(cerr)
		return apperr.ExitCode(cerr), true, cerr
	}
	if g.ctrl != nil {
		cmdLabel := inv.Command
		if cmdLabel == "" && len(inv.PackageSpecs) > 0 {
			cmdLabel = inv.PackageSpecs[0].Raw
		}
		g.ctrl.SetRunnerCommand(cmdLabel)
	}
	det := dlx.DefaultInteractivityDetector{}
	interactive := det.IsInteractive(os.Stdin)
	_, err = app.DLX(ctx, ac, app.DLXOptions{
		ModeA:         inv.ModeA,
		PackageSpecs:  inv.PackageSpecs,
		Command:       inv.Command,
		ForwardedArgs: inv.ForwardedArgs,
		AssumeYes:     inv.AssumeYes,
		Offline:       inv.Offline || g.offline,
		Interactive:   interactive,
		Stdin:         os.Stdin,
		Stderr:        os.Stderr,
		Stdout:        os.Stdout,
	})
	if err == nil {
		return 0, true, nil
	}
	cerr := classifyCLIError(err)
	rep := g.newReporter(root)
	rep.Error(cerr)
	return apperr.ExitCode(cerr), true, cerr
}

// mxSelectorAfterFlags returns the first positional token after leading mx and CLI global flags.
func mxSelectorAfterFlags(argv []string) string {
	i := skipMXLeadingArgs(argv)
	if i < len(argv) {
		return argv[i]
	}
	return ""
}

// skipMXLeadingArgs returns the index of the first positional argument.
func skipMXLeadingArgs(argv []string) int {
	i := 0
	for i < len(argv) {
		arg := argv[i]
		if !strings.HasPrefix(arg, "-") {
			return i
		}
		switch {
		case arg == "--yes", arg == "--offline", arg == "--debug", arg == "--no-color", arg == "--prefer-offline", arg == "--unsafe-diagnostics":
			i++
		case arg == "-p", arg == "--package", arg == "--cwd", arg == "--config":
			if i+1 >= len(argv) {
				return len(argv)
			}
			i += 2
		case strings.HasPrefix(arg, "--filter"):
			if strings.Contains(arg, "=") {
				i++
			} else if i+1 < len(argv) {
				i += 2
			} else {
				i++
			}
		case arg == "--":
			if i+1 < len(argv) {
				return i + 1
			}
			return len(argv)
		default:
			return i
		}
	}
	return i
}

// mxArgvLooksLikeDLX reports whether argv should be parsed as DLX rather than bare Cobra globals.
func mxArgvLooksLikeDLX(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	if mxSelectorAfterFlags(argv) != "" {
		return true
	}
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "-p", "--package", "--yes", "--offline":
			return true
		}
	}
	return false
}
