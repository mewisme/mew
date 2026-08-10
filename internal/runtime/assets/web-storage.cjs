// Mew Web Storage implementation — canonical backend for localStorage
// and sessionStorage.  CJS module required by both preload.cjs (via
// require) and preload.mjs (via createRequire).
//
// localStorage: persisted to disk under the path given by the
//   MEW_LOCAL_STORAGE_PATH env var.  Cross-process mutations are
//   serialized through a directory-based lock (mkdir is atomic on
//   all supported platforms).  Every mutation reloads the latest
//   committed state while holding the lock, applies the change, and
//   writes atomically (temp+fsync+rename).  Readers check file mtime
//   and reload when the store was modified externally.
// sessionStorage: in-memory Map, never persisted.
//
// Keys and values are coerced to String.  Missing keys return null.
// Keys are enumerated in insertion order.
//
// Quota: 5 MiB default; override with MEW_STORAGE_QUOTA_BYTES.
// Property-style access (storage.foo, storage[key]) and
// Object.keys(storage) are deliberately unsupported — use the
// Storage methods.

'use strict';

var fs = require('node:fs');
var path = require('node:path');
var os = require('node:os');
var crypto = require('node:crypto');

// ---- constants ---------------------------------------------------------

var SCHEMA_VERSION = 1;
var DEFAULT_QUOTA = 5 * 1024 * 1024; // 5 MiB
var LOCK_MAX_WAIT = 30 * 1000;       // 30 s
var LOCK_RETRY = 25;                 // 25 ms
var LOCK_GRACE = 5 * 1000;           // 5 s grace for malformed/missing owner
var HEARTBEAT_MAX_AGE = 60 * 1000;   // 60 s: stale heartbeat → owner abandoned (PID reuse guard)
var TEMP_CLEANUP_AGE = 5 * 60 * 1000; // 5 min for abandoned temp files

// ---- helpers -----------------------------------------------------------

function quotaFromEnv() {
  var raw = process.env.MEW_STORAGE_QUOTA_BYTES;
  if (raw === undefined) return DEFAULT_QUOTA;
  var n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return DEFAULT_QUOTA;
  return n;
}

function storageError(name, message) {
  try {
    return new DOMException(message, name);
  } catch (_) {
    var e = new Error(message);
    e.name = name;
    e.code = name;
    return e;
  }
}

function randomHex(bytes) {
  return crypto.randomBytes(bytes).toString('hex');
}

// sleepSync blocks the current thread for at least ms milliseconds.
// Uses Atomics.wait (true OS-level block) when available; falls back to
// a spin loop on older Node versions where Atomics.wait is disallowed on
// the main thread.
function sleepSync(ms) {
  try {
    var sab = new SharedArrayBuffer(4);
    var view = new Int32Array(sab);
    Atomics.wait(view, 0, 0, ms);
  } catch (_) {
    // Atomics.wait not available on main thread — fall back to spin.
    var end = Date.now() + ms;
    while (Date.now() < end) { /* spin */ }
  }
}

// ---- lock directory protocol ------------------------------------------

// lockPath returns the lock directory path derived from the storage file.
function lockPath(filePath) {
  return filePath + '.lock';
}

// ownerPath returns the owner.json path inside a lock directory.
function ownerPath(lockDir) {
  return path.join(lockDir, 'owner.json');
}

// tombstoneRoot returns the sibling tombstone directory for lockDir.
function tombstoneRoot(lockDir) {
  return path.join(path.dirname(lockDir), '.lock-tombstones');
}

// isProcessAlive reports whether a PID is likely alive.
// Uses kill(pid, 0) which works on all platforms: ESRCH means the process
// does not exist.  EPERM/EACCES mean it exists but we lack permission to
// signal it — treat as alive (conservative).  PID reuse is a known
// limitation; the heartbeat in owner.json guards against it.
function isProcessAlive(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch (e) {
    // ESRCH: target process does not exist → dead.
    // EPERM/EACCES: process exists but no permission → conservatively alive.
    return e.code !== 'ESRCH';
  }
}

// isLockStale checks whether an existing lock directory can be safely
// taken over.  dirMod is the lock directory mtime (ms since epoch).
//
// Staleness rules (in order):
//   1. Live process + recent heartbeat        → NOT stale (owner alive)
//   2. Live process + stale heartbeat         → stale (PID reuse: original
//      owner stopped renewing, new process got same PID)
//   3. Live process + legacy owner (no hb)    → NOT stale (conservative;
//      PID reuse is documented limitation for pre-heartbeat locks)
//   4. Dead process (ESRCH)                   → stale after LOCK_GRACE
//   *. Malformed / missing owner             → stale after LOCK_GRACE
//
// Lock age alone is NEVER sufficient to steal a lock whose owner is
// provably or conservatively assumed to still be alive.
function isLockStale(lockDir, dirMod) {
  var ownerFile = ownerPath(lockDir);
  var now = Date.now();
  try {
    var data = fs.readFileSync(ownerFile, 'utf8');
    var owner = JSON.parse(data);

    if (!owner || typeof owner !== 'object') {
      // Malformed owner metadata — wait grace period before reclaim.
      return now - dirMod > LOCK_GRACE;
    }

    // Rule 4: process confirmed dead → reclaim after grace.
    if (owner.pid && typeof owner.pid === 'number' && !isProcessAlive(owner.pid)) {
      return now - dirMod > LOCK_GRACE;
    }

    // Process appears alive (or PID missing/unusable).
    if (owner.pid && typeof owner.pid === 'number') {
      // Check heartbeat for PID reuse protection.
      if (owner.heartbeat && typeof owner.heartbeat === 'number') {
        // Rule 1: recent heartbeat → definitely alive.
        if (now - owner.heartbeat <= HEARTBEAT_MAX_AGE) {
          return false;
        }
        // Rule 2: stale heartbeat → PID likely reused, lock abandoned.
        return true;
      }

      // Rule 3: legacy lock without heartbeat.  Process appears alive.
      // Conservative: assume the original owner is still alive.
      // PID reuse for legacy locks is a documented limitation.
      return false;
    }

    // No PID in metadata — malformed.  Wait grace period.
    return now - dirMod > LOCK_GRACE;
  } catch (e) {
    if (e.code === 'ENOENT') {
      // Owner file missing — wait grace period.
      return now - dirMod > LOCK_GRACE;
    }
    // Unreadable owner file — wait grace period.
    return now - dirMod > LOCK_GRACE;
  }
}

// tryTakeoverStaleLock attempts to atomically tombstone a stale lock.
// Returns true if the lock was removed (by us or concurrently).
function tryTakeoverStaleLock(lockDir) {
  var root = tombstoneRoot(lockDir);
  try {
    fs.mkdirSync(path.join(root, 'stale'), { recursive: true });
  } catch (_) {
    // ignore
  }
  var tomb = path.join(root, 'stale', 'tomb-' + Date.now() + '-' + randomHex(4));
  try {
    fs.renameSync(lockDir, tomb);
    cleanupTombstones(root);
    return true;
  } catch (e) {
    if (e.code === 'ENOENT') return true;
    return false;
  }
}

function cleanupTombstones(root) {
  try {
    var staleDir = path.join(root, 'stale');
    var entries = fs.readdirSync(staleDir);
    for (var i = 0; i < entries.length; i++) {
      try {
        fs.rmSync(path.join(staleDir, entries[i]), { recursive: true, force: true });
      } catch (_) { /* best-effort */ }
    }
  } catch (_) { /* best-effort */ }
}

// acquireLock attempts to create lockDir exclusively via mkdir.
// Returns a release function on success, null if lock is held.
// The release closure captures the unique lockId so it can verify
// ownership before deleting the lock directory — a stale owner whose
// lock was taken over must not delete the successor's lock.
//
// Owner metadata written to owner.json contains:
//   lockId       — unique per acquisition (ABA guard)
//   pid          — owner process ID (for liveness check)
//   processStart — approximate wall-clock process start
//   heartbeat    — last liveness refresh (PID reuse guard)
function acquireLock(lockDir) {
  try {
    fs.mkdirSync(lockDir, 0o755);
    var lockId = randomHex(8);
    var now = Date.now();
    var owner = JSON.stringify({
      lockId: lockId,
      pid: process.pid,
      processStart: now - (process.uptime() * 1000),
      heartbeat: now,
    });
    fs.writeFileSync(ownerPath(lockDir), owner, { mode: 0o644 });
    return function release() {
      releaseLock(lockDir, lockId);
    };
  } catch (e) {
    if (e.code === 'EEXIST') return null;
    throw e;
  }
}

// refreshHeartbeat updates the heartbeat timestamp in owner.json.
// Only updates if the lockId still matches (lock not taken over).
// Returns true on success, false if ownership cannot be confirmed.
// Never throws — failure to refresh is non-fatal (the lock remains owned
// and isLiveProcess will still report the owner as alive).
function refreshHeartbeat(lockDir, lockId) {
  try {
    var ownerFile = ownerPath(lockDir);
    var data = fs.readFileSync(ownerFile, 'utf8');
    var owner = JSON.parse(data);
    if (!owner || typeof owner !== 'object') return false;
    if (owner.lockId !== lockId) return false;
    owner.heartbeat = Date.now();
    fs.writeFileSync(ownerFile, JSON.stringify(owner), { mode: 0o644 });
    return true;
  } catch (e) {
    return false;
  }
}

// releaseLock removes lockDir only if it still belongs to lockId.
// Best-effort, never throws.  If the lock was taken over by another
// owner (stale takeover), the canonical lock directory belongs to the
// successor and must not be deleted.
function releaseLock(lockDir, lockId) {
  try {
    var data = fs.readFileSync(ownerPath(lockDir), 'utf8');
    var current = JSON.parse(data);
    if (current.lockId !== lockId) {
      // Lock was taken over — the successor owns the canonical directory.
      return;
    }
  } catch (e) {
    if (e.code === 'ENOENT') {
      // Lock directory already gone (tombstoned by a stale takeover).
      return;
    }
    // Can't verify ownership — fail closed.
    return;
  }
  try {
    fs.rmSync(lockDir, { recursive: true, force: true });
  } catch (_) { /* best-effort */ }
}

// acquireStorageLock blocks until the lock is acquired or timeout.
// Returns a release function.  Throws on timeout.
function acquireStorageLock(filePath) {
  var lDir = lockPath(filePath);
  var parent = path.dirname(lDir);

  try { fs.mkdirSync(parent, { recursive: true }); } catch (_) { /* ignore */ }

  // Clean abandoned temp files before acquiring (best-effort).
  cleanupTempFiles(parent);

  var deadline = Date.now() + LOCK_MAX_WAIT;
  while (true) {
    var release = acquireLock(lDir);
    if (release) return release;

    var stat;
    try { stat = fs.statSync(lDir); } catch (e) { continue; }

    if (isLockStale(lDir, stat.mtimeMs)) {
      tryTakeoverStaleLock(lDir);
      continue;
    }

    if (Date.now() >= deadline) {
      throw new Error(
        'Failed to acquire localStorage lock: timeout after ' +
        (LOCK_MAX_WAIT / 1000) + 's'
      );
    }

    // Wait for retry interval with blocking sleep (no CPU spin).
    sleepSync(LOCK_RETRY);
  }
}

// ---- temp file cleanup -------------------------------------------------

var tempFilePattern = /\.tmp\.\d+\.\d+\.[a-z0-9]+$/;

function cleanupTempFiles(dir) {
  var entries;
  try { entries = fs.readdirSync(dir); } catch (_) { return; }
  var now = Date.now();
  for (var i = 0; i < entries.length; i++) {
    if (!tempFilePattern.test(entries[i])) continue;
    var fullPath = path.join(dir, entries[i]);
    try {
      var st = fs.statSync(fullPath);
      if (now - st.mtimeMs > TEMP_CLEANUP_AGE) {
        fs.unlinkSync(fullPath);
      }
    } catch (_) { /* best-effort */ }
  }
}

// ---- persistence -------------------------------------------------------

// loadStore reads and validates the on-disk JSON store.
// Returns {items, order, mtimeMs} or null (missing / empty / corrupt).
function loadStore(filePath) {
  var raw, stat;
  try {
    raw = fs.readFileSync(filePath, 'utf8');
    stat = fs.statSync(filePath);
  } catch (e) {
    if (e.code === 'ENOENT') return null;
    throw e;
  }

  var data;
  try {
    data = JSON.parse(raw);
  } catch (_) {
    console.warn('mew: localStorage file corrupt (invalid JSON), resetting.');
    return null;
  }

  if (!data || typeof data !== 'object') {
    console.warn('mew: localStorage file corrupt (not an object), resetting.');
    return null;
  }
  if (data.schemaVersion !== SCHEMA_VERSION) {
    console.warn(
      'mew: localStorage schema version ' + data.schemaVersion +
      ' unsupported (expected ' + SCHEMA_VERSION + '), resetting.'
    );
    return null;
  }
  if (!data.items || typeof data.items !== 'object') {
    console.warn('mew: localStorage file corrupt (missing items), resetting.');
    return null;
  }
  if (!Array.isArray(data.order)) {
    console.warn('mew: localStorage file corrupt (missing order), resetting.');
    return null;
  }

  // Rebuild order, filtering out keys whose values aren't strings.
  var order = [];
  var seen = {};
  for (var i = 0; i < data.order.length; i++) {
    var k = data.order[i];
    if (typeof k !== 'string') {
      console.warn('mew: localStorage file corrupt (non-string key in order), resetting.');
      return null;
    }
    if (!(k in data.items)) continue;
    if (typeof data.items[k] !== 'string') continue;
    if (seen[k]) continue;
    seen[k] = true;
    order.push(k);
  }

  // Copy only valid string entries.
  var items = {};
  var itemKeys = Object.keys(data.items);
  for (var j = 0; j < itemKeys.length; j++) {
    var ik = itemKeys[j];
    if (typeof data.items[ik] === 'string') {
      items[ik] = data.items[ik];
    }
  }

  return { items: items, order: order, mtimeMs: stat.mtimeMs };
}

// computeTotalSize returns the sum of all value string lengths.
function computeTotalSize(items, order) {
  var s = 0;
  for (var i = 0; i < order.length; i++) {
    var k = order[i];
    if (k in items) s += items[k].length;
  }
  return s;
}

// saveStore writes the store atomically (temp + fsync + rename).
// Must be called while holding the storage lock.
function saveStore(filePath, items, order) {
  var json = JSON.stringify({
    schemaVersion: SCHEMA_VERSION,
    items: items,
    order: order,
  });

  var dir = path.dirname(filePath);
  try {
    fs.mkdirSync(dir, { recursive: true });
  } catch (_) {
    // Directory already exists — ignore.
  }

  var tmpName = filePath + '.tmp.' + process.pid + '.' + Date.now() + '.' +
    Math.random().toString(36).slice(2, 8);
  try {
    fs.writeFileSync(tmpName, json, { flag: 'wx' });
  } catch (e) {
    if (e.code === 'EEXIST') {
      tmpName = filePath + '.tmp.' + process.pid + '.' + Date.now() + '.' +
        Math.random().toString(36).slice(2, 8);
      fs.writeFileSync(tmpName, json, { flag: 'wx' });
    } else {
      throw e;
    }
  }

  var fd;
  try {
    fd = fs.openSync(tmpName, 'r+');
    fs.fsyncSync(fd);
  } finally {
    if (fd !== undefined) fs.closeSync(fd);
  }

  try {
    fs.renameSync(tmpName, filePath);
  } catch (e) {
    try { fs.unlinkSync(tmpName); } catch (_) { /* best-effort */ }
    throw e;
  }
}

// ---- localStorage ------------------------------------------------------

function createLocalStorage(opts) {
  opts = opts || {};
  var filePath = opts.filePath || null;
  var quota = opts.quota || quotaFromEnv();

  // Mutable state — loaded on first access, reloaded on external changes.
  var items = Object.create(null);
  var order = [];
  var totalSize = 0;
  var loaded = false;
  var storeMtimeMs = 0;

  function reloadFromDisk() {
    if (!filePath) return;
    var stored = loadStore(filePath);
    if (stored) {
      items = stored.items;
      order = stored.order;
      totalSize = computeTotalSize(items, order);
      storeMtimeMs = stored.mtimeMs;
    } else {
      items = Object.create(null);
      order = [];
      totalSize = 0;
      storeMtimeMs = 0;
    }
    loaded = true;
  }

  // ensureFresh reloads from disk if the store file has been modified
  // externally (another process wrote it) or if never loaded.
  function ensureFresh() {
    if (!filePath) {
      if (!loaded) {
        items = Object.create(null);
        order = [];
        totalSize = 0;
        loaded = true;
      }
      return;
    }
    if (loaded) {
      try {
        var stat = fs.statSync(filePath);
        if (stat.mtimeMs === storeMtimeMs) return;
      } catch (e) {
        if (e.code === 'ENOENT') {
          items = Object.create(null);
          order = [];
          totalSize = 0;
          storeMtimeMs = 0;
          return;
        }
        return;
      }
    }
    reloadFromDisk();
  }

  // reloadLatestLocked reloads from disk unconditionally.
  // Called while holding the storage lock.
  function reloadLatestLocked() {
    if (!filePath) return;
    var stored = loadStore(filePath);
    if (stored) {
      items = stored.items;
      order = stored.order;
      totalSize = computeTotalSize(items, order);
      storeMtimeMs = stored.mtimeMs;
    } else {
      items = Object.create(null);
      order = [];
      totalSize = 0;
      storeMtimeMs = 0;
    }
  }

  function checkQuota(newBytes) {
    if (newBytes > quota) {
      throw storageError(
        'QuotaExceededError',
        "Failed to execute 'setItem' on 'Storage': " +
        'Setting the value exceeded the quota.'
      );
    }
  }

  function computeNewSize(key, newValue, curSize, curItems) {
    var s = curSize;
    if (key in curItems) {
      s -= curItems[key].length;
    }
    return s + newValue.length;
  }

  // writeLocked persists the current in-memory state atomically.
  // Must be called while holding the storage lock.
  function writeLocked() {
    if (!filePath) return;
    // Refresh heartbeat before potentially slow write so the lock owner
    // remains provably alive even for large stores on slow disks.
    // Uses PID match: only the owning process can be inside this critical
    // section, so PID equality proves ownership.
    try {
      var ownerFile = ownerPath(lockPath(filePath));
      var raw = fs.readFileSync(ownerFile, 'utf8');
      var meta = JSON.parse(raw);
      if (meta && meta.pid === process.pid) {
        meta.heartbeat = Date.now();
        fs.writeFileSync(ownerFile, JSON.stringify(meta), { mode: 0o644 });
      }
    } catch (_) { /* best-effort: stale heartbeat is non-fatal */ }
    saveStore(filePath, items, order);
    try { storeMtimeMs = fs.statSync(filePath).mtimeMs; } catch (_) { /* ignore */ }
  }

  return {
    getItem: function (key) {
      ensureFresh();
      var k = String(key);
      if (!(k in items)) return null;
      return items[k];
    },

    setItem: function (key, value) {
      var k = String(key);
      var v = String(value);

      if (filePath) {
        var release = acquireStorageLock(filePath);
        try {
          reloadLatestLocked();
          var newSize = computeNewSize(k, v, totalSize, items);
          checkQuota(newSize);
          if (!(k in items)) {
            order.push(k);
          }
          items[k] = v;
          totalSize = newSize;
          writeLocked();
        } finally {
          release();
        }
      } else {
        if (!loaded) {
          items = Object.create(null);
          order = [];
          totalSize = 0;
          loaded = true;
        }
        var memSize = computeNewSize(k, v, totalSize, items);
        checkQuota(memSize);
        if (!(k in items)) {
          order.push(k);
        }
        items[k] = v;
        totalSize = memSize;
      }
    },

    removeItem: function (key) {
      var k = String(key);

      if (filePath) {
        var release = acquireStorageLock(filePath);
        try {
          reloadLatestLocked();
          if (!(k in items)) return;
          totalSize -= items[k].length;
          delete items[k];
          var idx = order.indexOf(k);
          if (idx !== -1) order.splice(idx, 1);
          writeLocked();
        } finally {
          release();
        }
      } else {
        if (!loaded) {
          items = Object.create(null);
          order = [];
          totalSize = 0;
          loaded = true;
        }
        if (!(k in items)) return;
        totalSize -= items[k].length;
        delete items[k];
        var ix = order.indexOf(k);
        if (ix !== -1) order.splice(ix, 1);
      }
    },

    clear: function () {
      if (filePath) {
        var release = acquireStorageLock(filePath);
        try {
          reloadLatestLocked();
          items = Object.create(null);
          order = [];
          totalSize = 0;
          writeLocked();
        } finally {
          release();
        }
      } else {
        items = Object.create(null);
        order = [];
        totalSize = 0;
        loaded = true;
      }
    },

    key: function (index) {
      ensureFresh();
      if (index < 0 || index >= order.length) return null;
      return order[index];
    },

    get length() {
      ensureFresh();
      return order.length;
    },
  };
}

// ---- sessionStorage ----------------------------------------------------

function createSessionStorage() {
  var store = new Map();
  return {
    getItem: function (key) {
      var v = store.get(String(key));
      return v === undefined ? null : v;
    },
    setItem: function (key, value) { store.set(String(key), String(value)); },
    removeItem: function (key) { store.delete(String(key)); },
    clear: function () { store.clear(); },
    key: function (index) {
      var keys = Array.from(store.keys());
      return index >= 0 && index < keys.length ? keys[index] : null;
    },
    get length() { return store.size; },
  };
}

// ---- exports -----------------------------------------------------------

// __lockTest exposes internal lock functions for deterministic testing.
// Only populated when MEW_STORAGE_TEST_HOOKS=1 is set in the environment.
// Never use in production paths.
var __lockTest = undefined;
if (process.env.MEW_STORAGE_TEST_HOOKS === '1') {
  __lockTest = {
    acquireLock: acquireLock,
    releaseLock: releaseLock,
    refreshHeartbeat: refreshHeartbeat,
    tryTakeoverStaleLock: tryTakeoverStaleLock,
    isLockStale: isLockStale,
    isProcessAlive: isProcessAlive,
    lockPath: lockPath,
    tombstoneRoot: tombstoneRoot,
    HEARTBEAT_MAX_AGE: HEARTBEAT_MAX_AGE,
    LOCK_GRACE: LOCK_GRACE,
    LOCK_MAX_WAIT: LOCK_MAX_WAIT,
  };
}

module.exports = {
  createLocalStorage: createLocalStorage,
  createSessionStorage: createSessionStorage,
  __lockTest: __lockTest,
};
