// Test localStorage persistence.
// Mode: MEW_STORAGE_TEST_MODE=write|read
// Writes results to output.txt.

var fs = require('fs');
var mode = process.env.MEW_STORAGE_TEST_MODE || 'write';

if (mode === 'write') {
  localStorage.clear();
  localStorage.setItem('persisted', 'yes');
  localStorage.setItem('count', '42');
  localStorage.setItem('greeting', 'hello world');
  fs.writeFileSync('output.txt', 'WRITE_OK\n');
} else if (mode === 'read') {
  var greeting = localStorage.getItem('greeting');
  var count = localStorage.getItem('count');
  var persisted = localStorage.getItem('persisted');

  if (greeting === 'hello world' && count === '42' && persisted === 'yes') {
    if (localStorage.length === 3) {
      fs.writeFileSync('output.txt', 'READ_OK\n');
    } else {
      fs.writeFileSync('output.txt', 'FAIL: length expected 3 got ' + localStorage.length + '\n');
      process.exit(1);
    }
  } else {
    fs.writeFileSync('output.txt', 'FAIL: values mismatch greeting=' + greeting + ' count=' + count + ' persisted=' + persisted + '\n');
    process.exit(1);
  }
}
