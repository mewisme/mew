package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/project"
	"github.com/mewisme/mew/internal/registry"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the download cache",
	}
	cmd.AddCommand(newCacheDirCmd())
	cmd.AddCommand(newCacheVerifyCmd())
	cmd.AddCommand(newCacheExplainCmd())
	meta := &cobra.Command{Use: "metadata", Short: "Registry metadata cache"}
	meta.AddCommand(newCacheMetadataInspectCmd())
	cmd.AddCommand(meta)
	return cmd
}

func newCacheVerifyCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify tarball blob cache integrity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "cache.verify", "", "missing app context")
			}
			res, err := app.VerifyBlobCache(cmd.Context(), ac)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				return enc.Encode(res)
			}
			g := ownerFlags(cmd.Root())
			r := g.mustStaticRenderer(cmd)
			return writeStaticOut(cmd, r.Summary(cacheVerifySummary(res.OK, res.Bad, res.Skip)))
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print as JSON")
	return cmd
}

func newCacheDirCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dir",
		Short: "Print the registry metadata cache directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "cache.dir", "", "missing app context")
			}
			dir := config.RegistryMetadataCacheDir(ac.Config)
			_, err := fmt.Fprintln(cmd.OutOrStdout(), dir)
			return err
		},
	}
}

func newCacheMetadataInspectCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "inspect <name>",
		Short: "Inspect a cached packument entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "cache.inspect", "", "missing app context")
			}
			name := args[0]
			p, err := app.OpenProject(cmd.Context(), ac)
			if err != nil {
				return err
			}
			base := registry.ResolveBaseForPackage(ac.Config, p.Root, p.Identity, name)
			cache := &registry.DiskCache{Root: config.RegistryMetadataCacheDir(ac.Config)}
			dir, etag, present := cache.Inspect(registry.OriginKey(base), name)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				return enc.Encode(map[string]any{
					"name":     name,
					"dir":      dir,
					"etag":     etag,
					"present":  present,
					"registry": base,
				})
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "name=%s\n", name)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "registry=%s\n", base)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dir=%s\n", dir)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "present=%v\n", present)
			if etag != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "etag=%s\n", etag)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print as JSON")
	return cmd
}

func newViewCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "view <name>[@version]",
		Short: "View package metadata from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "view", "", "missing app context")
			}
			name, version := splitNameVersion(args[0])
			var root string
			id := project.IdentityMew
			if p, err := app.OpenProject(cmd.Context(), ac); err == nil {
				root = p.Root
				id = p.Identity
			} else if apperr.CodeOf(err) != apperr.NotFound {
				return err
			}
			client, err := registry.NewFromApp(ac.Config, root, id)
			if err != nil {
				return err
			}
			base := registry.ResolveBaseForPackage(ac.Config, root, id, name)
			pack, err := client.Packument(cmd.Context(), base, name)
			if err != nil {
				return err
			}
			if version != "" {
				meta, err := pack.SelectVersion(version)
				if err != nil {
					return err
				}
				if asJSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetEscapeHTML(false)
					return enc.Encode(meta)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s@%s\n", name, meta.Version)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "integrity=%s\n", meta.Dist.Integrity)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "tarball=%s\n", meta.Dist.Tarball)
				return nil
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetEscapeHTML(false)
				doc := map[string]any{
					"name":      pack.Name,
					"dist-tags": pack.DistTags,
					"versions":  pack.SortedVersions(),
				}
				return enc.Encode(doc)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "name=%s\n", pack.Name)
			if latest, ok := pack.DistTags["latest"]; ok {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "latest=%s\n", latest)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "versions=%d\n", len(pack.Versions))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print as JSON")
	return cmd
}

func splitNameVersion(spec string) (name, version string) {
	if strings.HasPrefix(spec, "@") {
		// @scope/name or @scope/name@version
		rest := spec[1:]
		if i := strings.LastIndexByte(rest, '@'); i > 0 {
			return spec[:i+1], rest[i+1:]
		}
		return spec, ""
	}
	name, version, _ = strings.Cut(spec, "@")
	return name, version
}
