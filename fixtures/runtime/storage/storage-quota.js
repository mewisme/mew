// Test quota enforcement.  MEW_STORAGE_QUOTA_BYTES sets a low limit.
// Writes results to output.txt.

var fs = require('fs');
var limit = parseInt(process.env.MEW_STORAGE_QUOTA_BYTES || '100', 10);

localStorage.clear();

// Write a value within quota.
var smallValue = 'x'.repeat(limit - 1);
try {
  localStorage.setItem('small', smallValue);
} catch (e) {
  fs.writeFileSync('output.txt', 'FAIL: setItem within quota threw: ' + e.message + '\n');
  process.exit(1);
}
if (localStorage.getItem('small') !== smallValue) {
  fs.writeFileSync('output.txt', 'FAIL: small value not stored correctly\n');
  process.exit(1);
}

// Write a value that exceeds quota.
var bigValue = 'y'.repeat(limit * 2);
var threw = false;
try {
  localStorage.setItem('big', bigValue);
} catch (e) {
  threw = true;
  var name = e.name || '';
  if (name !== 'QuotaExceededError' && e.code !== 'QuotaExceededError') {
    fs.writeFileSync('output.txt', 'FAIL: wrong error name: ' + name + ' / code: ' + (e.code || 'none') + '\n');
    process.exit(1);
  }
}
if (!threw) {
  fs.writeFileSync('output.txt', 'FAIL: quota exceeded did not throw\n');
  process.exit(1);
}

// Verify no partial mutation — small key still has its value.
if (localStorage.getItem('small') !== smallValue) {
  fs.writeFileSync('output.txt', 'FAIL: quota failure caused partial mutation\n');
  process.exit(1);
}

fs.writeFileSync('output.txt', 'QUOTA_OK\n');
