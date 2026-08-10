package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/graph"
	"github.com/mewisme/mew/internal/presentation"
	"github.com/mewisme/mew/internal/project"
	"github.com/mewisme/mew/internal/resolver"
)

func newExplainCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "explain [name]",
		Short:   "Explain resolution decisions",
		Aliases: []string{"why"},
		Long:    "Explain version selection for a package, or use `explain peer` for unsatisfied peer dependencies.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runExplainPackage(cmd, args[0], asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print explanation as JSON")
	cmd.AddCommand(newExplainPeerCmd())
	return cmd
}

func runExplainPackage(cmd *cobra.Command, name string, asJSON bool) error {
	ac := app.FromContext(cmd.Context())
	if ac == nil {
		return apperr.New(apperr.Internal, "explain", "", "missing app context")
	}
	proj, eng, prior, err := explainEngine(cmd, ac)
	if err != nil {
		return err
	}
	ex, err := eng.ExplainPackage(cmd.Context(), proj.Root, name, resolver.ResolveOptions{
		Prior: prior,
		Hints: prior,
	})
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return enc.Encode(ex)
	}
	g := ownerFlags(cmd.Root())
	r := g.mustStaticRenderer(cmd)
	return writeStaticOut(cmd, formatExplainHuman(r, ex))
}

func newExplainPeerCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "peer <name>",
		Short: "Explain an unsatisfied peer dependency",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "explain.peer", "", "missing app context")
			}
			proj, eng, prior, err := explainEngine(cmd, ac)
			if err != nil {
				return err
			}
			tree, err := eng.ExplainPeer(cmd.Context(), proj.Root, args[0], resolver.ResolveOptions{
				Prior: prior,
				Hints: prior,
			})
			if err != nil {
				return err
			}
			if tree == nil {
				g := ownerFlags(cmd.Root())
				return writeStaticOut(cmd, g.mustStaticRenderer(cmd).Status(presentation.StatusLine{
					Status: presentation.StatusInfo,
					Text:   fmt.Sprintf("no peer conflict for %q", args[0]),
				}))
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				return enc.Encode(tree)
			}
			return printConflictTree(cmd, *tree)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print conflict tree as JSON")
	return cmd
}

func explainEngine(cmd *cobra.Command, ac *app.Context) (*project.Project, *resolver.Engine, *graph.Graph, error) {
	proj, err := app.OpenProject(cmd.Context(), ac)
	if err != nil {
		return nil, nil, nil, err
	}
	prior, err := app.ReadLockGraph(cmd.Context(), ac)
	if err != nil {
		if apperr.CodeOf(err) != apperr.IO {
			return nil, nil, nil, err
		}
		prior = nil
	}
	eng, err := resolver.NewFromApp(ac.Config, proj)
	if err != nil {
		return nil, nil, nil, err
	}
	return proj, eng, prior, nil
}

func printConflictTree(cmd *cobra.Command, tree resolver.ConflictTree) error {
	g := ownerFlags(cmd.Root())
	r := g.mustStaticRenderer(cmd)
	return writeStaticOut(cmd, formatExplainConflictTree(r, tree))
}

func formatExplainHuman(r presentation.StaticRenderer, ex *resolver.PackageExplanation) string {
	if ex == nil {
		return ""
	}
	settings := r.Settings()
	sym := settings.Symbols

	var b strings.Builder
	b.WriteString(ex.Package)
	b.WriteByte('\n')

	if ex.Conflict != nil {
		b.WriteString(formatExplainConflictTree(r, *ex.Conflict))
		return b.String()
	}

	for _, d := range ex.Decisions {
		arrow := presentation.RenderSymbolRole(sym, presentation.Theme{}, presentation.RoleArrow, false)
		line := fmt.Sprintf("%s@%s %s %s (%s)", d.Package, d.Requested, arrow, d.Selected, d.Reason)
		if detail := resolver.ReasonDetailFor(d.Reason); detail.Text != "" {
			line += " " + sym.Separator + " " + detail.Text
			if detail.Code != "" {
				line += " [" + detail.Code + "]"
			}
		}
		if len(d.PeerProviders) > 0 {
			line += fmt.Sprintf(" peerProviders=%v", d.PeerProviders)
		}
		if d.OverrideFrom != "" {
			line += fmt.Sprintf(" override=%q", d.OverrideFrom)
		}
		if len(d.Rejected) > 0 {
			line += fmt.Sprintf(" rejected=%v", d.Rejected)
		}
		b.WriteString(r.PlainText(line))
		b.WriteByte('\n')
	}

	if len(ex.Paths) > 0 {
		b.WriteString("imported by:\n")
		for _, p := range ex.Paths {
			arrow := presentation.RenderSymbolRole(sym, presentation.Theme{}, presentation.RoleArrow, false)
			b.WriteString(r.PlainText(fmt.Sprintf("  %s", strings.Join(p.Chain, " "+arrow+" "))))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatExplainConflictTree(r presentation.StaticRenderer, tree resolver.ConflictTree) string {
	sym := r.Settings().Symbols
	arrow := presentation.RenderSymbolRole(sym, presentation.Theme{}, presentation.RoleArrow, false)
	sep := sym.Separator
	return resolver.FormatConflictTreeWithSymbols(tree, arrow, sep)
}
