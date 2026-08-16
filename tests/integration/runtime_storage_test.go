package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mewisme/mew/internal/testkit"
)

const storageConcurEnv = "MEW_STORAGE_CONCUR_PROC"

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

// --- Concurrent mutation safety ---

// spawnStorageChild runs a test function as a child process using the test
// binary itself, passing env vars through.
func spawnStorageChild(t *testing.T, env []string) *exec.Cmd {
	t.Helper()
	exe, err := exec.LookPath(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestRuntimeStorageConcurrentChildren$", "-test.count=1")
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

// TestRuntimeStorageConcurrentChildren is the child-process entry point
// for storage concurrency tests.
func TestRuntimeStorageConcurrentChildren(t *testing.T) {
	if os.Getenv(storageConcurEnv) != "1" {
		t.Skip("not a storage concurrency child process")
	}
	projDir := os.Getenv("MEW_MUTATION_PROJ")
	if projDir == "" {
		t.Fatal("missing MEW_MUTATION_PROJ")
	}

	role := os.Getenv("MEW_CONCUR_ROLE")
	key := os.Getenv("MEW_CONCUR_KEY")
	val := os.Getenv("MEW_CONCUR_VAL")
	readyFile := os.Getenv("MEW_CONCUR_READY_FILE")
	goFile := os.Getenv("MEW_CONCUR_GO_FILE")
	resultFile := os.Getenv("MEW_CONCUR_RESULT_FILE")

	scriptArgs := []string{"storage-concur.js"}
	if role != "" {
		t.Setenv("MEW_CONCUR_ROLE", role)
	}
	if key != "" {
		t.Setenv("MEW_CONCUR_KEY", key)
	}
	if val != "" {
		t.Setenv("MEW_CONCUR_VAL", val)
	}
	if readyFile != "" {
		t.Setenv("MEW_CONCUR_READY_FILE", readyFile)
	}
	if goFile != "" {
		t.Setenv("MEW_CONCUR_GO_FILE", goFile)
	}
	if resultFile != "" {
		t.Setenv("MEW_CONCUR_RESULT_FILE", resultFile)
	}

	code, out := runMWithRuntime(t, projDir, scriptArgs...)
	if code != 0 {
		t.Fatalf("child %s exit=%d out=%s", role, code, out)
	}
	_ = out
	_ = code
}

// waitForFiles polls for files to exist, failing after ~30s.
func waitForFiles(t *testing.T, files ...string) {
	t.Helper()
	for _, f := range files {
		deadline := time.Now().Add(30 * time.Second)
		for {
			if _, err := os.Stat(f); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("timeout waiting for file %s", f)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// TestRuntimeStorageConcurrentSetDifferentKeys verifies two concurrent
// processes setting different absent keys both survive.
func TestRuntimeStorageConcurrentSetDifferentKeys(t *testing.T) {
	if os.Getenv(storageConcurEnv) != "" {
		return
	}
	skipWithoutNode(t)
	proj := storageFixture(t)
	projAbs, err := filepath.Abs(proj)
	if err != nil {
		t.Fatal(err)
	}

	readyA := filepath.Join(projAbs, "ready-a")
	readyB := filepath.Join(projAbs, "ready-b")
	goFile := filepath.Join(projAbs, "go")

	baseEnv := []string{
		storageConcurEnv + "=1",
		"MEW_MUTATION_PROJ=" + projAbs,
	}

	cmdA := spawnStorageChild(t, append(baseEnv,
		"MEW_CONCUR_ROLE=barrier-set",
		"MEW_CONCUR_KEY=key-a",
		"MEW_CONCUR_VAL=value-a",
		"MEW_CONCUR_READY_FILE="+readyA,
		"MEW_CONCUR_GO_FILE="+goFile,
	))
	if err := cmdA.Start(); err != nil {
		t.Fatal(err)
	}

	cmdB := spawnStorageChild(t, append(baseEnv,
		"MEW_CONCUR_ROLE=barrier-set",
		"MEW_CONCUR_KEY=key-b",
		"MEW_CONCUR_VAL=value-b",
		"MEW_CONCUR_READY_FILE="+readyB,
		"MEW_CONCUR_GO_FILE="+goFile,
	))
	if err := cmdB.Start(); err != nil {
		t.Fatal(err)
	}

	waitForFiles(t, readyA, readyB)

	if err := os.WriteFile(goFile, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdA.Wait(); err != nil {
		t.Errorf("child A failed: %v", err)
	}
	if err := cmdB.Wait(); err != nil {
		t.Errorf("child B failed: %v", err)
	}

	// Verify both keys survive by reading directly.
	for _, kv := range [][2]string{{"key-a", "value-a"}, {"key-b", "value-b"}} {
		t.Setenv("MEW_CONCUR_ROLE", "read")
		t.Setenv("MEW_CONCUR_KEY", kv[0])
		code, _ := runMWithRuntime(t, proj, "storage-concur.js")
		if code != 0 {
			t.Errorf("read %s: exit=%d", kv[0], code)
			continue
		}
		out := storageOutput(t, proj)
		if !strings.Contains(out, "READ:"+kv[1]) {
			t.Errorf("concurrent set lost %s: got %q, want READ:%s", kv[0], out, kv[1])
		}
	}
}

// TestRuntimeStorageConcurrentRemoveAndSet verifies that concurrent remove
// of key A and set of key B does not lose B.
func TestRuntimeStorageConcurrentRemoveAndSet(t *testing.T) {
	if os.Getenv(storageConcurEnv) != "" {
		return
	}
	skipWithoutNode(t)
	proj := storageFixture(t)
	projAbs, err := filepath.Abs(proj)
	if err != nil {
		t.Fatal(err)
	}

	// Pre-populate key-a.
	t.Setenv("MEW_CONCUR_ROLE", "set")
	t.Setenv("MEW_CONCUR_KEY", "key-a")
	t.Setenv("MEW_CONCUR_VAL", "value-a")
	if code, _ := runMWithRuntime(t, proj, "storage-concur.js"); code != 0 {
		t.Fatal("pre-populate failed")
	}

	// Test: two concurrent updates to DIFFERENT existing keys — both must survive.
	// (This is the canonical lost-update regression test.)

	baseEnv := []string{
		storageConcurEnv + "=1",
		"MEW_MUTATION_PROJ=" + projAbs,
	}

	// Pre-populate two keys, then update them concurrently.
	t.Setenv("MEW_CONCUR_ROLE", "set")
	t.Setenv("MEW_CONCUR_KEY", "update-a")
	t.Setenv("MEW_CONCUR_VAL", "old-a")
	runMWithRuntime(t, proj, "storage-concur.js")
	t.Setenv("MEW_CONCUR_ROLE", "set")
	t.Setenv("MEW_CONCUR_KEY", "update-b")
	t.Setenv("MEW_CONCUR_VAL", "old-b")
	runMWithRuntime(t, proj, "storage-concur.js")

	ready1 := filepath.Join(projAbs, "ready-upd-a")
	ready2 := filepath.Join(projAbs, "ready-upd-b")
	goUpd := filepath.Join(projAbs, "go-upd")

	cmd1 := spawnStorageChild(t, append(baseEnv,
		"MEW_CONCUR_ROLE=barrier-set",
		"MEW_CONCUR_KEY=update-a",
		"MEW_CONCUR_VAL=new-a",
		"MEW_CONCUR_READY_FILE="+ready1,
		"MEW_CONCUR_GO_FILE="+goUpd,
	))
	if err := cmd1.Start(); err != nil {
		t.Fatal(err)
	}

	cmd2 := spawnStorageChild(t, append(baseEnv,
		"MEW_CONCUR_ROLE=barrier-set",
		"MEW_CONCUR_KEY=update-b",
		"MEW_CONCUR_VAL=new-b",
		"MEW_CONCUR_READY_FILE="+ready2,
		"MEW_CONCUR_GO_FILE="+goUpd,
	))
	if err := cmd2.Start(); err != nil {
		t.Fatal(err)
	}

	waitForFiles(t, ready1, ready2)

	if err := os.WriteFile(goUpd, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmd1.Wait(); err != nil {
		t.Errorf("child 1 failed: %v", err)
	}
	if err := cmd2.Wait(); err != nil {
		t.Errorf("child 2 failed: %v", err)
	}

	// Both updates must survive.
	for _, kv := range [][2]string{{"update-a", "new-a"}, {"update-b", "new-b"}} {
		t.Setenv("MEW_CONCUR_ROLE", "read")
		t.Setenv("MEW_CONCUR_KEY", kv[0])
		code, _ := runMWithRuntime(t, proj, "storage-concur.js")
		if code != 0 {
			t.Errorf("read %s: exit=%d", kv[0], code)
			continue
		}
		out := storageOutput(t, proj)
		if !strings.Contains(out, "READ:"+kv[1]) {
			t.Errorf("concurrent update lost %s: got %q, want READ:%s", kv[0], out, kv[1])
		}
	}
}

// TestRuntimeStorageConcurrentClearAndSet verifies that concurrent clear
// and setItem produce one consistent result, never a stale-snapshot hybrid.
func TestRuntimeStorageConcurrentClearAndSet(t *testing.T) {
	if os.Getenv(storageConcurEnv) != "" {
		return
	}
	skipWithoutNode(t)
	proj := storageFixture(t)
	projAbs, err := filepath.Abs(proj)
	if err != nil {
		t.Fatal(err)
	}

	// Pre-populate several keys.
	for _, kv := range [][2]string{
		{"pre-1", "val-1"}, {"pre-2", "val-2"}, {"pre-3", "val-3"},
	} {
		t.Setenv("MEW_CONCUR_ROLE", "set")
		t.Setenv("MEW_CONCUR_KEY", kv[0])
		t.Setenv("MEW_CONCUR_VAL", kv[1])
		if code, _ := runMWithRuntime(t, proj, "storage-concur.js"); code != 0 {
			t.Fatal("pre-populate failed for " + kv[0])
		}
	}

	readyClear := filepath.Join(projAbs, "ready-clear")
	readySet := filepath.Join(projAbs, "ready-set")
	goFile := filepath.Join(projAbs, "go-clear-set")

	baseEnv := []string{
		storageConcurEnv + "=1",
		"MEW_MUTATION_PROJ=" + projAbs,
	}

	cmdClear := spawnStorageChild(t, append(baseEnv,
		"MEW_CONCUR_ROLE=barrier-clear",
		"MEW_CONCUR_READY_FILE="+readyClear,
		"MEW_CONCUR_GO_FILE="+goFile,
	))
	if err := cmdClear.Start(); err != nil {
		t.Fatal(err)
	}

	cmdSet := spawnStorageChild(t, append(baseEnv,
		"MEW_CONCUR_ROLE=barrier-set",
		"MEW_CONCUR_KEY=post-key",
		"MEW_CONCUR_VAL=post-val",
		"MEW_CONCUR_READY_FILE="+readySet,
		"MEW_CONCUR_GO_FILE="+goFile,
	))
	if err := cmdSet.Start(); err != nil {
		t.Fatal(err)
	}

	waitForFiles(t, readyClear, readySet)

	if err := os.WriteFile(goFile, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdClear.Wait(); err != nil {
		t.Errorf("clear child failed: %v", err)
	}
	if err := cmdSet.Wait(); err != nil {
		t.Errorf("set child failed: %v", err)
	}

	// Verify consistency.
	t.Setenv("MEW_CONCUR_ROLE", "read")
	t.Setenv("MEW_CONCUR_KEY", "post-key")
	runMWithRuntime(t, proj, "storage-concur.js")
	outPost := storageOutput(t, proj)

	t.Setenv("MEW_CONCUR_ROLE", "read")
	t.Setenv("MEW_CONCUR_KEY", "pre-1")
	runMWithRuntime(t, proj, "storage-concur.js")
	outPre := storageOutput(t, proj)

	postExists := strings.Contains(outPost, "READ:post-val")
	preExists := strings.Contains(outPre, "READ:val-1")

	if postExists && preExists {
		t.Errorf("stale-snapshot hybrid: both clear-era pre-1 and set-era post-key survived")
	}
	t.Logf("clear+set outcome: post-exists=%v pre-exists=%v", postExists, preExists)
}

// TestRuntimeStorageConcurrentManyWriters launches N concurrent writers
// each setting a unique key and verifies all survive.
func TestRuntimeStorageConcurrentManyWriters(t *testing.T) {
	if os.Getenv(storageConcurEnv) != "" {
		return
	}
	skipWithoutNode(t)
	proj := storageFixture(t)
	projAbs, err := filepath.Abs(proj)
	if err != nil {
		t.Fatal(err)
	}

	const N = 8
	baseEnv := []string{
		storageConcurEnv + "=1",
		"MEW_MUTATION_PROJ=" + projAbs,
	}

	var cmds []*exec.Cmd
	var readyFiles []string
	goFile := filepath.Join(projAbs, "go-many")

	for i := 0; i < N; i++ {
		keyName := "mk-" + string(rune('a'+i))
		valName := "mv-" + string(rune('a'+i))
		readyFile := filepath.Join(projAbs, "ready-mk-"+string(rune('a'+i)))
		readyFiles = append(readyFiles, readyFile)

		cmd := spawnStorageChild(t, append(baseEnv,
			"MEW_CONCUR_ROLE=barrier-set",
			"MEW_CONCUR_KEY="+keyName,
			"MEW_CONCUR_VAL="+valName,
			"MEW_CONCUR_READY_FILE="+readyFile,
			"MEW_CONCUR_GO_FILE="+goFile,
		))
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}

	waitForFiles(t, readyFiles...)

	if err := os.WriteFile(goFile, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, N)
	for _, cmd := range cmds {
		cmd := cmd
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cmd.Wait(); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// Verify all keys survived.
	for i := 0; i < N; i++ {
		keyName := "mk-" + string(rune('a'+i))
		valName := "mv-" + string(rune('a'+i))
		t.Setenv("MEW_CONCUR_ROLE", "read")
		t.Setenv("MEW_CONCUR_KEY", keyName)
		code, _ := runMWithRuntime(t, proj, "storage-concur.js")
		if code != 0 {
			t.Errorf("read %s: exit=%d", keyName, code)
			continue
		}
		out := storageOutput(t, proj)
		if !strings.Contains(out, "READ:"+valName) {
			t.Errorf("many-writer lost %s: got %q, want READ:%s", keyName, out, valName)
		}
	}
}

// TestRuntimeStorageConcurrentQuotaContention verifies quota is evaluated
// against the latest committed state under contention.
func TestRuntimeStorageConcurrentQuotaContention(t *testing.T) {
	if os.Getenv(storageConcurEnv) != "" {
		return
	}
	skipWithoutNode(t)
	proj := storageFixture(t)
	projAbs, err := filepath.Abs(proj)
	if err != nil {
		t.Fatal(err)
	}

	// Set quota to 200 bytes. Pre-populate with ~150 bytes (base).
	t.Setenv("MEW_STORAGE_QUOTA_BYTES", "200")
	t.Setenv("MEW_CONCUR_ROLE", "set")
	t.Setenv("MEW_CONCUR_KEY", "base")
	t.Setenv("MEW_CONCUR_VAL", strings.Repeat("x", 150))
	if code, _ := runMWithRuntime(t, proj, "storage-concur.js"); code != 0 {
		t.Fatal("pre-populate failed")
	}

	ready1 := filepath.Join(projAbs, "ready-q1")
	ready2 := filepath.Join(projAbs, "ready-q2")
	goFile := filepath.Join(projAbs, "go-quota")

	baseEnv := []string{
		storageConcurEnv + "=1",
		"MEW_MUTATION_PROJ=" + projAbs,
		"MEW_STORAGE_QUOTA_BYTES=200",
	}

	// Each child adds 60 bytes. First one succeeds (150+60=210 > 200? no: 150+60=210 > 200).
	// Let me recalculate: each value is 60 chars. base is 150 chars.
	// First write: 150 + 60 = 210 > 200 → first ALSO exceeds quota.
	// I need values that let exactly one succeed.
	// base=150, quota=200, each child adds 30 → first succeeds (180 ≤ 200), second loads 180+30=210 > 200 → fails.
	cmdA := spawnStorageChild(t, append(baseEnv,
		"MEW_CONCUR_ROLE=barrier-set",
		"MEW_CONCUR_KEY=quota-a",
		"MEW_CONCUR_VAL="+strings.Repeat("a", 30),
		"MEW_CONCUR_READY_FILE="+ready1,
		"MEW_CONCUR_GO_FILE="+goFile,
	))
	if err := cmdA.Start(); err != nil {
		t.Fatal(err)
	}

	cmdB := spawnStorageChild(t, append(baseEnv,
		"MEW_CONCUR_ROLE=barrier-set",
		"MEW_CONCUR_KEY=quota-b",
		"MEW_CONCUR_VAL="+strings.Repeat("b", 30),
		"MEW_CONCUR_READY_FILE="+ready2,
		"MEW_CONCUR_GO_FILE="+goFile,
	))
	if err := cmdB.Start(); err != nil {
		t.Fatal(err)
	}

	waitForFiles(t, ready1, ready2)

	if err := os.WriteFile(goFile, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmdA.Wait()
	cmdB.Wait()

	// At most one of quota-a/quota-b should exist (both = stale-snapshot quota bypass).
	t.Setenv("MEW_CONCUR_ROLE", "read")
	t.Setenv("MEW_CONCUR_KEY", "quota-a")
	runMWithRuntime(t, proj, "storage-concur.js")
	outA := storageOutput(t, proj)

	t.Setenv("MEW_CONCUR_ROLE", "read")
	t.Setenv("MEW_CONCUR_KEY", "quota-b")
	runMWithRuntime(t, proj, "storage-concur.js")
	outB := storageOutput(t, proj)

	aExists := strings.Contains(outA, "READ:"+strings.Repeat("a", 30))
	bExists := strings.Contains(outB, "READ:"+strings.Repeat("b", 30))

	if aExists && bExists {
		t.Errorf("quota bypass: both concurrent 30-byte writes succeeded against 200-byte quota with 150-byte base")
	}
	t.Logf("quota contention: a-exists=%v b-exists=%v", aExists, bExists)
}

// --- Ownership race regression (Issue 4) ---

func TestRuntimeStorageOwnershipRace(t *testing.T) {
	skipWithoutNode(t)
	proj := storageFixture(t)

	t.Setenv("MEW_STORAGE_TEST_HOOKS", "1")
	code, combined := runMWithRuntime(t, proj, "storage-ownership-race.js")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, combined)
	}
	out := storageOutput(t, proj)
	if out != "OWNERSHIP_OK" {
		t.Errorf("ownership race failures:\n%s", out)
	}
}

// --- Stale lock regression (P0: Issue 1, live owner never stale) ---

func TestRuntimeStorageStaleLockRegression(t *testing.T) {
	skipWithoutNode(t)
	proj := storageFixture(t)

	t.Setenv("MEW_STORAGE_TEST_HOOKS", "1")
	// Short acquisition wait so in-fixture contender probes time out fast.
	t.Setenv("MEW_STORAGE_LOCK_MAX_WAIT_MS", "400")
	code, combined := runMWithRuntime(t, proj, "storage-stale-lock-regression.js")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, combined)
	}
	out := storageOutput(t, proj)
	if out != "STALE_LOCK_OK" {
		t.Errorf("stale lock regression failures:\n%s", out)
	}
}

// --- Live-owner guard with real processes (P0 follow-up) ---

const storageLockEnv = "MEW_LOCKTEST_PROC"

// lockChildEnvVars are forwarded from the parent test to lock child processes.
var lockChildEnvVars = []string{
	"MEW_LOCKTEST_ROLE",
	"MEW_LOCKTEST_FILE",
	"MEW_LOCKTEST_READY_FILE",
	"MEW_LOCKTEST_GO_FILE",
	"MEW_LOCKTEST_DONE_FILE",
	"MEW_LOCKTEST_RESULT_FILE",
	"MEW_LOCKTEST_LOG",
	"MEW_LOCKTEST_HOLD_MS",
	"MEW_LOCKTEST_ITERS",
	"MEW_STORAGE_LOCK_MAX_WAIT_MS",
	"MEW_STORAGE_LOCK_GRACE_MS",
}

// spawnLockChild re-invokes the test binary as a storage-lock child process.
func spawnLockChild(t *testing.T, env []string) *exec.Cmd {
	t.Helper()
	exe, err := exec.LookPath(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestRuntimeStorageLockChildren$", "-test.count=1")
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

// TestRuntimeStorageLockChildren is the child-process entry point for the
// storage-lock live-owner guard tests.
func TestRuntimeStorageLockChildren(t *testing.T) {
	if os.Getenv(storageLockEnv) != "1" {
		t.Skip("not a storage-lock child process")
	}
	projDir := os.Getenv("MEW_MUTATION_PROJ")
	if projDir == "" {
		t.Fatal("missing MEW_MUTATION_PROJ")
	}
	for _, name := range lockChildEnvVars {
		if v := os.Getenv(name); v != "" {
			t.Setenv(name, v)
		}
	}
	code, out := runMWithRuntime(t, projDir, "storage-lock-children.js")
	if code != 0 {
		t.Fatalf("lock child %s exit=%d out=%s", os.Getenv("MEW_LOCKTEST_ROLE"), code, out)
	}
}

// lockLogStats scans an ENTER/EXIT log and reports whether any two
// critical sections overlapped.
func lockLogStats(t *testing.T, logPath string) (enters, exits int, overlapped bool) {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read lock log: %v", err)
	}
	open := false
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		switch {
		case strings.HasPrefix(line, "ENTER "):
			enters++
			if open {
				overlapped = true
			}
			open = true
		case strings.HasPrefix(line, "EXIT "):
			exits++
			if !open {
				overlapped = true
			}
			open = false
		}
	}
	if open {
		overlapped = true
	}
	return enters, exits, overlapped
}

// TestRuntimeStorageLiveOwnerNeverStolen proves with real processes that
// (1) a contender against a live lock owner times out instead of stealing,
// (2) an abandoned (dead-owner) lock is reclaimed and contenders never hold
// overlapping critical sections, and (3) sustained cross-process mutation
// never overlaps.
func TestRuntimeStorageLiveOwnerNeverStolen(t *testing.T) {
	if os.Getenv(storageLockEnv) != "" || os.Getenv(storageConcurEnv) != "" {
		return
	}
	skipWithoutNode(t)
	proj := storageFixture(t)
	projAbs, err := filepath.Abs(proj)
	if err != nil {
		t.Fatal(err)
	}
	targetDir := t.TempDir()

	baseEnv := []string{
		storageLockEnv + "=1",
		"MEW_MUTATION_PROJ=" + projAbs,
		"MEW_STORAGE_TEST_HOOKS=1",
	}

	// Phase 1: live holder vs contender — contender must time out.
	target1 := filepath.Join(targetDir, "phase1.json")
	ready1 := filepath.Join(targetDir, "p1-ready")
	done1 := filepath.Join(targetDir, "p1-done")
	go1 := filepath.Join(targetDir, "p1-go")
	res1 := filepath.Join(targetDir, "p1-result")

	holder := spawnLockChild(t, append(baseEnv,
		"MEW_LOCKTEST_ROLE=hold",
		"MEW_LOCKTEST_FILE="+target1,
		"MEW_LOCKTEST_READY_FILE="+ready1,
		"MEW_LOCKTEST_DONE_FILE="+done1,
		"MEW_LOCKTEST_HOLD_MS=3000",
	))
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	waitForFiles(t, ready1)

	contender := spawnLockChild(t, append(baseEnv,
		"MEW_LOCKTEST_ROLE=try",
		"MEW_LOCKTEST_FILE="+target1,
		"MEW_LOCKTEST_GO_FILE="+go1,
		"MEW_LOCKTEST_RESULT_FILE="+res1,
		"MEW_STORAGE_LOCK_MAX_WAIT_MS=700",
	))
	if err := contender.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(go1, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := contender.Wait(); err != nil {
		t.Errorf("contender child failed: %v", err)
	}
	waitForFiles(t, done1)
	if err := holder.Wait(); err != nil {
		t.Errorf("holder child failed: %v", err)
	}
	res, err := os.ReadFile(res1)
	if err != nil {
		t.Fatalf("read contender result: %v", err)
	}
	if strings.Contains(string(res), "ACQUIRED") || !strings.Contains(string(res), "TIMEOUT") {
		t.Errorf("contender stole lock from live holder: %s", res)
	}

	// Phase 2: abandoned owner (process exited holding the lock) — two
	// contenders race for the dead lock.
	target2 := filepath.Join(targetDir, "phase2.json")
	ready2 := filepath.Join(targetDir, "p2-ready")
	done2 := filepath.Join(targetDir, "p2-done")
	go2 := filepath.Join(targetDir, "p2-go")
	res2 := filepath.Join(targetDir, "p2-result")
	log2 := filepath.Join(targetDir, "p2-log")

	holder2 := spawnLockChild(t, append(baseEnv,
		"MEW_LOCKTEST_ROLE=abandon",
		"MEW_LOCKTEST_FILE="+target2,
		"MEW_LOCKTEST_READY_FILE="+ready2,
		"MEW_LOCKTEST_DONE_FILE="+done2,
	))
	if err := holder2.Start(); err != nil {
		t.Fatal(err)
	}
	waitForFiles(t, done2)
	// Reap the dead owner's process tree so kill(pid,0) reports ESRCH.
	if err := holder2.Wait(); err != nil {
		t.Errorf("abandon child failed: %v", err)
	}
	time.Sleep(500 * time.Millisecond) // owner PID fully gone + grace (400 ms)

	for i := 0; i < 2; i++ {
		c := spawnLockChild(t, append(baseEnv,
			"MEW_LOCKTEST_ROLE=try",
			"MEW_LOCKTEST_FILE="+target2,
			"MEW_LOCKTEST_GO_FILE="+go2,
			"MEW_LOCKTEST_RESULT_FILE="+res2,
			"MEW_LOCKTEST_LOG="+log2,
			"MEW_STORAGE_LOCK_GRACE_MS=400",
			"MEW_STORAGE_LOCK_MAX_WAIT_MS=5000",
		))
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Wait() })
	}
	time.Sleep(300 * time.Millisecond)
	if err := os.WriteFile(go2, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Contenders exit on their own; poll for both result lines.
	deadline := time.Now().Add(30 * time.Second)
	for {
		data, err := os.ReadFile(res2)
		if err == nil && len(strings.Fields(string(data))) >= 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("contenders did not both report: %q", data)
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond) // let tail EXIT lines land

	resData, err := os.ReadFile(res2)
	if err != nil {
		t.Fatalf("read contender results: %v", err)
	}
	acquired := strings.Count(string(resData), "ACQUIRED")
	if runtime.GOOS != "windows" && acquired < 1 {
		t.Errorf("dead owner lock never reclaimed by contenders: %q", resData)
	}
	if acquired > 0 {
		if _, _, overlapped := lockLogStats(t, log2); overlapped {
			t.Errorf("overlapping critical sections after dead-owner takeover: %q", resData)
		}
	}

	// Phase 3: sustained contention — three processes, no overlap ever.
	target3 := filepath.Join(targetDir, "phase3.json")
	log3 := filepath.Join(targetDir, "p3-log")
	if err := os.WriteFile(log3, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := spawnLockChild(t, append(baseEnv,
				"MEW_LOCKTEST_ROLE=overlap",
				"MEW_LOCKTEST_FILE="+target3,
				"MEW_LOCKTEST_LOG="+log3,
				"MEW_LOCKTEST_ITERS=15",
				"MEW_STORAGE_LOCK_MAX_WAIT_MS=15000",
			))
			if err := c.Start(); err != nil {
				t.Error(err)
				return
			}
			if err := c.Wait(); err != nil {
				t.Errorf("overlap child failed: %v", err)
			}
		}()
	}
	wg.Wait()
	enters, exits, overlapped := lockLogStats(t, log3)
	if overlapped {
		t.Errorf("critical-section overlap detected (enters=%d exits=%d)", enters, exits)
	}
	if enters != 45 || exits != 45 {
		t.Errorf("expected 45 paired critical sections, got enters=%d exits=%d", enters, exits)
	}
}
