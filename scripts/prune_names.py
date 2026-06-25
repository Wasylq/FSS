#!/usr/bin/env python3
"""Remove tracking-JSON entries by EXACT name only (not by parent)."""
import json, sys
FILES = ["docs/stashdb-scene-counts.json","docs/stashdb-studios.json","docs/partially-covered.json"]
names = set(sys.argv[1:])
for fn in FILES:
    try: data=json.load(open(fn))
    except FileNotFoundError: continue
    before=len(data)
    kept=[e for e in data if e.get("name") not in names]
    if len(kept)!=before:
        json.dump(kept, open(fn,"w"), indent=2, ensure_ascii=False); open(fn,"a").write("\n")
    print(f"{fn}: {before} -> {len(kept)}")
