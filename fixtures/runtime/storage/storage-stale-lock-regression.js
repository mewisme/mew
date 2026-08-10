// P0 regression test: live lock owner must never be stolen by age alone.
//
// Tests the lock staleness protocol when a live process holds the lock
// longer than the old STALE_LOCK_MAX_AGE threshold.  Verifies:
//   1. Live owner + recent heartbeat → NOT stale (regardless of age)
//   2. Dead owner → stale after grace period
//   3. Live owner + stale heartbeat → stale (PID reuse protection)
//   4. Legacy owner (no heartbeat) + alive PID → NOT stale (conservative)
//   5. ABA: takeover + old release does not delete successor lock
//   6. Malformed/missing owner → stale only after grace period
//
// Uses internal lock test hooks (MEW_STORAGE_TEST_HOOKS=1), exposed
// via globalThis.__mewLockTest by preload.cjs.
//
// Exit 0 + "STALE_LOCK_OK" on success; exit 1 + failure details on error.

var path = require('node:path');
var fs = require('node:fs');
var os = require('node:os');

var __lock = globalThis.__mewLockTest;
if (!__lock) {
  fs.writeFileSync('output.txt', 'FAIL: __mewLockTest not available (set MEW_STORAGE_TEST_HOOKS=1)\n');
  process.exit(1);
}

var tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'mew-stale-test-'));
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

// --- Test 1: live owner with recent heartbeat is NOT stale (the P0 fix) ---
// A acquires the lock.  We verify isLockStale returns false even though
// the lock is brand new, and would have been considered stale under the
// old age-only threshold if we advanced time.
var relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test1: acquire A failed');
var stat = fs.statSync(lockDir);
var staleResult = __lock.isLockStale(lockDir, stat.mtimeMs);
check(staleResult === false, 'Test1: live owner with fresh heartbeat reported stale');
relA();
check(!lockExists(), 'Test1: lock not released');

// --- Test 2: live owner with recent heartbeat is NOT stale regardless of dirMod age ---
// Simulate an old lock by passing an artificially old dirMod.
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test2: acquire A failed');
// Pass a dirMod that is 5 minutes old (well past the old 60s threshold).
var oldDirMod = Date.now() - (5 * 60 * 1000);
staleResult = __lock.isLockStale(lockDir, oldDirMod);
check(staleResult === false, 'Test2: live owner with fresh heartbeat reported stale despite old dirMod');
relA();

// --- Test 3: dead owner → stale after grace period ---
// Create a lock with a dead PID (one that cannot exist).
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test3: acquire A failed');
var owner = readOwner();
check(owner !== null, 'Test3: owner missing');
// Rewrite owner.json with a PID that is almost certainly dead (PID 1 on most
// systems is init, but we test with a very high PID that cannot exist).
var deadPid = 0x7FFFFFFF; // 2^31-1, virtually guaranteed to not exist
// Check if isProcessAlive says this PID is dead.
var deadCheck = __lock.isProcessAlive(deadPid);
// On most systems, this PID does not exist.  If it somehow does (container
// with high PID namespace), skip this sub-test.
if (!deadCheck) {
  // Rewrite owner with dead PID, keeping the same lockId so release works.
  var deadOwner = {
    lockId: owner.lockId,
    pid: deadPid,
    processStart: owner.processStart,
    heartbeat: owner.heartbeat,
  };
  fs.writeFileSync(path.join(lockDir, 'owner.json'), JSON.stringify(deadOwner), { mode: 0o644 });
  stat = fs.statSync(lockDir);
  // The lock was just created, so dirMod is recent (< LOCK_GRACE).
  // isLockStale should return false (within grace period).
  staleResult = __lock.isLockStale(lockDir, stat.mtimeMs);
  check(staleResult === false, 'Test3: dead owner stale before grace period');
  // However, isLockStale with an old enough dirMod should return true.
  var pastGrace = Date.now() - (__lock.LOCK_GRACE + 1000);
  staleResult = __lock.isLockStale(lockDir, pastGrace);
  check(staleResult === true, 'Test3: dead owner not stale after grace period');
}
relA();

// --- Test 4: stale heartbeat → stale lock (PID reuse guard) ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test4: acquire A failed');
owner = readOwner();
check(owner !== null, 'Test4: owner missing');
// Rewrite owner with a stale heartbeat (past HEARTBEAT_MAX_AGE).
var staleHb = {
  lockId: owner.lockId,
  pid: process.pid, // PID is alive
  processStart: owner.processStart,
  heartbeat: Date.now() - (__lock.HEARTBEAT_MAX_AGE + 1000),
};
fs.writeFileSync(path.join(lockDir, 'owner.json'), JSON.stringify(staleHb), { mode: 0o644 });
stat = fs.statSync(lockDir);
staleResult = __lock.isLockStale(lockDir, stat.mtimeMs);
check(staleResult === true, 'Test4: live PID with stale heartbeat not reported stale');
relA();

// --- Test 5: legacy owner (no heartbeat) + alive PID → NOT stale ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test5: acquire A failed');
owner = readOwner();
check(owner !== null, 'Test5: owner missing');
// Remove heartbeat field.
delete owner.heartbeat;
fs.writeFileSync(path.join(lockDir, 'owner.json'), JSON.stringify(owner), { mode: 0o644 });
stat = fs.statSync(lockDir);
staleResult = __lock.isLockStale(lockDir, stat.mtimeMs);
check(staleResult === false, 'Test5: legacy owner (alive PID, no heartbeat) reported stale');
// Even with an artificially old dirMod, it should not be stale.
var veryOldMod = Date.now() - (10 * 60 * 1000); // 10 min old
staleResult = __lock.isLockStale(lockDir, veryOldMod);
check(staleResult === false, 'Test5: legacy owner reported stale with old dirMod');
relA();

// --- Test 6: malformed owner → stale after grace period ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test6: acquire A failed');
// Write malformed JSON.
fs.writeFileSync(path.join(lockDir, 'owner.json'), 'not-valid{{{', { mode: 0o644 });
stat = fs.statSync(lockDir);
staleResult = __lock.isLockStale(lockDir, stat.mtimeMs);
check(staleResult === false, 'Test6: malformed owner stale before grace period');
staleResult = __lock.isLockStale(lockDir, Date.now() - (__lock.LOCK_GRACE + 1000));
check(staleResult === true, 'Test6: malformed owner not stale after grace period');
// Clean up — release would fail-closed (malformed), force remove.
fs.rmSync(lockDir, { recursive: true, force: true });

// --- Test 7: missing owner → stale after grace period ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test7: acquire A failed');
fs.unlinkSync(path.join(lockDir, 'owner.json'));
stat = fs.statSync(lockDir);
staleResult = __lock.isLockStale(lockDir, stat.mtimeMs);
check(staleResult === false, 'Test7: missing owner stale before grace period');
staleResult = __lock.isLockStale(lockDir, Date.now() - (__lock.LOCK_GRACE + 1000));
check(staleResult === true, 'Test7: missing owner not stale after grace period');
// Clean up.
fs.rmSync(lockDir, { recursive: true, force: true });

// --- Test 8: ABA safety — takeover + old release does not delete successor ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test8: acquire A failed');
var takenOver = __lock.tryTakeoverStaleLock(lockDir);
check(takenOver, 'Test8: takeover failed');
var relB = __lock.acquireLock(lockDir);
check(relB !== null, 'Test8: acquire B (replacement) failed');
check(lockExists(), 'Test8: lock missing after B acquire');
var ownerB = readOwner();
check(ownerB !== null && typeof ownerB.lockId === 'string', 'Test8: B owner missing lockId');
// A releases with its OLD closure — MUST NOT delete B's lock.
relA();
check(lockExists(), 'Test8: stale A release deleted B lock');
var currentOwner = readOwner();
check(currentOwner !== null && currentOwner.lockId === ownerB.lockId,
  'Test8: owner changed after stale A release');
relB();
check(!lockExists(), 'Test8: B release did not clean up');

// --- Test 9: heartbeat refresh keeps lock non-stale ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test9: acquire A failed');
// Manually age the heartbeat.
owner = readOwner();
check(owner !== null, 'Test9: owner missing');
var almostStale = {
  lockId: owner.lockId,
  pid: process.pid,
  processStart: owner.processStart,
  heartbeat: Date.now() - (__lock.HEARTBEAT_MAX_AGE - 5000),
};
fs.writeFileSync(path.join(lockDir, 'owner.json'), JSON.stringify(almostStale), { mode: 0o644 });
// Should still be non-stale (within HEARTBEAT_MAX_AGE).
stat = fs.statSync(lockDir);
staleResult = __lock.isLockStale(lockDir, stat.mtimeMs);
check(staleResult === false, 'Test9: near-stale heartbeat reported stale');
// Refresh heartbeat.
var refreshed = __lock.refreshHeartbeat(lockDir, owner.lockId);
check(refreshed === true, 'Test9: heartbeat refresh failed');
owner = readOwner();
check(owner !== null && owner.heartbeat > almostStale.heartbeat, 'Test9: heartbeat not updated');
// Should now be fresh again.
staleResult = __lock.isLockStale(lockDir, stat.mtimeMs);
check(staleResult === false, 'Test9: refreshed heartbeat reported stale');
relA();

// --- Test 10: refreshHeartbeat fails with wrong lockId ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test10: acquire A failed');
var wrongResult = __lock.refreshHeartbeat(lockDir, 'wrong-lock-id');
check(wrongResult === false, 'Test10: refreshHeartbeat succeeded with wrong lockId');
relA();

// --- Test 11: releaseLock fails closed with wrong lockId ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test11: acquire A failed');
// Try to release with wrong lockId — must leave lock intact.
__lock.releaseLock(lockDir, 'wrong-lock-id');
check(lockExists(), 'Test11: releaseLock with wrong lockId deleted lock');
// Try to release with missing owner.
fs.unlinkSync(path.join(lockDir, 'owner.json'));
__lock.releaseLock(lockDir, 'some-id');
check(lockExists(), 'Test11: releaseLock with missing owner deleted lock');
fs.rmSync(lockDir, { recursive: true, force: true });

// Clean up tmp dir.
try { fs.rmSync(tmpDir, { recursive: true, force: true }); } catch (_) {}

if (failures.length > 0) {
  fs.writeFileSync('output.txt', failures.join('\n') + '\n');
  process.exit(1);
}
fs.writeFileSync('output.txt', 'STALE_LOCK_OK\n');
