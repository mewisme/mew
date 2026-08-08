// Entrypoint that imports an undeclared dependency through PnP.
// The PnP API should throw an UNDECLARED_DEPENDENCY error.
import { writeFileSync } from 'node:fs';
import { name } from 'undeclared-dep';

// Should not reach here — PnP throws during resolution.
writeFileSync('output.txt', 'unexpected:success\n');
