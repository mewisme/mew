// Nested importer entrypoint with static PnP import.
import { writeFileSync } from 'node:fs';
import { name } from 'inner-dep';

writeFileSync('output.txt', 'nested:' + name + '\n');
