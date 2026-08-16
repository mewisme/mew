package runtime_test

import (
	"strings"
	"testing"
)

// --- Env auto-discovery ---

func TestConformanceEnvAutoDiscovery(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// Create .env file and verify the child process can read it
	writeFile(t, proj, ".env", "CONFORMANCE_AUTO_VAR=auto-discovered\n")
	writeFile(t, proj, "env-auto.js",
		`const fs = require("node:fs"); fs.writeFileSync("output.txt", process.env.CONFORMANCE_AUTO_VAR || "absent");`)
	code, _ := runM(t, proj, "env-auto.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "auto-discovered" {
		t.Fatalf("expected 'auto-discovered', got %q", got)
	}
}

func TestConformanceEnvShellPrecedence(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// env-dump.js writes a specific env var to output.txt
	t.Setenv("MEW_CONFORMANCE_TEST_VAR", "shell-wins")
	writeFile(t, proj, ".env", "MEW_CONFORMANCE_TEST_VAR=dotenv-value\n")
	writeFile(t, proj, "env-dump-custom.js",
		`const fs = require("node:fs"); fs.writeFileSync("output.txt", process.env.MEW_CONFORMANCE_TEST_VAR || "absent");`)
	code, _ := runM(t, proj, "env-dump-custom.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "shell-wins" {
		t.Fatalf("shell env should take precedence over dotenv, got %q", got)
	}
}

// --- Mode-aware env loading ---

func TestConformanceEnvMode(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// Create a mode-specific env file
	writeFile(t, proj, ".env.staging", "MEW_MODE_VAR=staging-value\n")
	writeFile(t, proj, "env-dump-mode.js",
		`const fs = require("node:fs"); fs.writeFileSync("output.txt", process.env.MEW_MODE_VAR || "absent");`)
	code, _ := runM(t, proj, "--mode", "staging", "env-dump-mode.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "staging-value" {
		t.Fatalf("expected staging-value, got %q", got)
	}
}

// --- Explicit env-file ---

func TestConformanceEnvExplicitFile(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	writeFile(t, proj, "custom.env", "EXPLICIT_VAR=explicit-value\n")
	writeFile(t, proj, "env-dump-explicit.js",
		`const fs = require("node:fs"); fs.writeFileSync("output.txt", process.env.EXPLICIT_VAR || "absent");`)
	code, _ := runM(t, proj, "--env-file", "custom.env", "env-dump-explicit.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "explicit-value" {
		t.Fatalf("expected explicit-value, got %q", got)
	}
}

// --- Env expansion ---

func TestConformanceEnvExpansion(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	writeFile(t, proj, ".env",
		"BASE=root\nEXPANDED=${BASE}/app\n")
	writeFile(t, proj, "env-dump-expand.js",
		`const fs = require("node:fs"); fs.writeFileSync("output.txt", process.env.EXPANDED || "absent");`)
	code, _ := runM(t, proj, "env-dump-expand.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "root/app") {
		t.Fatalf("expected expansion 'root/app', got %q", got)
	}
}

// --- --no-env-file ---

func TestConformanceEnvNoFile(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	writeFile(t, proj, ".env", "SUPPRESSED_VAR=should-not-appear\n")
	writeFile(t, proj, "env-dump-noenv.js",
		`const fs = require("node:fs"); fs.writeFileSync("output.txt", process.env.SUPPRESSED_VAR || "absent");`)
	code, _ := runM(t, proj, "--no-env-file", "env-dump-noenv.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if got != "absent" {
		t.Fatalf("expected 'absent' with --no-env-file, got %q", got)
	}
}
