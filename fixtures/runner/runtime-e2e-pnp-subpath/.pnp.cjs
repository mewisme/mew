// PnP API mock supporting subpath resolution.
'use strict';
const path = require('path');
const fs = require('fs');
const ROOT = __dirname;
const PKG_DIR = path.join(ROOT, 'packages', 'test-lib');

function resolveRequest(request, issuer, opts) {
  if (!request || request.startsWith('.') || request.startsWith('/')) return null;
  if (request === 'test-lib') return path.join(PKG_DIR, 'index.js');
  if (request === 'test-lib/sub') return path.join(PKG_DIR, 'sub.js');
  return null;
}

module.exports = {
  VERSIONS: { std: 3 },
  topLevel: { name: 'pnp-subpath-test', reference: 'workspace:.' },
  resolveRequest,
  resolveVirtual: function () { return null; },
};
