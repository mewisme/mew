// Test spawn overload matrix for Issue 3.
// Verifies spawn(cmd[, args][, options]) preserves all Node overloads
// while still augmenting Node children and passing non-Node children through.
import { spawn } from 'node:child_process';
import { writeFileSync } from 'node:fs';

const results = [];
let passed = 0;
let failed = 0;

function check(name, ok, detail) {
  if (ok) { passed++; }
  else { failed++; results.push('FAIL ' + name + ': ' + (detail || 'unexpected')); }
}

function runSpawn(name, cmd, args, options, nodeAugmentExpected) {
  return new Promise(function (resolve) {
    var child;
    try {
      if (args === undefined && options === undefined) {
        child = spawn(cmd);
      } else if (options === undefined) {
        child = spawn(cmd, args);
      } else if (args === undefined) {
        child = spawn(cmd, options);
      } else {
        child = spawn(cmd, args, options);
      }
    } catch (e) {
      check(name, false, 'spawn threw: ' + e.message);
      resolve();
      return;
    }

    check(name + ':pid', typeof child.pid === 'number', 'expected ChildProcess');

    var stdout = '';
    if (child.stdout) {
      child.stdout.on('data', function (chunk) { stdout += chunk; });
    }

    child.on('error', function (err) {
      check(name + ':no-spawn-error', false, err.message);
      resolve();
    });

    child.on('exit', function (code) {
      if (nodeAugmentExpected) {
        try {
          var data = JSON.parse(stdout.trim());
          check(name + ':augmented', data.augmented === true, 'Node child missing Mew bootstrap');
        } catch (e) {
          check(name + ':augmented', false, 'failed to parse child output: ' + stdout.slice(0, 80));
        }
      }
      resolve();
    });

    // Timeout safety: kill after 5s
    setTimeout(function () { child.kill(); resolve(); }, 5000);
  });
}

// Helper to get echo command for this platform.
var echoCmd, echoArgs;
if (process.platform === 'win32') {
  echoCmd = 'cmd.exe';
  echoArgs = ['/c', 'echo', 'hello'];
} else {
  echoCmd = 'echo';
  echoArgs = ['hello'];
}

// Node child task: prints JSON with augmentation info.
// Uses process.execArgv because -e mode strips everything from process.argv.
var nodeAugScript = 'process.stdout.write(JSON.stringify({augmented:process.execArgv.some(function(a){return a.indexOf("credential-grabber")!==-1})}));process.exit(0);';

(async function () {
  // spawn with non-Node command

  // spawn(command)
  await runSpawn('spawn(cmd)', echoCmd, undefined, undefined, false);

  // spawn(command, args)
  await runSpawn('spawn(cmd,args)', echoCmd, echoArgs, undefined, false);

  // spawn(command, options) - KEY REGRESSION: options in second position
  await runSpawn('spawn(cmd,options)', echoCmd, undefined, { stdio: 'ignore', timeout: 2000 }, false);

  // spawn(command, args, options)
  await runSpawn('spawn(cmd,args,options)', echoCmd, echoArgs, { stdio: 'ignore', timeout: 2000 }, false);

  // spawn with Node executable

  // spawn(process.execPath, args)
  await runSpawn('spawn(node,args)', process.execPath, ['-e', nodeAugScript], undefined, true);

  // spawn(process.execPath, options) - KEY REGRESSION
  // Without args Node starts REPL; just verify the call doesn't crash.
  await runSpawn('spawn(node,options)', process.execPath, undefined, { stdio: ['pipe', 'pipe', 'ignore'], timeout: 5000 }, false);

  // spawn(process.execPath, args, options)
  await runSpawn('spawn(node,args,options)', process.execPath, ['-e', nodeAugScript], { stdio: ['pipe', 'pipe', 'ignore'], timeout: 5000 }, true);

  // Non-Node children must NOT be augmented
  var nonNodeChild = spawn(echoCmd, echoArgs, { stdio: 'pipe', timeout: 2000 });
  var nonNodeStdout = '';
  nonNodeChild.stdout.on('data', function (chunk) { nonNodeStdout += chunk; });
  await new Promise(function (resolve) {
    nonNodeChild.on('exit', function () {
      check('non-node:passthrough', nonNodeStdout.indexOf('credential-grabber') === -1, 'non-Node child should not get Mew args');
      resolve();
    });
    setTimeout(function () { nonNodeChild.kill(); resolve(); }, 5000);
  });

  // Mutation safety
  var userArgs = ['-e', '0'];
  var userOpts = { stdio: 'ignore', timeout: 2000 };
  var argsBefore = JSON.stringify(userArgs);
  var optsBefore = JSON.stringify(userOpts);
  var mutChild = spawn(echoCmd, userArgs, userOpts);
  await new Promise(function (resolve) {
    mutChild.on('exit', function () {
      check('mutation:args', JSON.stringify(userArgs) === argsBefore, 'caller args mutated');
      check('mutation:options', JSON.stringify(userOpts) === optsBefore, 'caller options mutated');
      resolve();
    });
    setTimeout(function () { mutChild.kill(); resolve(); }, 5000);
  });

  // Explicit empty args must remain distinct
  var emptyArgsChild = spawn(echoCmd, [], { stdio: 'ignore', timeout: 2000 });
  check('empty-args:created', !!emptyArgsChild && typeof emptyArgsChild.pid === 'number', 'spawn with empty args failed');
  emptyArgsChild.kill();

  // Report.
  results.push('passed=' + passed, 'failed=' + failed);
  writeFileSync('output.txt', results.join('\n'));
})();
