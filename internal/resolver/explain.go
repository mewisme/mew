package resolver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	fc "github.com/fatih/color"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/graph"
	"github.com/mewisme/mew/internal/semver"
)

// PeerSearchStep records one environment searched during peer provider lookup.
type PeerSearchStep struct {
	Environment string               `json:"environment"`
	Candidates  []string             `json:"candidates,omitempty"`
	Rejected    []CandidateRejection `json:"rejected,omitempty"`
}

// CandidateRejection records a version considered but rejected for a peer range.
type CandidateRejection struct {
	Version string `json:"version,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// ConflictNode is one node in a peer conflict explanation tree.
type ConflictNode struct {
	Constraint        string               `json:"constraint,omitempty"`
	RequiringPackage  string               `json:"requiringPackage,omitempty"`
	Importer          string               `json:"importer,omitempty"`
	SearchPath        []string             `json:"searchPath,omitempty"`
	Candidates        []string             `json:"candidates,omitempty"`
	Rejected          []CandidateRejection `json:"rejected,omitempty"`
	AutoInstallPolicy bool                 `json:"autoInstallPolicy,omitempty"`
	Optional          bool                 `json:"optional,omitempty"`
	Remediation       string               `json:"remediation,omitempty"`
	Children          []ConflictNode       `json:"children,omitempty"`
}

// ConflictTree explains an unsatisfiable peer dependency.
type ConflictTree struct {
	Peer string       `json:"peer"`
	Root ConflictNode `json:"root"`
}

// ImportPath is one route from an importer to a resolved package instance.
type ImportPath struct {
	Importer string   `json:"importer"`
	Chain    []string `json:"chain"`
}

// PackageExplanation documents version selection for one package name.
type PackageExplanation struct {
	Package   string               `json:"package"`
	Decisions []ResolutionDecision `json:"decisions,omitempty"`
	Paths     []ImportPath         `json:"paths,omitempty"`
	Conflict  *ConflictTree        `json:"conflict,omitempty"`
}

// ReasonDetail is human-readable text and an optional stable error code hint.
type ReasonDetail struct {
	Text string `json:"text"`
	Code string `json:"code,omitempty"`
}

// PeerConflictFromError extracts a structured peer conflict from a resolve error.
func PeerConflictFromError(err error) (PeerConflict, bool) {
	var pc *peerConflictErr
	if errors.As(err, &pc) {
		return pc.PeerConflict, true
	}
	return PeerConflict{}, false
}

// BuildPeerConflictTree builds a human/machine conflict tree for a peer failure.
func BuildPeerConflictTree(conf PeerConflict, prior *graph.Graph) ConflictTree {
	root := ConflictNode{
		Constraint:        conf.Peer + "@" + conf.Range,
		RequiringPackage:  conf.Package,
		Importer:          conf.Importer,
		SearchPath:        append([]string(nil), conf.SearchPath...),
		AutoInstallPolicy: conf.AutoInstallPolicy,
		Optional:          conf.Optional,
		Remediation:       peerRemediation(conf),
	}
	if conf.Incompatible && conf.FoundVersion != "" {
		root.Rejected = []CandidateRejection{{Version: conf.FoundVersion, Reason: "range mismatch (nearest provider)"}}
	}
	root.Children = conflictChildrenFromSteps(conf)
	if len(root.Children) == 0 {
		candidates, rejected := peerCandidateAnalysis(prior, conf.Peer, conf.Range)
		root.Candidates = candidates
		root.Rejected = rejected
	}
	return ConflictTree{Peer: conf.Peer, Root: root}
}

func conflictChildrenFromSteps(conf PeerConflict) []ConflictNode {
	if len(conf.SearchSteps) == 0 {
		return nil
	}
	children := make([]ConflictNode, 0, len(conf.SearchPath))
	for i, env := range conf.SearchPath {
		ancestry := ConflictNode{
			Constraint: "ancestry: " + env,
			Importer:   env,
		}
		if i < len(conf.SearchSteps) {
			step := conf.SearchSteps[i]
			envNode := ConflictNode{
				Constraint:        "environment: " + step.Environment,
				Importer:          step.Environment,
				Candidates:        append([]string(nil), step.Candidates...),
				Rejected:          append([]CandidateRejection(nil), step.Rejected...),
				Optional:          conf.Optional,
				AutoInstallPolicy: conf.AutoInstallPolicy,
			}
			if conf.StrictPeers {
				envNode.Constraint += " (strict peers)"
			}
			ancestry.Children = []ConflictNode{envNode}
		}
		children = append(children, ancestry)
	}
	for _, step := range conf.SearchSteps[len(conf.SearchPath):] {
		children = append(children, ConflictNode{
			Constraint:        "environment: " + step.Environment,
			Importer:          step.Environment,
			Candidates:        append([]string(nil), step.Candidates...),
			Rejected:          append([]CandidateRejection(nil), step.Rejected...),
			Optional:          conf.Optional,
			AutoInstallPolicy: conf.AutoInstallPolicy,
		})
	}
	return children
}

func peerRemediation(conf PeerConflict) string {
	if conf.Optional {
		return fmt.Sprintf("mark %s optional in peerDependenciesMeta or install a compatible provider", conf.Peer)
	}
	if conf.AutoInstallPolicy {
		return fmt.Sprintf("install %s@%s or relax auto-install peer policy", conf.Peer, conf.Range)
	}
	return fmt.Sprintf("install %s@%s in the dependency tree or enable resolve.autoInstallPeers", conf.Peer, conf.Range)
}

func peerCandidateAnalysis(g *graph.Graph, peerName, rng string) (candidates []string, rejected []CandidateRejection) {
	if g == nil {
		return []string{"(none resolved)"}, nil
	}
	type ver struct {
		raw string
	}
	var versions []ver
	for _, p := range g.Packages {
		if p.ID.Name != peerName {
			continue
		}
		versions = append(versions, ver{raw: p.ID.Version})
	}
	sort.Slice(versions, func(i, j int) bool {
		cmp, err := semver.Compare(versions[i].raw, versions[j].raw)
		if err != nil {
			return versions[i].raw < versions[j].raw
		}
		return cmp < 0
	})
	for _, v := range versions {
		ok, err := semver.Satisfies(v.raw, rng)
		if err != nil {
			rejected = append(rejected, CandidateRejection{Version: v.raw, Reason: "invalid version"})
			continue
		}
		if ok {
			candidates = append(candidates, v.raw)
		} else {
			rejected = append(rejected, CandidateRejection{Version: v.raw, Reason: "range mismatch"})
		}
	}
	if len(candidates) == 0 && len(rejected) == 0 {
		candidates = []string{"(none resolved)"}
	}
	return candidates, rejected
}

var reasonDetails = map[string]ReasonDetail{
	"reuse-key":        {Text: "reused locked package key from prior graph"},
	"hint":             {Text: "preferred version from lock hints"},
	"tag-or-exact":     {Text: "exact version or dist-tag match"},
	"max-satisfying":   {Text: "highest version matching range"},
	"platform-skipped": {Text: "skipped due to platform or OS restriction", Code: string(apperr.Resolve)},
	"optional-failed":  {Text: "optional dependency failed to resolve", Code: string(apperr.Resolve)},
	"workspace":        {Text: "resolved workspace package"},
	"git":              {Text: "resolved git dependency"},
	"tarball":          {Text: "resolved tarball dependency"},
	"file":             {Text: "resolved local file dependency"},
	"link":             {Text: "resolved link dependency"},
	"portal":           {Text: "resolved portal dependency"},
}

// ReasonDetailFor maps a decision reason code to readable text and error hints.
func ReasonDetailFor(reason string) ReasonDetail {
	if d, ok := reasonDetails[reason]; ok {
		return d
	}
	if reason == "" {
		return ReasonDetail{}
	}
	return ReasonDetail{Text: strings.ReplaceAll(reason, "-", " ")}
}

// ExplainPackage dry-resolves and explains version selection for name.
func (e *Engine) ExplainPackage(ctx context.Context, root, name string, opts ResolveOptions) (*PackageExplanation, error) {
	if strings.TrimSpace(name) == "" {
		return nil, apperr.New(apperr.Usage, "resolver.explain", name, "package name is required")
	}
	res, err := e.Resolve(ctx, root, opts)
	if err != nil {
		if conf, ok := PeerConflictFromError(err); ok && conf.Peer == name {
			prior := opts.Prior
			if prior == nil {
				prior = opts.Hints
			}
			tree := BuildPeerConflictTree(conf, prior)
			return &PackageExplanation{Package: name, Conflict: &tree}, nil
		}
		return nil, err
	}
	ex := &PackageExplanation{Package: name}
	for _, d := range res.Decisions {
		if d.Package == name {
			ex.Decisions = append(ex.Decisions, d)
		}
	}
	found := len(ex.Decisions) > 0
	if !found {
		for _, p := range res.Graph.Packages {
			if p.ID.Name == name {
				found = true
				break
			}
		}
	}
	if !found {
		return nil, apperr.New(apperr.NotFound, "resolver.explain", name,
			fmt.Sprintf("package %q not in resolution graph", name))
	}
	ex.Paths = buildImportPaths(res.Graph, name)
	return ex, nil
}

func buildImportPaths(g *graph.Graph, name string) []ImportPath {
	if g == nil {
		return nil
	}
	importers := make(map[string]struct{}, len(g.Importers))
	for _, imp := range g.Importers {
		importers[string(imp.ID)] = struct{}{}
	}
	parents := make(map[string][]string)
	for _, e := range g.Edges {
		parents[e.To] = append(parents[e.To], e.From)
	}
	var targets []string
	for _, p := range g.Packages {
		if p.ID.Name == name {
			targets = append(targets, p.ID.Key())
		}
	}
	sort.Strings(targets)

	var paths []ImportPath
	seen := make(map[string]struct{})
	for _, target := range targets {
		for _, path := range pathsToImporters(target, parents, importers) {
			key := path.Importer + "\x00" + strings.Join(path.Chain, "\x00")
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			paths = append(paths, path)
		}
	}
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].Importer != paths[j].Importer {
			return paths[i].Importer < paths[j].Importer
		}
		return strings.Join(paths[i].Chain, "\x00") < strings.Join(paths[j].Chain, "\x00")
	})
	return paths
}

func pathsToImporters(target string, parents map[string][]string, importers map[string]struct{}) []ImportPath {
	var out []ImportPath
	var walk func(node string, chain []string, visited map[string]struct{})
	walk = func(node string, chain []string, visited map[string]struct{}) {
		if _, ok := importers[node]; ok {
			copied := append([]string(nil), chain...)
			for i, j := 0, len(copied)-1; i < j; i, j = i+1, j-1 {
				copied[i], copied[j] = copied[j], copied[i]
			}
			out = append(out, ImportPath{Importer: node, Chain: copied})
			return
		}
		for _, parent := range parents[node] {
			if _, ok := visited[parent]; ok {
				continue
			}
			visited[parent] = struct{}{}
			walk(parent, append(chain, parent), visited)
			delete(visited, parent)
		}
	}
	walk(target, []string{target}, map[string]struct{}{target: {}})
	return out
}

// FormatPackageExplanation renders human output scoped to one package.
func FormatPackageExplanation(ex *PackageExplanation, w io.Writer, color bool) error {
	if ex == nil {
		return nil
	}
	boldStyle := fc.New(fc.Bold)
	dimStyle := fc.New(fc.Faint)
	bold := func(s string) string {
		if !color {
			return s
		}
		return boldStyle.Sprint(s)
	}
	dim := func(s string) string {
		if !color {
			return s
		}
		return dimStyle.Sprint(s)
	}
	if _, err := fmt.Fprintf(w, "package %s\n", bold(ex.Package)); err != nil {
		return err
	}
	if ex.Conflict != nil {
		_, err := fmt.Fprint(w, FormatConflictTree(*ex.Conflict))
		return err
	}
	for _, d := range ex.Decisions {
		line := fmt.Sprintf("%s@%s → %s (%s)", d.Package, d.Requested, d.Selected, d.Reason)
		if detail := ReasonDetailFor(d.Reason); detail.Text != "" {
			line += " — " + detail.Text
			if detail.Code != "" {
				line += dim(" [" + detail.Code + "]")
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
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if len(ex.Paths) > 0 {
		if _, err := fmt.Fprintln(w, "imported by:"); err != nil {
			return err
		}
		for _, p := range ex.Paths {
			if _, err := fmt.Fprintf(w, "  %s\n", strings.Join(p.Chain, " → ")); err != nil {
				return err
			}
		}
	}
	return nil
}

func ColorEnabledForWriter(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// ExplainPeer dry-resolves and returns a conflict tree when peerName is unsatisfied.
func (e *Engine) ExplainPeer(ctx context.Context, root, peerName string, opts ResolveOptions) (*ConflictTree, error) {
	res, err := e.Resolve(ctx, root, opts)
	if err == nil {
		for _, p := range res.Graph.Packages {
			if p.ID.Name == peerName {
				return nil, nil
			}
		}
		return nil, nil
	}
	conf, ok := PeerConflictFromError(err)
	if !ok || conf.Peer != peerName {
		return nil, err
	}
	prior := opts.Prior
	if prior == nil {
		prior = opts.Hints
	}
	tree := BuildPeerConflictTree(conf, prior)
	return &tree, nil
}

// FormatConflictTree renders a human-readable conflict tree.
func FormatConflictTree(tree ConflictTree) string {
	var b strings.Builder
	fmt.Fprintf(&b, "peer %s\n", tree.Peer)
	formatConflictNode(&b, tree.Root, 0)
	return b.String()
}

func formatConflictNode(b *strings.Builder, n ConflictNode, depth int) {
	prefix := strings.Repeat("  ", depth)
	line := prefix + n.Constraint
	if n.RequiringPackage != "" {
		line += fmt.Sprintf(" required by %s", n.RequiringPackage)
	}
	if n.Importer != "" && n.Importer != n.RequiringPackage {
		line += fmt.Sprintf(" (importer %s)", n.Importer)
	}
	fmt.Fprintln(b, line)
	if len(n.SearchPath) > 0 {
		fmt.Fprintf(b, "%ssearch: %s\n", prefix, strings.Join(n.SearchPath, " → "))
	}
	if len(n.Candidates) > 0 {
		fmt.Fprintf(b, "%scandidates: %s\n", prefix, strings.Join(n.Candidates, ", "))
	}
	for _, r := range n.Rejected {
		fmt.Fprintf(b, "%srejected %s: %s\n", prefix, r.Version, r.Reason)
	}
	if n.Optional {
		fmt.Fprintf(b, "%soptional peer\n", prefix)
	}
	if n.AutoInstallPolicy {
		fmt.Fprintf(b, "%sauto-install peers enabled\n", prefix)
	}
	if n.Remediation != "" {
		fmt.Fprintf(b, "%sremediation: %s\n", prefix, n.Remediation)
	}
	for _, child := range n.Children {
		formatConflictNode(b, child, depth+1)
	}
}
