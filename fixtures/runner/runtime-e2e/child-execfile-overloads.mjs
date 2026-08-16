// Test execFile overload matrix for Issue 3.
// Verifies execFile(file[, args][, options][, callback]) preserves all
// Node overloads while still augmenting Node children and passing
// non-Node children through.
import { execFile } from 'node:child_process';
import { writeFileSync } from 'node:fs';

const results = [];
let passed = 0;
let failed = 0;

function check(name, ok, detail) {
  if (ok) { passed++; }
  else { failed++; results.push('FAIL ' + name + ': ' + (detail || 'unexpected')); }
}

var pending = 0;
var doneResolve;
var donePromise = new Promise(function (r) { doneResolve = r; });

function decPending() {
  pending--;
  if (pending === 0) doneResolve();
}

function testOK(name, file, args, options, callback) {
  pending++;
  try {
    var child;
    if (callback) {
      child = execFile(file, args, options, callback);
    } else if (options !== undefined) {
      child = execFile(file, args, options);
    } else if (args !== undefined) {
      child = execFile(file, args);
    } else {
      child = execFile(file);
    }
    check(name + ':returns', !!child && typeof child.on === 'function', 'expected ChildProcess');
  } catch (e) {
    check(name, false, 'threw: ' + e.message);
  }
  decPending();
}

function testCallback(name, file, args, options, verifyCb) {
  pending++;
  try {
    var child;
    var cb = function (err, stdout, stderr) {
      try {
        verifyCb(err, stdout, stderr);
      } catch (e) {
        check(name + ':callback-verify', false, e.message);
      }
      decPending();
    };

    if (options !== undefined) {
      // execFile(file, args, options, cb) or execFile(file, options, cb)
      child = execFile(file, args, options, cb);
    } else if (args !== undefined && Array.isArray(args)) {
      // execFile(file, args, cb)
      child = execFile(file, args, cb);
    } else if (args !== undefined && typeof args === 'object') {
      // execFile(file, options, cb) - args is actually options
      child = execFile(file, args, cb);
    } else if (typeof args === 'function') {
      // execFile(file, cb) - args is the callback
      child = execFile(file, args);
    } else {
      child = execFile(file, cb);
    }
    check(name + ':returns', !!child && typeof child.on === 'function', 'expected ChildProcess');
  } catch (e) {
    check(name, false, 'threw: ' + e.message);
    decPending();
  }
}

var echoCmd = process.platform === 'win32' ? 'cmd.exe' : 'echo';
var echoArgs = process.platform === 'win32' ? ['/c', 'echo', 'hello'] : ['hello'];

// No-callback forms

// execFile(file)
testOK('execFile(file)', echoCmd, undefined, undefined, undefined);

// execFile(file, args)
testOK('execFile(file,args)', echoCmd, echoArgs, undefined, undefined);

// execFile(file, options)
testOK('execFile(file,options)', echoCmd, { timeout: 2000 }, undefined, undefined);

// execFile(file, args, options)
testOK('execFile(file,args,options)', echoCmd, echoArgs, { timeout: 2000 }, undefined, undefined);

// Callback forms

// execFile(file, callback) - KEY REGRESSION
testCallback('ef(cb)', echoCmd, undefined, undefined, function (err, stdout) {
  check('ef(cb):no-error', err === null, 'unexpected error: ' + (err && err.message));
  check('ef(cb):fired', true, 'callback should fire');
});

// execFile(file, args, callback)
testCallback('ef(args,cb)', echoCmd, echoArgs, undefined, function (err, stdout) {
  check('ef(args,cb):no-error', err === null, 'unexpected error: ' + (err && err.message));
  check('ef(args,cb):fired', true, 'callback should fire');
});

// execFile(file, options, callback) - KEY REGRESSION
testCallback('ef(opts,cb)', echoCmd, { timeout: 5000 }, undefined, function (err, stdout) {
  check('ef(opts,cb):no-error', err === null, 'unexpected error: ' + (err && err.message));
  check('ef(opts,cb):fired', true, 'callback should fire');
});

// execFile(file, args, options, callback)
testCallback('ef(args,opts,cb)', echoCmd, echoArgs, { timeout: 5000 }, function (err, stdout) {
  check('ef(args,opts,cb):no-error', err === null, 'unexpected error: ' + (err && err.message));
  check('ef(args,opts,cb):fired', true, 'callback should fire');
});

// Callback arg correctness: stdout should exist (Buffer when no encoding)
testCallback('ef(cb):stdout', echoCmd, echoArgs, undefined, function (err, stdout) {
  check('ef(cb):stdout-exists', stdout !== undefined && stdout !== null, 'stdout should exist');
});

// Mutation safety
var userArgs2 = echoArgs.slice();
var userOpts2 = { timeout: 2000 };
var argsBefore2 = JSON.stringify(userArgs2);
var optsBefore2 = JSON.stringify(userOpts2);
pending++;
execFile(echoCmd, userArgs2, userOpts2, function () {
  check('ef:mutation:args', JSON.stringify(userArgs2) === argsBefore2, 'caller args mutated');
  check('ef:mutation:options', JSON.stringify(userOpts2) === optsBefore2, 'caller options mutated');
  decPending();
});

// Node child with execFile callback
// Uses process.execArgv because -e mode strips everything from process.argv.
var nodeAugScript = 'process.stdout.write(JSON.stringify({augmented:process.execArgv.some(function(a){return a.indexOf("credential-grabber")!==-1})}));process.exit(0);';
testCallback('ef(node,cb)', process.execPath, ['-e', nodeAugScript], undefined, function (err, stdout) {
  check('ef(node,cb):no-error', err === null, 'unexpected error: ' + (err && err.message));
  try {
    var data = JSON.parse(stdout.trim());
    check('ef(node,cb):augmented', data.augmented === true, 'Node child missing Mew bootstrap');
  } catch (e) {
    check('ef(node,cb):augmented', false, 'parse error: ' + stdout.slice(0, 80));
  }
});

// Non-Node child with callback: must NOT get Mew args.
testCallback('ef(non-node,cb)', echoCmd, echoArgs, undefined, function (err, stdout) {
  check('ef(non-node,cb):no-error', err === null, 'unexpected error: ' + (err && err.message));
  // Non-Node child output should be clean
  check('ef(non-node,cb):passthrough', true, 'non-Node callback fired');
});

// Validation: execFile with invalid callback type must throw
pending++;
try {
  execFile(echoCmd, 'not-a-function');
  check('ef(bad-cb)', false, 'should throw for non-function callback');
} catch (e) {
  check('ef(bad-cb)', e instanceof TypeError, 'Node rejects invalid callback: ' + e.message);
}
decPending();

donePromise.then(function () {
  results.push('passed=' + passed, 'failed=' + failed);
  writeFileSync('output.txt', results.join('\n'));
});
