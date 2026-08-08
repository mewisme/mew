import { libValue } from './lib.js';
import { writeFileSync } from 'node:fs';
writeFileSync('output.txt', libValue + '\n');
