package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/presentation"
	"github.com/mewisme/mew/internal/prompt"
)

func newConfigCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `Manage Mew configuration.`,
	}
	cmd.AddCommand(newConfigGetCmd(g))
	cmd.AddCommand(newConfigSetCmd(g))
	cmd.AddCommand(newConfigUnsetCmd(g))
	cmd.AddCommand(newConfigListCmd(g))
	cmd.AddCommand(newConfigExplainCmd(g))
	cmd.AddCommand(newConfigEditCmd(g))
	cmd.AddCommand(newConfigPathCmd(g))
	cmd.AddCommand(newConfigValidateCmd(g))
	cmd.AddCommand(newConfigMigrateCmd(g))
	cmd.AddCommand(newConfigResetCmd(g))
	return cmd
}

// ── get ───────────────────────────────────────────────────────

func newConfigGetCmd(g *globalFlags) *cobra.Command {
	var (
		flags   configWriteFlags
		verbose bool
	)
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Print a config value (default: user scope)",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeConfigKeys(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if err := flags.validateScope(); err != nil {
				return err
			}
			eff, err := invocationConfig(g)
			if err != nil {
				return err
			}
			scope := flags.resolvedScope()
			view, err := resolveConfigGet(eff, key, scope)
			if err != nil {
				if configOutputStructured(g, cmd) {
					// Structured consumers get the shape they expect even for a
					// key the scope does not declare; the typed error still sets
					// the exit code.
					if nse := (*notSetError)(nil); errors.As(err, &nse) {
						_ = writeConfigJSON(cmd, configGetNotSetView(eff, nse.key, scope).json())
					}
				}
				return err
			}

			if configOutputStructured(g, cmd) {
				return writeConfigJSON(cmd, view.json())
			}

			if !verbose {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), view.Entry.Value)
				return err
			}
			return writeStaticOut(cmd, renderConfigGetVerbose(g, cmd, view))
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show full metadata")
	return cmd
}

// renderConfigGetVerbose renders the metadata block for `config get -v`.
// Only semantically available fields appear.
func renderConfigGetVerbose(g *globalFlags, cmd *cobra.Command, view configGetView) string {
	r := g.mustStaticRenderer(cmd)
	kvs := []presentation.KeyValue{
		{Key: "Key", Value: view.Entry.Key},
	}
	if view.Entry.Configured {
		kvs = append(kvs, presentation.KeyValue{Key: "Configured", Value: view.Entry.Value})
	}
	if view.EffectiveKnown {
		kvs = append(kvs, presentation.KeyValue{Key: "Effective", Value: view.EffectiveValue})
	}
	kvs = append(kvs,
		presentation.KeyValue{Key: "Scope", Value: string(view.Entry.Scope)},
		presentation.KeyValue{Key: "Source", Value: view.Entry.Source},
	)
	if view.Entry.Path != "" {
		kvs = append(kvs, presentation.KeyValue{Key: "File", Value: view.Entry.Path, Style: presentation.ValuePath})
	}
	if spec := view.Spec; spec != nil {
		typeStr := string(spec.Type)
		if spec.Type == config.TypeEnum {
			typeStr = "enum"
		}
		kvs = append(kvs,
			presentation.KeyValue{Key: "Type", Value: typeStr},
			presentation.KeyValue{Key: "Default", Value: config.RedactString(view.Entry.Key, formatConfigValue(spec.Default))},
		)
		if len(spec.Enum) > 0 {
			kvs = append(kvs, presentation.KeyValue{Key: "Allowed", Value: strings.Join(spec.Enum, ", ")})
		}
	}
	if view.Entry.IsSecret {
		kvs = append(kvs, presentation.KeyValue{Key: "Is secret", Value: "true"})
	}
	return "\n" + r.KeyValues(kvs)
}

// ── set ───────────────────────────────────────────────────────

func newConfigSetCmd(g *globalFlags) *cobra.Command {
	var flags configWriteFlags
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config key (default: user scope)",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return completeConfigKeys(toComplete), cobra.ShellCompDirectiveNoFileComp
			}
			if len(args) == 1 {
				return completeConfigEnumValues(args[0]), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key, raw := args[0], args[1]
			if err := flags.validateWritable(); err != nil {
				return err
			}
			val, err := config.ParseValue(key, raw)
			if err != nil {
				return apperr.Wrap(apperr.Usage, "config.set", key, err)
			}
			scope := flags.resolvedScope()
			target, err := resolveConfigWriteTarget(scope, g.cwd)
			if err != nil {
				return err
			}
			if err := checkWritableScope(key, target.Scope); err != nil {
				return err
			}

			// Previous is the value this scope held, read from its own layer.
			// The effective winner may come from a different layer entirely, and
			// reporting that as "previous" would describe a value the write did
			// not replace.
			eff, err := invocationConfig(g)
			if err != nil {
				return err
			}
			prevDisplay, prevRaw, prevSet := scopeValueOrUnset(eff, scope, key)

			if err := config.SetFile(target.Path, key, val); err != nil {
				return err
			}

			// One reload republishes the invocation snapshot so the reported
			// target-scope and effective values both come from the same state.
			reloaded, reloadErr := reloadInvocationConfig(cmd.Context(), g)
			canon := canonicalConfigKey(key)
			currentDisplay := config.RedactString(canon, formatConfigValue(val))
			currentRaw := config.RedactValue(canon, val)
			effectiveDisplay := ""
			effectiveRaw := any(nil)
			effectiveSrc := ""
			if reloadErr == nil {
				if curDisplay, curRaw, ok := scopeValueOrUnset(reloaded, scope, key); ok {
					currentDisplay = curDisplay
					currentRaw = curRaw
				}
				if ev, eerr := config.GetEffective(reloaded, canon); eerr == nil {
					effectiveDisplay = config.RedactString(canon, formatConfigValue(ev.Raw))
					effectiveRaw = config.RedactValue(canon, ev.Raw)
					effectiveSrc = displayConfigSource(ev.Source)
				}
			}
			mv := configMutationView{
				Key:              canon,
				Scope:            scope,
				Path:             target.Path,
				Previous:         prevRaw,
				PreviousDisplay:  prevDisplay,
				PreviousSet:      prevSet,
				Current:          currentRaw,
				CurrentDisplay:   currentDisplay,
				EffectiveValue:   effectiveRaw,
				EffectiveDisplay: effectiveDisplay,
				EffectiveSource:  effectiveSrc,
				Changed:          true,
			}
			return writeConfigSetResult(g, cmd, mv)
		},
	}
	flags.bind(cmd)
	return cmd
}

func writeConfigSetResult(g *globalFlags, cmd *cobra.Command, mv configMutationView) error {
	if configOutputStructured(g, cmd) {
		return writeConfigJSON(cmd, mv.json())
	}
	if configOutputQuiet(g, cmd) {
		return nil
	}
	r := g.mustStaticRenderer(cmd)
	headline := fmt.Sprintf("%s Updated %s", r.Symbol(presentation.StatusSuccess), mv.Key)
	prevDisplay := mv.PreviousDisplay
	if !mv.PreviousSet {
		prevDisplay = "(unset)"
	}
	kvs := []presentation.KeyValue{
		{Key: "Previous", Value: prevDisplay},
		{Key: "Current", Value: mv.CurrentDisplay},
		{Key: "Scope", Value: string(mv.Scope)},
	}
	if mv.EffectiveDisplay != "" && mv.EffectiveDisplay != mv.CurrentDisplay {
		kvs = append(kvs, presentation.KeyValue{Key: "Effective", Value: mv.EffectiveDisplay})
	}
	kvs = append(kvs, presentation.KeyValue{Key: "File", Value: mv.Path, Style: presentation.ValuePath})
	return writeStaticOut(cmd, headline+"\n\n"+r.KeyValues(kvs))
}

// ── unset ─────────────────────────────────────────────────────

func newConfigUnsetCmd(g *globalFlags) *cobra.Command {
	var flags configWriteFlags
	cmd := &cobra.Command{
		Use:     "unset <key>",
		Aliases: []string{"rm"},
		Short:   "Remove a config key (default: user scope)",
		Args:    cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeConfigKeys(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if err := flags.validateWritable(); err != nil {
				return err
			}
			scope := flags.resolvedScope()
			target, err := resolveConfigWriteTarget(scope, g.cwd)
			if err != nil {
				return err
			}
			if err := checkWritableScope(key, target.Scope); err != nil {
				return err
			}
			canon := canonicalConfigKey(key)

			// Snapshot the value before removal so we can report what was removed.
			eff, err := invocationConfig(g)
			if err != nil {
				return err
			}
			prevDisplay, prevRaw, prevSet := scopeValueOrUnset(eff, scope, key)

			// UnsetFile writes nothing when the key is absent, so an already
			// unset key is an idempotent no-op and other layers are untouched.
			if err := config.UnsetFile(target.Path, key); err != nil {
				return err
			}

			// One reload, after the write, so the reported fallback is the layer
			// that actually wins now rather than the one that won before.
			var fallbackDisplay, fallbackSrc string
			var fallbackRaw any
			if reloaded, err := reloadInvocationConfig(cmd.Context(), g); err == nil {
				if v, gerr := config.GetEffective(reloaded, canon); gerr == nil {
					fallbackDisplay = config.RedactString(canon, formatConfigValue(v.Raw))
					fallbackRaw = config.RedactValue(canon, v.Raw)
					fallbackSrc = displayConfigSource(v.Source)
				}
			}
			mv := configMutationView{
				Key:              canon,
				Scope:            scope,
				Path:             target.Path,
				Previous:         prevRaw,
				PreviousDisplay:  prevDisplay,
				PreviousSet:      prevSet,
				Current:          fallbackRaw,
				CurrentDisplay:   fallbackDisplay,
				EffectiveValue:   fallbackRaw,
				EffectiveDisplay: fallbackDisplay,
				EffectiveSource:  fallbackSrc,
				Changed:          true,
			}
			return writeConfigUnsetResult(g, cmd, mv)
		},
	}
	flags.bind(cmd)
	return cmd
}

func writeConfigUnsetResult(g *globalFlags, cmd *cobra.Command, mv configMutationView) error {
	if configOutputStructured(g, cmd) {
		return writeConfigJSON(cmd, mv.json())
	}
	if configOutputQuiet(g, cmd) {
		return nil
	}
	r := g.mustStaticRenderer(cmd)
	headline := fmt.Sprintf("%s Removed %s from %s configuration",
		r.Symbol(presentation.StatusSuccess), mv.Key, mv.Scope)
	kvs := make([]presentation.KeyValue, 0, 3)
	if mv.CurrentDisplay != "" {
		kvs = append(kvs, presentation.KeyValue{Key: "Effective", Value: mv.CurrentDisplay})
	}
	if mv.EffectiveSource != "" {
		kvs = append(kvs, presentation.KeyValue{Key: "Source", Value: mv.EffectiveSource})
	}
	kvs = append(kvs, presentation.KeyValue{Key: "File", Value: mv.Path, Style: presentation.ValuePath})
	return writeStaticOut(cmd, headline+"\n\n"+r.KeyValues(kvs))
}

// ── list ──────────────────────────────────────────────────────

func newConfigListCmd(g *globalFlags) *cobra.Command {
	var (
		flags        configWriteFlags
		showOrigin   bool
		changed      bool
		inclDefaults bool
		prefix       string
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configuration",
		Long:    "List configuration values. Default scope is user.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validateScope(); err != nil {
				return err
			}
			scope := flags.resolvedScope()
			eff, err := invocationConfig(g)
			if err != nil {
				return err
			}
			opts := configListOptions{prefix: prefix, changed: changed, inclDefaults: inclDefaults}
			entries := resolveConfigList(eff, scope, opts)
			view := configListView{Scope: scope, Entries: entries, InclDefaults: inclDefaults}

			if configOutputStructured(g, cmd) {
				return writeConfigListJSON(cmd, view, showOrigin)
			}
			return writeConfigListHuman(g, cmd, view, showOrigin)
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVar(&showOrigin, "show-origin", false, "show value source and file")
	cmd.Flags().BoolVar(&changed, "changed", false, "show only values different from defaults")
	cmd.Flags().BoolVar(&inclDefaults, "defaults", false, "include registered schema defaults")
	cmd.Flags().StringVar(&prefix, "prefix", "", "filter keys by namespace prefix")
	return cmd
}

func writeConfigListHuman(g *globalFlags, cmd *cobra.Command, view configListView, showOrigin bool) error {
	r := g.mustStaticRenderer(cmd)
	settings := r.Settings()
	// Narrow terminals and accessible mode get one field per line; the same
	// threshold the shared KeyValues renderer uses.
	stacked := settings.Width < 60 || settings.Accessible
	theme := presentation.NewTheme(settings.ThemeMode)
	useColor := settings.UseColor
	dot := presentation.RenderSemanticSymbol(settings.Symbols, theme, presentation.StatusRunning, useColor)

	var b strings.Builder
	b.WriteString(configScopeLabel(view.Scope))
	b.WriteString("\n\n")

	var configured, defaulted int
	lastGroup := ""
	keyWidth := 0
	for _, e := range view.Entries {
		if w := presentation.CellWidth(e.Key); w > keyWidth {
			keyWidth = w
		}
	}
	for _, e := range view.Entries {
		if e.Configured {
			configured++
		} else {
			defaulted++
		}
		if e.Group != lastGroup {
			if lastGroup != "" {
				b.WriteString("\n")
			}
			group := e.Group
			if group == "" {
				group = "Other"
			}
			b.WriteString(group)
			b.WriteString("\n")
			lastGroup = e.Group
		}
		b.WriteString(configListRow(e, keyWidth, showOrigin, stacked, dot, theme, useColor))
		b.WriteString("\n")
	}
	if len(view.Entries) > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%d configured, %d defaults\n", configured, defaulted)
	if !view.InclDefaults && configured == 0 {
		fmt.Fprint(&b, "Add --defaults to see schema defaults.\n")
	}
	return writeStaticOut(cmd, b.String())
}

// configListRow renders one list row, padded to keyWidth so columns align on
// visible width rather than byte length.
func configListRow(e configEntryView, keyWidth int, showOrigin, stacked bool, dot string, theme presentation.Theme, useColor bool) string {
	indicator := "  "
	if e.Configured {
		indicator = dot + " "
	}
	key := e.Key
	if useColor && theme.Muted != nil {
		key = theme.Muted.Sprint(key)
	}
	if stacked {
		var b strings.Builder
		b.WriteString(indicator)
		b.WriteString(key)
		b.WriteString("\n    ")
		b.WriteString(e.Value)
		if showOrigin {
			b.WriteString("\n    ")
			b.WriteString(e.Source)
			if e.Path != "" {
				b.WriteString(" ")
				b.WriteString(e.Path)
			}
		}
		return b.String()
	}
	pad := keyWidth - presentation.CellWidth(e.Key)
	if pad < 0 {
		pad = 0
	}
	line := indicator + key + strings.Repeat(" ", pad+2) + e.Value
	if showOrigin {
		line += "  [" + e.Source + "]"
		if e.Path != "" {
			line += " " + e.Path
		}
	}
	return line
}

func configScopeLabel(scope configScope) string {
	switch scope {
	case configScopeProject:
		return "Project configuration"
	case configScopeEffective:
		return "Effective configuration"
	default:
		return "User configuration"
	}
}

func writeConfigListJSON(cmd *cobra.Command, view configListView, showOrigin bool) error {
	out := view.json()
	if !showOrigin {
		// Provenance detail is opt-in; the source name itself always stays.
		for i := range out.Entries {
			out.Entries[i].Path = ""
		}
	}
	return writeConfigJSON(cmd, out)
}

// ── explain ───────────────────────────────────────────────────

func newConfigExplainCmd(g *globalFlags) *cobra.Command {
	var flags configWriteFlags
	cmd := &cobra.Command{
		Use:   "explain <key>",
		Aliases: []string{"why"},
		Short: "Show config key resolution chain",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeConfigKeys(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if err := flags.validateScope(); err != nil {
				return err
			}
			eff, err := invocationConfig(g)
			if err != nil {
				return err
			}
			view, err := resolveConfigExplain(eff, key, flags.resolvedScope())
			if err != nil {
				return err
			}

			if configOutputStructured(g, cmd) {
				return writeConfigJSON(cmd, view.json())
			}
			return writeStaticOut(cmd, renderConfigExplainHuman(g, cmd, view))
		},
	}
	flags.bind(cmd)
	return cmd
}

func renderConfigExplainHuman(g *globalFlags, cmd *cobra.Command, view configResolutionView) string {
	r := g.mustStaticRenderer(cmd)
	var b strings.Builder
	fmt.Fprintf(&b, "%s = %s\n", view.Key, view.Effective.Value)
	if view.Spec != nil && view.Spec.Description != "" {
		b.WriteString(view.Spec.Description)
		b.WriteString("\n")
	}
	b.WriteString("\nResolution\n")

	// Layer rows align on the widest source name so the effective marker lines
	// up regardless of which layers are present.
	srcWidth := 0
	for _, l := range view.Layers {
		if w := presentation.CellWidth(l.Source); w > srcWidth {
			srcWidth = w
		}
	}
	for _, l := range view.Layers {
		pad := srcWidth - presentation.CellWidth(l.Source)
		if pad < 0 {
			pad = 0
		}
		b.WriteString("  " + l.Source + strings.Repeat(" ", pad+2) + l.Value)
		if l.Effective {
			b.WriteString("  <- effective")
		}
		b.WriteString("\n")
	}

	if view.Selected != nil {
		// Labelled so the requested-scope value cannot be mistaken for another
		// rung of the resolution chain above.
		b.WriteString("\nRequested scope\n")
		b.WriteString(r.KeyValues([]presentation.KeyValue{
			{Key: string(view.Selected.Scope), Value: view.Selected.Value},
		}))
		b.WriteString("\n")
	}

	if spec := view.Spec; spec != nil {
		b.WriteString("\nSchema\n")
		typeStr := string(spec.Type)
		if spec.Type == config.TypeEnum {
			typeStr = "enum"
		}
		kv := []presentation.KeyValue{
			{Key: "Type", Value: typeStr},
			{Key: "Default", Value: config.RedactString(view.Key, formatConfigValue(spec.Default))},
		}
		if len(spec.Enum) > 0 {
			kv = append(kv, presentation.KeyValue{Key: "Allowed", Value: strings.Join(spec.Enum, ", ")})
		}
		if rng := configRangeText(spec); rng != "" {
			kv = append(kv, presentation.KeyValue{Key: "Range", Value: rng})
		}
		if scopes := configScopeNames(spec); len(scopes) > 0 {
			kv = append(kv, presentation.KeyValue{Key: "Scopes", Value: strings.Join(scopes, ", ")})
		}
		if len(spec.Commands) > 0 {
			kv = append(kv, presentation.KeyValue{Key: "Used by", Value: strings.Join(spec.Commands, ", ")})
		}
		if env := config.EnvVar(view.Key); env != "" {
			kv = append(kv, presentation.KeyValue{Key: "Env var", Value: env})
		}
		if view.LegacyKey != "" {
			kv = append(kv, presentation.KeyValue{Key: "Legacy key", Value: view.LegacyKey})
		}
		if spec.Deprecated {
			kv = append(kv, presentation.KeyValue{Key: "Deprecated", Value: "true"})
		}
		if spec.Replacement != "" {
			kv = append(kv, presentation.KeyValue{Key: "Replaced by", Value: spec.Replacement})
		}
		if spec.Secret {
			kv = append(kv, presentation.KeyValue{Key: "Is secret", Value: "true"})
		}
		b.WriteString(r.KeyValues(kv))
		b.WriteString("\n")
	}
	return b.String()
}

// configRangeText renders an int key's min/max bounds, or "" when unbounded.
func configRangeText(spec *config.ConfigKeySpec) string {
	if spec.Minimum == nil && spec.Maximum == nil {
		return ""
	}
	switch {
	case spec.Minimum != nil && spec.Maximum != nil:
		return fmt.Sprintf("%d..%d", *spec.Minimum, *spec.Maximum)
	case spec.Minimum != nil:
		return fmt.Sprintf(">= %d", *spec.Minimum)
	default:
		return fmt.Sprintf("<= %d", *spec.Maximum)
	}
}

// ── edit ──────────────────────────────────────────────────────

func newConfigEditCmd(g *globalFlags) *cobra.Command {
	var flags configWriteFlags
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Open config file in editor (default: user scope)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validateWritable(); err != nil {
				return err
			}
			scope := flags.resolvedScope()
			target, err := resolveConfigWriteTarget(scope, g.cwd)
			if err != nil {
				return err
			}

			if err := editConfigFile(cmd.Context(), cmd, g, target.Path, configScopeToConfig(scope)); err != nil {
				return err
			}

			r := g.mustStaticRenderer(cmd)
			sym := r.Symbol(presentation.StatusSuccess)
			return writeStaticOut(cmd, sym+" Config saved\n\n"+r.KeyValues([]presentation.KeyValue{
				{Key: "Scope", Value: string(target.Scope)},
				{Key: "File", Value: target.Path, Style: presentation.ValuePath},
			}))
		},
	}
	flags.bind(cmd)
	return cmd
}

// ── path ──────────────────────────────────────────────────────

func newConfigPathCmd(g *globalFlags) *cobra.Command {
	var (
		flags configWriteFlags
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Print config file path (default: user scope)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			scope := flags.resolvedScope()
			userPath, projPath := resolveEffectivePaths(g.cwd)

			if configOutputStructured(g, cmd) {
				return writeConfigPathJSON(cmd, scope, userPath, projPath, all)
			}

			if all {
				r := g.mustStaticRenderer(cmd)
				kv := []presentation.KeyValue{
					{Key: "User", Value: userPath, Style: presentation.ValuePath},
				}
				if projPath != "" {
					kv = append(kv, presentation.KeyValue{Key: "Project", Value: projPath, Style: presentation.ValuePath})
				} else {
					kv = append(kv, presentation.KeyValue{Key: "Project", Value: "unavailable"})
				}
				return writeStaticOut(cmd, r.KeyValues(kv))
			}

			if scope == configScopeEffective {
				r := g.mustStaticRenderer(cmd)
				kv := []presentation.KeyValue{
					{Key: "User", Value: userPath, Style: presentation.ValuePath},
				}
				if projPath != "" {
					kv = append(kv, presentation.KeyValue{Key: "Project", Value: projPath, Style: presentation.ValuePath})
				}
				return writeStaticOut(cmd, r.KeyValues(kv))
			}

			target, err := resolveConfigWriteTarget(scope, g.cwd)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), target.Path)
			return err
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVar(&all, "all", false, "show all config file paths")
	return cmd
}

// configPathJSON is the stable typed model for structured config path output.
type configPathJSON struct {
	Scope    string   `json:"scope"`
	Selected string   `json:"selected,omitempty"`
	User     string   `json:"user"`
	Project  string   `json:"project,omitempty"`
	Paths    []string `json:"paths,omitempty"`
}

func writeConfigPathJSON(cmd *cobra.Command, scope configScope, userPath, projPath string, all bool) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)

	out := configPathJSON{
		Scope: string(scope),
		User:  userPath,
	}

	// --all shows every available raw config path.
	if all {
		out.Paths = []string{userPath}
		if projPath != "" {
			out.Paths = append(out.Paths, projPath)
			out.Project = projPath
		}
		return enc.Encode(out)
	}

	// Selected path depends on scope.
	switch scope {
	case configScopeUser:
		out.Selected = userPath
	case configScopeProject:
		out.Project = projPath
		out.Selected = projPath
	case configScopeEffective:
		out.Project = projPath
		// Effective shows both paths, no single "selected".
	}

	if out.Selected != "" {
		// Deterministic ordering for the paths list.
		out.Paths = []string{out.Selected}
	}

	return enc.Encode(out)
}

// ── validate ──────────────────────────────────────────────────

// newConfigValidateCmd validates the files a scope selects and fails when any
// of them is invalid.
//
// It owns no validation rules of its own: `internal/config` decides what is
// legal, so a document this command calls invalid is exactly a document that
// will not load. The command's job is target resolution, reporting, and exit
// status.
func newConfigValidateCmd(g *globalFlags) *cobra.Command {
	var (
		flags  configWriteFlags
		strict bool
	)
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate config file (default: user scope)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validateScope(); err != nil {
				return err
			}
			scope := flags.resolvedScope()
			paths, scopes, err := configValidateTargets(scope, g.cwd)
			if err != nil {
				return err
			}

			report := config.ValidateFiles(configScopeToConfig(scope), paths, scopes,
				config.ValidateOptions{Strict: strict})

			// The report is emitted first and always, then the outcome decides the
			// exit status. Invalid configuration never exits zero.
			if configOutputStructured(g, cmd) {
				if err := writeConfigJSON(cmd, configValidateJSON(scope, report)); err != nil {
					return err
				}
				// The report is the machine document for this command; suppress the
				// reporter so stdout does not also carry an unrelated error doc.
				return suppressReport(configValidateErr(report))
			}
			if err := writeStaticOut(cmd, configValidateView(g.mustStaticRenderer(cmd), scope, report)); err != nil {
				return err
			}
			return configValidateErr(report)
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVar(&strict, "strict", false, "treat legacy keys as errors")
	return cmd
}

// configValidateTargets resolves the files a scope selects, paired with the
// scope each file backs. Effective spans both files in resolution order.
func configValidateTargets(scope configScope, cwd string) ([]string, []config.Scope, error) {
	switch scope {
	case configScopeUser:
		return []string{config.GlobalConfigPath()}, []config.Scope{config.ScopeUser}, nil
	case configScopeProject:
		target, err := resolveConfigWriteTarget(configScopeProject, cwd)
		if err != nil {
			return nil, nil, err
		}
		return []string{target.Path}, []config.Scope{config.ScopeProject}, nil
	default:
		userPath, projPath := resolveEffectivePaths(cwd)
		paths := []string{userPath}
		scopes := []config.Scope{config.ScopeUser}
		if projPath != "" {
			paths = append(paths, projPath)
			scopes = append(scopes, config.ScopeProject)
		}
		return paths, scopes, nil
	}
}

// configValidateErr converts a report into the command's outcome. Warnings alone
// are not fatal: --strict already promoted them to errors inside the validator
// when the user asked for that.
func configValidateErr(report config.ValidationReport) error {
	errs := report.Errors()
	if len(errs) == 0 {
		return nil
	}
	subject := errs[0].File
	if k := errs[0].ReportedKey(); k != "" {
		subject += ":" + k
	}
	return apperr.New(apperr.Config, "config.validate", subject,
		fmt.Sprintf("invalid configuration: %d error(s)", len(errs)))
}

// configValidateJSON is the machine report: a small stable schema, no ANSI and
// no human headings. Unavailable fields are omitted rather than emitted empty.
func configValidateJSON(scope configScope, report config.ValidationReport) map[string]any {
	files := make([]any, 0, len(report.Files))
	for _, f := range report.Files {
		entry := map[string]any{
			"path":        f.Path,
			"valid":       f.Valid(),
			"keys":        f.KeyCount,
			"diagnostics": configDiagnosticsJSON(f.Diagnostics),
		}
		if f.Scope != "" {
			entry["scope"] = string(f.Scope)
		}
		files = append(files, entry)
	}
	return map[string]any{
		"scope":       string(scope),
		"valid":       report.Valid,
		"keys":        report.KeyCount(),
		"files":       files,
		"errors":      configDiagnosticsJSON(report.Errors()),
		"warnings":    configDiagnosticsJSON(report.Warnings()),
		"diagnostics": configDiagnosticsJSON(report.Diagnostics()),
	}
}

func configDiagnosticsJSON(ds []config.Diagnostic) []any {
	out := make([]any, 0, len(ds))
	for _, d := range ds {
		entry := map[string]any{
			"severity": string(d.Severity),
			"code":     string(d.Code),
			"message":  d.Message,
		}
		if k := d.ReportedKey(); k != "" {
			entry["key"] = k
		}
		if d.LegacyKey != "" && d.Key != "" {
			entry["canonical_key"] = d.Key
		}
		if d.File != "" {
			entry["path"] = d.File
		}
		if d.Replacement != "" {
			entry["replacement"] = d.Replacement
		}
		out = append(out, entry)
	}
	return out
}

// configValidateView renders the complete report for humans. Every collected
// diagnostic is printed, including warnings on an otherwise valid document.
func configValidateView(r presentation.StaticRenderer, scope configScope, report config.ValidationReport) string {
	label := strings.ToUpper(string(scope)[:1]) + string(scope)[1:] + " configuration"
	var b strings.Builder
	if report.Valid {
		fmt.Fprintf(&b, "%s %s is valid\n\n", r.Symbol(presentation.StatusSuccess), label)
	} else {
		fmt.Fprintf(&b, "%s %s is invalid\n\n", r.Symbol(presentation.StatusError), label)
	}
	b.WriteString(r.KeyValues([]presentation.KeyValue{
		{Key: "Keys", Value: strconv.Itoa(report.KeyCount())},
	}))
	if paths := configReportPaths(report); len(paths) > 0 {
		b.WriteString(r.KeyValues([]presentation.KeyValue{
			{Key: "File", Value: strings.Join(paths, "\n"), Style: presentation.ValuePath},
		}))
	}
	writeDiagnosticSection(&b, "Errors", report.Errors())
	writeDiagnosticSection(&b, "Warnings", report.Warnings())
	return b.String()
}

func configReportPaths(report config.ValidationReport) []string {
	out := make([]string, 0, len(report.Files))
	for _, f := range report.Files {
		out = append(out, f.Path)
	}
	return out
}

func writeDiagnosticSection(b *strings.Builder, title string, ds []config.Diagnostic) {
	if len(ds) == 0 {
		return
	}
	b.WriteString("\n" + title + ":\n")
	for _, d := range ds {
		b.WriteString("  " + d.String() + "\n")
		if d.File != "" {
			b.WriteString("  File " + d.File + "\n")
		}
	}
}

// ── migrate ───────────────────────────────────────────────────

func newConfigMigrateCmd(g *globalFlags) *cobra.Command {
	var (
		flags configWriteFlags
		check bool
	)
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate legacy config keys to canonical snake_case (default: user scope)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validateWritable(); err != nil {
				return err
			}
			scope := flags.resolvedScope()
			cwd := g.cwd
			target, err := resolveConfigWriteTarget(scope, cwd)
			if err != nil {
				return err
			}

			if check {
				plan, err := config.PlanMigration(target.Path)
				if err != nil {
					return err
				}
				return writeConfigMigrationCheck(g, cmd, plan, target)
			}

			plan, err := config.PlanMigration(target.Path)
			if err != nil {
				return err
			}
			count, err := plan.Apply()
			if err != nil {
				return err
			}
			return writeConfigMigrationResult(g, cmd, plan, count, target)
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVar(&check, "check", false, "dry-run: report what would change without writing")
	return cmd
}

// writeConfigMigrationCheck renders the --check report from the shared plan.
// Human and structured output use the same deterministic step ordering.
func writeConfigMigrationCheck(g *globalFlags, cmd *cobra.Command, plan config.MigrationPlan, target configWriteTarget) error {
	if configOutputStructured(g, cmd) {
		steps := make([]map[string]any, 0, len(plan.Steps))
		for _, s := range plan.Steps {
			steps = append(steps, map[string]any{
				"from": s.From, "to": s.To,
			})
		}
		if steps == nil {
			steps = []map[string]any{}
		}
		conflicts := plan.Conflicts
		if conflicts == nil {
			conflicts = []string{}
		}
		return writeConfigJSON(cmd, configMigrationCheckJSON{
			NeedsMigration: len(plan.Steps) > 0,
			Steps:          steps,
			Conflicts:      conflicts,
			Path:           target.Path,
		})
	}
	if plan.Empty() {
		return writeStaticOut(cmd, "Already canonical.")
	}
	if len(plan.Conflicts) > 0 {
		return plan.ConflictError()
	}
	var b strings.Builder
	r := g.mustStaticRenderer(cmd)
	b.WriteString(r.Symbol(presentation.StatusWarning) + " Configuration uses deprecated keys\n\n")
	for _, s := range plan.Steps {
		fmt.Fprintf(&b, "  %s\n    Use %s\n\n", s.From, s.To)
	}
	b.WriteString("Run `m config migrate` to update the file.\n")
	return writeStaticOut(cmd, b.String())
}

// writeConfigMigrationResult renders the apply result from the shared plan.
func writeConfigMigrationResult(g *globalFlags, cmd *cobra.Command, plan config.MigrationPlan, count int, target configWriteTarget) error {
	if configOutputStructured(g, cmd) {
		steps := make([]map[string]any, 0, len(plan.Steps))
		for _, s := range plan.Steps {
			steps = append(steps, map[string]any{
				"from": s.From, "to": s.To,
			})
		}
		if steps == nil {
			steps = []map[string]any{}
		}
		return writeConfigJSON(cmd, configMigrationApplyJSON{
			Migrated: count,
			Steps:    steps,
			Path:     target.Path,
		})
	}
	r := g.mustStaticRenderer(cmd)
	if count == 0 {
		return writeStaticOut(cmd, "Already canonical.")
	}
	sym := r.Symbol(presentation.StatusSuccess)
	return writeStaticOut(cmd, fmt.Sprintf("%s Migrated %d key(s) in %s\n\n%s",
		sym, count, target.Path,
		r.KeyValues([]presentation.KeyValue{
			{Key: "Scope", Value: string(target.Scope)},
			{Key: "File", Value: target.Path, Style: presentation.ValuePath},
		})))
}

// ── reset ─────────────────────────────────────────────────────

func newConfigResetCmd(g *globalFlags) *cobra.Command {
	var (
		flags configWriteFlags
		yes   bool
	)
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset config to defaults (default: user scope)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validateWritable(); err != nil {
				return err
			}
			scope := flags.resolvedScope()
			target, err := resolveConfigWriteTarget(scope, g.cwd)
			if err != nil {
				return err
			}

			// Missing file is an idempotent success.
			if fi, err := os.Lstat(target.Path); err != nil {
				if os.IsNotExist(err) {
					r := g.mustStaticRenderer(cmd)
					return writeStaticOut(cmd,
						fmt.Sprintf("No %s config file to reset.\n\n%s\n\nEffective: defaults",
							scope,
							r.KeyValues([]presentation.KeyValue{
								{Key: "File", Value: target.Path, Style: presentation.ValuePath},
							})))
				}
				return apperr.Wrap(apperr.IO, "config.reset", target.Path, err)
			} else if fi.IsDir() {
				return apperr.New(apperr.IO, "config.reset", target.Path,
					"unexpected directory at config path")
			} else if fi.Mode()&os.ModeSymlink != 0 {
				return apperr.New(apperr.IO, "config.reset", target.Path,
					"config path is a symlink; refusing to delete")
			}

			if !yes {
				prompter, canPrompt := g.ensurePrompter(cmd)
				if !canPrompt {
					return apperr.New(apperr.Usage, "config.reset", string(scope),
						"non-interactive session: use --yes to confirm reset")
				}
				answer, err := prompter.Prompt(cmd.Context(), prompt.PromptRequest{
					ID:          "config.reset",
					Kind:        prompt.PromptConfirm,
					Title:       fmt.Sprintf("Reset %s configuration?", scope),
					Description: fmt.Sprintf("This will delete the %s config file so defaults and lower layers become effective.", scope),
					Fields: []prompt.Field{
						{Key: "Scope", Value: string(target.Scope)},
						{Key: "File", Value: target.Path},
					},
					DefaultID: prompt.OptionReject,
					Dangerous: true,
				})
				if err != nil {
					return err
				}
				if answer.Cancelled || answer.OptionID != prompt.OptionApprove {
					return nil // clean exit without deleting
				}
			}

			if err := os.Remove(target.Path); err != nil {
				return apperr.Wrap(apperr.IO, "config.reset", target.Path, err)
			}
			r := g.mustStaticRenderer(cmd)
			sym := r.Symbol(presentation.StatusSuccess)
				return writeStaticOut(cmd, fmt.Sprintf("%s Reset %s configuration\n\n%s\n\nEffective: defaults",
					sym,
				scope,
				r.KeyValues([]presentation.KeyValue{
					{Key: "File", Value: target.Path, Style: presentation.ValuePath},
				})))
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

// ── completion helpers ────────────────────────────────────────

func completeConfigKeys(toComplete string) []string {
	var out []string
	for _, k := range config.RegisteredKeys() {
		if strings.HasPrefix(k, toComplete) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func completeConfigEnumValues(key string) []string {
	spec := config.KeySpec(key)
	if spec == nil || len(spec.Enum) == 0 {
		return nil
	}
	return spec.Enum
}

// ── helpers ───────────────────────────────────────────────────

// invocationConfig returns the one configuration snapshot bootstrap resolved
// for this invocation. Config commands never load a second time: --config, the
// project root, and the environment were all interpreted once, and reading them
// again here could disagree with what the rest of the invocation sees.
//
// A nil snapshot means the command was driven without going through the
// bootstrap path, which only happens in tests; loading once on that path keeps
// them working without giving production a second loader.
func invocationConfig(g *globalFlags) (*config.Effective, error) {
	if g != nil && g.snapshot != nil && g.snapshot.Config != nil {
		return g.snapshot.Config, nil
	}
	return reloadInvocationConfig(context.Background(), g)
}

// reloadInvocationConfig republishes the invocation snapshot from the same
// inputs bootstrap used, and returns the fresh effective config. Write commands
// call it exactly once after mutating a file so every value they report comes
// from one post-write state.
func reloadInvocationConfig(ctx context.Context, g *globalFlags) (*config.Effective, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	snap, err := loadConfigFn(ctx, app.Options{
		CWD:           g.cwd,
		ConfigPath:    g.configPath,
		Offline:       g.offline,
		PreferOffline: g.preferOffline,
	})
	if err != nil {
		return nil, err
	}
	g.snapshot = &snap
	g.cwd = snap.CWD
	return snap.Config, nil
}

// canonicalConfigKey returns the canonical spelling of key, or key itself when
// it is not a registered or legacy name.
func canonicalConfigKey(key string) string {
	if c := config.CanonicalKey(key); c != "" {
		return c
	}
	return key
}

// scopeValueOrUnset returns the display and raw values a raw scope holds for
// key, and whether the scope declares it at all. Both values are redacted when
// the key is secret.
func scopeValueOrUnset(eff *config.Effective, scope configScope, key string) (display string, raw any, set bool) {
	canon := canonicalConfigKey(key)
	v, err := config.GetAtScope(eff, configScopeToConfig(scope), canon)
	if err != nil {
		return "", nil, false
	}
	return config.RedactString(canon, formatConfigValue(v.Raw)), config.RedactValue(canon, v.Raw), true
}

// checkWritableScope enforces the schema's per-key writable scopes. The scope
// list comes from ConfigKeySpec so the CLI keeps no second copy of the policy.
// Unknown Mew-owned keys are rejected earlier by ParseValue and UnsetFile;
// dynamic registries.* keys have no spec and keep their existing any-scope
// policy.
func checkWritableScope(key string, scope configScope) error {
	spec := config.KeySpec(canonicalConfigKey(key))
	if spec == nil {
		return nil
	}
	target := configScopeToConfig(scope)
	for _, s := range spec.Scopes {
		if s == target {
			return nil
		}
	}
	if len(spec.Scopes) == 0 {
		return nil
	}
	return apperr.New(apperr.Usage, "config.write", spec.Key,
		fmt.Sprintf("%s is writable in %s scope; cannot write to %s config",
			spec.Key, strings.Join(configScopeNames(spec), ", "), scope))
}

func configOutputStructured(g *globalFlags, cmd *cobra.Command) bool {
	ctrl, err := g.controller(cmd)
	if err != nil {
		return false
	}
	return ctrl.Options().Structured()
}

func configOutputQuiet(g *globalFlags, cmd *cobra.Command) bool {
	ctrl, err := g.controller(cmd)
	if err != nil {
		return false
	}
	opts := ctrl.Options()
	return opts.Structured() || opts.Output == presentation.OutputSilent
}
