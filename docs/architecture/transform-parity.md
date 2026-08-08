# Transform Parity Reference

Mew is a transpiler, not a type checker. This document records intentional
differences from the TypeScript compiler (`tsc`) and the current support
level for each transform feature.

## JSX

| Feature | Support | Notes |
|---|---|---|
| Classic runtime (`jsx: "react"`) | Full | `React.createElement` calls |
| Automatic runtime (`jsx: "react-jsx"`) | Full | `react/jsx-runtime` imports |
| Development mode (`jsx: "react-jsxdev"`) | Full | `jsx-dev-runtime` imports with `__self`/`__source` |
| Preserve (`jsx: "preserve"`) | Full | JSX left intact for downstream tool |
| Custom factory (`jsxFactory`) | Full | e.g. `h` for Preact classic |
| Custom fragment (`jsxFragmentFactory`) | Full | e.g. `Fragment` |
| Custom import source (`jsxImportSource`) | Full | e.g. `preact` for Preact automatic |
| `.tsx` / `.jsx` file extensions | `.tsx` supported; `.jsx` supported via extension substitution (`.jsx` → `.tsx` probe); direct `.jsx` entrypoints unsupported | |

### JSX default importSource

When `jsx: "react-jsx"` is set without `jsxImportSource`, Mew defaults to
`react` (same as esbuild and tsc). Explicit `jsxImportSource` overrides.

## Decorators

| Feature | Support | Notes |
|---|---|---|
| Legacy TypeScript decorators | Full | Via esbuild; enabled by tsconfig `experimentalDecorators`; uses `__decorateClass` helper |
| TC39 standard decorators | Full | Via esbuild; default when `experimentalDecorators` is not set; uses `__decorateElement` helper |
| `emitDecoratorMetadata` | Unsupported | Fails with `ERR_M_TRANSFORM_UNSUPPORTED` diagnostic; metadata emission requires type-checker information unavailable to a transpiler |

### Decorator mode selection

Mew passes `experimentalDecorators` from tsconfig through `TsconfigRaw` to
esbuild, which selects the decorator transform mode:

- `experimentalDecorators: true` → legacy TypeScript decorators (`__decorateClass`)
- `experimentalDecorators` absent or `false` → TC39 standard decorators (`__decorateElement`)

Both modes are included in cache keys via `NormalizedOptions`.

### Decorator metadata strategy

`emitDecoratorMetadata` is explicitly rejected during tsconfig normalization.
Mew is a transpiler, not a type checker; it lacks the type information
required to emit `design:type`, `design:paramtypes`, and `design:returntype`
metadata. Setting `emitDecoratorMetadata: true` produces an actionable
diagnostic rather than silently producing incomplete output.

## Source Maps

| Feature | Support | Notes |
|---|---|---|
| No source map (`sourceMap: false`) | Default | Engine defaults to no map when no mode requested and no tsconfig flags are set |
| Inline source maps (`inlineSourceMap: true`) | Full | `sourceMappingURL` data URL in output |
| External source maps (`sourceMap: true`) | Full (protocol) | Separate `.map` in TransformResult; engine supports external, loader requests inline by default |
| Source content inclusion (`inlineSources`) | Full | Tri-state `*bool`: nil/absent → include (tsc default), `true` → include, `false` → exclude |
| Source root (`sourceRoot`) | Full | Passed through to esbuild `sourceRoot` field |
| Map root (`mapRoot`) | Carried | In NormalizedOptions for cache keys; esbuild does not use mapRoot directly; informational only |
| Stack trace mapping | Via Node `--enable-source-maps` | Automatically injected when Node >= 20.6; maps original sources in stack traces |

### Source map mode precedence

The effective source map mode is determined by combining the request-level
mode with tsconfig options:

1. Request `SourceMapMode` sets the baseline (none/inline/external)
2. tsconfig `sourceMap: true` upgrades `none` to `external`
3. tsconfig `inlineSourceMap: true` upgrades `none` to `inline` (but
   does not override an explicit `external` request)

The Node loader (ts-loader.mjs) requests `inline` by default, so the
runtime path produces inline source maps for every transformed module.

### Inline sources tri-state

`inlineSources` uses `*bool` to distinguish three states:

- **Absent** (nil): default behavior — sources are included in the map
  (matching tsc's default)
- **Explicit true**: sources are included
- **Explicit false**: sources are excluded

Absent and explicit `true` may produce different cache keys (they
serialize differently) but identical transform output. This is
conservative for cache safety — no collision risk.

### Stack trace mapping

Mew automatically passes `--enable-source-maps` to Node when the Node
installation supports it (>= 20.6). Node's `--enable-source-maps` flag
reads `sourceMappingURL` directives from module source and resolves
`.map` files for error stack traces. When source maps are present
(determined by tsconfig `sourceMap: true` or `inlineSourceMap: true`),
error stack traces show original TypeScript source file names and line
numbers rather than transformed JavaScript locations.

## Module Format

| tsconfig `module` | Mew behavior |
|---|---|
| `CommonJS` | `FormatCJS` |
| `ES6`, `ES2015`, `ES2020`, `ES2022`, `ESNext`, `NodeNext`, `Node16` | `FormatESM` |
| `Preserve` | Keeps the loader-inferred format |
| Unset | Inferred from file extension (`.mts`/`.mjs` → ESM, `.cts`/`.cjs` → CJS) |

## Target

| tsconfig `target` | esbuild target |
|---|---|
| `ES3` | `ES5` (esbuild minimum) |
| `ES5` | `ES5` |
| `ES2015` / `ES6` | `ES2015` |
| `ES2016` – `ES2024` | Matched exactly |
| `ESNext` | `ESNext` |
| Unset | Node major version heuristic (≥22 → ESNext, ≥20 → ES2023, ≥18 → ES2022, else ES2020) |

## Differences from TypeScript compiler (tsc)

1. **No type checking.** Mew strips types and emits JavaScript. Use `tsc --noEmit`
   for type checking in CI.

2. **No const enum inlining.** esbuild does not inline const enums from other
   modules. Use regular enums or inline constants.

3. **No declaration files.** `.d.ts` generation requires `tsc`.

4. **Path alias resolution is deterministic.** `baseUrl` and `paths` are
   carried for cache key stability and resolved at runtime by the Node
   loader (0053 resolve hook). The transform engine itself does not
   consume them. Path patterns are sorted by TypeScript specificity
   (exact matches first, then longest prefix before `*`, then shortest
   suffix) before serialization to the loader. Target arrays preserve
   declaration order. Node fallback errors are preserved rather than
   swallowed. Repeated runs produce identical resolution order
   regardless of Go map iteration.

5. **Decorator mode controlled by tsconfig.** `experimentalDecorators: true`
   selects legacy decorator helpers (`__decorateClass`); absent selects
   TC39 standard helpers (`__decorateElement`). Decorators are always
   transpiled when present regardless of the flag.

6. **No `emitDecoratorMetadata` emission.** Setting `emitDecoratorMetadata: true`
   fails with an `ERR_M_TRANSFORM_UNSUPPORTED` diagnostic. Mew is a transpiler
   and cannot emit `design:*` metadata without type-checker information.

7. **Source map content controlled by `inlineSources`.** The tri-state
   `*bool` option distinguishes absent (default include) from explicit
   false (exclude). esbuild does not have a direct `inlineSources` flag;
   instead `SourcesContent` is set to `Include` (absent/true) or
   `Exclude` (false).

8. **`mapRoot` is carried but not applied.** esbuild has no `mapRoot`
   option. The value is parsed from tsconfig, included in cache keys,
   and available programmatically, but does not affect emitted maps.

9. **Extension substitution resolves .js/.jsx/.mjs/.cjs to TypeScript.**
   When a JavaScript-specifier import resolves to a file that does not
   exist on disk, the Node loader probes for a corresponding TypeScript
   file before failing. The substitution matrix is:
   `.js` → `.ts` then `.tsx`, `.jsx` → `.tsx`,
   `.mjs` → `.mts`, `.cjs` → `.cts`.
   Candidate generation is deterministic; existing JavaScript files
   take precedence over substituted TypeScript files. Node fallback
   errors are preserved when no candidate exists. Directories and
   bare package specifiers are not rewritten. Extension substitution
   does not determine CJS/ESM module format (that is a separate concern).
   The loader is active for all entrypoints when runtime augmentation
   is enabled, ensuring `.js`→`.ts` mapping works even from JavaScript
   entrypoints that import TypeScript modules.

## Cache key coverage

Every field on NormalizedOptions flows into the transform cache key via
canonical JSON serialization. Cache keys change when:
- JSX mode, factory, fragment factory, or import source changes
- Decorator flags change
- Source map options change
- Target or module format changes
- Path aliases change

## Diagnostic code frames

Transform errors and warnings include file path, line, column, length, and
source snippet (line text) from esbuild. These point to the original source,
not the emitted JavaScript.

## Unsupported tsconfig options

The `UnsupportedOptions` function in `internal/transform/tsconfig.go`
identifies compilerOptions keys that Mew does not recognize. Currently
recognized keys (derived from the capability registry):

`target`, `module`, `useDefineForClassFields`, `verbatimModuleSyntax`,
`importHelpers`, `baseUrl`, `paths`, `jsx`, `jsxFactory`,
`jsxFragmentFactory`, `jsxImportSource`, `sourceMap`, `inlineSourceMap`,
`inlineSources`, `sourceRoot`, `mapRoot`, `experimentalDecorators`,
`emitDecoratorMetadata`

Unrecognized compilerOptions keys are classified as unsupported and available
programmatically via `UnsupportedOptions()`.

Options that cannot be honored produce stable typed diagnostics:
- `emitDecoratorMetadata: true` → `ERR_M_TRANSFORM_UNSUPPORTED` (requires type-checker information)
- `importHelpers: true` → `ERR_M_TRANSFORM_UNSUPPORTED` (esbuild always inlines helpers; no tslib integration)

The canonical capability report is available via `m transform report` (JSON
and human-readable table). See also `m transform report --help`.
