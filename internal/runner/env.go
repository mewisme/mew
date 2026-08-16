package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mewisme/mew/internal/process"
)

// EnvOptions configures npm-compatible script environment construction.
type EnvOptions struct {
	HostEnv     []string
	InitCWD     string
	PackageDir  string
	NodeModules string
	PackageJSON string
	PackageName string
	PackageVer  string
	Event       string
	Script      string
}

// BuildEnv constructs npm-compatible script environment from a host snapshot.
// It mutates PATH to prepend node_modules/.bin and sets npm lifecycle variables.
// Host env is copied as-is except for overridden keys; RestrictedEnv is not used.
func BuildEnv(opts EnvOptions) ScriptEnv {
	dir := opts.PackageDir
	if dir == "" {
		dir = opts.InitCWD
	}

	pathKey := "PATH"
	if runtime.GOOS == "windows" {
		pathKey = "Path"
	}
	binDir := process.BinDirForPackage(opts.PackageDir, opts.NodeModules)

	set := map[string]string{
		"INIT_CWD":             opts.InitCWD,
		"npm_lifecycle_event":  opts.Event,
		"npm_lifecycle_script": opts.Script,
		"npm_package_name":     opts.PackageName,
		"npm_package_version":  opts.PackageVer,
		"npm_package_json":     opts.PackageJSON,
		pathKey:                prependPath(binDir, opts.HostEnv, pathKey),
	}

	// Build a normalized set of overridden keys so Windows case-insensitive
	// comparison catches casing variants (e.g. init_cwd vs INIT_CWD).
	replacedKeys := make(map[string]bool, len(set))
	for k := range set {
		replacedKeys[strings.ToUpper(k)] = true
	}

	out := make([]string, 0, len(opts.HostEnv)+len(set))
	for _, kv := range opts.HostEnv {
		key := envKey(kv)
		if replacedKeys[strings.ToUpper(key)] {
			continue
		}
		out = append(out, kv)
	}
	keys := []string{
		"INIT_CWD",
		"npm_lifecycle_event",
		"npm_lifecycle_script",
		"npm_package_name",
		"npm_package_version",
		"npm_package_json",
		pathKey,
	}
	for _, key := range keys {
		out = append(out, key+"="+set[key])
	}
	return ScriptEnv{Dir: dir, Vars: out}
}

// PackageJSONPath returns the absolute package.json path for env construction.
func PackageJSONPath(packageDir string) string {
	if packageDir == "" {
		return ""
	}
	return filepath.Join(packageDir, "package.json")
}

func prependPath(binDir string, host []string, pathKey string) string {
	if binDir == "" {
		if old, ok := lookupEnv(host, pathKey); ok {
			return old
		}
		return ""
	}
	old, ok := lookupEnv(host, pathKey)
	if !ok || old == "" {
		return binDir
	}
	return binDir + string(os.PathListSeparator) + old
}

func envKey(kv string) string {
	if i := strings.IndexByte(kv, '='); i >= 0 {
		return kv[:i]
	}
	return kv
}

func lookupEnv(env []string, key string) (string, bool) {
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			if strings.EqualFold(kv[:i], key) {
				return kv[i+1:], true
			}
		}
	}
	return "", false
}
