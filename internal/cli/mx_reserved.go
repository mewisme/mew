package cli

import (
	"github.com/spf13/cobra"
)

// MXReservedNames returns built-in mx commands that preempt DLX dispatch.
func MXReservedNames(root *cobra.Command) []string {
	if root == nil {
		return []string{"version", "completion", "cache"}
	}
	names := []string{}
	for _, c := range root.Commands() {
		if c == nil || c.Hidden {
			continue
		}
		names = append(names, c.Name())
		names = append(names, c.Aliases...)
	}
	return names
}

// IsMXReserved reports whether selector is a built-in mx command.
func IsMXReserved(root *cobra.Command, selector string) bool {
	for _, name := range MXReservedNames(root) {
		if name == selector {
			return true
		}
	}
	return false
}

// isMXBuiltin reports whether selector is an mx built-in command (version,
// completion, cache) or a registered subcommand of the mx root. Unlike
// IsMXReserved, it only considers the true mx root — not the m root — so it
// is safe to use from m x dispatch.
func isMXBuiltin(root *cobra.Command, selector string) bool {
	if root == nil {
		return IsMXReserved(nil, selector)
	}
	// If we are inside m (not mx), only check the fixed built-in list.
	if root.Name() != "mx" && root.Name() != "mewx" {
		return IsMXReserved(nil, selector)
	}
	return IsMXReserved(root, selector)
}
