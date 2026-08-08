// Entrypoint with static subpath import through PnP.
import { writeFileSync } from 'node:fs';
import { subpath } from 'test-lib/sub';

writeFileSync('output.txt', 'subpath:' + subpath + '\n');
