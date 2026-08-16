// PnP API for multi-project test — project B.
'use strict';
const path = require('path');
const ROOT = __dirname;
const PKGS = {
  'dep-b': path.join(ROOT, 'packages', 'dep-b', 'index.js'),
};
function resolveRequest(request, issuer, opts) {
  if (!request || request.startsWith('.') || request.startsWith('/')) return null;
  if (PKGS[request]) return PKGS[request];
  return null;
}
module.exports = {
  VERSIONS: { std: 3 },
  topLevel: { name: 'pnp-multi-b', reference: 'workspace:.' },
  resolveRequest,
  resolveVirtual: function () { return null; },
};
