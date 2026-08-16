// Entrypoint with static import to test PnP resolution.
import { writeFileSync } from 'node:fs';
import { name } from 'test-dep';

writeFileSync('output.txt', 'pnp-resolved:' + name + '\n');
