// Test basic Storage API: getItem, setItem, removeItem, clear, key, length.
// Writes results to output.txt.

var fs = require('fs');
var failures = [];

function check(cond, msg) {
  if (!cond) failures.push('FAIL: ' + msg);
}

// getItem / setItem
check(localStorage.getItem('nonexistent') === null, 'getItem missing returns null');

localStorage.setItem('a', 'hello');
check(localStorage.getItem('a') === 'hello', 'getItem returns set value');

// String coercion
localStorage.setItem(123, 456);
check(localStorage.getItem('123') === '456', 'setItem coerces key and value to string');
check(typeof localStorage.getItem('123') === 'string', 'getItem returns string');

// removeItem
localStorage.setItem('b', 'world');
check(localStorage.getItem('b') === 'world', 'getItem before remove');
localStorage.removeItem('b');
check(localStorage.getItem('b') === null, 'getItem after remove returns null');
localStorage.removeItem('nonexistent'); // should not throw

// clear
localStorage.setItem('x', '1');
localStorage.setItem('y', '2');
check(localStorage.length >= 2, 'length after set');
localStorage.clear();
check(localStorage.length === 0, 'length after clear is 0');
check(localStorage.getItem('x') === null, 'getItem after clear returns null');

// key enumeration and ordering
localStorage.clear();
localStorage.setItem('first', '1');
localStorage.setItem('second', '2');
localStorage.setItem('third', '3');
check(localStorage.length === 3, 'length === 3');
check(localStorage.key(0) === 'first', 'key(0) === first');
check(localStorage.key(1) === 'second', 'key(1) === second');
check(localStorage.key(2) === 'third', 'key(2) === third');
check(localStorage.key(-1) === null, 'key(-1) returns null');
check(localStorage.key(3) === null, 'key(3) out of range returns null');
check(localStorage.key(999) === null, 'key(999) returns null');

// Re-insert preserves position, update doesn't add duplicate
localStorage.setItem('second', 'updated');
check(localStorage.getItem('second') === 'updated', 'update value');
check(localStorage.length === 3, 'length still 3 after update');
check(localStorage.key(1) === 'second', 'key(1) still second');

// Property-style access: deliberately unsupported
localStorage.foo = 'bar';
check(localStorage.getItem('foo') === null, 'property set does not affect store');
check(localStorage.foo === 'bar', 'property is a plain JS property');

fs.writeFileSync('output.txt', failures.length === 0 ? 'ALL_PASS\n' : failures.join('\n') + '\n');
process.exit(failures.length === 0 ? 0 : 1);
