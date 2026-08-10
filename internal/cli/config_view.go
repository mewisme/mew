package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/presentation"
)

// This file holds the view models every config reader renders through. Human
// and structured output share them so source naming, secret redaction, default
// detection, and scope conversion have exactly one implementation each. The
// models carry resolved data only: they read no files and resolve no config.

// ── scope conversion ──────────────────────────────────────────

// configScopeToConfig maps the CLI scope flag onto the config package scope.
// The two enums carry the same names; this is the single conversion point so
// no renderer invents its own mapping.
func configScopeToConfig(s configScope) config.Scope {
	switch s {
	case configScopeProject:
		return config.ScopeProject
	case configScopeEffective:
		return config.ScopeEffective
	default:
		return config.ScopeUser
	}
}

// configScopeSource returns the layer backing a raw scope. Effective spans
// every layer and has none, matching config.SourceForScope.
func configScopeSource(s configScope) (config.Source, bool) {
	return config.SourceForScope(configScopeToConfig(s))
}

// displayConfigSource renames the internal "global" layer to the user-facing
// "user". Every rendered source string passes through here.
func displayConfigSource(src config.Source) string {
	if src == config.SourceGlobal {
		return "user"
	}
	return string(src)
}

// ── typed errors ──────────────────────────────────────────────

// notSetError reports a key that is registered but carries no value at the
// requested raw scope. It wraps config.ErrNotSet so errors.Is keeps working
// through the CLI boundary, while presenting a typed ERR_M_CONFIG to the user.
type notSetError struct {
	key   string
	scope configScope
	err   *apperr.Error
}

func (e *notSetError) Error() string { return e.err.Error() }

// Unwrap exposes both the typed apperr and config.ErrNotSet. errors.Is walks
// the slice, so errors.Is(err, config.ErrNotSet) and apperr.CodeOf both
// resolve on the same value.
func (e *notSetError) Unwrap() []error { return []error{e.err, config.ErrNotSet} }

// newNotSetError builds the typed error for a key absent at a raw scope.
func newNotSetError(key string, scope configScope) error {
	return &notSetError{
		key:   key,
		scope: scope,
		err: apperr.New(apperr.Config, "config.get", key,
			fmt.Sprintf("%s is not set in %s configuration", key, scope)),
	}
}

// ── entry view ────────────────────────────────────────────────

// configEntryView is one rendered configuration row. It is the single model
// behind both `config list` renderers and the layer rows of `config explain`.
type configEntryView struct {
	Key        string
	Value      string // display value; already redacted
	Raw        any    // structured value; already redacted
	Scope      configScope
	Source     string
	Path       string
	Configured bool // declared by a real layer rather than shown as a default
	IsDefault  bool // value came from the schema defaults layer
	IsSecret   bool
	Type       string
	Group      string
}

// configEntryJSON is the structured form of configEntryView. Field names and
// order are fixed so consumers can rely on them.
type configEntryJSON struct {
	Key        string `json:"key"`
	Value      any    `json:"value"`
	Scope      string `json:"scope"`
	Source     string `json:"source"`
	Path       string `json:"path,omitempty"`
	Configured bool   `json:"configured"`
	IsDefault  bool   `json:"is_default"`
	IsSecret   bool   `json:"is_secret"`
	Type       string `json:"type"`
}

func (v configEntryView) json() configEntryJSON {
	return configEntryJSON{
		Key:        v.Key,
		Value:      v.Raw,
		Scope:      string(v.Scope),
		Source:     v.Source,
		Path:       v.Path,
		Configured: v.Configured,
		IsDefault:  v.IsDefault,
		IsSecret:   v.IsSecret,
		Type:       v.Type,
	}
}

// newConfigEntryView assembles one row from a resolved value. Redaction, type
// lookup, and source naming all happen here rather than at each call site.
func newConfigEntryView(key string, raw any, src config.Source, path string, scope configScope, configured bool, symbols presentation.Symbols) configEntryView {
	spec := config.KeySpec(key)
	view := configEntryView{
		Key:        key,
		Value:      config.RedactString(key, formatConfigValue(raw, symbols)),
		Raw:        config.RedactValue(key, raw),
		Scope:      scope,
		Source:     displayConfigSource(src),
		Path:       path,
		Configured: configured,
		IsDefault:  src == config.SourceDefaults,
		IsSecret:   config.IsSecret(key),
	}
	if spec != nil {
		view.Type = string(spec.Type)
		view.Group = spec.Group
	}
	return view
}

// ── get view ──────────────────────────────────────────────────

// configGetView is the resolved answer to `config get`. Value is what the
// selected scope holds; EffectiveValue is the merged winner, which differs
// whenever a higher layer overrides the selected scope.
type configGetView struct {
	Entry          configEntryView
	EffectiveValue string // display form; already redacted
	EffectiveRaw   any    // structured form; already redacted
	EffectiveKnown bool
	EffectiveSrc   string
	Spec           *config.ConfigKeySpec
}

type configGetJSON struct {
	Key            string `json:"key"`
	Value          any    `json:"value"`
	EffectiveValue any    `json:"effective_value,omitempty"`
	Scope          string `json:"scope"`
	Source         string `json:"source"`
	Path           string `json:"path,omitempty"`
	Configured     bool   `json:"configured"`
	IsDefault      bool   `json:"is_default"`
	IsSecret       bool   `json:"is_secret"`
	Type           string `json:"type"`
}

func (v configGetView) json() configGetJSON {
	out := configGetJSON{
		Key:        v.Entry.Key,
		Value:      v.Entry.Raw,
		Scope:      string(v.Entry.Scope),
		Source:     v.Entry.Source,
		Path:       v.Entry.Path,
		Configured: v.Entry.Configured,
		IsDefault:  v.Entry.IsDefault,
		IsSecret:   v.Entry.IsSecret,
		Type:       v.Entry.Type,
	}
	if v.EffectiveKnown {
		out.EffectiveValue = v.EffectiveRaw
	}
	return out
}

// configGetNotSetView builds a configGetView for a key absent at the requested
// raw scope, so the JSON shape stays stable for consumers that parse stdout.
func configGetNotSetView(eff *config.Effective, key string, scope configScope, symbols presentation.Symbols) configGetView {
	canon := key
	if c := config.CanonicalKey(key); c != "" {
		canon = c
	}
	spec := config.KeySpec(canon)
	entry := configEntryView{
		Key:        canon,
		Value:      config.RedactString(canon, formatConfigValue(nil, symbols)),
		Raw:        config.RedactValue(canon, nil),
		Scope:      scope,
		Configured: false,
		IsDefault:  false,
		IsSecret:   config.IsSecret(canon),
	}
	if spec != nil {
		entry.Type = string(spec.Type)
	}
	view := configGetView{
		Entry: entry,
		Spec:  spec,
	}
	if ev, err := config.GetEffective(eff, canon); err == nil {
		view.EffectiveKnown = true
		view.EffectiveRaw = config.RedactValue(canon, ev.Raw)
		view.EffectiveValue = config.RedactString(canon, formatConfigValue(ev.Raw, symbols))
		view.EffectiveSrc = displayConfigSource(ev.Source)
		view.Entry.Source = displayConfigSource(ev.Source)
		view.Entry.IsDefault = ev.Source == config.SourceDefaults
	}
	return view
}

// ── resolution view ───────────────────────────────────────────

// configLayerView is one rung of a resolution chain.
type configLayerView struct {
	Source    string
	Value     string // display form; already redacted
	Raw       any    // structured form; already redacted
	Path      string
	Effective bool
}

type configLayerJSON struct {
	Source     string `json:"source"`
	Value      any    `json:"value"`
	Path       string `json:"path,omitempty"`
	Configured bool   `json:"configured"`
	Effective  bool   `json:"effective"`
}

// configResolutionView is the full ordered chain behind `config explain`.
type configResolutionView struct {
	Key       string
	Layers    []configLayerView
	Effective configEntryView
	Selected  *configEntryView // raw selected-scope value, when one exists
	Spec      *config.ConfigKeySpec
	LegacyKey string
}

type configResolutionJSON struct {
	Key            string            `json:"key"`
	Scope          string            `json:"scope"`
	Value          any               `json:"value"`
	EffectiveValue any               `json:"effective_value"`
	Source         string            `json:"source"`
	Path           string            `json:"path,omitempty"`
	Type           string            `json:"type"`
	Default        any               `json:"default"`
	Allowed        []string          `json:"allowed,omitempty"`
	Scopes         []string          `json:"scopes"`
	Description    string            `json:"description"`
	IsSecret       bool              `json:"is_secret"`
	Deprecated     bool              `json:"deprecated,omitempty"`
	Replacement    string            `json:"replacement,omitempty"`
	LegacyKey      string            `json:"legacy_key,omitempty"`
	Layers         []configLayerJSON `json:"layers"`
}

func (v configResolutionView) json() configResolutionJSON {
	out := configResolutionJSON{
		Key:            v.Key,
		Scope:          string(v.Effective.Scope),
		EffectiveValue: v.Effective.Raw,
		Source:         v.Effective.Source,
		Path:           v.Effective.Path,
		Type:           v.Effective.Type,
		IsSecret:       v.Effective.IsSecret,
		LegacyKey:      v.LegacyKey,
		Layers:         make([]configLayerJSON, 0, len(v.Layers)),
	}
	// value reports the selected scope when one was requested, and falls back
	// to the effective winner so the field is never absent.
	out.Value = v.Effective.Raw
	if v.Selected != nil {
		out.Value = v.Selected.Raw
		out.Scope = string(v.Selected.Scope)
	}
	if v.Spec != nil {
		out.Default = config.RedactValue(v.Key, v.Spec.Default)
		out.Description = v.Spec.Description
		out.Deprecated = v.Spec.Deprecated
		out.Replacement = v.Spec.Replacement
		if len(v.Spec.Enum) > 0 {
			out.Allowed = v.Spec.Enum
		}
		out.Scopes = configScopeNames(v.Spec)
	}
	if out.Scopes == nil {
		out.Scopes = []string{}
	}
	for _, l := range v.Layers {
		out.Layers = append(out.Layers, configLayerJSON{
			Source: l.Source,
			Value:  l.Raw,
			Path:   l.Path,
			// Every rung of a resolution chain exists because some layer
			// declared the key; defaults included.
			Configured: true,
			Effective:  l.Effective,
		})
	}
	return out
}

// configScopeNames returns the scopes a key may be written in. An empty spec
// list means every writable scope, which is what the schema documents.
func configScopeNames(spec *config.ConfigKeySpec) []string {
	if spec == nil {
		return nil
	}
	if len(spec.Scopes) == 0 {
		return []string{string(config.ScopeUser), string(config.ScopeProject)}
	}
	out := make([]string, len(spec.Scopes))
	for i, s := range spec.Scopes {
		out[i] = string(s)
	}
	return out
}

// ── mutation view ──────────────────────────────────────────────

// configMutationView is the resolved result of a config set or unset operation.
// Previous, Current, and EffectiveValue are already redacted.
type configMutationView struct {
	Key              string
	Scope            configScope
	Path             string
	Previous         any    // structured; already redacted
	PreviousDisplay  string // human form; already redacted
	PreviousSet      bool
	Current          any    // structured; already redacted
	CurrentDisplay   string // human form; already redacted
	EffectiveValue   any    // structured; already redacted
	EffectiveDisplay string // human form; already redacted
	EffectiveSource  string
	Changed          bool
}

type configMutationJSON struct {
	Key             string `json:"key"`
	Scope           string `json:"scope"`
	Path            string `json:"path"`
	Previous        any    `json:"previous"`
	Current         any    `json:"current"`
	EffectiveValue  any    `json:"effective_value,omitempty"`
	EffectiveSource string `json:"effective_source,omitempty"`
	Changed         bool   `json:"changed"`
}

func (v configMutationView) json() configMutationJSON {
	return configMutationJSON{
		Key:             v.Key,
		Scope:           string(v.Scope),
		Path:            v.Path,
		Previous:        v.Previous,
		Current:         v.Current,
		EffectiveValue:  v.EffectiveValue,
		EffectiveSource: v.EffectiveSource,
		Changed:         v.Changed,
	}
}

// ── list view ───────────────────────────────────────────────────

// configListView is the filtered, ordered collection for `config list`.
type configListView struct {
	Scope        configScope
	Entries      []configEntryView
	InclDefaults bool
}

type configListJSON struct {
	Scope   string            `json:"scope"`
	Entries []configEntryJSON `json:"entries"`
}

func (v configListView) json() configListJSON {
	rows := make([]configEntryJSON, 0, len(v.Entries))
	for _, e := range v.Entries {
		rows = append(rows, e.json())
	}
	return configListJSON{Scope: string(v.Scope), Entries: rows}
}

// ── migration view ─────────────────────────────────────────────

type configMigrationCheckJSON struct {
	NeedsMigration bool             `json:"needs_migration"`
	Steps          []map[string]any `json:"changes"`
	Conflicts      []string         `json:"conflicts,omitempty"`
	Path           string           `json:"path"`
}

type configMigrationApplyJSON struct {
	Migrated int              `json:"migrated"`
	Steps    []map[string]any `json:"changes"`
	Path     string           `json:"path"`
}

// ── resolution ────────────────────────────────────────────────

// resolveConfigGet answers `config get` from the retained layers.
//
// A raw scope reads only its own layer, so a key the scope does not declare is
// a typed not-set error rather than a silent fallback to the default or to
// whichever layer happens to win. The effective value is resolved separately
// and reported alongside.
func resolveConfigGet(eff *config.Effective, key string, scope configScope, symbols presentation.Symbols) (configGetView, error) {
	canon := key
	if c := config.CanonicalKey(key); c != "" {
		canon = c
	}

	v, err := config.GetAtScope(eff, configScopeToConfig(scope), key)
	if err != nil {
		if errors.Is(err, config.ErrNotSet) {
			return configGetView{}, newNotSetError(canon, scope)
		}
		return configGetView{}, err
	}

	view := configGetView{
		Entry: newConfigEntryView(canon, v.Raw, v.Source, v.Path, scope, true, symbols),
		Spec:  config.KeySpec(canon),
	}
	// The effective winner is reported next to the raw value so callers can see
	// that a scope value is shadowed without running a second command.
	if ev, eerr := config.GetEffective(eff, canon); eerr == nil {
		view.EffectiveKnown = true
		view.EffectiveRaw = config.RedactValue(canon, ev.Raw)
		view.EffectiveValue = config.RedactString(canon, formatConfigValue(ev.Raw, symbols))
		view.EffectiveSrc = displayConfigSource(ev.Source)
	}
	return view, nil
}

// configListOptions carries the display filters shared by both list renderers.
type configListOptions struct {
	prefix       string
	changed      bool
	inclDefaults bool
}

// matchesPrefix reports whether key falls under the requested namespace.
// "registry" matches both "registry" and "registry.*" but never "registries.*",
// so a prefix names a namespace rather than a character run.
func (o configListOptions) matchesPrefix(key string) bool {
	if o.prefix == "" {
		return true
	}
	return key == o.prefix || strings.HasPrefix(key, o.prefix+".")
}

// resolveConfigList builds the rows for `config list`.
//
// Raw scopes list what the scope itself declares, taken from its retained
// layer. Schema defaults are added only under --defaults, and are marked
// unconfigured so a displayed default never reads as a value someone set.
func resolveConfigList(eff *config.Effective, scope configScope, opts configListOptions, symbols presentation.Symbols) []configEntryView {
	var out []configEntryView

	// Effective already spans every layer including defaults, so --defaults has
	// nothing to add there.
	if scope == configScopeEffective {
		for _, e := range config.ListAtScope(eff, config.ScopeEffective) {
			if !opts.matchesPrefix(e.Key) {
				continue
			}
			v, err := config.GetEffective(eff, e.Key)
			if err != nil {
				continue
			}
			if opts.changed && isSchemaDefaultValue(e.Key, v.Raw, symbols) {
				continue
			}
			// A row is "configured" when a real layer declared it; a schema
			// fallback is displayed but was set by nobody.
			out = append(out, newConfigEntryView(e.Key, v.Raw, v.Source, v.Path, scope, v.Source != config.SourceDefaults, symbols))
		}
		return sortConfigEntries(out)
	}

	seen := map[string]bool{}
	for _, e := range config.ListAtScope(eff, configScopeToConfig(scope)) {
		if !opts.matchesPrefix(e.Key) {
			continue
		}
		v, err := config.GetAtScope(eff, configScopeToConfig(scope), e.Key)
		if err != nil {
			continue
		}
		if opts.changed && isSchemaDefaultValue(e.Key, v.Raw, symbols) {
			continue
		}
		seen[e.Key] = true
		out = append(out, newConfigEntryView(e.Key, v.Raw, v.Source, v.Path, scope, true, symbols))
	}

	if opts.inclDefaults {
		for _, key := range config.RegisteredKeys() {
			if seen[key] || !opts.matchesPrefix(key) {
				continue
			}
			// --changed asks for values that differ from their default, which a
			// default row never does.
			if opts.changed {
				continue
			}
			spec := config.KeySpec(key)
			if spec == nil {
				continue
			}
			out = append(out, newConfigEntryView(key, spec.Default, config.SourceDefaults, "defaults", scope, false, symbols))
		}
	}
	return sortConfigEntries(out)
}

// isSchemaDefaultValue reports whether raw equals the schema default for key.
// Comparison is on the formatted form because a JSONC number and a Go int
// default describe the same value in different types.
func isSchemaDefaultValue(key string, raw any, symbols presentation.Symbols) bool {
	spec := config.KeySpec(key)
	if spec == nil {
		return false
	}
	return formatConfigValue(raw, symbols) == formatConfigValue(spec.Default, symbols)
}

// sortConfigEntries orders rows by schema group then canonical key, so output
// is deterministic across runs and across map iteration order.
func sortConfigEntries(entries []configEntryView) []configEntryView {
	groupRank := map[string]int{}
	for i, g := range config.Groups() {
		groupRank[g] = i
	}
	sort.SliceStable(entries, func(i, j int) bool {
		gi, oki := groupRank[entries[i].Group]
		gj, okj := groupRank[entries[j].Group]
		if !oki {
			gi = len(groupRank)
		}
		if !okj {
			gj = len(groupRank)
		}
		if gi != gj {
			return gi < gj
		}
		return entries[i].Key < entries[j].Key
	})
	return entries
}

// resolveConfigExplain builds the ordered resolution chain for a key.
//
// The chain comes from config.Explain, which replays the retained layers, so
// a value shadowed by a higher layer still appears at its own rung and exactly
// one rung is marked effective.
func resolveConfigExplain(eff *config.Effective, key string, scope configScope, symbols presentation.Symbols) (configResolutionView, error) {
	chain, err := config.Explain(eff, key)
	if err != nil {
		return configResolutionView{}, err
	}
	canon := key
	if c := config.CanonicalKey(key); c != "" {
		canon = c
	}
	winner, err := config.GetEffective(eff, canon)
	if err != nil {
		return configResolutionView{}, err
	}

	view := configResolutionView{
		Key:       canon,
		Effective: newConfigEntryView(canon, winner.Raw, winner.Source, winner.Path, configScopeEffective, true, symbols),
		Spec:      config.KeySpec(canon),
		LegacyKey: config.LegacyKey(canon),
		Layers:    make([]configLayerView, 0, len(chain)),
	}
	for _, rung := range chain {
		view.Layers = append(view.Layers, configLayerView{
			Source:    displayConfigSource(rung.Source),
			Value:     config.RedactString(canon, formatConfigValue(rung.Raw, symbols)),
			Raw:       config.RedactValue(canon, rung.Raw),
			Path:      rung.Path,
			Effective: rung.Effective,
		})
	}
	// A raw scope was explicitly requested: report what that scope holds
	// alongside the chain. Absence is not an error here — the chain is still
	// the answer.
	if scope != configScopeEffective {
		if sv, serr := config.GetAtScope(eff, configScopeToConfig(scope), canon); serr == nil {
			sel := newConfigEntryView(canon, sv.Raw, sv.Source, sv.Path, scope, true, symbols)
			view.Selected = &sel
		}
	}
	return view, nil
}

// ── shared formatting ─────────────────────────────────────────

// formatConfigValue renders a config value for display. Structured output uses
// the raw value; this is the human form. The symbols parameter selects the
// active placeholder glyph (Unicode or ASCII).
func formatConfigValue(v any, symbols presentation.Symbols) string {
	switch t := v.(type) {
	case nil:
		return symbols.Placeholder
	case string:
		if t == "" {
			return symbols.Placeholder
		}
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	default:
		return fmt.Sprint(t)
	}
}

// writeConfigJSON encodes v as the command's structured output.
func writeConfigJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
