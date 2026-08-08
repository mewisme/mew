package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/node"
	"github.com/mewisme/mew/internal/runtime/assets"
	"github.com/mewisme/mew/internal/transform"
)

// diagResult is the structured JSON output from resolve-diagnostic.mjs.
type diagResult struct {
	SchemaVersion int         `json:"schemaVersion"`
	Specifier     string      `json:"specifier"`
	Importer      string      `json:"importer"`
	Resolved      bool        `json:"resolved"`
	Target        *diagTarget `json:"target"`
	Error         *diagError  `json:"error"`
	PnP           *diagPnP    `json:"pnp,omitempty"`
	Trace         []diagStep  `json:"trace"`
}

type diagTarget struct {
	URL    string `json:"url"`
	Path   string `json:"path,omitempty"`
	Format string `json:"format,omitempty"`
}

type diagError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Stage   string `json:"stage,omitempty"`
}

type diagPnP struct {
	Root string `json:"root,omitempty"`
}

type diagStep struct {
	Stage       string   `json:"stage"`
	Outcome     string   `json:"outcome"`
	Resolved    string   `json:"resolved,omitempty"`
	Substituted string   `json:"substituted,omitempty"`
	Format      string   `json:"format,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	Targets     []string `json:"targets,omitempty"`
	Error       string   `json:"error,omitempty"`
	Code        string   `json:"code,omitempty"`
	Note        string   `json:"note,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Candidate   string   `json:"candidate,omitempty"`
	PnPRoot     string   `json:"pnpRoot,omitempty"`
	Specifier   string   `json:"specifier,omitempty"`
	URL         string   `json:"url,omitempty"`
}

func newResolveModuleCmd() *cobra.Command {
	var fromDir string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "resolve-module <specifier>",
		Short: "Show how Mew resolves a module specifier",
		Long: `Show the full resolution trace for a module specifier as Mew's
runtime loader would resolve it, including tsconfig paths, PnP,
extension substitution, and Node native resolution.

This is a diagnostic tool. It runs the same resolution algorithm
used by the runtime loader and reports the actual result.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ac := app.FromContext(cmd.Context())
			if ac == nil {
				return apperr.New(apperr.Internal, "resolve-module", "", "no app context")
			}

			specifier := args[0]
			cwd := ac.CWD
			if fromDir != "" {
				cwd = fromDir
			}
			if !filepath.IsAbs(cwd) {
				var err error
				cwd, err = filepath.Abs(cwd)
				if err != nil {
					return apperr.Wrap(apperr.IO, "resolve-module", cwd, err)
				}
			}

			// Discover tsconfig from the target directory.
			configPath, err := transform.DiscoverTsconfig(cwd)
			if err != nil {
				return apperr.Wrap(apperr.TransformConfigParse, "resolve-module", cwd, err)
			}

			// Build diagnostic options from tsconfig.
			diagOpts := buildDiagnosticOptions(configPath, cwd)

			// Run the real resolution via Node diagnostic script.
			result, err := runDiagnostic(cmd.Context(), specifier, cwd, diagOpts)
			if err != nil {
				// If Node isn't available, fall back to static tsconfig analysis.
				if jsonOutput {
					return renderResolveModuleJSON(cmd, specifier, cwd, configPath, nil)
				}
				return renderResolveModuleText(cmd, specifier, cwd, configPath, nil)
			}

			var renderErr error
			if jsonOutput {
				renderErr = renderResolveModuleJSON(cmd, specifier, cwd, configPath, result)
			} else {
				renderErr = renderResolveModuleText(cmd, specifier, cwd, configPath, result)
			}
			// Exit non-zero when resolution failed.
			if renderErr == nil && result != nil && !result.Resolved {
				return apperr.New(apperr.Resolve, "resolve-module", specifier, "module not found")
			}
			return renderErr
		},
	}

	cmd.Flags().StringVar(&fromDir, "from", "", "directory to resolve from (default: cwd)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")

	return cmd
}

// buildDiagnosticOptions creates the JSON options blob for the diagnostic script.
func buildDiagnosticOptions(configPath, cwd string) map[string]interface{} {
	opts := map[string]interface{}{}
	if configPath == "" {
		opts["configDir"] = cwd
		return opts
	}

	chain, err := transform.LoadTsconfigChain(configPath)
	if err != nil {
		opts["configDir"] = cwd
		return opts
	}

	normalized, err := transform.NormalizeOptions(chain)
	if err != nil {
		opts["configDir"] = filepath.Dir(configPath)
		return opts
	}

	opts["configDir"] = filepath.Dir(configPath)
	if normalized.BaseURL != "" {
		opts["baseUrl"] = normalized.BaseURL
	}
	if len(normalized.PathMappings) > 0 {
		opts["pathMappings"] = normalized.PathMappings
	} else if len(normalized.Paths) > 0 {
		opts["paths"] = normalized.Paths
	}
	return opts
}

// runDiagnostic extracts the diagnostic assets, finds Node, and runs the
// resolution script. Returns the parsed result or an error.
func runDiagnostic(ctx context.Context, specifier, importer string, opts map[string]interface{}) (*diagResult, error) {
	// Find Node.
	nodeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	nodeInst, err := node.Discover(nodeCtx, node.Request{
		WorkingDir:        importer,
		ExplicitCandidate: "",
	})
	if err != nil {
		return nil, err
	}

	// Extract diagnostic assets to a temp directory.
	tmpDir, err := os.MkdirTemp("", "mew-resolve-diag-*")
	if err != nil {
		return nil, apperr.Wrap(apperr.IO, "resolve-module", tmpDir, err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write resolve-utils.mjs and resolve-diagnostic.mjs.
	assetsToWrite := []string{"resolve-utils.mjs", "resolve-diagnostic.mjs"}
	for _, name := range assetsToWrite {
		data, err := assets.ReadAsset(name)
		if err != nil {
			return nil, apperr.Wrap(apperr.RuntimeAssetCache, "resolve-module", name, err)
		}
		dest := filepath.Join(tmpDir, name)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return nil, apperr.Wrap(apperr.IO, "resolve-module", dest, err)
		}
	}

	// Build options JSON.
	optsJSON, err := json.Marshal(opts)
	if err != nil {
		return nil, apperr.Wrap(apperr.Internal, "resolve-module", "", err)
	}

	// Build Node command.
	scriptPath := filepath.Join(tmpDir, "resolve-diagnostic.mjs")
	nodeExe := nodeInst.ExePath

	// Determine if we need --experimental-import-meta-resolve.
	extraArgs := nodeImportMetaResolveFlag(nodeInst.NormalizedVersion)

	args := append([]string{}, extraArgs...)
	args = append(args, scriptPath)

	cmd := exec.CommandContext(ctx, nodeExe, args...)
	cmd.Dir = importer
	cmd.Env = append(os.Environ(),
		"MEW_DIAG_SPECIFIER="+specifier,
		"MEW_DIAG_IMPORTER="+filepath.Join(importer, "_mew_diag_.mjs"),
		"MEW_DIAG_OPTIONS="+string(optsJSON),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()

	// Parse JSON output — the diagnostic script writes valid JSON even on
	// resolution failure (exit 1). Only treat missing/invalid JSON as an
	// infrastructure error that triggers the static analysis fallback.
	var result diagResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("diagnostic parse failed: %w\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	return &result, nil
}

// nodeImportMetaResolveFlag returns extra Node flags needed for import.meta.resolve.
// Node 18.x requires --experimental-import-meta-resolve.
// Node 20.6+ has it stable.
// Node < 18 doesn't support it at all.
func nodeImportMetaResolveFlag(version string) []string {
	major := parseNodeMajor(version)
	if major >= 20 {
		return nil
	}
	if major == 18 {
		return []string{"--experimental-import-meta-resolve"}
	}
	return nil
}

func parseNodeMajor(version string) int {
	v := strings.TrimPrefix(version, "v")
	parts := strings.SplitN(v, ".", 2)
	if len(parts) > 0 {
		var major int
		_, _ = fmt.Sscanf(parts[0], "%d", &major)
		return major
	}
	return 0
}

// ── Text renderer ────────────────────────────────────────────────────

func renderResolveModuleText(cmd *cobra.Command, specifier, cwd, configPath string, result *diagResult) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Specifier: %s\n", specifier)
	fmt.Fprintf(w, "From:      %s\n", cwd)

	if configPath != "" {
		fmt.Fprintf(w, "Tsconfig:  %s\n", configPath)
	} else {
		fmt.Fprintln(w, "Tsconfig:  (none found)")
	}

	if result == nil {
		fmt.Fprintln(w, "\nDiagnostic resolution unavailable (Node not found or script failed).")
		_, _ = fmt.Fprint(w, "Showing tsconfig path analysis only.\n\n")
		return renderStaticAnalysis(w, specifier, cwd, configPath)
	}

	// Render trace.
	fmt.Fprintln(w, "\nResolution trace:")
	for i, step := range result.Trace {
		fmt.Fprintf(w, "  %d. %-20s — %s", i+1, step.Stage, step.Outcome)
		switch step.Outcome {
		case "resolved":
			if step.Substituted != "" {
				fmt.Fprintf(w, " (%s → %s)", step.Resolved, step.Substituted)
			} else if step.Resolved != "" {
				fmt.Fprintf(w, " (%s)", step.Resolved)
			}
			if step.Format != "" {
				fmt.Fprintf(w, " [%s]", step.Format)
			}
			if step.Pattern != "" {
				fmt.Fprintf(w, "\n      pattern: %s", step.Pattern)
			}
			if len(step.Targets) > 0 {
				fmt.Fprintf(w, "\n      targets: %s", strings.Join(step.Targets, ", "))
			}
		case "miss":
			if step.Error != "" {
				fmt.Fprintf(w, " (%s)", step.Error)
			}
		case "error":
			if step.Error != "" {
				fmt.Fprintf(w, " — %s", step.Error)
			}
			if step.Code != "" {
				fmt.Fprintf(w, " [%s]", step.Code)
			}
		case "skipped":
			if step.Reason != "" {
				fmt.Fprintf(w, " (%s)", step.Reason)
			}
		}
		if step.Note != "" {
			fmt.Fprintf(w, "\n      note: %s", step.Note)
		}
		if step.PnPRoot != "" {
			fmt.Fprintf(w, "\n      pnp root: %s", step.PnPRoot)
		}
		fmt.Fprintln(w)
	}

	// Render final result.
	fmt.Fprintln(w)
	if result.Resolved && result.Target != nil {
		fmt.Fprintf(w, "Resolved: %s\n", result.Target.URL)
		if result.Target.Path != "" {
			fmt.Fprintf(w, "Path:     %s\n", result.Target.Path)
		}
		if result.Target.Format != "" {
			fmt.Fprintf(w, "Format:   %s\n", result.Target.Format)
		}
	} else if result.Error != nil {
		fmt.Fprintf(w, "Error:    [%s] %s\n", result.Error.Code, result.Error.Message)
	}

	if result.PnP != nil && result.PnP.Root != "" {
		fmt.Fprintf(w, "PnP root: %s\n", result.PnP.Root)
	}

	return nil
}

// renderStaticAnalysis prints tsconfig path analysis without Node resolution.
func renderStaticAnalysis(w io.Writer, specifier, cwd, configPath string) error {
	if configPath == "" {
		fmt.Fprintln(w, "No tsconfig paths configured. Resolution falls through to Node defaults.")
		return nil
	}

	chain, err := transform.LoadTsconfigChain(configPath)
	if err != nil {
		return apperr.Wrap(apperr.TransformConfigParse, "resolve-module", configPath, err)
	}

	opts, err := transform.NormalizeOptions(chain)
	if err != nil {
		return apperr.Wrap(apperr.TransformConfigOption, "resolve-module", configPath, err)
	}

	baseDir := filepath.Dir(configPath)
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = "."
	}
	fmt.Fprintf(w, "BaseUrl:   %s", baseURL)
	if !filepath.IsAbs(baseURL) {
		resolved := filepath.Join(baseDir, baseURL)
		fmt.Fprintf(w, "  (resolved: %s)", resolved)
	}
	fmt.Fprintln(w)

	if len(opts.Paths) == 0 {
		fmt.Fprintln(w, "Paths:     (none)")
		return nil
	}

	fmt.Fprintln(w, "Paths:")
	for _, pm := range opts.PathMappings {
		fmt.Fprintf(w, "  %s → %s\n", pm.Pattern, strings.Join(pm.Targets, ", "))
	}

	fmt.Fprintf(w, "\nMatching %q against path patterns:\n", specifier)
	matched := false
	for _, pm := range opts.PathMappings {
		captures := matchPathPattern(specifier, pm.Pattern)
		if captures == nil {
			continue
		}
		matched = true
		fmt.Fprintf(w, "  %s matched (captures: %v)\n", pm.Pattern, captures)
		resolveBase := baseDir
		if baseURL != "." && baseURL != "" {
			resolveBase = filepath.Join(baseDir, baseURL)
		}
		for _, replacement := range pm.Targets {
			resolved := replacement
			for _, cap := range captures {
				resolved = strings.Replace(resolved, "*", cap, 1)
			}
			full := filepath.Join(resolveBase, resolved)
			fmt.Fprintf(w, "    → %s\n", full)
		}
	}
	if !matched {
		fmt.Fprintln(w, "  (no patterns matched)")
	}

	pnpRoot := findPnpRoot(cwd)
	if pnpRoot != "" {
		fmt.Fprintf(w, "\nPnP root:  %s\n", pnpRoot)
	}

	return nil
}

// ── JSON renderer ─────────────────────────────────────────────────────

func renderResolveModuleJSON(cmd *cobra.Command, specifier, cwd, configPath string, result *diagResult) error {
	w := cmd.OutOrStdout()

	if result == nil {
		// No diagnostic result — emit a minimal JSON report.
		report := map[string]interface{}{
			"schemaVersion": 1,
			"specifier":     specifier,
			"importer":      cwd,
			"resolved":      false,
			"target":        nil,
			"error": map[string]interface{}{
				"code":    "ERR_M_DIAGNOSTIC_UNAVAILABLE",
				"message": "Node diagnostic unavailable; tsconfig analysis only",
			},
			"trace":        []interface{}{},
			"tsconfigPath": configPath,
		}
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// ── Path pattern matching (Go port, used in static analysis fallback) ─

// matchPathPattern is a Go port of the ts-loader.mjs path-matching logic.
func matchPathPattern(specifier, pattern string) []string {
	if !strings.Contains(pattern, "*") {
		if specifier == pattern {
			return []string{""}
		}
		return nil
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		prefix, suffix := parts[0], parts[1]
		sufLen := len(suffix)
		if strings.HasPrefix(specifier, prefix) && strings.HasSuffix(specifier, suffix) &&
			len(specifier) >= len(prefix)+sufLen {
			captured := specifier[len(prefix) : len(specifier)-sufLen]
			return []string{captured}
		}
		return nil
	}
	// Multiple wildcards: sequential match.
	remaining := specifier
	var captures []string
	for i, part := range parts {
		switch {
		case i == 0:
			if !strings.HasPrefix(remaining, part) {
				return nil
			}
			remaining = remaining[len(part):]
		case i == len(parts)-1:
			if part == "" {
				captures = append(captures, remaining)
			} else if !strings.HasSuffix(remaining, part) {
				return nil
			} else {
				captures = append(captures, remaining[:len(remaining)-len(part)])
			}
		default:
			idx := strings.Index(remaining, part)
			if idx == -1 {
				return nil
			}
			captures = append(captures, remaining[:idx])
			remaining = remaining[idx+len(part):]
		}
	}
	return captures
}

// findPnpRoot walks up from dir looking for .pnp.cjs.
func findPnpRoot(dir string) string {
	current := dir
	for {
		candidate := filepath.Join(current, ".pnp.cjs")
		if _, err := os.Stat(candidate); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}
