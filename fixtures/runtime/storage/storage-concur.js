// Concurrent localStorage mutation test — parameterized via env vars.
//
// MEW_CONCUR_ROLE: "set" | "read" | "clear" | "barrier-set"
// MEW_CONCUR_KEY: key name (for set/read)
// MEW_CONCUR_VAL: value to set (for set)
// MEW_CONCUR_READY_FILE: write this file when ready (barrier)
// MEW_CONCUR_GO_FILE: wait for this file before acting (barrier)
// MEW_CONCUR_RESULT_FILE: write result to this file
// MEW_CONCUR_EXPECT_FILE: read expected value from this file (for post-barrier read)
//
// Writes result to the file named by MEW_CONCUR_RESULT_FILE, or output.txt.

var fs = require('fs');

var role = process.env.MEW_CONCUR_ROLE || 'set';
var outFile = process.env.MEW_CONCUR_RESULT_FILE || 'output.txt';

function writeResult(text) {
  fs.writeFileSync(outFile, text + '\n');
}

// Barrier: signal readiness and wait for go.
function barrier() {
  var readyFile = process.env.MEW_CONCUR_READY_FILE;
  var goFile = process.env.MEW_CONCUR_GO_FILE;
  if (readyFile) {
    fs.writeFileSync(readyFile, 'ready');
  }
  if (goFile) {
    // Wait up to 30s for go signal.
    var deadline = Date.now() + 30000;
    while (true) {
      try {
        fs.statSync(goFile);
        break;
      } catch (e) {
        if (Date.now() > deadline) {
          writeResult('FAIL: barrier timeout waiting for ' + goFile);
          process.exit(1);
        }
      }
    }
  }
}

switch (role) {
  case 'set': {
    var key = process.env.MEW_CONCUR_KEY || 'default-key';
    var val = process.env.MEW_CONCUR_VAL || 'default-value';
    localStorage.setItem(key, val);
    writeResult('SET_OK:' + key + '=' + val);
    break;
  }

  case 'read': {
    var rkey = process.env.MEW_CONCUR_KEY || 'default-key';
    var result = localStorage.getItem(rkey);
    writeResult('READ:' + String(result));
    break;
  }

  case 'clear': {
    localStorage.clear();
    writeResult('CLEAR_OK');
    break;
  }

  case 'barrier-set': {
    barrier();
    var bkey = process.env.MEW_CONCUR_KEY || 'default-key';
    var bval = process.env.MEW_CONCUR_VAL || 'default-value';
    localStorage.setItem(bkey, bval);
    writeResult('BARRIER_SET_OK:' + bkey + '=' + bval);
    break;
  }

  case 'barrier-clear': {
    barrier();
    localStorage.clear();
    writeResult('BARRIER_CLEAR_OK');
    break;
  }

  case 'barrier-read': {
    barrier();
    var brkey = process.env.MEW_CONCUR_KEY || 'default-key';
    var brval = localStorage.getItem(brkey);
    writeResult('BARRIER_READ:' + String(brval));
    break;
  }

  default:
    writeResult('FAIL: unknown role ' + role);
    process.exit(1);
}
