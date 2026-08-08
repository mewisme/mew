// PnP API for multi-project test — project A.
'use strict';
const path = require('path');
const ROOT = __dirname;
const PKGS = {
  'dep-a': path.join(ROOT, 'packages', 'dep-a', 'index.js'),
};
function resolveRequest(request, issuer, opts) {
  if (!request || request.startsWith('.') || request.startsWith('/')) return null;
  if (PKGS[request]) return PKGS[request];
  return null;
}
module.exports = {
  VERSIONS: { std: 3 },
  topLevel: { name: 'pnp-multi-a', reference: 'workspace:.' },
  resolveRequest,
  resolveVirtual: function () { return null; },
};
