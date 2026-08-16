// Worker task: inspect workerData and report its type and value.
// Used by worker-data-*.mjs to verify workerData preservation.
import { parentPort, workerData } from 'node:worker_threads';

const result = {
  type: typeof workerData,
  isNull: workerData === null,
  isUndefined: workerData === undefined,
};

if (workerData === null) {
  result.value = 'null';
} else if (workerData === undefined) {
  result.value = 'undefined';
} else if (typeof workerData === 'object') {
  if (Array.isArray(workerData)) {
    result.value = JSON.stringify(workerData);
    result.isArray = true;
    result.length = workerData.length;
  } else {
    // Plain object: report keys and any Mew-injected properties.
    result.keys = Object.keys(workerData).sort();
    result.hasMewKeys = result.keys.some(k => k.includes('MEW_') || k.includes('_mew_'));
    result.isFrozen = Object.isFrozen(workerData);
    result.isSealed = Object.isSealed(workerData);
    result.isExtensible = Object.isExtensible(workerData);
    result.hasSymbolKeys = Object.getOwnPropertySymbols(workerData).length > 0;
    // Report known keys for specific assertions.
    if (workerData.name) result.name = workerData.name;
    if (workerData.count !== undefined) result.count = workerData.count;
  }
} else {
  // Primitive: report exact value.
  result.value = String(workerData);
}

parentPort.postMessage(JSON.stringify(result));
