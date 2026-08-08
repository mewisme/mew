// Attacker-style probe: malicious --require preload that attempts to
// recover transform credentials through every available channel.
// Writes probe results to output.txt as key=value lines.
// Expected result: all probes report "absent" or "none".
'use strict';

var fs = require('node:fs');
var os = require('node:os');
var path = require('node:path');
var results = [];

// Probe 1: env vars (must be absent — grabber stripped them)
var envVars = [
  'MEW_TRANSFORM_ENDPOINT',
  'MEW_TRANSFORM_TOKEN',
  'MEW_TRANSFORM_OPTIONS',
  'MEW_TRANSFORM_OPTS_DIGEST',
  'MEW_TRANSFORM_CONFIG_DIR',
];
for (var i = 0; i < envVars.length; i++) {
  results.push('env-' + envVars[i] + '=' + (process.env[envVars[i]] || 'absent'));
}

// Probe 2: old PID-based temp file (must not exist)
var oldCredsFile = path.join(os.tmpdir(), '.mew-creds-' + process.pid + '.json');
try {
  if (fs.existsSync(oldCredsFile)) {
    results.push('old-pid-creds-file=present');
    var data = fs.readFileSync(oldCredsFile, 'utf8');
    results.push('old-pid-creds-content=' + data.substring(0, 200));
  } else {
    results.push('old-pid-creds-file=absent');
  }
} catch (e) {
  results.push('old-pid-creds-file=error:' + e.message);
}

// Probe 3: scan tmpdir for any .mew-creds-* files
try {
  var files = fs.readdirSync(os.tmpdir());
  var mewCredsFiles = [];
  for (var j = 0; j < files.length; j++) {
    if (files[j].startsWith('.mew-creds-')) {
      mewCredsFiles.push(files[j]);
    }
  }
  results.push('tmpdir-creds-count=' + mewCredsFiles.length);
  if (mewCredsFiles.length > 0) {
    results.push('tmpdir-creds-files=' + mewCredsFiles.join(','));
  }
} catch (e) {
  results.push('tmpdir-creds-scan=error:' + e.message);
}

// Probe 4: process.argv (must not contain secrets)
var hasEndpointInArgv = false;
for (var k = 0; k < process.argv.length; k++) {
  if (process.argv[k].indexOf('127.0.0.1:') >= 0 && process.argv[k].indexOf('MEW_TRANSFORM') >= 0) {
    hasEndpointInArgv = true;
  }
}
results.push('argv-has-endpoint=' + (hasEndpointInArgv ? 'present' : 'absent'));

// Probe 5: execArgv (must not contain secrets)
var hasEndpointInExecArgv = false;
for (var m = 0; m < process.execArgv.length; m++) {
  if (process.execArgv[m].indexOf('127.0.0.1:') >= 0) {
    hasEndpointInExecArgv = true;
  }
}
results.push('execArgv-has-endpoint=' + (hasEndpointInExecArgv ? 'present' : 'absent'));

// Probe 6: find credential-grabber path from execArgv, check require cache
var grabberPath = null;
for (var n = 0; n < process.execArgv.length; n++) {
  if (process.execArgv[n].endsWith && process.execArgv[n].endsWith('credential-grabber.cjs')) {
    grabberPath = process.execArgv[n];
    break;
  }
}
if (grabberPath && fs.existsSync(grabberPath)) {
  try {
    var cached = require.cache[require.resolve(grabberPath)];
    if (cached && cached.exports) {
      var exp = cached.exports;
      if (exp.endpoint) {
        results.push('grabber-cache-endpoint=' + String(exp.endpoint).substring(0, 50));
      } else {
        results.push('grabber-cache-endpoint=absent');
      }
      if (exp.token) {
        results.push('grabber-cache-token=present');
      } else {
        results.push('grabber-cache-token=absent');
      }
    }
  } catch (e) {
    results.push('grabber-cache-probe=error:' + e.message);
  }
} else {
  results.push('grabber-cache-probe=not-found');
}

// Probe 7: check globalThis for credential-related symbol keys
try {
  var syms = Object.getOwnPropertySymbols(globalThis);
  var suspiciousCount = 0;
  for (var p = 0; p < syms.length; p++) {
    var desc = syms[p].toString();
    if (desc.indexOf('mew') >= 0 || desc.indexOf('creds') >= 0 || desc.indexOf('token') >= 0) {
      suspiciousCount++;
    }
  }
  results.push('globalThis-suspicious-symbols=' + suspiciousCount);
} catch (e) {
  results.push('globalThis-suspicious-symbols=error:' + e.message);
}

// Probe 8: verify we're in main thread
results.push('is-main-thread=' + String(require('node:worker_threads').isMainThread));

fs.writeFileSync('output.txt', results.join('\n'));
