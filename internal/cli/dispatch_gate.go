package cli

import (
	"os"

	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/workspace"
)

// DirectScriptsEnabled reports whether direct m <script> shortcuts are turned on.
func DirectScriptsEnabled(eff *config.Effective) bool {
	if config.Bool(eff, "runner.direct_scripts.enabled", true) {
		return true
	}
	if eff != nil && eff.Env.Initialized() {
		if v, ok := eff.Env.Lookup("MEW_EXPERIMENTAL_DIRECT_SCRIPTS"); ok && v == "1" {
			return true
		}
	}
	return os.Getenv("MEW_EXPERIMENTAL_DIRECT_SCRIPTS") == "1"
}

// WorkspaceDirectEnabled requires both direct-script and workspace gates.
func WorkspaceDirectEnabled(eff *config.Effective) bool {
	return DirectScriptsEnabled(eff) && workspace.Enabled(eff)
}
