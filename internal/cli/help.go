package cli

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type helpGroup struct {
	id    string // stable internal key; safe to keep when the display title is renamed
	title string // user-facing label
	names []string
}

const (
	helpGroupCommon       = "common"
	helpGroupDependencies = "dependencies"
	helpGroupRun          = "run"
	helpGroupInspect      = "inspect"
	helpGroupSecurity     = "security"
	helpGroupArtifacts    = "artifacts"
	helpGroupTooling      = "tooling"
)

type cmdHelpMeta struct {
	group    string
	examples []string
	related  []string
	workflow int // lower ranks first in the common-workflows group
}

var helpGroups = []helpGroup{
	{id: helpGroupCommon, title: "Common workflows", names: []string{"install", "add", "run", "x", "exec", "ci", "update"}},
	{id: helpGroupDependencies, title: "Dependencies and packages", names: []string{"init", "remove", "link", "dedupe", "prune", "resolve", "fetch", "lock", "patch", "publish", "pkg", "project"}},
	{id: helpGroupRun, title: "Run and scripts", names: []string{"env", "view", "watch"}},
	{id: helpGroupInspect, title: "Inspect and recover", names: []string{"ls", "outdated", "explain", "plan", "history", "snapshot", "doctor", "features", "diff", "recover", "rollback"}},
	{id: helpGroupSecurity, title: "Security and trust", names: []string{"audit", "policy", "verify", "sbom", "builds", "trust", "approve-builds"}},
	{id: helpGroupArtifacts, title: "Cache and artifacts", names: []string{"cache", "store", "pack", "capsule"}},
	{id: helpGroupTooling, title: "Configuration and tooling", names: []string{"config", "development", "benchmark", "conformance", "version", "completion"}},
}

// renameHelpGroup changes only the user-facing group title.
// The stable group ID remains unchanged, so metadata can safely keep referring to it.
func renameHelpGroup(id, title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return false
	}

	for i := range helpGroups {
		if helpGroups[i].id == id {
			helpGroups[i].title = title
			return true
		}
	}
	return false
}

var commandHelpRegistry = map[string]cmdHelpMeta{
	"install":  {group: helpGroupCommon, workflow: 1, examples: []string{"m install", "m install --frozen-lockfile"}, related: []string{"add", "ci", "plan"}},
	"add":      {group: helpGroupCommon, workflow: 2, examples: []string{"m add lodash", "m add -D typescript"}},
	"run":      {group: helpGroupCommon, workflow: 3, examples: []string{"m run build", "m run test -- --watch"}},
	"exec":     {group: helpGroupCommon, workflow: 4, examples: []string{"m exec eslint ."}},
	"ci":       {group: helpGroupCommon, workflow: 5, examples: []string{"m ci"}},
	"update":   {group: helpGroupCommon, workflow: 6, examples: []string{"m update", "m update lodash"}},
	"config":   {group: helpGroupTooling, examples: []string{"m config list", "m config get ui.theme", "m config set ui.theme dark", "m config set install.linker isolated --scope project", "m config validate"}},
	"doctor":   {group: helpGroupInspect, examples: []string{"m doctor", "m doctor --json"}},
	"ls":       {group: helpGroupInspect, examples: []string{"m ls", "m ls -r"}},
	"outdated": {group: helpGroupInspect, examples: []string{"m outdated", "m outdated --json"}},
	"explain":  {group: helpGroupInspect, examples: []string{"m explain lodash"}},
	"plan":     {group: helpGroupInspect, examples: []string{"m plan", "m plan update"}},
	"audit":    {group: helpGroupSecurity, examples: []string{"m audit", "m audit --fail-on high"}},
	"policy":   {group: helpGroupSecurity, examples: []string{"m policy check"}},
	"features": {group: helpGroupInspect, examples: []string{"m features --format table"}},
	"project":  {group: helpGroupDependencies, examples: []string{"m project info"}},
	"pkg":      {group: helpGroupDependencies, examples: []string{"m pkg get name", "m pkg get version"}},
	"cache":    {group: helpGroupArtifacts, examples: []string{"m cache dir", "m cache verify"}},
	"store":    {group: helpGroupArtifacts, examples: []string{"m store status", "m store path"}},
}

func configureGroupedHelp(root *cobra.Command) {
	root.SetHelpTemplate(groupedRootHelpTemplate)
	root.SetUsageTemplate(groupedUsageTemplate)
	cobra.AddTemplateFunc("mewBareScripts", renderBareScripts)
	cobra.AddTemplateFunc("mewGroupedCommands", renderGroupedCommands)
	cobra.AddTemplateFunc("mewCommandSections", renderCommandSections)
	cobra.AddTemplateFunc("dimParens", dimParens)
	cobra.AddTemplateFunc("styleMew", styleMew)
	for _, cmd := range root.Commands() {
		applyCommandHelp(cmd)
	}
	configureTopicHelp(root)
}

func applyCommandHelp(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	if _, ok := commandHelpRegistry[cmd.Name()]; ok {
		cmd.SetHelpTemplate(commandHelpTemplate)
	}
	for _, sub := range cmd.Commands() {
		applyCommandHelp(sub)
	}
}

const groupedRootHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces | styleMew | dimParens}}

{{end}}{{- if .HasSubCommands}}Usage: {{styleMew .CommandPath}} <command> [...flags] [...args]
{{end}}{{mewBareScripts .}}{{if .HasSubCommands}}
{{mewGroupedCommands .}}
{{- end}}
{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces | styleMew | dimParens}}
{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces | styleMew | dimParens}}
{{end}}{{if .HasExample}}

Examples:
{{.Example}}
{{- end}}
{{- if .HasHelpSubCommands}}

Additional help topics:
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad (styleMew .CommandPath) 28}} {{.Short}}{{end}}{{end}}
{{- end}}
Use "{{styleMew .CommandPath}} [command] --help" for more information about a command.
{{printf "Use \"%s help <topic>\" for curated topics (errors, runner, lifecycle-trust, …)." (.CommandPath | styleMew) | dimParens}}
`

const groupedUsageTemplate = `Usage:{{if .Runnable}} {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
{{mewGroupedCommands .}}{{end}}{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces | styleMew | dimParens}}{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces | styleMew | dimParens}}{{end}}
`

const commandHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces | styleMew | dimParens}}

{{end}}Usage: {{.UseLine}}
{{if .HasSubCommands}}
Available Commands:
{{range .Commands}}{{if (and .IsAvailableCommand (not .IsAdditionalHelpTopicCommand))}}  {{rpad (print .Name ":") 15}} {{.Short | styleMew | dimParens}}
{{end}}{{end}}{{end}}{{mewCommandSections .}}
{{- if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces | styleMew | dimParens}}
{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces | styleMew | dimParens}}
{{end}}
`

// aliasSuffix returns " (m a)" or " (m a, m in)" for commands with public aliases.
func aliasSuffix(cmd *cobra.Command) string {
	aliases := cmd.Aliases
	if len(aliases) == 0 {
		return ""
	}
	bin := rootBinaryName(cmd)
	var parts []string
	for _, a := range aliases {
		if a == "" || a == cmd.Name() {
			continue
		}
		parts = append(parts, bin+" "+a)
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

var (
	reParens  = regexp.MustCompile(`\([^)]+\)`)
	reANSISGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	// reMew matches "Mew" and binary names m, mx, mew, mewx at word boundaries.
	reMew = regexp.MustCompile(`\b(Mew|mew|mewx|mx|m)\b`)
)

// dimParens wraps every (...) group with faint/dim ANSI styling.
// Strips any nested ANSI inside the parens so content is dim-only.
func dimParens(s string) string {
	return reParens.ReplaceAllStringFunc(s, func(m string) string {
		clean := reANSISGR.ReplaceAllString(m, "")
		return "\x1b[2m" + clean + "\x1b[22m"
	})
}

// styleMew wraps Mew and binary names (m, mx, mew, mewx) in bright magenta.
func styleMew(s string) string {
	return reMew.ReplaceAllStringFunc(s, func(m string) string {
		return "\x1b[95;1m" + m + "\x1b[39;22m"
	})
}

func formatCommandLine(cmd *cobra.Command) string {
	name := cmd.Name() + ":"
	line := fmt.Sprintf("  %-15s %s%s", name, cmd.Short, aliasSuffix(cmd))
	return dimParens(styleMew(line))
}

func renderGroupedCommands(cmd *cobra.Command) string {
	byName := map[string]*cobra.Command{}
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.Hidden {
			continue
		}
		byName[c.Name()] = c
	}
	var b strings.Builder
	seen := make(map[string]struct{})
	for _, g := range helpGroups {
		var lines []string
		for _, name := range g.names {
			c, ok := byName[name]
			if !ok {
				continue
			}
			seen[name] = struct{}{}
			lines = append(lines, formatCommandLine(c))
		}
		if len(lines) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(g.title)
		b.WriteByte('\n')
		for _, line := range lines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	var other []string
	var otherNames []string
	for name := range byName {
		if _, ok := seen[name]; ok {
			continue
		}
		otherNames = append(otherNames, name)
	}
	sort.Strings(otherNames)
	for _, name := range otherNames {
		other = append(other, formatCommandLine(byName[name]))
	}
	if len(other) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("Other")
		b.WriteByte('\n')
		for _, line := range other {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderCommandSections(cmd *cobra.Command) string {
	meta, ok := commandHelpRegistry[cmd.Name()]
	if !ok {
		return ""
	}
	bin := rootBinaryName(cmd)
	var b strings.Builder
	if len(meta.examples) > 0 {
		b.WriteString("Examples:\n")
		for _, ex := range meta.examples {
			b.WriteString("  ")
			b.WriteString(binaryReplace(ex, bin))
			b.WriteByte('\n')
		}
	}
	if len(meta.related) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("Related:\n")
		for _, rel := range meta.related {
			b.WriteString("  ")
			b.WriteString(rel)
			b.WriteByte('\n')
		}
	}
	if b.Len() > 0 {
		return styleMew("\n" + b.String())
	}
	return ""
}

// binaryReplace substitutes "{binary}" with the actual invoked binary name.
// Falls back to replacing "m " prefix for backward compatibility with hardcoded examples.
func binaryReplace(s, bin string) string {
	s = strings.ReplaceAll(s, "{binary}", bin)
	if bin != "m" && strings.HasPrefix(s, "m ") {
		s = bin + s[1:]
	}
	return s
}

// rootBinaryName returns the binary name from the root command (m, mew, mx, mewx).
func rootBinaryName(cmd *cobra.Command) string {
	if cmd == nil {
		return "m"
	}
	if cmd.Root() != nil && cmd.Root().Use != "" {
		return cmd.Root().Use
	}
	return "m"
}

func renderBareScripts(cmd *cobra.Command) string {
	// Only the root command's own help shows scripts. Subcommands inherit the
	// root template via cobra, but scripts belong only on the root.
	if cmd == nil || cmd.Root() != cmd {
		return ""
	}
	// MX roots do not dispatch package.json scripts.
	if isMXRoot(cmd) {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	names, total, err := listBareMScripts(cwd)
	if err != nil || len(names) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nAvailable scripts (from package.json):\n  ")
	show := names
	if len(show) > bareMScriptListLimit {
		show = show[:bareMScriptListLimit]
	}
	b.WriteString(strings.Join(show, ", "))
	if total > bareMScriptListLimit {
		fmt.Fprintf(&b, ", … and %d more", total-bareMScriptListLimit)
	}
	bin := rootBinaryName(cmd)
	fmt.Fprintf(&b, "\n\nRun `%s run <script>` to execute.\n", bin)
	return b.String()
}