#!/usr/bin/env python3
"""Generate/check internal/runtime/assets/manifest.json from asset files.

Canonical implementation. No platform-specific logic — wrappers handle
Python discovery when needed.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_ASSETS_DIR = ROOT / "internal" / "runtime" / "assets"
DEFAULT_MANIFEST = DEFAULT_ASSETS_DIR / "manifest.json"

# Files in the assets directory that are NOT runtime assets.
EXCLUDE_NAMES = {"manifest.json"}

# Only files with these extensions are runtime assets (JS modules injected
# into Node).  Go sources, docs, and config files live alongside them
# because of go:embed's directory constraint but are not assets.
VALID_ASSET_EXTENSIONS = frozenset({".js", ".mjs", ".cjs"})

# Filesystem entries to skip during scan.
SKIP_PREFIXES = (".", "~")
SKIP_SUFFIXES = (".tmp", ".swp", ".bak", "~")

# Valid values per the Go validator.
VALID_MODULE_TYPES = frozenset({"cjs", "esm"})
VALID_ROLES = frozenset({
    "preload-cjs",
    "preload-esm",
    "loader-registration",
    "loader-support",
    "credential-grabber",
})

# Exit codes.
EXIT_OK = 0
EXIT_STALE = 1
EXIT_USAGE = 2
EXIT_INVALID_MANIFEST = 3
EXIT_SCAN = 4
EXIT_WRITE = 5


# ── helpers ──────────────────────────────────────────────────────────

def _ext_to_module_type(suffix: str) -> str:
    """Map file suffix to manifest moduleType."""
    return "cjs" if suffix == ".cjs" else "esm"


def _default_role(module_type: str, name: str = "") -> str:
    """Default role for a new asset based on moduleType and filename conventions."""
    # Support modules are not injected into Node argv; they are loaded on demand.
    if name.startswith("resolve-"):
        return "loader-support"
    return "preload-cjs" if module_type == "cjs" else "preload-esm"


def _normalize_path(rel: str) -> str:
    """Normalize path to forward-slash, repo-relative form."""
    return rel.replace("\\", "/")


# ── scan ─────────────────────────────────────────────────────────────

def scan_assets(assets_dir: Path) -> dict[str, Path]:
    """Return {normalized_rel_path: absolute_path} for every asset file."""
    result: dict[str, Path] = {}
    for dirpath, _, filenames in os.walk(assets_dir):
        for name in filenames:
            if name in EXCLUDE_NAMES:
                continue
            if name.startswith(SKIP_PREFIXES):
                continue
            if name.endswith(SKIP_SUFFIXES):
                continue
            suffix = Path(name).suffix
            if suffix not in VALID_ASSET_EXTENSIONS:
                continue
            abs_path = Path(dirpath) / name
            if abs_path.is_symlink():
                continue
            rel = _normalize_path(str(abs_path.relative_to(assets_dir)))
            result[rel] = abs_path
    return result


# ── hash ─────────────────────────────────────────────────────────────

def hash_file(file_path: Path) -> tuple[int, str]:
    """Return (byte_size, hex_sha256) for a file."""
    h = hashlib.sha256()
    size = 0
    with open(file_path, "rb") as f:
        while True:
            chunk = f.read(1 << 16)  # 64 KiB
            if not chunk:
                break
            h.update(chunk)
            size += len(chunk)
    return size, h.hexdigest()


# ── manifest read / validate ─────────────────────────────────────────

def read_manifest(path: Path) -> dict:
    """Read and parse manifest.json. Exits on malformed JSON."""
    try:
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except json.JSONDecodeError as exc:
        print(f"Error: {path}: malformed JSON: {exc}", file=sys.stderr)
        raise SystemExit(EXIT_INVALID_MANIFEST)
    except OSError as exc:
        print(f"Error: cannot read {path}: {exc}", file=sys.stderr)
        raise SystemExit(EXIT_INVALID_MANIFEST)
    if not isinstance(data, dict):
        print(f"Error: {path}: manifest must be a JSON object", file=sys.stderr)
        raise SystemExit(EXIT_INVALID_MANIFEST)
    return data


def validate_manifest(data: dict) -> None:
    """Validate manifest structure. Exits on violation."""
    if "schemaVersion" not in data:
        print("Error: manifest missing schemaVersion", file=sys.stderr)
        raise SystemExit(EXIT_INVALID_MANIFEST)
    sv = data["schemaVersion"]
    if sv not in (1, 2):
        print(f"Error: unsupported schemaVersion {sv}", file=sys.stderr)
        raise SystemExit(EXIT_INVALID_MANIFEST)

    if not data.get("bundleVersion"):
        print("Error: manifest missing bundleVersion", file=sys.stderr)
        raise SystemExit(EXIT_INVALID_MANIFEST)

    assets = data.get("assets")
    if not isinstance(assets, list):
        print("Error: manifest assets must be an array", file=sys.stderr)
        raise SystemExit(EXIT_INVALID_MANIFEST)

    seen_names: set[str] = set()
    seen_paths: set[str] = set()

    for i, entry in enumerate(assets):
        if not isinstance(entry, dict):
            print(f"Error: assets[{i}] is not an object", file=sys.stderr)
            raise SystemExit(EXIT_INVALID_MANIFEST)

        name = entry.get("name", "")
        path = entry.get("path", "")

        if name in seen_names:
            print(f"Error: duplicate asset name {name!r}", file=sys.stderr)
            raise SystemExit(EXIT_INVALID_MANIFEST)
        seen_names.add(name)

        if path in seen_paths:
            print(f"Error: duplicate asset path {path!r}", file=sys.stderr)
            raise SystemExit(EXIT_INVALID_MANIFEST)
        seen_paths.add(path)

        # Path traversal check.
        if ".." in path or os.path.isabs(path):
            print(f"Error: asset path {path!r} must be relative, no ..", file=sys.stderr)
            raise SystemExit(EXIT_INVALID_MANIFEST)

        # SHA-256 format (may be missing for new entries that need regeneration).
        sha = entry.get("sha256", "")
        if sha and (len(sha) != 64 or not all(c in "0123456789abcdefABCDEF" for c in sha)):
            print(f"Error: {name!r}: invalid SHA-256 digest {sha!r}", file=sys.stderr)
            raise SystemExit(EXIT_INVALID_MANIFEST)

        # Module type.
        mt = entry.get("moduleType", "")
        if mt not in VALID_MODULE_TYPES:
            print(f"Error: {name!r}: unknown moduleType {mt!r}", file=sys.stderr)
            raise SystemExit(EXIT_INVALID_MANIFEST)

        # Role (optional in schema, but validate if present).
        role = entry.get("role", "")
        if role and role not in VALID_ROLES:
            print(f"Error: {name!r}: unknown role {role!r}", file=sys.stderr)
            raise SystemExit(EXIT_INVALID_MANIFEST)


# ── case-collision detection ─────────────────────────────────────────

def check_case_collisions(paths: list[str]) -> None:
    """Detect names that collide on case-insensitive filesystems."""
    lower: dict[str, str] = {}
    for p in paths:
        key = p.lower()
        if key in lower:
            print(
                f"Error: case collision: {lower[key]!r} and {p!r}",
                file=sys.stderr,
            )
        lower[key] = p
    if len(lower) != len(paths):
        raise SystemExit(EXIT_INVALID_MANIFEST)


# ── generation ───────────────────────────────────────────────────────

def generate_manifest(
    existing_data: dict,
    scanned: dict[str, Path],
    assets_dir: Path,
    verbose: bool = False,
) -> dict:
    """Generate updated manifest merging scanned files with existing metadata.

    - Updates size/sha256 for every asset that exists on disk.
    - Adds entries for newly discovered files.
    - Removes entries for deleted files.
    - Preserves all manual fields (name, path, role, moduleType, schemaVersion,
      bundleVersion).
    """
    new: dict = {
        "schemaVersion": existing_data.get("schemaVersion", 2),
        "bundleVersion": existing_data.get("bundleVersion", ""),
        "assets": [],
    }

    existing_by_path: dict[str, dict] = {}
    for entry in existing_data.get("assets", []):
        existing_by_path[entry.get("path", "")] = entry

    current_paths = set(scanned.keys())
    manifest_paths = set(existing_by_path.keys())

    # Sort entries by name for deterministic output.
    # First build a combined set: existing names + new names (from scanned paths).
    all_paths = sorted(current_paths | manifest_paths)

    for path in all_paths:
        entry: dict

        if path in scanned:
            file_path = scanned[path]
            try:
                size, sha256_digest = hash_file(file_path)
            except OSError as exc:
                print(f"Error: cannot hash {file_path}: {exc}", file=sys.stderr)
                raise SystemExit(EXIT_SCAN)

            if path in existing_by_path:
                # Update existing entry.
                entry = dict(existing_by_path[path])
                old_size = entry.get("size")
                old_sha = entry.get("sha256")
                entry["size"] = size
                entry["sha256"] = sha256_digest

                if verbose and (old_size != size or old_sha != sha256_digest):
                    print(f"changed: {path} (size {old_size}→{size})")
            else:
                # New asset — auto-detect what we can.
                suffix = Path(path).suffix
                module_type = _ext_to_module_type(suffix)
                role = _default_role(module_type, name=Path(path).name)
                entry = {
                    "name": Path(path).name,
                    "path": path,
                    "role": role,
                    "moduleType": module_type,
                    "size": size,
                    "sha256": sha256_digest,
                }
                if verbose:
                    print(f"added: {path} (role={role})")
        else:
            # Stale entry — file deleted.
            if verbose:
                print(f"removed: {path}")
            continue  # Skip this entry.

        new["assets"].append(entry)

    # Deterministic sort by name.
    new["assets"].sort(key=lambda e: e.get("name", ""))

    # Check for metadata issues.
    paths_list = [e["path"] for e in new["assets"]]
    check_case_collisions(paths_list)

    return new


# ── JSON formatting ──────────────────────────────────────────────────

def format_manifest(data: dict) -> str:
    """Format manifest deterministically with stdlib json.

    Matches the repository convention: 2-space indent, trailing newline.
    Key order is preserved from dict insertion order (Python 3.7+).
    """
    text = json.dumps(data, indent=2, ensure_ascii=False)
    return text + "\n"


# ── atomic write ─────────────────────────────────────────────────────

def write_manifest(data: dict, manifest_path: Path) -> None:
    """Atomically write manifest via temp file + rename."""
    text = format_manifest(data)
    tmp_path = None
    try:
        # Write to temp file in the same directory (same filesystem → atomic rename).
        fd, tmp_path = tempfile.mkstemp(
            dir=str(manifest_path.parent),
            prefix=".manifest-",
            suffix=".tmp",
        )
        try:
            os.write(fd, text.encode("utf-8"))
        finally:
            os.close(fd)
        os.replace(tmp_path, str(manifest_path))
        tmp_path = None  # Successfully renamed.
    except OSError as exc:
        print(f"Error: cannot write {manifest_path}: {exc}", file=sys.stderr)
        raise SystemExit(EXIT_WRITE)
    finally:
        if tmp_path is not None:
            try:
                os.remove(tmp_path)
            except OSError:
                pass


# ── CLI ──────────────────────────────────────────────────────────────

def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description="Generate/check runtime asset manifest from asset files.",
    )
    mode = p.add_mutually_exclusive_group()
    mode.add_argument(
        "--write",
        action="store_true",
        default=False,
        help="Regenerate and atomically update the manifest.",
    )
    mode.add_argument(
        "--check",
        action="store_true",
        default=False,
        help="Regenerate in memory and fail when the checked-in manifest is stale.",
    )
    p.add_argument(
        "--manifest",
        type=Path,
        default=None,
        help="Path to manifest.json (default: internal/runtime/assets/manifest.json).",
    )
    p.add_argument(
        "--assets-dir",
        type=Path,
        default=None,
        help="Path to assets directory (default: internal/runtime/assets).",
    )
    p.add_argument(
        "--verbose",
        action="store_true",
        default=False,
        help="Print changed, added, and removed entries.",
    )
    return p


def main() -> None:
    args = build_parser().parse_args()

    # Default: if neither --write nor --check, default to --check (safer).
    write_mode: bool = args.write
    check_mode: bool = args.check or (not args.write)

    manifest_path: Path = args.manifest if args.manifest else DEFAULT_MANIFEST
    assets_dir: Path = args.assets_dir if args.assets_dir else DEFAULT_ASSETS_DIR

    if not assets_dir.is_dir():
        print(f"Error: assets directory not found: {assets_dir}", file=sys.stderr)
        raise SystemExit(EXIT_USAGE)

    # Read existing manifest.
    existing = read_manifest(manifest_path)
    validate_manifest(existing)

    # Scan assets on disk.
    try:
        scanned = scan_assets(assets_dir)
    except OSError as exc:
        print(f"Error: cannot scan {assets_dir}: {exc}", file=sys.stderr)
        raise SystemExit(EXIT_SCAN)

    # Generate updated manifest.
    updated = generate_manifest(existing, scanned, assets_dir, verbose=args.verbose)

    if check_mode:
        existing_text = format_manifest(existing)
        updated_text = format_manifest(updated)
        if existing_text == updated_text:
            if args.verbose:
                print("Manifest is up to date.")
            raise SystemExit(EXIT_OK)
        else:
            print("Error: manifest is stale. Run with --write to update.", file=sys.stderr)
            raise SystemExit(EXIT_STALE)

    if write_mode:
        try:
            # In write mode, validate the generated manifest too (belt and suspenders).
            validate_manifest(updated)
        except SystemExit:
            print("Error: generated manifest failed validation; not written.", file=sys.stderr)
            raise SystemExit(EXIT_WRITE)

        write_manifest(updated, manifest_path)
        if args.verbose:
            print(f"Wrote {manifest_path}")


if __name__ == "__main__":
    main()
