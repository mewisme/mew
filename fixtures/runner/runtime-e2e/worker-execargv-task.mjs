// Worker task: report process.execArgv for execArgv preservation test.
import { parentPort } from 'node:worker_threads';

// Report whether custom flags are present and whether Mew bootstrap is present.
const execArgv = process.execArgv;
const hasCustomVM = execArgv.some(a => a === '--experimental-vm-modules');
const hasCustomNoWarnings = execArgv.some(a => a === '--no-warnings');
const hasMewRequire = execArgv.some(a => a.includes('credential-grabber'));

const result = {
  hasCustomVM,
  hasCustomNoWarnings,
  hasMewRequire,
  // Custom flag must appear after Mew bootstrap (preserving user order).
  customFlagsAfterMew: (() => {
    let mewIdx = -1;
    let noWarnIdx = -1;
    for (let i = 0; i < execArgv.length; i++) {
      if (execArgv[i].includes('credential-grabber')) mewIdx = i;
      if (execArgv[i] === '--no-warnings') noWarnIdx = i;
    }
    return noWarnIdx > mewIdx;
  })(),
};
parentPort.postMessage(JSON.stringify(result));
