#!/usr/bin/env python3
"""Remove studio entries from the three tracking JSONs by tree-root name(s).

Usage: prune_tracking.py "Root Name" ["Another Root" ...]

For each root name, removes from all three files every entry whose `name` OR
`parent` equals the root. Preserves JSON formatting (2-space indent).
"""
import json
import sys

FILES = [
    "docs/stashdb-scene-counts.json",
    "docs/stashdb-studios.json",
    "docs/partially-covered.json",
]


def main():
    roots = set(sys.argv[1:])
    if not roots:
        print("need at least one root name", file=sys.stderr)
        sys.exit(1)
    for fn in FILES:
        try:
            with open(fn) as f:
                data = json.load(f)
        except FileNotFoundError:
            continue
        before = len(data)
        kept = [
            e for e in data
            if e.get("name") not in roots and e.get("parent") not in roots
        ]
        if len(kept) != before:
            with open(fn, "w") as f:
                json.dump(kept, f, indent=2, ensure_ascii=False)
                f.write("\n")
        print(f"{fn}: {before} -> {len(kept)} (removed {before - len(kept)})")


if __name__ == "__main__":
    main()
