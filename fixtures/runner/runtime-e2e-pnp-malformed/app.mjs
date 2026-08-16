// Entrypoint for malformed PnP project.
// .pnp.cjs throws on load, so PnP is unavailable.
// The project should still run with Node resolution.
import { writeFileSync } from 'node:fs';
writeFileSync('output.txt', 'malformed-works\n');
