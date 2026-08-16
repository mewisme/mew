// Entrypoint for .pnp.data.json-only project.
// Without .pnp.cjs, PnP resolution is unavailable.
// Bare imports fall through to Node resolution.
import { writeFileSync } from 'node:fs';

// Use a relative import (always works) to prove the project runs.
writeFileSync('output.txt', 'datajson-works\n');
