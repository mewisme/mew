package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
)

// runtimeBenchResult is the structured output for m benchmark runtime --json.
type runtimeBenchResult struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Packages      []string                `json:"packages"`
	Cold          bool                    `json:"cold"`
	Environment   runtimeBenchEnv         `json:"environment"`
	Results       []runtimeBenchPkgResult `json:"results"`
}

type runtimeBenchEnv struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	GoVersion   string `json:"goVersion"`
	LogicalCPUs int    `json:"logicalCpus"`
}

type runtimeBenchPkgResult struct {
	Package string   `json:"package"`
	Output  []string `json:"output"`
}

func newBenchRuntimeCmd() *cobra.Command {
	var (
		cold   bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Benchmark runtime hot paths (transform, cache, launch)",
		Long:  "Run Go benchmarks for internal/runtime and internal/transform packages.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := findModuleRoot()
			if err != nil {
				return apperr.Wrap(apperr.Internal, "bench runtime", "", err)
			}

			if cold {
				if err := clearTransformCache(); err != nil {
					return err
				}
			}

			benchPkgs := []string{
				"./internal/runtime",
				"./internal/transform",
			}

			var results []runtimeBenchPkgResult
			var allOut strings.Builder
			for _, pkg := range benchPkgs {
				out, err := runBenchPkg(cmd.Context(), repoRoot, pkg)
				if err != nil {
					return apperr.Wrap(apperr.Internal, "bench runtime", pkg, err)
				}
				results = append(results, runtimeBenchPkgResult{
					Package: pkg,
					Output:  strings.Split(strings.TrimSpace(string(out)), "\n"),
				})
				allOut.Write(out)
				allOut.WriteByte('\n')
			}

			if asJSON {
				report := runtimeBenchResult{
					SchemaVersion: 1,
					Packages:      []string{"internal/runtime", "internal/transform"},
					Cold:          cold,
					Environment: runtimeBenchEnv{
						OS:          runtime.GOOS,
						Arch:        runtime.GOARCH,
						GoVersion:   runtime.Version(),
						LogicalCPUs: runtime.NumCPU(),
					},
					Results: results,
				}
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return apperr.Wrap(apperr.Internal, "bench runtime", "", err)
				}
				return writeStaticOut(cmd, string(data))
			}

			return writeStaticOut(cmd, allOut.String())
		},
	}
	cmd.Flags().BoolVar(&cold, "cold", false, "clear transform cache before benchmarking")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON result with measurements")
	return cmd
}

// clearTransformCache removes the transform cache directory using the
// production config CacheRoot to locate the correct path.
func clearTransformCache() error {
	cacheRoot := config.CacheRoot(nil)
	if cacheRoot == "" {
		return apperr.New(apperr.Internal, "bench runtime", "", "cannot resolve cache root")
	}
	transformCache := filepath.Join(cacheRoot, "transform")
	if err := os.RemoveAll(transformCache); err != nil {
		return apperr.Wrap(apperr.IO, "bench runtime", transformCache, err)
	}
	return nil
}

func runBenchPkg(ctx context.Context, repoRoot, pkg string) ([]byte, error) {
	args := []string{"test", pkg, "-bench=.", "-benchmem", "-count=1"}
	c := exec.CommandContext(ctx, "go", args...)
	c.Dir = repoRoot
	c.Env = append(os.Environ(), "CGO_ENABLED=0")
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}
