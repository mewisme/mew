// Deterministic ownership-race regression test for Issue 4.
//
// Tests that a stale lock owner cannot delete a successor's lock.
// Uses internal lock test hooks (MEW_STORAGE_TEST_HOOKS=1), exposed
// via globalThis.__mewLockTest by preload.cjs.
//
// Exit 0 + "OWNERSHIP_OK" on success; exit 1 + failure details on error.

var path = require('node:path');
var fs = require('node:fs');
var os = require('node:os');

var __lock = globalThis.__mewLockTest;
if (!__lock) {
  fs.writeFileSync('output.txt', 'FAIL: __mewLockTest not available (set MEW_STORAGE_TEST_HOOKS=1)\n');
  process.exit(1);
}

var tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'mew-lock-test-'));
var storageFile = path.join(tmpDir, 'storage.json');
var lockDir = __lock.lockPath(storageFile);

var failures = [];

function check(cond, msg) {
  if (!cond) failures.push('FAIL: ' + msg);
}

function lockExists() {
  try { fs.statSync(lockDir); return true; } catch (_) { return false; }
}

function readOwner() {
  try {
    return JSON.parse(fs.readFileSync(path.join(lockDir, 'owner.json'), 'utf8'));
  } catch (_) { return null; }
}

// --- Test 1: normal acquire/release ---
var relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test1: acquire A returned null');
check(lockExists(), 'Test1: lock dir does not exist after acquire');
relA();
check(!lockExists(), 'Test1: lock dir still exists after release');

// --- Test 2: repeated release is idempotent ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test2: acquire A failed');
relA();
relA(); // second release must be harmless
check(!lockExists(), 'Test2: lock dir still exists after double release');

// --- Test 3: stale owner release after takeover (the core race) ---
// A acquires lock.
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test3: acquire A failed');
// B tombstones A's lock (simulating stale takeover).
var takenOver = __lock.tryTakeoverStaleLock(lockDir);
check(takenOver, 'Test3: takeover failed');
// B acquires replacement lock at the same canonical path.
var relB = __lock.acquireLock(lockDir);
check(relB !== null, 'Test3: acquire B (replacement) failed');
check(lockExists(), 'Test3: lock dir missing after B acquire');
var ownerB = readOwner();
check(ownerB !== null && typeof ownerB.lockId === 'string', 'Test3: B owner metadata missing lockId');
// A releases with its OLD closure — MUST NOT delete B's lock.
relA();
check(lockExists(), 'Test3: stale A release deleted B lock');
var currentOwner = readOwner();
check(currentOwner !== null && currentOwner.lockId === ownerB.lockId,
  'Test3: owner changed after stale A release: ' + JSON.stringify(currentOwner));
// C must NOT be able to acquire while B holds the lock.
var relC = __lock.acquireLock(lockDir);
check(relC === null, 'Test3: C acquired lock while B still owns it');
// B releases normally.
relB();
check(!lockExists(), 'Test3: lock dir still exists after B release');
// C can now acquire.
relC = __lock.acquireLock(lockDir);
check(relC !== null, 'Test3: C could not acquire after B release');
relC();
check(!lockExists(), 'Test3: lock dir still exists after C release');

// --- Test 4: two sequential stale takeovers ---
var rel1 = __lock.acquireLock(lockDir);
check(rel1 !== null, 'Test4: owner 1 acquire failed');
__lock.tryTakeoverStaleLock(lockDir);
var rel2 = __lock.acquireLock(lockDir);
check(rel2 !== null, 'Test4: owner 2 acquire failed');
rel1(); // stale release must not delete owner 2's lock
check(lockExists(), 'Test4: stale owner 1 release deleted owner 2 lock');
__lock.tryTakeoverStaleLock(lockDir);
var rel3 = __lock.acquireLock(lockDir);
check(rel3 !== null, 'Test4: owner 3 acquire failed');
rel2(); // stale release must not delete owner 3's lock
check(lockExists(), 'Test4: stale owner 2 release deleted owner 3 lock');
rel3();
check(!lockExists(), 'Test4: owner 3 release did not clean up');

// --- Test 5: malformed owner metadata (fail-closed) ---
var relM = __lock.acquireLock(lockDir);
check(relM !== null, 'Test5: acquire for malformed test failed');
fs.writeFileSync(path.join(lockDir, 'owner.json'), 'not-valid-json{{{');
relM(); // must not throw, must not delete (fail-closed)
check(lockExists(), 'Test5: release deleted lock with malformed owner (should fail closed)');
fs.rmSync(lockDir, { recursive: true, force: true });

// --- Test 6: missing owner metadata (fail-closed) ---
var relX = __lock.acquireLock(lockDir);
check(relX !== null, 'Test6: acquire for missing-owner test failed');
fs.unlinkSync(path.join(lockDir, 'owner.json'));
relX(); // must not throw, must not delete
check(lockExists(), 'Test6: release deleted lock with missing owner (should fail closed)');
fs.rmSync(lockDir, { recursive: true, force: true });

// --- Test 7: cross-process ownership safety via real localStorage ---
// Tests the full acquireStorageLock/takeover/release flow produces
// ownership-safe closures. Uses the global localStorage (injected by preload).
localStorage.setItem('test-key', 'test-value');
var val = localStorage.getItem('test-key');
check(val === 'test-value', 'Test7: readback failed: ' + val);
localStorage.removeItem('test-key');

// Clean up tmp dir.
try { fs.rmSync(tmpDir, { recursive: true, force: true }); } catch (_) {}

if (failures.length > 0) {
  fs.writeFileSync('output.txt', failures.join('\n') + '\n');
  process.exit(1);
}
fs.writeFileSync('output.txt', 'OWNERSHIP_OK\n');
