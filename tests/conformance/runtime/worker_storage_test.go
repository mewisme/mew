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

// --- Child process ---

func TestConformanceChildProcessNoCreds(t *testing.T) {
	skipWithoutNode(t)
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	proj := setupRuntimeFixture(t, "runtime-e2e")
	// child-check.js spawns a child and checks no credential leakage
	code, _ := runM(t, proj, "child-check.js")
	if code != 0 {
		t.Fatalf("exit=%d", code)
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
