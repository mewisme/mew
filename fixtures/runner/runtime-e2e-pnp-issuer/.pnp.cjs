// PnP API that validates the issuer is a native filesystem path.
// The ts-loader must convert file:// URLs before calling resolveRequest.
'use strict';
const path = require('path');
const fs = require('fs');
const ROOT = __dirname;

function resolveRequest(request, issuer, opts) {
  if (!request || request.startsWith('.') || request.startsWith('/')) return null;

  // Verify issuer: must be an absolute native path, not a file:// URL.
  if (typeof issuer !== 'string' || issuer.length === 0) {
    throw new Error('ISSUER_MISSING');
  }
  if (issuer.startsWith('file://')) {
    throw new Error('ISSUER_IS_URL_NOT_PATH: ' + issuer);
  }
  if (!path.isAbsolute(issuer)) {
    throw new Error('ISSUER_NOT_ABSOLUTE: ' + issuer);
  }

  if (request === 'test-dep') {
    const pkgDir = path.join(ROOT, 'packages', 'test-dep');
    return path.join(pkgDir, 'index.js');
  }
  return null;
}

module.exports = {
  VERSIONS: { std: 3 },
  topLevel: { name: 'pnp-issuer-test', reference: 'workspace:.' },
  resolveRequest,
  resolveVirtual: function () { return null; },
};
