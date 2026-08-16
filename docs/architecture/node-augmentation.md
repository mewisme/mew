# Node augmentation boundary

Mew runs the user's selected or installed Node binary and augments it through
supported extension surfaces. Mew does not fork Node, patch Node source, or
embed a private libnode.

## Compatibility test

> Could the same behavior be implemented with stock Node and a supported loader,
> preload, addon, environment, or command-line surface?

If not, surface an architecture decision before implementation.

## Allowed JavaScript surface

Embedded JavaScript is limited to Node extension APIs that cannot execute Go
directly:

- Loader hooks (`--import`, custom loaders)
- Preload modules (`--require` / `--import` preload)
- Worker-thread bootstrap and loader registration
- Small PnP / resolution helpers consumed by loaders

Do not grow a general-purpose JS application layer inside the repository.

## Embedding and integrity

1. Source assets live under `runtime/` (and are packaged via `internal/runtime/assets`).
2. Embed with `go:embed`.
3. On extraction to the user cache, verify a content digest.
4. Version the cache path so asset changes do not collide with stale extracts.

Digest verification and cache versioning are implemented with the runtime MVP
(0050). This document freezes the contract for later MVPs.

## Transform bridge

TypeScript/JSX transform for synchronous loader hooks may use a local IPC
service or an evaluated in-process path. Protocol sketch:
[transform-ipc-sketch.md](transform-ipc-sketch.md). Replacing Nub's OXC native
addon with a Go transform pipeline is an intentional divergence.
