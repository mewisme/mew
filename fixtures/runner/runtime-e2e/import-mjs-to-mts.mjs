import { modValue } from './mod.mjs';
import { writeFileSync } from 'node:fs';
writeFileSync('output.txt', modValue + '\n');
