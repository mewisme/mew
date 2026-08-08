package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type helpGroup struct {
	title string
	names []string
}

type cmdHelpMeta struct {
	group    string
	examples []string
	related  []string
	workflow int // lower ranks first in Common workflows
}

var helpGroups = []helpGroup{
	{title: "Common workflows", names: []string{"install", "add", "run", "exec", "ci", "update"}},
	{title: "Project and dependencies", names: []string{"init", "remove", "link", "dedupe", "prune", "resolve", "fetch", "lock", "patch", "publish", "pkg", "project"}},
	{title: "Run and execute", names: []string{"env", "view", "watch"}},
	{title: "Inspect and diagnose", names: []string{"ls", "outdated", "explain", "plan", "history", "snapshot", "doctor", "features", "diff", "recover", "rollback"}},
	{title: "Security and policy", names: []string{"audit", "policy", "verify", "sbom", "builds", "trust", "approve-builds"}},
	{title: "Cache, store, and artifacts", names: []string{"cache", "store", "pack", "capsule"}},
	{title: "Configuration and development", names: []string{"config", "development", "benchmark", "conformance", "version", "completion"}},
}

var commandHelpRegistry = map[string]cmdHelpMeta{
	"install":  {group: "Common workflows", workflow: 1, examples: []string{"m install", "m install --frozen-lockfile"}, related: []string{"add", "ci", "plan"}},
	"add":      {group: "Common workflows", workflow: 2, examples: []string{"m add lodash", "m add -D typescript"}},
	"run":      {group: "Common workflows", workflow: 3, examples: []string{"m run build", "m run test -- --watch"}},
	"exec":     {group: "Common workflows", workflow: 4, examples: []string{"m exec eslint ."}},
	"ci":       {group: "Common workflows", workflow: 5, examples: []string{"m ci"}},
	"update":   {group: "Common workflows", workflow: 6, examples: []string{"m update", "m update lodash"}},
	"config":   {group: "Configuration and development", examples: []string{"m config list", "m config get ui.theme", "m config set ui.theme dark", "m config set install.linker isolated --scope project", "m config validate"}},
	"doctor":   {group: "Inspect and diagnose", examples: []string{"m doctor", "m doctor --json"}},
	"ls":       {group: "Inspect and diagnose", examples: []string{"m ls", "m ls -r"}},
	"outdated": {group: "Inspect and diagnose", examples: []string{"m outdated", "m outdated --json"}},
	"explain":  {group: "Inspect and diagnose", examples: []string{"m explain lodash"}},
	"plan":     {group: "Inspect and diagnose", examples: []string{"m plan", "m plan update"}},
	"audit":    {group: "Security and policy", examples: []string{"m audit", "m audit --fail-on high"}},
	"policy":   {group: "Security and policy", examples: []string{"m policy check"}},
	"features": {group: "Inspect and diagnose", examples: []string{"m features --format table"}},
	"project":  {group: "Project and dependencies", examples: []string{"m project info"}},
	"pkg":      {group: "Project and dependencies", examples: []string{"m pkg get name", "m pkg get version"}},
	"cache":    {group: "Cache, store, and artifacts", examples: []string{"m cache dir", "m cache verify"}},
	"store":    {group: "Cache, store, and artifacts", examples: []string{"m store status", "m store path"}},
}

func configureGroupedHelp(root *cobra.Command) {
	root.SetHelpTemplate(groupedRootHelpTemplate)
	root.SetUsageTemplate(groupedUsageTemplate)
	cobra.AddTemplateFunc("mewBareScripts", renderBareScripts)
	cobra.AddTemplateFunc("mewGroupedCommands", renderGroupedCommands)
	cobra.AddTemplateFunc("mewCommandSections", renderCommandSections)
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

const groupedRootHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{- if .HasSubCommands}}Usage:
  {{.CommandPath}} [command]
{{end}}{{mewBareScripts .}}{{if .HasSubCommands}}
{{mewGroupedCommands .}}
{{- end}}
{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}{{if .HasExample}}

Examples:
{{.Example}}
{{- end}}
{{- if .HasHelpSubCommands}}

Additional help topics:
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath 28}} {{.Short}}{{end}}{{end}}
{{- end}}
Use "{{.CommandPath}} [command] --help" for more information about a command.
Use "{{.CommandPath}} help <topic>" for curated topics (errors, runner, lifecycle-trust, …).
`

const groupedUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
{{mewGroupedCommands .}}{{end}}{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}
`

const commandHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}Usage:
  {{.UseLine}}
{{if .HasSubCommands}}
Available Commands:
{{range .Commands}}{{if (and .IsAvailableCommand (not .IsAdditionalHelpTopicCommand))}}  {{rpad .Name 14}} {{.Short}}
{{end}}{{end}}{{end}}{{mewCommandSections .}}
{{- if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
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

func formatCommandLine(cmd *cobra.Command) string {
	return fmt.Sprintf("  %-14s %s%s", cmd.Name(), cmd.Short, aliasSuffix(cmd))
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
		return "\n" + b.String()
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
