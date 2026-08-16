package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/presentation"
)

func TestHelpViaExecutePath(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"root", []string{"help"}, "Use \"\x1b[95;1mm\x1b[39;22m help <topic>\""},
		{"errors-index", []string{"help", "--pager=never", "errors"}, "Error help index"},
		{"errors-code", []string{"help", "--pager=never", "errors", "ERR_M_LOCKFILE"}, "ERR_M_LOCKFILE"},
		{"command", []string{"help", "install"}, "Examples:"},
		{"flag-help", []string{"--help"}, "Use \"\x1b[95;1mm\x1b[39;22m help <topic>\""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := testBuildInfo()
			root := NewMRoot(info)
			buf := new(strings.Builder)
			root.SetOut(buf)
			root.SetErr(buf)
			code := execute(root, info, append([]string(nil), tc.argv...))
			out := buf.String()
			if code != 0 {
				t.Fatalf("exit %d\nout:\n%s", code, out)
			}
			if strings.Contains(out, `unknown command "help"`) {
				t.Fatalf("direct dispatch stole help:\n%s", out)
			}
			if !strings.Contains(out, tc.want) {
				t.Fatalf("missing %q in:\n%s", tc.want, out)
			}
		})
	}
}

func TestHelpTopicRunner(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	buf := new(strings.Builder)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"help", "--pager=never", "runner"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "m run") {
		t.Fatalf("topic help missing content:\n%s", out)
	}
}

func TestTopicHelpUsePlainRichColorForcesRich(t *testing.T) {
	caps := presentation.Capabilities{
		StdoutTTY:    true,
		StderrTTY:    true,
		ColorProfile: presentation.ColorProfileTrueColor,
		Width:        80,
	}
	opts := presentation.ResolvedOptions{
		Output: presentation.OutputRich,
		Color:  true,
	}
	eff := presentation.Effective(opts, caps, nil)
	if topicHelpUsePlain(opts, eff) {
		t.Fatal("expected rich path for color-enabled rich on TTY")
	}
}

func TestTopicHelpUsePlainNoColorStaysPlain(t *testing.T) {
	caps := presentation.Capabilities{
		StdoutTTY:    false,
		StderrTTY:    false,
		ColorProfile: presentation.ColorProfileTrueColor,
	}
	opts := presentation.ResolvedOptions{
		Output: presentation.OutputRich,
		Color:  false,
	}
	eff := presentation.Effective(opts, caps, nil)
	if !topicHelpUsePlain(opts, eff) {
		t.Fatal("expected plain path for no-color")
	}
}

func TestHelpTopicNoColorUsesPlain(t *testing.T) {
	clearHelpEnv(t)
	root := NewMRoot(testBuildInfo())
	buf := new(strings.Builder)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--no-color", "help", "--pager=never", "runner"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected no ANSI with --no-color:\n%q", out)
	}
}

func clearHelpEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "NO_COLOR", "COLORFGBG"} {
		key := key
		prev, had := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		if had {
			t.Cleanup(func() { _ = os.Setenv(key, prev) })
		}
	}
}

func TestThemeModeResolvesCorrectly(t *testing.T) {
	caps := presentation.Capabilities{
		StdoutTTY:    true,
		ColorProfile: presentation.ColorProfileANSI,
		Width:        80,
	}
	// Explicit light with color → ThemeLight.
	opts := presentation.ResolvedOptions{
		Color:  true,
		Theme:  "light",
		Output: presentation.OutputRich,
	}
	eff := presentation.Effective(opts, caps, nil)
	if eff.ThemeMode != presentation.ThemeLight {
		t.Fatalf("ThemeMode=%q want light with color", eff.ThemeMode)
	}

	// No color → ThemeNone regardless of ui.theme.
	opts = presentation.ResolvedOptions{
		Color:  false,
		Theme:  "dark",
		Output: presentation.OutputRich,
	}
	eff = presentation.Effective(opts, caps, nil)
	if eff.ThemeMode != presentation.ThemeNone {
		t.Fatalf("ThemeMode=%q want none without color", eff.ThemeMode)
	}

	// ResolveTheme with explicit dark → ThemeDark.
	if got := presentation.ResolveTheme("dark", nil); got != presentation.ThemeDark {
		t.Fatalf("ResolveTheme(dark)=%q want dark", got)
	}
	// ResolveTheme with auto + nil detector → ThemeLight (fallback).
	if got := presentation.ResolveTheme("auto", nil); got != presentation.ThemeLight {
		t.Fatalf("ResolveTheme(auto, nil)=%q want light (fallback)", got)
	}
}

func TestHelpCommandWinsOverTopicAlias(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	buf := new(strings.Builder)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"help", "trust"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Trust lifecycle scripts") {
		t.Fatalf("expected trust command help:\n%s", out)
	}
	if strings.Contains(out, "lifecycle.script_trust") {
		t.Fatalf("got topic content instead of command help:\n%s", out)
	}
}

func TestHelpInstallStillCommand(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	buf := new(strings.Builder)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"help", "install"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Examples:") {
		t.Fatalf("install help missing examples:\n%s", out)
	}
}

func TestHelpErrorsCode(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	buf := new(strings.Builder)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"help", "--pager=never", "errors", "ERR_M_LOCKFILE"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ERR_M_LOCKFILE") {
		t.Fatalf("missing lockfile topic:\n%s", buf.String())
	}
}

func TestHelpUnknownTopic(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	buf := new(strings.Builder)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"help", "no-such-topic-xyz"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error")
	}
}
