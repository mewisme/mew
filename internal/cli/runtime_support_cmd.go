package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/support"
)

func newRuntimeSupportBundleCmd() *cobra.Command {
	var (
		outputPath string
		forceOut   bool
		asJSON     bool
	)
	cmd := &cobra.Command{
		Use:   "support-bundle",
		Short: "Collect a redacted diagnostic bundle for bug reports",
		Long: `Gather a deterministic, redacted diagnostic archive suitable for
attaching to bug reports.

Every collected entry is explicitly allowlisted and sanitized:
credentials, environment values, source code, storage contents,
and private paths are never included. The bundle is a gzipped tar
archive with a machine-readable manifest.

By default the bundle is written to ./mew-support-<timestamp>.tgz.
Use --output to specify a path. Use --json to print the manifest
to stdout after collection.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "support-bundle", "", "missing app context")
			}

			// Resolve output path.
			out := outputPath
			if out == "" {
				out = filepath.Join(ac.CWD, "mew-support-bundle.tgz")
			}
			if !filepath.IsAbs(out) {
				out = filepath.Join(ac.CWD, out)
			}

			// Refuse overwrite unless --force.
			if !forceOut {
				if _, err := os.Stat(out); err == nil {
					return apperr.New(apperr.Usage, "support-bundle", out,
						"output file exists; use --force to overwrite")
				}
			}

			// Build collectors.
			collectors := []support.Collector{
				support.VersionCollector{},
				support.OSCollector{},
				support.NodeCollector{},
				support.FeaturesCollector{},
				support.DoctorCollector{},
				support.ConfigMetaCollector{},
			}

			bundle, err := support.Collect(context.Background(), ac, collectors)
			if err != nil {
				return err
			}

			// Write archive atomically.
			if err := support.WriteBundle(out, bundle); err != nil {
				return err
			}

			// Output.
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				_ = enc.Encode(bundle.Manifest)
				return nil
			}

			g := ownerFlags(cmd.Root())
			r := g.mustStaticRenderer(cmd)
			return writeStaticPrint(cmd, r.PlainText(out))
		},
	}
	cmd.Flags().StringVar(&outputPath, "output", "", "bundle archive path (default ./mew-support-bundle.tgz)")
	cmd.Flags().BoolVar(&forceOut, "force", false, "overwrite existing --output file")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print manifest as JSON to stdout after collection")
	return cmd
}
