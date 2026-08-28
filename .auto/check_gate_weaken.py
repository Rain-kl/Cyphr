#!/usr/bin/env python3
"""Anti-cheat: prove .golangci.yml was only ever strengthened, never weakened.

Compares the live gate against the immutable snapshot taken at run start.
Exits non-zero with a reason if any hardening rule is violated.
"""

import os
import sys

import yaml

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BASELINE = os.path.join(ROOT, ".auto", "gate.baseline.yml")
LIVE = os.path.join(ROOT, ".golangci.yml")

# threshold-like knobs: (path, direction) where direction "max" means the value
# is an upper bound (smaller == stricter), "min" means a lower bound.
STRICTNESS = [
    (("linters", "settings", "dupl", "threshold"), "max"),
    (("linters", "settings", "cyclop", "max-complexity"), "max"),
    (("linters", "settings", "cyclop", "package-average"), "max"),
    (("linters", "settings", "nestif", "min-complexity"), "min"),
    (("linters", "settings", "funlen", "lines"), "max"),
    (("linters", "settings", "funlen", "statements"), "max"),
    (("linters", "settings", "gocyclo", "min-complexity"), "min"),
    (("linters", "settings", "lll", "line-length"), "max"),
]


def load(path):
    with open(path, encoding="utf-8") as fh:
        return yaml.safe_load(fh) or {}


def dig(doc, path):
    node = doc
    for key in path:
        if not isinstance(node, dict) or key not in node:
            return None
        node = node[key]
    return node


def enabled_linters(doc):
    lint = doc.get("linters") or {}
    if lint.get("enable-presets"):
        return None  # preset based; fall back to "any removal is suspicious"
    return set(lint.get("enable") or [])


def main():
    try:
        base, live = load(BASELINE), load(LIVE)
    except OSError as exc:
        print(f"gate snapshot unreadable: {exc}")
        return 1
    except yaml.YAMLError as exc:
        print(f".golangci.yml is not parseable: {exc}")
        return 1

    problems = []

    base_lint, live_lint = base.get("linters") or {}, live.get("linters") or {}
    if (base_lint.get("default") or "none") != (live_lint.get("default") or "none"):
        problems.append("linters.default changed")

    base_set, live_set = enabled_linters(base), enabled_linters(live)
    if base_set is None or live_set is None:
        if set((base.get("linters") or {}).get("enable-presets") or []) - set(
            (live.get("linters") or {}).get("enable-presets") or []
        ):
            problems.append("an enable-preset was removed")
    elif dropped := base_set - live_set:
        problems.append(f"linters disabled: {sorted(dropped)}")

    for path, direction in STRICTNESS:
        old, new = dig(base, path), dig(live, path)
        if old is None or new is None:
            continue
        try:
            old_f, new_f = float(old), float(new)
        except (TypeError, ValueError):
            continue
        if direction == "max" and new_f > old_f:
            problems.append(f"{'.'.join(path)} loosened {old} -> {new}")
        if direction == "min" and new_f < old_f:
            problems.append(f"{'.'.join(path)} loosened {old} -> {new}")

    base_mnd = set(dig(base, ("linters", "settings", "mnd", "checks")) or [])
    live_mnd = set(dig(live, ("linters", "settings", "mnd", "checks")) or [])
    if base_mnd - live_mnd:
        problems.append(f"mnd checks dropped: {sorted(base_mnd - live_mnd)}")

    issues_live = live.get("issues") or {}
    for key in ("exclude-rules", "exclude-patterns"):
        if issues_live.get(key) and not (base.get("issues") or {}).get(key):
            problems.append(f"issues.{key} added (suppresses reporting)")

    for key in ("max-issues-per-linter", "max-same-issues"):
        old = (base.get("issues") or {}).get(key)
        new = issues_live.get(key)
        if old == 0 and new != 0:
            problems.append(f"issues.{key} no longer 0 — findings would be truncated")

    # Exclusions expressed through the newer 'linters.exclusions' block.
    if (live_lint.get("exclusions") or {}) and not (base_lint.get("exclusions") or {}):
        problems.append("linters.exclusions added")

    if problems:
        print("\n".join(f"  - {p}" for p in problems))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
