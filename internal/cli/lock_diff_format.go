package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/lockfile"
	"github.com/mewisme/mew/internal/presentation"
)

func runLockDiff(cmd *cobra.Command, ac *app.Context, opts app.LockDiffOptions, asJSON bool) error {
	diff, err := app.LockDiff(cmd.Context(), ac, opts)
	if err != nil {
		return err
	}
	if asJSON {
		data, err := lockfile.EncodeDiffJSON(diff)
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}
	g := ownerFlags(cmd.Root())
	r := g.mustStaticRenderer(cmd)
	return writeStaticOut(cmd, formatLockDiffHuman(r, diff))
}

// FormatLockDiffHuman writes a human-readable lock diff summary (plain renderer).
func FormatLockDiffHuman(w io.Writer, diff *lockfile.GraphDiff) error {
	r := presentation.NewStaticRenderer(presentation.EffectiveSettings{
		ThemeMode:  presentation.ThemeNone,
		Width:      80,
		UseUnicode: false,
		Symbols:    presentation.ASCIISymbols,
	})
	text := formatLockDiffHuman(r, diff)
	if text == "" {
		return nil
	}
	_, err := fmt.Fprintln(w, text)
	return err
}

func formatLockDiffHuman(r presentation.StaticRenderer, diff *lockfile.GraphDiff) string {
	if diff == nil {
		return ""
	}
	empty := len(diff.PackagesAdded) == 0 && len(diff.PackagesRemoved) == 0 &&
		len(diff.EdgesAdded) == 0 && len(diff.EdgesRemoved) == 0 && len(diff.Specifiers) == 0
	if empty {
		return r.Notice(emptyNotice("no changes"))
	}

	var parts []string
	var deltas []presentation.PackageDelta
	for _, pkg := range diff.PackagesAdded {
		name, ver := splitLockPkgID(pkg)
		deltas = append(deltas, presentation.PackageDelta{
			Kind: presentation.DeltaAdded, Name: name, Version: ver,
		})
	}
	for _, pkg := range diff.PackagesRemoved {
		name, ver := splitLockPkgID(pkg)
		deltas = append(deltas, presentation.PackageDelta{
			Kind: presentation.DeltaRemoved, Name: name, Version: ver,
		})
	}
	if len(deltas) > 0 {
		parts = append(parts, r.PackageDeltas(deltas))
	}
	for _, sp := range diff.Specifiers {
		parts = append(parts, r.PlainText(formatSpecifierDiff(sp, r.Settings().Symbols)))
	}
	return strings.Join(parts, "\n")
}

func splitLockPkgID(id string) (name, version string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ""
	}
	if strings.HasPrefix(id, "@") {
		i := strings.LastIndex(id, "@")
		if i > 0 {
			return id[:i], id[i+1:]
		}
		return id, ""
	}
	name, version, ok := strings.Cut(id, "@")
	if !ok {
		return id, ""
	}
	return name, version
}

func formatSpecifierDiff(sp lockfile.ImporterSpecifierDiff, sym presentation.Symbols) string {
	importer := sp.Importer
	if importer == "" {
		importer = "."
	}
	kind := sp.Kind
	if kind != "" {
		kind = " " + kind
	}
	name := sp.Name
	switch {
	case sp.Before == "" && sp.After != "":
		return fmt.Sprintf("%s %s %s%s: %s%s", sym.Updated, importer, name, kind, sym.Added, sp.After)
	case sp.Before != "" && sp.After == "":
		return fmt.Sprintf("%s %s %s%s: %s%s", sym.Updated, importer, name, kind, sym.Removed, sp.Before)
	default:
		return fmt.Sprintf("%s %s %s%s: %s %s %s", sym.Updated, importer, name, kind, sp.Before, sym.Arrow, sp.After)
	}
}

// shortDigest truncates a digest with the canonical ellipsis when available.
// When no symbols are available, uses literal "..." as fallback.
func shortDigest(digest string) string { return shortDigestWith(digest, "...") }

func shortDigestWith(digest string, ellipsis string) string {
	const prefix = "sha256:"
	if strings.HasPrefix(digest, prefix) && len(digest) > len(prefix)+12 {
		return digest[:len(prefix)+12] + ellipsis
	}
	if len(digest) > 16 {
		return digest[:16] + ellipsis
	}
	return digest
}
