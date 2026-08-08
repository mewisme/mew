package runtime_test

import (
	"strings"
	"testing"
)

// --- Worker thread isolation ---

func TestConformanceWorkerImportsTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// worker-ts.mjs spawns a worker that imports worker-ts-task.mjs
	code, _ := runM(t, proj, "worker-ts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "libValue=resolved-lib-ts") {
		t.Fatalf("expected 'libValue=resolved-lib-ts', got %q", got)
	}
}

func TestConformanceWorkerCredentialIsolation(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// worker-creds.mjs checks that worker threads cannot read transform credentials
	code, _ := runM(t, proj, "worker-creds.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	// Worker should see "absent" for transform credentials
	if strings.Contains(got, "MEW_TRANSFORM_TOKEN=") && !strings.Contains(got, "MEW_TRANSFORM_TOKEN=absent") {
		t.Fatalf("worker leaked transform credentials: %s", got)
	}
}

func TestConformanceWorkerEnviron(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// worker-env.mjs checks env propagation to workers
	code, _ := runM(t, proj, "worker-env.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

// --- Child process (Issue 39) ---

func TestConformanceChildProcessNoCreds(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// child-check.js spawns a child via spawnSync and verifies
	// MEW_TRANSFORM_* are absent in the child environment.
	code, _ := runM(t, proj, "child-check.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	// Every line must show "absent" for credential vars.
	for _, line := range strings.Split(got, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			val := parts[1]
			if val != "absent" {
				t.Errorf("credential leaked: %s", line)
			}
		}
	}
}

func TestConformanceChildForkImportsTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// child-fork-ts.mjs forks a child that imports lib.ts.
	code, _ := runM(t, proj, "child-fork-ts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "libValue=resolved-lib-ts") {
		t.Fatalf("expected 'libValue=resolved-lib-ts', got %q", got)
	}
}

func TestConformanceChildSpawnImportsTS(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// child-spawn-ts.mjs uses spawn(process.execPath, ...) to create
	// a child that imports lib.ts.
	code, _ := runM(t, proj, "child-spawn-ts.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "libValue=resolved-lib-ts") {
		t.Fatalf("expected 'libValue=resolved-lib-ts', got %q", got)
	}
}

func TestConformanceChildForkCredentialIsolation(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// child-creds.mjs forks a child that probes env, argv, execArgv,
	// and require.cache for credential leakage, AND imports lib.ts.
	code, _ := runM(t, proj, "child-creds.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)

	// Every credential-env probe must be "absent".
	for _, line := range strings.Split(got, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch {
		case strings.HasPrefix(key, "MEW_TRANSFORM_"):
			if val != "absent" {
				t.Errorf("child env leak: %s=%s", key, val)
			}
		case key == "argv-has-endpoint":
			if val != "no" {
				t.Errorf("child argv leaked endpoint: %s", val)
			}
		case key == "argv-has-long-token":
			if val != "no" {
				t.Errorf("child argv leaked token: %s", val)
			}
		case key == "execArgv-has-credentials":
			if val != "no" {
				t.Errorf("child execArgv leaked credentials: %s", val)
			}
		case key == "require-cache-endpoint" || key == "require-cache-token":
			if val != "absent" {
				t.Errorf("child require.cache leaked credentials: %s=%s", key, val)
			}
		case key == "libValue":
			if val != "resolved-lib-ts" {
				t.Errorf("child TypeScript import failed: %s", val)
			}
		}
	}
}

func TestConformanceChildNestedGrandchild(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// child-nested.mjs creates a child that creates a grandchild
	// which imports lib.ts. Verifies propagation across generations.
	code, _ := runM(t, proj, "child-nested.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "grandchild-libValue=resolved-lib-ts") {
		t.Fatalf("expected 'grandchild-libValue=resolved-lib-ts', got %q", got)
	}
}

func TestConformanceChildNonNodeUnaffected(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// Spawn a non-Node child and verify MEW_TRANSFORM_* credentials
	// are NOT injected into it.
	writeFile(t, proj, "non-node-child.mjs",
		`import { spawnSync } from "node:child_process";
import { writeFileSync } from "node:fs";
var r = spawnSync("env", [], { encoding: "utf8" });
writeFileSync("output.txt",
  "has_endpoint=" + (r.stdout.includes("MEW_TRANSFORM_ENDPOINT") ? "yes" : "no") +
  " has_token=" + (r.stdout.includes("MEW_TRANSFORM_TOKEN") ? "yes" : "no"));
`)
	code, _ := runM(t, proj, "non-node-child.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if strings.Contains(got, "has_endpoint=yes") || strings.Contains(got, "has_token=yes") {
		t.Fatalf("non-Node child received MEW_TRANSFORM_*: %s", got)
	}
}

func TestConformanceChildZeroAugNoInject(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// In --node mode (zero augmentation), child processes must NOT
	// receive Mew runtime injection.
	writeFile(t, proj, "zero-aug-child.mjs",
		`import { fork } from "node:child_process";
import { writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
var __d = dirname(fileURLToPath(import.meta.url));
writeFileSync(join(__d, "zero-aug-child-task.cjs"),
  "var results = [];" +
  "['MEW_TRANSFORM_ENDPOINT','MEW_TRANSFORM_TOKEN','MEW_TRANSFORM_OPTIONS']." +
  "forEach(function(v) { results.push(v + '=' + (process.env[v] || 'absent')); });" +
  "var fs = require('node:fs');" +
  "var p = require('node:path');" +
  "fs.writeFileSync(p.join(__dirname, 'output.txt'), results.join('\\n'));"
);
var c = fork(join(__d, "zero-aug-child-task.cjs"), [], {
  stdio: ['inherit', 'inherit', 'inherit', 'ipc'],
});
c.on('exit', function(code) {
  // output written by child task
});
`)
	// Run with --node to disable augmentation.
	code, _ := runM(t, proj, "--node", "zero-aug-child.mjs")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	for _, line := range strings.Split(got, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[1] != "absent" {
			t.Errorf("--node child leaked credential: %s", line)
		}
	}
}

// --- localStorage ---

func TestConformanceStorageBasicAPI(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	// The web-storage.cjs preload provides localStorage/sessionStorage
	proj := setupRuntimeFixture(t, "runtime-e2e")
	writeFile(t, proj, "storage-test.js",
		`const fs = require("node:fs");
try {
  localStorage.setItem("conformance-key", "conformance-value");
  const val = localStorage.getItem("conformance-key");
  localStorage.removeItem("conformance-key");
  const removed = localStorage.getItem("conformance-key");
  fs.writeFileSync("output.txt",
    "set=" + (val === "conformance-value" ? "ok" : "fail") +
    " removed=" + (removed === null ? "ok" : "fail"));
} catch(e) {
  fs.writeFileSync("output.txt", "error:" + e.message);
}`)
	code, _ := runM(t, proj, "storage-test.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "set=ok") || !strings.Contains(got, "removed=ok") {
		t.Fatalf("localStorage API failure: %s", got)
	}
}

func TestConformanceStorageSession(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	writeFile(t, proj, "session-test.js",
		`const fs = require("node:fs");
try {
  sessionStorage.setItem("sess", "active");
  fs.writeFileSync("output.txt", "session-ok:" + sessionStorage.getItem("sess"));
} catch(e) {
  fs.writeFileSync("output.txt", "session-error:" + e.message);
}`)
	code, _ := runM(t, proj, "session-test.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	got := readOutput(t, proj)
	if !strings.Contains(got, "session-ok:active") {
		t.Fatalf("sessionStorage failure: %s", got)
	}
}

// --- Exit code propagation ---

func TestConformanceExitCode(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// exit-code.js calls process.exit(42)
	code, _ := runM(t, proj, "exit-code.js")
	if code != 42 {
		t.Fatalf("expected exit=42, got exit=%d", code)
	}
}
