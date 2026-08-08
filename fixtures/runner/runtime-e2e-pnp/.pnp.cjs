// Minimal PnP API mock for integration testing.
// Resolves known test packages to real filesystem paths inside the fixture.
'use strict';

const path = require('path');
const fs = require('fs');

const PROJECT_ROOT = __dirname;

// Map package names to their real fixture paths.
const PACKAGE_MAP = {
  'test-dep': path.join(PROJECT_ROOT, 'packages', 'test-dep'),
};

const VERSIONS = { std: 3, resolveVirtual: 1, getAllLocators: 1 };

function topLevel() {
  return { name: 'pnp-test-project', reference: 'workspace:.' };
}

function getLocator(name, reference) {
  return { name, reference };
}

function getDependencyTreeRoots() {
  return [{ name: 'pnp-test-project', reference: 'workspace:.' }];
}

function getPackageInformation(locator) {
  if (!locator || !locator.name) return null;
  const pkgDir = PACKAGE_MAP[locator.name];
  if (!pkgDir) return null;
  return {
    packageLocation: pkgDir,
    packageDependencies: new Map(),
  };
}

function findPackageLocator(location) {
  for (const [name, dir] of Object.entries(PACKAGE_MAP)) {
    if (location === dir || location.startsWith(dir + path.sep)) {
      return { name, reference: 'workspace:.' };
    }
  }
  return null;
}

function resolveToUnqualified(request, issuer, opts) {
  // Bare specifier: look up in package map.
  const slashIdx = request.indexOf('/');
  const pkgName = slashIdx >= 0 ? request.slice(0, slashIdx) : request;
  if (!PACKAGE_MAP[pkgName]) return null;
  return path.join(PACKAGE_MAP[pkgName], 'index.js');
}

function resolveUnqualified(unqualified, opts) {
  return unqualified;
}

// Primary resolution entry point called by ts-loader.
function resolveRequest(request, issuer, opts) {
  if (typeof issuer !== 'string') {
    // issuer must be a native path; reject URL strings early
    throw new Error('PnP resolveRequest: issuer must be a filesystem path');
  }
  if (!request) return null;
  if (request.startsWith('.') || request.startsWith('/')) return null;

  const slashIdx = request.indexOf('/');
  const pkgName = slashIdx >= 0 ? request.slice(0, slashIdx) : request;
  const subpath = slashIdx >= 0 ? request.slice(slashIdx + 1) : null;

  const pkgDir = PACKAGE_MAP[pkgName];
  if (!pkgDir) return null;

  let resolved;
  if (subpath) {
    resolved = path.join(pkgDir, subpath);
    // append .js if no extension
    if (!path.extname(resolved)) resolved += '.js';
  } else {
    resolved = path.join(pkgDir, 'index.js');
  }
  if (fs.existsSync(resolved)) return resolved;
  return null;
}

function resolveVirtual(request) {
  return null;
}

module.exports = {
  VERSIONS,
  topLevel,
  getLocator,
  getDependencyTreeRoots,
  getPackageInformation,
  findPackageLocator,
  resolveToUnqualified,
  resolveUnqualified,
  resolveRequest,
  resolveVirtual,
};
