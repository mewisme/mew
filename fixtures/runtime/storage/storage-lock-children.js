// Multi-process storage-lock fixture — child roles driven by env vars.
//
// MEW_LOCKTEST_ROLE:
//   hold    — acquire lock on MEW_LOCKTEST_FILE, signal
//             MEW_LOCKTEST_READY_FILE, stay alive for
//             MEW_LOCKTEST_HOLD_MS, release, write MEW_LOCKTEST_DONE_FILE.
//   abandon — acquire lock, signal READY and DONE, then exit WITHOUT
//             releasing — creates a genuinely dead lock owner.
//   try     — wait for MEW_LOCKTEST_GO_FILE, then attempt
//             acquireStorageLock; append "TIMEOUT <pid>" or "ACQUIRED
//             <pid>" to MEW_LOCKTEST_RESULT_FILE (acquirers append
//             ENTER/EXIT to MEW_LOCKTEST_LOG while inside).
//   overlap — MEW_LOCKTEST_ITERS iterations of acquire, ENTER-log,
//             brief work, EXIT-log, release.
//
// Requires MEW_STORAGE_TEST_HOOKS=1 (exposes __mewLockTest via preload).

var fs = require('fs');

var __lock = globalThis.__mewLockTest;
if (!__lock) {
  fs.writeFileSync('output.txt', 'FAIL: __mewLockTest not available\n');
  process.exit(1);
}

var file = process.env.MEW_LOCKTEST_FILE;
var role = process.env.MEW_LOCKTEST_ROLE;
var readyFile = process.env.MEW_LOCKTEST_READY_FILE;
var goFile = process.env.MEW_LOCKTEST_GO_FILE;
var doneFile = process.env.MEW_LOCKTEST_DONE_FILE;
var resultFile = process.env.MEW_LOCKTEST_RESULT_FILE;
var logFile = process.env.MEW_LOCKTEST_LOG;
var holdMs = Number(process.env.MEW_LOCKTEST_HOLD_MS) || 4000;
var iters = Number(process.env.MEW_LOCKTEST_ITERS) || 20;

function sleepSync(ms) {
  try {
    var view = new Int32Array(new SharedArrayBuffer(4));
    Atomics.wait(view, 0, 0, ms);
  } catch (_) {
    var end = Date.now() + ms;
    while (Date.now() < end) { /* spin */ }
  }
}

function appendLog(line) {
  var fd = fs.openSync(logFile, 'a');
  try { fs.writeSync(fd, line + '\n'); } finally { fs.closeSync(fd); }
}

function waitFor(file) {
  var deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    try { fs.statSync(file); return true; } catch (_) { sleepSync(10); }
  }
  return false;
}

function fail(msg) {
  fs.writeFileSync('output.txt', 'FAIL: ' + msg + '\n');
  process.exit(1);
}

if (role === 'hold') {
  var release = __lock.acquireStorageLock(file);
  if (typeof release !== 'function') fail('hold: acquire returned ' + release);
  fs.writeFileSync(readyFile, 'ready');
  // Alive and inside the critical section — contenders must not enter.
  sleepSync(holdMs);
  release();
  fs.writeFileSync(doneFile, 'done');
} else if (role === 'abandon') {
  var relAbandon = __lock.acquireStorageLock(file);
  if (typeof relAbandon !== 'function') fail('abandon: acquire returned ' + relAbandon);
  if (readyFile) fs.writeFileSync(readyFile, 'ready');
  if (doneFile) fs.writeFileSync(doneFile, 'done');
  // Exit holding the lock — owner PID becomes dead.
} else if (role === 'try') {
  if (!waitFor(goFile)) fail('try: go signal never arrived');
  try {
    var rel = __lock.acquireStorageLock(file);
    if (logFile) {
      appendLog('ENTER ' + process.pid);
      sleepSync(20);
      appendLog('EXIT ' + process.pid);
    }
    rel();
    fs.appendFileSync(resultFile, 'ACQUIRED ' + process.pid + '\n');
  } catch (e) {
    if (!/timeout/.test(String(e.message))) fail('try: unexpected error ' + e.message);
    fs.appendFileSync(resultFile, 'TIMEOUT ' + process.pid + '\n');
  }
} else if (role === 'overlap') {
  for (var i = 0; i < iters; i++) {
    var r = __lock.acquireStorageLock(file);
    if (typeof r !== 'function') fail('overlap: acquire returned ' + r);
    appendLog('ENTER ' + process.pid);
    sleepSync(5);
    appendLog('EXIT ' + process.pid);
    r();
  }
} else {
  fail('unknown role ' + role);
}

fs.writeFileSync('output.txt', 'LOCK_CHILD_OK\n');
