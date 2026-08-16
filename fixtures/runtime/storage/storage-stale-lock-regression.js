// P0 regression test: a lock whose owner PID is alive (or conservatively
// assumed alive) must NEVER be classified stale, regardless of lock age,
// heartbeat age, or any other elapsed-time threshold.
//
// Stale policy under test:
//   1. Live owner PID                    → NOT stale, any age
//   2. Dead owner PID (ESRCH)            → stale after LOCK_GRACE
//   3. Missing / malformed owner         → stale after LOCK_GRACE
//   4. ABA: takeover + old release never deletes successor lock
//   5. Contender against live owner times out (acquireStorageLock)
//   6. Liveness classification (ESRCH vs EPERM-conservative)
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
var isWin = process.platform === 'win32';

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

function writeOwner(owner) {
  fs.writeFileSync(path.join(lockDir, 'owner.json'), JSON.stringify(owner), { mode: 0o644 });
}

// Ages spanning every threshold this protocol has ever used (old 60 s
// heartbeat lease, 10 min, 1 h) — none may make a live owner stale.
var AGES = [60 * 1000 + 1000, 10 * 60 * 1000, 60 * 60 * 1000];

// --- Test 1: live owner (self) is NOT stale, fresh or any age ---
AGES.forEach(function (age) {
  var rel = __lock.acquireLock(lockDir);
  check(rel !== null, 'Test1: acquire failed');
  var owner = readOwner();
  check(owner !== null && typeof owner.lockId === 'string', 'Test1: owner missing lockId');
  check(owner !== null && owner.pid === process.pid, 'Test1: owner missing pid');
  var stale = __lock.isLockStale(lockDir, Date.now() - age);
  check(stale === false, 'Test1: live owner reported stale at dirMod age ' + age + 'ms');
  rel();
});
check(!lockExists(), 'Test1: lock not released');

// --- Test 2: live owner + ancient heartbeat field → NOT stale ---
// Old lock-format owners carrying a heartbeat must not be treated as
// abandoned: heartbeat age is not evidence while the PID is alive.
var relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test2: acquire A failed');
var ownA = readOwner();
AGES.forEach(function (age) {
  writeOwner({
    lockId: ownA.lockId,
    pid: process.pid,
    heartbeat: Date.now() - age,
  });
  var stale = __lock.isLockStale(lockDir, Date.now() - age);
  check(stale === false, 'Test2: live PID with ' + age + 'ms-old heartbeat reported stale');
});
relA();

// --- Test 3: acquireStorageLock times out against a live owner ---
// Self holds the lock (this process is alive); a full acquisition
// attempt must fail with a timeout, never steal.  Test harness sets
// MEW_STORAGE_LOCK_MAX_WAIT_MS low.
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test3: acquire A failed');
var timedOut = false;
try {
  __lock.acquireStorageLock(storageFile);
} catch (e) {
  timedOut = /timeout/.test(String(e.message));
}
check(timedOut, 'Test3: acquireStorageLock stole or failed wrongly against live owner');
check(lockExists(), 'Test3: live owner lock vanished after contender timeout');
relA();

// --- Test 4: dead owner → stale only after grace ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test4: acquire A failed');
var deadPid = 0x7FFFFFFF; // virtually guaranteed nonexistent on POSIX
if (!isWin && !__lock.isProcessAlive(deadPid)) {
  writeOwner({ lockId: readOwner().lockId, pid: deadPid });
  var fresh = __lock.isLockStale(lockDir, Date.now());
  check(fresh === false, 'Test4: dead owner stale before grace');
  var past = __lock.isLockStale(lockDir, Date.now() - (__lock.LOCK_GRACE + 1000));
  check(past === true, 'Test4: dead owner not stale after grace');
}
relA();

// --- Test 5: owner without PID → grace, never immediate steal ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test5: acquire A failed');
writeOwner({ lockId: readOwner().lockId });
check(__lock.isLockStale(lockDir, Date.now()) === false, 'Test5: pid-less owner stale before grace');
check(
  __lock.isLockStale(lockDir, Date.now() - (__lock.LOCK_GRACE + 1000)) === true,
  'Test5: pid-less owner not stale after grace'
);
fs.rmSync(lockDir, { recursive: true, force: true });

// --- Test 6: malformed owner → grace ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test6: acquire A failed');
fs.writeFileSync(path.join(lockDir, 'owner.json'), 'not-valid{{{', { mode: 0o644 });
check(__lock.isLockStale(lockDir, Date.now()) === false, 'Test6: malformed owner stale before grace');
check(
  __lock.isLockStale(lockDir, Date.now() - (__lock.LOCK_GRACE + 1000)) === true,
  'Test6: malformed owner not stale after grace'
);
fs.rmSync(lockDir, { recursive: true, force: true });

// --- Test 7: missing owner file → grace ---
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test7: acquire A failed');
fs.unlinkSync(path.join(lockDir, 'owner.json'));
check(__lock.isLockStale(lockDir, Date.now()) === false, 'Test7: missing owner stale before grace');
check(
  __lock.isLockStale(lockDir, Date.now() - (__lock.LOCK_GRACE + 1000)) === true,
  'Test7: missing owner not stale after grace'
);
fs.rmSync(lockDir, { recursive: true, force: true });

// --- Test 8: ABA — takeover + old release + blocked contender ---
// A holds → lock tombstoned as stale → B acquires successor → A's stale
// release must not delete B's lock → contender C times out while B
// holds → after B releases, acquisition works again.
relA = __lock.acquireLock(lockDir);
check(relA !== null, 'Test8: acquire A failed');
check(__lock.tryTakeoverStaleLock(lockDir), 'Test8: takeover failed');
var relB = __lock.acquireLock(lockDir);
check(relB !== null, 'Test8: acquire B failed');
var ownerB = readOwner();
check(ownerB !== null && typeof ownerB.lockId === 'string', 'Test8: B owner missing lockId');
relA(); // A's stale release — must leave successor lock intact
check(lockExists(), 'Test8: stale A release deleted B lock');
check(readOwner().lockId === ownerB.lockId, 'Test8: owner changed after stale A release');
timedOut = false;
try {
  __lock.acquireStorageLock(storageFile);
} catch (e) {
  timedOut = /timeout/.test(String(e.message));
}
check(timedOut, 'Test8: contender C entered while live B held lock');
relB();
var relC = __lock.acquireStorageLock(storageFile);
check(typeof relC === 'function', 'Test8: acquisition failed after B released');
relC();

// --- Test 9: liveness classification ---
check(__lock.isProcessAlive(process.pid) === true, 'Test9: own PID classified dead');
if (!isWin && process.getuid && process.getuid() !== 0) {
  // Signaling init without privilege yields EPERM — must classify alive.
  check(__lock.isProcessAlive(1) === true, 'Test9: EPERM PID 1 classified dead');
}

// Clean up tmp dir.
try { fs.rmSync(tmpDir, { recursive: true, force: true }); } catch (_) {}

if (failures.length > 0) {
  fs.writeFileSync('output.txt', failures.join('\n') + '\n');
  process.exit(1);
}
fs.writeFileSync('output.txt', 'STALE_LOCK_OK\n');
