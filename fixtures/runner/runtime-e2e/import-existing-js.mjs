import { realValue } from './real.js';
import { writeFileSync } from 'node:fs';
writeFileSync('output.txt', realValue + '\n');
