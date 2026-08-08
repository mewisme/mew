package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/testkit"
)

// storageFixture copies the runtime/storage fixture into a temp dir.
func storageFixture(t *testing.T) string {
	t.Helper()
	testkit.CleanEnv(t)
	projDir := t.TempDir()
	testkit.CopyFixture(t, "runtime/storage", projDir)
	return projDir
}

// storageOutput reads output.txt from the fixture dir.
func storageOutput(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatalf("read output.txt: %v", err)
	}
	return strings.TrimSpace(string(data))
}

// --- Basic Storage API ---

func TestRuntimeStorageBasicAPI(t *testing.T) {
	skipWithoutNode(t)
	proj := storageFixture(t)
	code, _ := runMWithRuntime(t, proj, "storage-basic.js")
	out := storageOutput(t, proj)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if out != "ALL_PASS" {
		t.Errorf("basic API failures:\n%s", out)
	}
}

// --- localStorage Persistence ---

func TestRuntimeStorageLocalPersist(t *testing.T) {
	skipWithoutNode(t)
	proj := storageFixture(t)

	// First invocation: write data.
	t.Setenv("MEW_STORAGE_TEST_MODE", "write")
	code, _ := runMWithRuntime(t, proj, "storage-persist.js")
	out := storageOutput(t, proj)
	if code != 0 {
		t.Fatalf("write exit %d: %s", code, out)
	}
	if out != "WRITE_OK" {
		t.Fatalf("write failed: %s", out)
	}

	// Second invocation in same project dir: read data back.
	t.Setenv("MEW_STORAGE_TEST_MODE", "read")
	code, _ = runMWithRuntime(t, proj, "storage-persist.js")
	out = storageOutput(t, proj)
	if code != 0 {
		t.Fatalf("read exit %d: %s", code, out)
	}
	if out != "READ_OK" {
		t.Errorf("persistence failure:\n%s", out)
	}
}

// --- localStorage Isolation Between Projects ---

func TestRuntimeStorageLocalIsolation(t *testing.T) {
	skipWithoutNode(t)
	projA := storageFixture(t)
	projB := storageFixture(t)

	// Write data in projA.
	code, _ := runMWithRuntime(t, projA, "storage-iso-write.js")
	if code != 0 {
		t.Fatal("projA write failed")
	}
	if storageOutput(t, projA) != "value-a" {
		t.Fatal("projA did not store value")
	}

	// projB: should NOT see projA's data.
	code, _ = runMWithRuntime(t, projB, "storage-iso-read.js")
	if code != 0 {
		t.Fatal("projB read failed")
	}
	outB := storageOutput(t, projB)
	if outB != "null" {
		t.Errorf("isolation breach: projB saw projA's localStorage data: %s", outB)
	}
}

// --- sessionStorage Non-Persistence ---

func TestRuntimeStorageSessionNonPersist(t *testing.T) {
	skipWithoutNode(t)
	proj := storageFixture(t)

	// First run: sessionStorage run-count should start at 0.
	code, _ := runMWithRuntime(t, proj, "storage-session.js")
	out1 := storageOutput(t, proj)
	if code != 0 || out1 != "SESSION_OK" {
		t.Fatalf("first run failed: code=%d out=%s", code, out1)
	}

	// Second run: sessionStorage should be fresh again.
	code, _ = runMWithRuntime(t, proj, "storage-session.js")
	out2 := storageOutput(t, proj)
	if code != 0 || out2 != "SESSION_OK" {
		t.Fatalf("second run failed (session persisted?): code=%d out=%s", code, out2)
	}
}

// --- Quota Enforcement ---

func TestRuntimeStorageQuota(t *testing.T) {
	skipWithoutNode(t)
	proj := storageFixture(t)

	t.Setenv("MEW_STORAGE_QUOTA_BYTES", "50")
	code, _ := runMWithRuntime(t, proj, "storage-quota.js")
	out := storageOutput(t, proj)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if out != "QUOTA_OK" {
		t.Errorf("quota test failures:\n%s", out)
	}
}

// --- CJS/ESM Installation Parity ---

func TestRuntimeStorageCJSESMParity(t *testing.T) {
	skipWithoutNode(t)
	proj := storageFixture(t)

	code, _ := runMWithRuntime(t, proj, "storage-parity.js")
	out := storageOutput(t, proj)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if out != "PARITY_OK" {
		t.Errorf("CJS/ESM storage parity failure:\n%s", out)
	}
}

// --- Corrupt Store Recovery ---

func TestRuntimeStorageCorruptRecovery(t *testing.T) {
	skipWithoutNode(t)
	proj := storageFixture(t)

	// First run creates the persisted file.
	code, _ := runMWithRuntime(t, proj, "storage-corrupt-setup.js")
	if code != 0 {
		t.Fatal("initial write failed")
	}

	// Locate the persisted JSON storage file under MEW_CACHE_DIR.
	cacheDir := os.Getenv("MEW_CACHE_DIR")
	if cacheDir == "" {
		t.Skip("MEW_CACHE_DIR not set")
	}
	var corruptPath string
	_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || corruptPath != "" {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			corruptPath = path
		}
		return nil
	})

	if corruptPath == "" {
		t.Skip("cannot locate persisted storage file for corruption test")
	}

	// Corrupt it.
	if err := os.WriteFile(corruptPath, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatalf("corrupt write: %v", err)
	}

	// Run again — should recover gracefully and still function.
	code, _ = runMWithRuntime(t, proj, "storage-basic.js")
	out := storageOutput(t, proj)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if out != "ALL_PASS" {
		t.Errorf("corrupt recovery failures:\n%s", out)
	}
}
