// PnP API for nested importer test.
'use strict';
const path = require('path');
const ROOT = __dirname;
const PKGS = {
  'inner-dep': path.join(ROOT, 'packages', 'inner-dep', 'index.js'),
};
function resolveRequest(request, issuer, opts) {
  if (!request || request.startsWith('.') || request.startsWith('/')) return null;
  // Verify issuer is a native path (not a file:// URL).
  if (issuer && typeof issuer === 'string' && issuer.startsWith('file://')) {
    throw new Error('ISSUER_IS_URL: ' + issuer);
  }
  if (PKGS[request]) return PKGS[request];
  return null;
}
module.exports = {
  VERSIONS: { std: 3 },
  topLevel: { name: 'pnp-nested-test', reference: 'workspace:.' },
  resolveRequest,
  resolveVirtual: function () { return null; },
};
