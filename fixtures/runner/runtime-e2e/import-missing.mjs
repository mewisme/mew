// This import should fail: neither missing.js nor missing.ts exist.
import { missing } from './missing.js';
import { writeFileSync } from 'node:fs';
writeFileSync('output.txt', missing + '\n');
