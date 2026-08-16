// Project B entrypoint — static import of dep-b through PnP.
import { writeFileSync } from 'node:fs';
import { name } from 'dep-b';

writeFileSync('output.txt', 'project-b:' + name + '\n');
