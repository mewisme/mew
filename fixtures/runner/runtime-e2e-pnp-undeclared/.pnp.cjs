// PnP API mock that rejects undeclared dependencies.
// Only 'allowed-dep' is declared; anything else throws.
'use strict';

const path = require('path');

const PROJECT_ROOT = __dirname;
const ALLOWED = {
  'allowed-dep': path.join(PROJECT_ROOT, 'packages', 'allowed-dep', 'index.js'),
};

function resolveRequest(request, issuer, opts) {
  if (!request) return null;
  if (request.startsWith('.') || request.startsWith('/')) return null;

  if (ALLOWED[request]) {
    return ALLOWED[request];
  }
  // Undeclared dependency: throw a boundary error.
  const err = new Error(
    `Package "${request}" is not declared in the PnP map. ` +
    `Your project may have a dependency on "${request}" that is not ` +
    `listed in the PnP lockfile.`
  );
  err.code = 'ERR_PNP_UNDECLARED_DEPENDENCY';
  err.pnpCode = 'UNDECLARED_DEPENDENCY';
  throw err;
}

module.exports = {
  VERSIONS: { std: 3 },
  topLevel: { name: 'pnp-undeclared-test', reference: 'workspace:.' },
  resolveRequest,
  resolveVirtual: function () { return null; },
};
