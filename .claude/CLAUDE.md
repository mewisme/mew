# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project identity

Module: `github.com/mewisme/mew` — Go 1.26.5+. Product: **MewJS** (short: **Mew**). Binaries: `m` (primary, alias `mew`), `mx` (package runner, alias `mewx`). Lockfile: `m.lock` (native).

Authoritative architecture: [`docs/architecture/package-map.md`](docs/architecture/package-map.md). Key docs: `docs/charter.md`, `docs/engineering.md`, `docs/errors.md`, `docs/testing.md`, `docs/architecture/forbidden-imports.md`.

## Architecture

Four-layer dependency direction. Presentation must not own domain logic. Domain resolves a complete immutable graph before any mutation.

| Layer | Packages | Owns |
|---|---|---|
| Entry | `cmd/m`, `cmd/mx` | Process exit codes only. May only import `internal/app`, `internal/cli`, stdlib. |
| Presentation | `internal/cli`, `internal/app` | Parsing, dispatch, orchestration, user output |
| Domain | `manifest`, `project`, `workspace`, `registry`, `resolver`, `lockfile`, `graph`, `plan`, `snapshot`, `capsule`, `policy` | Read/plan models. Free of network/mutate. |
| Mutation | `fetch`, `archive`, `store`, `linker`, `transaction` | Staged filesystem changes |
| Runtime | `runner`, `process`, `runtime`, `transform`, `node` | Execution and Node launch. Free of PM engine. |

Key import rules enforced by `internal/archcheck`:
- `internal/resolver` must not import `linker`, `transaction`, `fetch`, `store` (resolve-complete-before-mutate)
- Domain packages must not import `internal/presentation` or Charm (`charm.land/*`, `github.com/charmbracelet/*`)
- `internal/config`, `internal/project` stay free of mutate path
- `internal/graph`, `plan`, `snapshot`, `manifest`, `policy`, `capsule` stay free of network/mutate

Full rules: [`docs/architecture/forbidden-imports.md`](docs/architecture/forbidden-imports.md).

### Transaction boundary

Every install-family mutation follows this pipeline. Previous manifest, lockfile, and `node_modules` remain usable until commit. On failure before `committed`, rollback restores pre-mutation state.

```
inspect → resolve → plan → fetch → verify → stage → validate → plan journal
  → backup → commit (all live publishes) → post-commit cleanup
  ↘ rollback on failure (before committed)
```

Single mutation entrypoint: `BeginMutation` acquires project lock at `.mew/txn/lock`, runs idempotent recovery, refuses to begin when incomplete journals remain.

Full docs: [`docs/architecture/transaction-boundary.md`](docs/architecture/transaction-boundary.md).

## Build, test, lint

Prefer `make` targets. `make help` lists them all.

Test execution uses `tools/testexec`, the canonical adaptive test orchestrator.
Override worker count: `make test TESTEXEC_WORKERS=1` (serial) or `TESTEXEC_WORKERS=4`.
Direct use: `go run ./tools/testexec [-workers N] [-short] [-race] [-tags TAGS] [packages...]`.

```sh
# Build
make build

# Test
make test              # full suite (adaptive parallel)
make test-short        # fast suite (skips soak)
make test-unit         # unit tests only
make test-integration  # integration tests (process-level sharding)
make test-e2e          # runtime E2E + Node version tests
make test-crash        # crash recovery suite (build tag: crash)
make test-race         # race detector (requires CGO)

# Single package
go test ./internal/resolver/... -count=1
go test ./internal/resolver/... -run TestIncrementalDiff -count=1

# Quality
make quality           # all gates: fmt, generate, vet, lint, diff, arch, docs, allowlist
make vet
make lint

# CI
make ci                # mirror normal PR CI locally

# Generation
make generate          # run all generators
make generate-check    # fail if generated files are stale
make plans             # regenerate plans and checklists
make assets            # regenerate runtime asset manifest

# Other
make fuzz-smoke        # smoke-test fuzz targets
make conformance       # lock bridge conformance suite
make vuln              # vulnerability scan
```

## Repository tools

[`TOOLS.md`](TOOLS.md) is the canonical inventory of every repository-local tool. Read it before creating scripts, changing build/test/generation tooling, or claiming no existing command covers a task.

Testing quick reference: [`TESTING.md`](TESTING.md). Test strategy and architecture: [`docs/testing.md`](docs/testing.md).

Prefer, in order:
1. Documented Make targets (`make help` lists them all)
2. Documented canonical Python or shell tools
3. Repository-native Go commands
4. Direct low-level command sequences only when no canonical tool exists

Do not reimplement behavior a documented tool already provides. Do not bypass repository wrappers when they add validation or deterministic output. Any change to a repository tool must update [`TOOLS.md`](TOOLS.md) in the same change.

### Scripts and generated content

When a file is produced by a documented script or generator (checklist, manifest, plan enrichment, runtime asset manifest, generated fixtures, conformance outputs), run the script to produce the update — do not hand-edit the generated output. If the script's output is wrong, update the script or its input sources, then regenerate. Never manually patch a generated file while leaving the generator out of sync.

Indicators a file is generated: a `Regenerate:` or `Generated by` comment near the top, a `make` target that writes it, or a companion script in `tools/` or `plans/scripts/`.

## Conventions

### Error codes

Pattern: `ERR_M_<DOMAIN>_<DETAIL>`. Every public failure returns `*apperr.Error` (or wraps into one at CLI boundary). Package: `internal/apperr` (`New`, `Wrap`, `CodeOf`, `ExitCode`). Stable codes: `ERR_M_USAGE` (2), `ERR_M_CANCELLED` (130), `ERR_M_TRANSACTION`, `ERR_M_INTEGRITY`, `ERR_M_RESOLVE`, `ERR_M_LOCKFILE`, `ERR_M_STORE`, `ERR_M_POLICY`, etc. Unknown codes → exit 1. Full registry: [`docs/errors.md`](docs/errors.md).

### Engineering

- **Hermetic tests only.** Never hit public npm. Fixture registry at `fixtures/registry/v1/` with SHA-256 manifest. `testkit.CleanEnv`/`TempHome` isolates home dirs.
- **errcheck** on all resource cleanup. `defer func() { _ = f.Close() }()`. `fmt.Print*` excluded.
- **Dependency allowlist** at `tools/allowlist/modules.txt` — update in same PR as new deps. Prefer stdlib.
- **Tool versions** pinned in `tools/versions.env`. No floating `latest` in CI.
- **Experimental features** behind `MEW_EXPERIMENTAL_<NAME>=1` or `--experimental-<name>` flags.
- **Crash tests** use `crash` build tag, excluded from default `go test ./...`.
- **CGO_ENABLED=0** for production builds. Race tests are the only CGO exception.
- **Fixtures** are source-of-truth. Never mutate checked-in fixtures from tests — copy via `testkit.CopyFixture`.

### Line endings

`.gitattributes` sets `* text eol=lf`. Every file created or rewritten must use **LF** (`\n`), never **CRLF** (`\r\n`). Verify with `git diff --check` before commit.

### Commit messages

Follow **Conventional Commits**: `type(scope): description`. Common types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`. Title: imperative mood, ≤72 chars, no trailing period. Body: blank line after title, bullet points (`-`), wrap at 72 chars. Focus on **what** changed and **why**. No Markdown in commit messages.

## Tool preferences

### Serena

Prefer Serena for symbol lookup, cross-file relationships, refactoring, and replacing whole function/class/method bodies. Prefer built-in tools for small known-file edits, config/docs/JSON/YAML/Markdown, plain text searches, shell commands, Git, builds, and tests.

## Output style

Output is shaped for an ADHD brain: lead with the next action, number multi-step tasks, suppress tangents, no preamble/recap/pleasantries. State explicitly, never hedge. Show completed work concretely. Cap lists at 5. Matter-of-fact tone for errors: state cause and fix, never "Uh oh."

Lazy senior dev mindset: stop at the first rung that holds — does this need to exist? Already in the codebase? Stdlib covers it? Native platform feature? Already-installed dependency? Can it be one line? Only then write minimum code. No unrequested abstractions, no scaffolding "for later." Deletion over addition.
