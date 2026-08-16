// Test sessionStorage: in-memory, independent of localStorage, not persisted.
// Writes results to output.txt.

var fs = require('fs');

localStorage.clear();
sessionStorage.clear();

// sessionStorage works the same as localStorage for basic API
sessionStorage.setItem('session-key', 'session-value');
if (sessionStorage.getItem('session-key') !== 'session-value') {
  fs.writeFileSync('output.txt', 'FAIL: sessionStorage getItem\n');
  process.exit(1);
}

// Does NOT affect localStorage
if (localStorage.getItem('session-key') !== null) {
  fs.writeFileSync('output.txt', 'FAIL: sessionStorage leaked into localStorage\n');
  process.exit(1);
}

// sessionStorage has its own namespace
localStorage.setItem('shared-key', 'from-local');
sessionStorage.setItem('shared-key', 'from-session');
if (localStorage.getItem('shared-key') !== 'from-local') {
  fs.writeFileSync('output.txt', 'FAIL: localStorage shared-key overwritten\n');
  process.exit(1);
}
if (sessionStorage.getItem('shared-key') !== 'from-session') {
  fs.writeFileSync('output.txt', 'FAIL: sessionStorage shared-key wrong\n');
  process.exit(1);
}

// sessionStorage is NOT persisted: run-count starts at 0 each invocation.
var count = parseInt(sessionStorage.getItem('run-count') || '0', 10);
if (count !== 0) {
  fs.writeFileSync('output.txt', 'FAIL: sessionStorage persisted across invocations (run-count=' + count + ')\n');
  process.exit(1);
}
sessionStorage.setItem('run-count', String(count + 1));

fs.writeFileSync('output.txt', 'SESSION_OK\n');
