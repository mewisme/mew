#!/usr/bin/env python3
"""Enrich all plans/00xx-*.md from enrichment-*.json catalogs,
then generate CHECKLIST.md and manifest.json.
Replaces: enrich-and-generate.ps1, enrichment-catalog.ps1, Read-Status.ps1,
           generate-checklist.ps1, update-manifest.ps1.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PLANS_ROOT = ROOT / "plans"
SCRIPTS_DIR = Path(__file__).resolve().parent
REPO_ROOT = ROOT


# ── status ──────────────────────────────────────────────────────────────

def load_status() -> dict:
    path = PLANS_ROOT / "status.json"
    if not path.is_file():
        raise SystemExit(f"missing status file: {path}")
    with open(path, encoding="utf-8") as f:
        status = json.load(f)

    if status.get("schemaVersion") != 1:
        raise SystemExit(f"unsupported status schemaVersion: {status.get('schemaVersion')}")

    current = str(status["currentMvp"])
    if not current:
        raise SystemExit("status.currentMvp is empty")

    completed = [str(c).zfill(4) for c in status.get("completedMvps", [])]
    planned = [str(p).zfill(4) for p in status.get("plannedMvps", [])]

    if current in completed:
        raise SystemExit(f"currentMvp {current} must not appear in completedMvps")
    if current in planned:
        raise SystemExit(f"currentMvp {current} must not appear in plannedMvps (current is distinct from planned)")

    # Duplicate detection
    all_ids = completed + planned
    seen: dict[str, str] = {}
    for mvp_id in all_ids:
        if mvp_id in seen and seen[mvp_id] != "completed":
            raise SystemExit(f"duplicate MVP id in status.json: {mvp_id}")
        seen[mvp_id] = "completed" if mvp_id in completed else "planned"

    # Verify every status.json ID has a plan file
    plan_files = sorted(PLANS_ROOT.glob("0*.md"))
    known: dict[str, Path] = {}
    for pf in plan_files:
        m = re.match(r"^(\d{4})-", pf.name)
        if m and not pf.name.startswith("0000-"):
            known[m.group(1)] = pf

    for mvp_id in seen:
        if mvp_id not in known:
            raise SystemExit(f"unknown MVP id in status.json (no plan file): {mvp_id}")

    # Predecessor ordering check
    completed_set = set(completed)

    def _get_predecessor_ids(pred_str: str) -> list[str]:
        if not pred_str or pred_str in ("-", "None"):
            return []
        return re.findall(r"\b(\d{4})\b", pred_str)

    for mvp_id in completed:
        if mvp_id in known:
            meta = parse_plan_meta(known[mvp_id])
            for pred in _get_predecessor_ids(meta.get("predecessors", "")):
                if pred not in completed_set:
                    raise SystemExit(
                        f"MVP {mvp_id} is completed but predecessor {pred} is not in completedMvps"
                    )

    return {
        "current_mvp": current,
        "completed_mvps": completed,
        "planned_mvps": planned,
        "last_certified_core_commit": status.get("lastCertifiedCoreCommit", ""),
        "last_updated": status.get("lastUpdated", ""),
    }


def is_mvp_completed(status: dict, mvp_id: str) -> bool:
    return mvp_id in status["completed_mvps"]


def mvp_rollup_status(status: dict, mvp_id: str) -> str:
    if is_mvp_completed(status, mvp_id):
        return "done"
    if mvp_id == status["current_mvp"]:
        return "in-progress"
    return "planned"


# ── catalog ─────────────────────────────────────────────────────────────

def load_catalog() -> dict[str, dict]:
    merged: dict[str, dict] = {}
    for cat_file in sorted(SCRIPTS_DIR.glob("enrichment-*.json")):
        with open(cat_file, encoding="utf-8") as f:
            obj = json.load(f)
        merged.update(obj)
    return merged


# ── plan metadata ───────────────────────────────────────────────────────

def parse_plan_meta(path: Path) -> dict:
    raw = path.read_text(encoding="utf-8")
    raw = _normalize_lf(raw)

    mvp_id = ""
    name = ""
    first_h1 = None
    for line in raw.split("\n"):
        m = re.match(r"^#\s+(\d{4})\s+\S+\s+(.+)$", line)
        if m:
            mvp_id = m.group(1)
            name = m.group(2).strip()
            break
        if re.match(r"^#\s", line) and first_h1 is None:
            first_h1 = line

    if not mvp_id:
        name_match = re.match(r"(\d{4})-([^.]+)", path.name)
        if name_match:
            mvp_id = name_match.group(1)
            name = name_match.group(2).replace("-", " ")

    objective = _table_field(raw, "Primary objective")
    predecessors = _table_field(raw, "Required predecessors")
    phase = _table_field(raw, "Phase")

    exit_items = re.findall(r"^-\s+\[\s\]\s+(.+)$", raw, re.MULTILINE)

    exit_section = re.search(r"## Exit Criteria\r?\n\r?\n(.*?)(?=\r?\n## )", raw, re.DOTALL)
    exit_section_items: list[str] = []
    if exit_section:
        exit_section_items = re.findall(r"^-\s+\[\s\]\s+(.+)$", exit_section.group(1), re.MULTILINE)

    non_goals_section = re.search(r"## Explicit Non-Goals\r?\n\r?\n(.*?)(?=\r?\n## )", raw, re.DOTALL)
    non_goals: list[str] = []
    if non_goals_section:
        non_goals = re.findall(r"^-\s+(.+)$", non_goals_section.group(1), re.MULTILINE)

    return {
        "id": mvp_id,
        "name": name,
        "objective": objective,
        "predecessors": predecessors,
        "phase": phase,
        "exit_items": exit_section_items,
        "non_goals": non_goals,
        "all_checkboxes": exit_items,
    }


def _table_field(text: str, field: str) -> str:
    m = re.search(rf"^\|\s*{re.escape(field)}\s*\|\s*(.+?)\s*\|", text, re.MULTILINE)
    return m.group(1).strip() if m else ""


def _normalize_lf(text: str) -> str:
    return text.replace("\r\n", "\n").replace("\r", "\n")


# ── enrichment ──────────────────────────────────────────────────────────

def _get_checked_items(raw: str) -> set[str]:
    return set(re.findall(r"^- \[x\] (.+)$", raw, re.MULTILINE))


def _format_checklist_item(text: str, checked: set[str]) -> str:
    mark = "x" if text in checked else " "
    return f"- [{mark}] {text}"


def _build_enrichment_block(mvp_id: str, entry: dict) -> str:
    feat = "\n".join(entry.get("featureRows", []))
    if not feat:
        feat = f"| (see 0002 inventory) | - | - | {mvp_id} |"

    pkg_lines = "\n".join(f"- `{p}`" for p in entry.get("packages", []))
    forbid = entry.get("forbiddenImports", [])
    forbid_lines = "\n".join(f"- {f}" for f in forbid) if forbid else "- None beyond architecture rules in 0003 / AGENTS.md."
    fix_lines = "\n".join(f"- `{f}`" for f in entry.get("fixtures", []))

    acc_lines = ""
    for n, a in enumerate(entry.get("acceptance", []), 1):
        acc_lines += f"{n}. {a}\n"

    conf_lines = "\n".join(f"- {c}" for c in entry.get("conformance", []))
    open_lines = "\n".join(f"- {d}" for d in entry.get("openDecisions", []))

    df_block = f"""```mermaid
flowchart LR
  {entry.get('dataFlowNodes', '')}
```"""

    return f"""<!-- ENRICHMENT:BEGIN -->

## Feature Inventory Links

Rows this MVP owns or primarily advances (from `0002` inventory themes):

| Feature | Nub baseline | Mew target | Primary MVP |
|---|---|---|---|
{feat}

## Go Package Map

**Packages / paths:**

{pkg_lines}

**Forbidden import edges:**

{forbid_lines}

## Data Flow

{df_block}

## Commands and Flags

{entry.get('commandsFlags', '')}

## Persistent Artifacts

{entry.get('artifacts', '')}

## Concrete Test Fixtures

{fix_lines}

## Acceptance Scenarios

{acc_lines}
## Nub Conformance Targets

{conf_lines}

## Open Decisions

{open_lines}

<!-- ENRICHMENT:END -->"""


def _build_checklist_section(entry: dict, checked: set[str]) -> str:
    groups: dict[str, list[str]] = {
        "Contracts & types": [],
        "Core logic": [],
        "CLI / UX": [],
        "Tests & fixtures": [],
        "Docs & observability": [],
    }
    keys = list(groups.keys())
    for i, item in enumerate(entry.get("scopeItems", [])):
        bucket = keys[i % 5]
        groups[bucket].append(item)

    lines = ["## Detailed Implementation Checklist", ""]
    for group_name in keys:
        items = groups[group_name]
        if not items:
            continue
        lines.append(f"### {group_name}")
        lines.append("")
        for it in items:
            lines.append(_format_checklist_item(it, checked))
        lines.append("")

    return "\n".join(lines)


def update_plan_file(path: Path, mvp_id: str, entry: dict) -> None:
    raw = _normalize_lf(path.read_text(encoding="utf-8"))
    checked = _get_checked_items(raw)

    # Strip prior enrichment block.
    raw = re.sub(r"(?s)<!-- ENRICHMENT:BEGIN -->.*?<!-- ENRICHMENT:END -->\r?\n*", "", raw)
    raw = re.sub(r"(?s)<!-- ENRICHMENT-TESTS -->.*?(?=\r?\nRequired test layers:|\r?\n## )", "", raw)

    enrichment = _build_enrichment_block(mvp_id, entry)
    new_checklist = _build_checklist_section(entry, checked)

    # Replace Detailed Implementation Checklist through Test Plan header.
    pattern = r"(?s)## Detailed Implementation Checklist\r?\n.*?(?=## Test Plan)"
    if not re.search(pattern, raw):
        raise SystemExit(f"No Detailed Implementation Checklist in {path}")
    raw = re.sub(pattern, new_checklist + "\n", raw)

    # Insert enrichment before AI-Agent Handoff Contract.
    marker = "## AI-Agent Handoff Contract"
    if marker in raw:
        raw = re.sub(r"(\r?\n){3,}(?=" + re.escape(marker) + r")", "\n\n", raw)
        raw = raw.replace(marker, f"\n\n{enrichment.strip()}\n\n{marker}")
    else:
        raw = raw.strip() + f"\n\n{enrichment}"

    # Expand Test Plan with MVP-specific bullets.
    if "<!-- ENRICHMENT-TESTS -->" not in raw:
        extra = ["<!-- ENRICHMENT-TESTS -->"]
        for a in entry.get("acceptance", []):
            extra.append(_format_checklist_item(f"Acceptance: {a}", checked))
        for f in entry.get("fixtures", []):
            extra.append(_format_checklist_item(f"Fixture ready: `{f}`", checked))
        extra.append("")
        block = "\n".join(extra) + "\n"
        raw = re.sub(r"(## Test Plan\r?\n\r?\n)", r"\1" + block, raw)

    out = _normalize_lf(raw.strip() + "\n")
    path.write_text(out, encoding="utf-8")


# ── CHECKLIST.md generation ──────────────────────────────────────────────

def _get_preserved_narrative(checklist_path: Path) -> str:
    if not checklist_path.is_file():
        return ""
    raw = checklist_path.read_text(encoding="utf-8")
    m = re.search(r"(?s)<!-- CHECKLIST:NARRATIVE:BEGIN -->\r?\n(.*?)\r?\n<!-- CHECKLIST:NARRATIVE:END -->", raw)
    if m:
        return m.group(1).strip()
    do_now = re.search(r"(?s)## Do now\r?\n\r?\n\*\*Next:\*\*[^\n]*\r?\n\r?\n(.*?)(?=\r?\n## MVP completion)", raw)
    if do_now:
        return do_now.group(1).strip()
    return ""


def write_checklist(catalog: dict, plan_files: list[Path], status: dict) -> Path:
    today = status.get("last_updated", "")
    checklist_path = PLANS_ROOT / "CHECKLIST.md"
    narrative = _get_preserved_narrative(checklist_path)

    current_meta = None
    current_slug = None
    for pf in sorted(plan_files):
        mvp_id = pf.name.split("-")[0]
        if mvp_id == status["current_mvp"]:
            current_meta = parse_plan_meta(pf)
            current_slug = pf.stem
            break

    if current_meta is None:
        raise SystemExit(f"no plan file for currentMvp {status['current_mvp']}")

    lines = [
        "# Mew Implementation Master Checklist",
        "",
        "## Program status",
        "",
        f"- Current MVP: **{status['current_mvp']}** — {current_meta['name']}",
        f"- Last updated: {today}",
        "- Source of truth: per-MVP files under `plans/00xx-*.md`",
        "- Regenerate: `python3 plans/scripts/enrich_and_generate.py`",
    ]
    if status.get("last_certified_core_commit"):
        lines.append(f"- Last certified core commit: `{status['last_certified_core_commit']}`")
    else:
        lines.append("- Runtime certification: pending (0057 stabilization gate not yet reached)")
    lines.append("")
    lines.append("## Do now")
    lines.append("")
    lines.append(f"**Next:** [{status['current_mvp']} - {current_meta['name']}]({current_slug}.md)")
    lines.append("")
    if narrative:
        lines.append("<!-- CHECKLIST:NARRATIVE:BEGIN -->")
        lines.append(narrative)
        lines.append("<!-- CHECKLIST:NARRATIVE:END -->")
        lines.append("")

    lines.append("## MVP completion (65)")
    lines.append("")
    lines.append("| ID | MVP | Phase | Predecessors | Status | Plan |")
    lines.append("|----|-----|-------|--------------|--------|------|")

    agg_lines = ["## Aggregated tasks by MVP", ""]

    for pf in sorted(plan_files):
        if not re.match(r"^(\d{4})-", pf.name):
            continue
        mvp_id = pf.name.split("-")[0]
        if mvp_id == "0000":
            continue
        meta = parse_plan_meta(pf)
        slug = pf.stem
        phase = meta["phase"] or catalog.get(mvp_id, {}).get("phase", "")
        pred = meta["predecessors"] or "-"
        short = meta["name"]
        if len(short) > 60:
            short = short[:57] + "..."
        done = is_mvp_completed(status, mvp_id)
        check_mark = "[x]" if done else "[ ]"
        lines.append(f"| {mvp_id} | {short} | {phase} | {pred} | {check_mark} | [{mvp_id}]({slug}.md) |")

        rollup = mvp_rollup_status(status, mvp_id)
        agg_lines.append(f"### {mvp_id} - {meta['name']}")
        agg_lines.append("")
        agg_lines.append(f"- status: {rollup}")
        agg_lines.append(f"- plan: [{slug}.md]({slug}.md)")
        agg_lines.append("")
        entry = catalog.get(mvp_id)
        if entry:
            for it in entry.get("scopeItems", []):
                agg_lines.append(f"- [{'x' if done else ' '}] {it}")
            for a in entry.get("acceptance", []):
                agg_lines.append(f"- [{'x' if done else ' '}] Acceptance: {a}")
            for e in meta.get("exit_items", []):
                agg_lines.append(f"- [{'x' if done else ' '}] Exit: {e}")
        else:
            agg_lines.append("- [ ] (enrichment catalog missing for this id)")
        agg_lines.append("")

    lines.append("")
    lines.extend(agg_lines)

    body = _normalize_lf("\n".join(lines).strip() + "\n")
    checklist_path.write_text(body, encoding="utf-8")
    return checklist_path


# ── manifest.json generation ─────────────────────────────────────────────

def _get_product_identity() -> dict:
    path = REPO_ROOT / "product" / "identity.json"
    if not path.is_file():
        raise SystemExit(f"missing product identity: {path}")
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def update_manifest() -> None:
    identity = _get_product_identity()
    entries: list[dict] = []

    all_files: list[Path] = []
    for dirpath_str, dirnames, filenames in os.walk(PLANS_ROOT):
        dirnames[:] = [d for d in dirnames if d != "__pycache__" and not d.startswith(".")]
        dirpath = Path(dirpath_str)
        for name in filenames:
            if name == "manifest.json":
                continue
            if name.endswith(".pyc"):
                continue
            fp = dirpath / name
            if ".git" in fp.parts:
                continue
            all_files.append(fp)

    all_files.sort(key=lambda p: str(p).lower())

    for fp in all_files:
        rel = str(fp.relative_to(PLANS_ROOT)).replace("\\", "/")
        sha = hashlib.sha256(fp.read_bytes()).hexdigest()
        entries.append({"path": rel, "bytes": fp.stat().st_size, "sha256": sha})

    plan_count = len([f for f in PLANS_ROOT.glob("0*.md") if re.match(r"^\d{4}-", f.name)])
    md_count = len(list(PLANS_ROOT.rglob("*.md")))

    manifest = {
        "name": "Mew Implementation Plan",
        "product": {
            "full_name": identity["full_name"],
            "short_name": identity["short_name"],
            "binary": identity["primary_binary"],
            "primary_alias": identity["primary_alias"],
            "executor_binary": identity["executor_binary"],
            "executor_alias": identity["executor_alias"],
            "native_lockfile": identity["native_lockfile"],
        },
        "language": "English",
        "reference": {
            "repository": "nubjs/nub",
            "commit": "08a804359ef301ef8b9307f1258cc185b3270698",
        },
        "plan_file_count": plan_count,
        "markdown_file_count": md_count,
        "validation_errors": [],
        "files": entries,
    }

    out_path = PLANS_ROOT / "manifest.json"
    text = json.dumps(manifest, indent=2, ensure_ascii=False) + "\n"
    out_path.write_text(_normalize_lf(text), encoding="utf-8")


# ── main ────────────────────────────────────────────────────────────────

def main() -> None:
    print("Loading status...")
    status = load_status()

    print("Loading catalogs...")
    catalog = load_catalog()
    print(f"Catalog entries: {len(catalog)}")

    plan_files = sorted(
        p for p in PLANS_ROOT.glob("0*.md")
        if re.match(r"^(\d{4})-", p.name) and not p.name.startswith("0000-")
    )

    missing = []
    for pf in plan_files:
        mvp_id = pf.name.split("-")[0]
        if mvp_id not in catalog:
            missing.append(mvp_id)
    if missing:
        raise SystemExit(f"Missing catalog for: {', '.join(missing)}")

    print("Enriching plan files...")
    for pf in plan_files:
        mvp_id = pf.name.split("-")[0]
        print(f"  enrich {mvp_id}")
        update_plan_file(pf, mvp_id, catalog[mvp_id])

    print("Writing CHECKLIST.md...")
    write_checklist(catalog, plan_files, status)

    print("Updating manifest.json...")
    update_manifest()

    print("Done.")


if __name__ == "__main__":
    main()
