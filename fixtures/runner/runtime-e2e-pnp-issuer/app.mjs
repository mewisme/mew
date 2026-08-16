// Issuer verification entrypoint with static PnP import.
import { writeFileSync } from 'node:fs';
import { name } from 'test-dep';

writeFileSync('output.txt', 'issuer-ok:' + name + '\n');
