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
var STALE_LOCK_MAX_AGE = 60 * 1000;  // 60 s fallback stale threshold
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
// On POSIX, kill(pid, 0) with ESRCH means dead.  On Windows, always
// returns true — mtime-based fallback handles staleness there.
function isProcessAlive(pid) {
  if (os.platform() === 'win32') return true;
  try {
    process.kill(pid, 0);
    return true;
  } catch (e) {
    return e.code !== 'ESRCH';
  }
}

// isLockStale checks whether an existing lock directory can be safely
// taken over.  dirMod is the lock directory mtime (ms since epoch).
function isLockStale(lockDir, dirMod) {
  var ownerFile = ownerPath(lockDir);
  var now = Date.now();
  try {
    var data = fs.readFileSync(ownerFile, 'utf8');
    var owner = JSON.parse(data);
    if (owner.pid && !isProcessAlive(owner.pid)) {
      return true;
    }
    var age = now - Math.min(dirMod, owner.processStart || dirMod);
    return age > STALE_LOCK_MAX_AGE;
  } catch (e) {
    if (e.code === 'ENOENT') {
      return now - dirMod > LOCK_GRACE;
    }
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
function acquireLock(lockDir) {
  try {
    fs.mkdirSync(lockDir, 0o755);
    var owner = JSON.stringify({
      lockId: randomHex(8),
      pid: process.pid,
      processStart: Date.now(),
    });
    fs.writeFileSync(ownerPath(lockDir), owner, { mode: 0o644 });
    return function release() {
      releaseLock(lockDir);
    };
  } catch (e) {
    if (e.code === 'EEXIST') return null;
    throw e;
  }
}

// releaseLock removes lockDir. Best-effort, never throws.
function releaseLock(lockDir) {
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

    // Spin for retry interval.
    var end = Date.now() + LOCK_RETRY;
    while (Date.now() < end) { /* spin */ }
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

module.exports = { createLocalStorage: createLocalStorage, createSessionStorage: createSessionStorage };
