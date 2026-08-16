// Hostile credential probe running inside a forked child process.
// Verifies that transform credentials are NOT observable through:
//   - process.env
//   - process.argv
//   - process.execArgv
//   - require cache / module exports
// Also verifies TypeScript import still works (proves runtime is active).
import { libValue } from './lib.ts';

var results = [];

// Probe 1: env vars must be absent.
var envProbes = [
  'MEW_TRANSFORM_ENDPOINT',
  'MEW_TRANSFORM_TOKEN',
  'MEW_TRANSFORM_OPTIONS',
  'MEW_TRANSFORM_OPTS_DIGEST',
  'MEW_TRANSFORM_CONFIG_DIR',
  'MEW_TRANSFORM_DEP_TRACE_FILE',
  'MEW_TRANSFORM_DEP_TRACE_ROOT',
];
for (var i = 0; i < envProbes.length; i++) {
  results.push(envProbes[i] + '=' + (process.env[envProbes[i]] || 'absent'));
}

// Probe 2: process.argv must not contain raw credentials.
var argvHasEndpoint = false;
var argvHasToken = false;
var endpointPattern = /127\.0\.0\.1:\d{4,5}/;
var tokenPattern = /^[0-9a-f]{64}$/;
for (var i = 0; i < process.argv.length; i++) {
  if (endpointPattern.test(process.argv[i])) argvHasEndpoint = true;
  if (tokenPattern.test(process.argv[i])) argvHasToken = true;
}
results.push('argv-has-endpoint=' + (argvHasEndpoint ? 'yes' : 'no'));
results.push('argv-has-long-token=' + (argvHasToken ? 'yes' : 'no'));

// Probe 3: process.execArgv must not contain raw credentials.
var execArgvHasCreds = false;
for (var i = 0; i < process.execArgv.length; i++) {
  var a = process.execArgv[i];
  if (endpointPattern.test(a)) execArgvHasCreds = true;
  if (tokenPattern.test(a)) execArgvHasCreds = true;
}
results.push('execArgv-has-credentials=' + (execArgvHasCreds ? 'yes' : 'no'));

// Probe 4: credential-grabber module exports only nulls.
try {
  var credGrabber = require.cache;
  // We can't directly find the grabber module, but we can check all cached modules.
  var foundCreds = false;
  var foundEndpoint = false;
  for (var key in credGrabber) {
    var mod = credGrabber[key];
    if (mod && mod.exports) {
      if (mod.exports.endpoint && mod.exports.endpoint !== null) foundEndpoint = true;
      if (mod.exports.token && mod.exports.token !== null) foundCreds = true;
    }
  }
  results.push('require-cache-endpoint=' + (foundEndpoint ? 'leaked' : 'absent'));
  results.push('require-cache-token=' + (foundCreds ? 'leaked' : 'absent'));
} catch (_) {
  results.push('require-cache-probe=error');
}

// Probe 5: TypeScript import works (proves runtime is active).
results.push('libValue=' + libValue);

if (process.send) {
  process.send(results.join('\n'));
}
