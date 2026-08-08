// Project A entrypoint — static import of dep-a through PnP.
import { writeFileSync } from 'node:fs';
import { name } from 'dep-a';

writeFileSync('output.txt', 'project-a:' + name + '\n');
