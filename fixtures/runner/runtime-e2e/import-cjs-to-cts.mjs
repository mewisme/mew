// ESM import of .cjs specifier — resolve hook should map dep.cjs → dep.cts.
// The import is a side-effect: dep.cts's top-level code executes and writes output.
import './dep.cjs';
