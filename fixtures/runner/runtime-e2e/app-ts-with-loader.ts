// TypeScript entrypoint used with a custom loader.
// Proves TS transform and custom loader hooks coexist.
import { writeFileSync } from 'node:fs';

function greet(name: string): string {
  return `Hello, ${name}`;
}

// Write to a fixed path relative to cwd (which is the project dir).
writeFileSync('output.txt', 'ts-entrypoint:' + greet('world') + '\n', { flag: 'a' });
