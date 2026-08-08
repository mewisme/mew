import { componentValue } from './component.jsx';
import { writeFileSync } from 'node:fs';
writeFileSync('output.txt', componentValue + '\n');
