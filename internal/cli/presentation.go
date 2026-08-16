package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/darkmode"
	"github.com/mewisme/mew/internal/diagnostics"
	"github.com/mewisme/mew/internal/presentation"
)

func (g *globalFlags) bindPresentation(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&g.output, "output", "", "output mode: rich | plain | json | ndjson | silent")
	cmd.PersistentFlags().StringVar(&g.logLevel, "log-level", "", "log level: error | warn | info | debug")
	cmd.PersistentFlags().BoolVar(&g.noColor, "no-color", false, "disable ANSI color")
	cmd.PersistentFlags().BoolVar(&g.noProgress, "no-progress", false, "disable progress output")
	cmd.PersistentFlags().BoolVar(&g.ascii, "ascii", false, "use ASCII instead of Unicode symbols")
	cmd.PersistentFlags().BoolVar(&g.noSummary, "no-summary", false, "suppress command summary output")
	cmd.PersistentFlags().BoolVar(&g.accessible, "accessible", false, "accessible append-only output")
}

// helpSettings returns presentation settings for help/usage template rendering.
// It avoids the full controller setup (no TTY detection) but uses the canonical
// theme resolution path so --theme dark/light/auto is respected consistently.
func (g *globalFlags) helpSettings() presentation.EffectiveSettings {
	useColor := !g.noColor
	useUnicode := !g.ascii
	var themeMode presentation.ThemeMode
	if !useColor {
		themeMode = presentation.ThemeNone
	} else if g.accessible {
		themeMode = presentation.ThemeAccessible
	} else {
		// Use canonical theme resolution from the configured theme flag.
		// Dark-mode detection is unavailable at help time; falls back to light.
		themeMode = presentation.ResolveTheme(g.theme, nil)
	}
	return presentation.EffectiveSettings{
		UseColor:   useColor,
		UseUnicode: useUnicode,
		ThemeMode:  themeMode,
		Width:      80,
		Symbols:    presentation.SelectSymbols(useUnicode),
		BinaryName: g.invokedBinary,
	}
}

// presentationInput builds resolver input from the parsed flags and the theme
// bootstrap resolved from ui.theme. Theme is empty only before bootstrap runs,
// which the resolver treats the same as "auto".
func (g *globalFlags) presentationInput() presentation.Input {
	return presentation.Input{
		OutputFlag:   g.output,
		NoColor:      g.noColor,
		ASCII:        g.ascii,
		NoProgress:   g.noProgress,
		Accessible:   g.accessible,
		NoSummary:    g.noSummary,
		LogLevelFlag: g.logLevel,
		Debug:        g.resolveDebug(),
		Unsafe:       g.unsafe,
		Theme:        g.theme,
		BinaryName:   g.invokedBinary,
	}
}

// presentationNewController is the production controller constructor behind
// newControllerFn.
func presentationNewController(resolved presentation.ResolvedOptions, caps presentation.Capabilities, streams presentation.StreamWriters) (presentation.Controller, error) {
	return presentation.NewController(resolved, caps, streams, darkmodeDetector{})
}

func (g *globalFlags) controller(cmd *cobra.Command) (presentation.Controller, error) {
	if g.ctrl != nil {
		return g.ctrl, nil
	}
	streams := presentation.WriterPair(cmd.OutOrStdout(), cmd.ErrOrStderr())
	caps := presentation.DetectCapabilities(cmd.InOrStdin(), streams.Out, streams.Err, nil)
	resolved, err := presentation.Resolve(g.presentationInput())
	if err != nil {
		return nil, err
	}
	ctrl, err := newControllerFn(resolved, caps, streams)
	if err != nil {
		return nil, err
	}
	g.ctrl = ctrl
	return ctrl, nil
}

// darkmodeDetector adapts the internal darkmode package to presentation.DarkModeDetector.
type darkmodeDetector struct{}

func (darkmodeDetector) IsDarkMode() (bool, error) {
	return darkmode.IsDarkMode()
}

// staticRenderer returns the design-system renderer for this invocation.
func (g *globalFlags) staticRenderer(cmd *cobra.Command) (presentation.StaticRenderer, error) {
	ctrl, err := g.controller(cmd)
	if err != nil {
		return nil, err
	}
	settings := ctrl.Settings()
	settings.BinaryName = g.invokedBinary
	return presentation.NewStaticRenderer(settings), nil
}

// mustStaticRenderer returns a plain renderer when controller setup fails.
func (g *globalFlags) mustStaticRenderer(cmd *cobra.Command) presentation.StaticRenderer {
	r, err := g.staticRenderer(cmd)
	if err != nil {
		return presentation.NewStaticRenderer(presentation.EffectiveSettings{
			ThemeMode:  presentation.ThemeNone,
			Width:      80,
			UseUnicode: false,
			Symbols:    presentation.ASCIISymbols,
			BinaryName: g.invokedBinary,
		})
	}
	return r
}

func (g *globalFlags) resolveReporter(cmd *cobra.Command) (diagnostics.Reporter, error) {
	ctrl, err := g.controller(cmd)
	if err != nil {
		return nil, err
	}
	return ctrl.Reporter(), nil
}

func (g *globalFlags) mustReporter(cmd *cobra.Command) diagnostics.Reporter {
	rep, err := g.resolveReporter(cmd)
	if err != nil {
		return diagnostics.NewReporter(diagnostics.Options{
			Out: cmd.OutOrStdout(),
			Err: cmd.ErrOrStderr(),
		})
	}
	return rep
}

func wrapPresentationErr(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *presentation.ConflictError, *presentation.InvalidModeError:
		return apperr.Wrap(apperr.Usage, "cli.presentation", "", err)
	default:
		return err
	}
}

func (g *globalFlags) validateStructuredConflict(cmd *cobra.Command) error {
	ctrl, err := g.controller(cmd)
	if err != nil {
		return wrapPresentationErr(err)
	}
	if cmd == nil {
		return nil
	}
	if f := cmd.Flags().Lookup("json"); f != nil && f.Changed {
		val, _ := cmd.Flags().GetBool("json")
		if val {
			if err := presentation.StructuredConflictsWithCommandJSON(ctrl.Options(), true); err != nil {
				return wrapPresentationErr(err)
			}
		}
	}
	return nil
}

// closePresentation closes the invocation controller exactly once, carrying the
// final outcome. The cleanup context drops the parent's cancellation so progress
// output is still flushed after Ctrl-C; the controller bounds it with its own
// timeout. A close failure is cosmetic and never masks the command error.
func closePresentation(cmd *cobra.Command, g *globalFlags, outcome presentation.Outcome) {
	if g == nil || g.ctrl == nil {
		return
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = cmd.Root().Context()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_ = g.ctrl.Close(context.WithoutCancel(ctx), outcome)
}

// ValidateCommandPresentation is exported for tests that need structured/json conflict checks.
func ValidateCommandPresentation(cmd *cobra.Command, g *globalFlags) error {
	if g == nil {
		return nil
	}
	return g.validateStructuredConflict(cmd)
}

func presentationOutcome(err error) presentation.Outcome {
	return presentation.Outcome{Err: err}
}
