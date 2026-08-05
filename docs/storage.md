# Storage: how FSS holds your data

What FSS actually does with a scrape, which store to use today, how to see
what's inside one, and what has to be true before SQLite becomes the default.

For the field-level model (`schemaVersion`, `firstSeenAt`, `externalIds`, merge
rules), see [metadata.md](metadata.md).

---

## The lifecycle

FSS is three stages, and only the middle one is storage:

```
   scrape                    store                     consume
┌────────────┐        ┌────────────────┐        ┌────────────────────┐
│ ~290 site  │───────▶│  Flat (JSON)   │───────▶│ fss stash import   │
│  scrapers  │        │       or       │        │ fss identify (nfo) │
│            │        │  SQLite (--db) │        │ fss export (csv)   │
└────────────┘        └────────────────┘        └────────────────────┘
```

1. **Scrape.** `fss scrape <studio-url>` runs one site scraper and collects
   `models.Scene` values. Three modes: incremental (default, stops early at a
   known ID), `--full`, and `--refresh` (also soft-deletes what vanished).
2. **Store.** The results are merged with what you already had and written.
   This is where the two implementations differ, and nowhere else — both satisfy
   the same `store.Store` contract and the same contract tests.
3. **Consume.** Push metadata into Stash, write `.nfo` sidecars for Kodi/Jellyfin,
   or export CSV for a spreadsheet.

The important consequence: **storage is an implementation detail of stage 2.**
Nothing about a scraper changes when you switch stores.

Except — today, it does change stage 3. See [What blocks the switch](#what-blocks-the-switch).

---

## The two stores

|  | Flat (default) | SQLite (`--db`) |
|---|---|---|
| Layout | one `<slug>.json` per studio | one file, all studios |
| Human-readable | yes, directly | via `fss export` or a viewer |
| Concurrent studios | one file each | one database, row-level keys |
| Queryable | no | yes |

### Measured cost

On the largest real catalogue to hand — **59,254 scenes, a 104 MB studio file** —
one incremental round trip (`Load`, add a single new scene, `Save`):

| | Flat | SQLite |
|---|---|---|
| Load + Save | 2.0 s | **2.5 s** |
| ↳ of which Save | 1.3 s | **0.5 s** |
| Peak RSS | 964 MB | **634 MB** |
| On disk | **104 MB** | 248 MB |
| Initial ingest | — | 52 s (one-off) |

The two are now within ~25% on wall clock, and SQLite uses a third less memory.

- **Flat** parses and re-marshals the entire file on every save. `encoding/json`
  is fast at that, but it costs *memory*: ~964 MB peak for a 104 MB file. On a
  small VPS that is an OOM waiting to happen, and it grows with the catalogue.
- **SQLite** writes only what changed. `Save` fingerprints each scene
  (`content_hash`) and skips any whose stored fingerprint matches, so an
  incremental scrape touches a handful of rows instead of all 59,254.

Earlier versions rewrote every row on every save — roughly **296,000 statements
to record one new scene** — which made the database several times slower than
brute-force JSON. That is fixed; if you are reading older notes claiming SQLite
is slower, they predate this.

The remaining one-off cost is the **initial ingest** (~52 s for 59k scenes),
paid once when you `fss import` an existing catalogue.

### Which should I use today

- **You want to query your data** — scenes by performer across every site, price
  history over time, per-studio counts: `--db`. The flat store cannot answer any
  of these, at all.
- **Large catalogue, or a memory-constrained host:** `--db`. A third less peak
  RSS, and the gap widens as the catalogue grows.
- **You want `fss list-studios` or `--name`:** `--db`. `Flat.UpsertStudio` and
  `Flat.ListStudios` are no-ops — studio tracking only exists in SQLite.
- **A couple of small studios and you want to read the raw file:** flat is
  simpler and there is no reason to move.

---

## Seeing what is inside a database

The common objection to SQLite is "now I need a SQL viewer to see my data."
In practice the opposite is closer to true — a 104 MB JSON file cannot be
opened in most editors, while a database answers a question instantly.

Four ways, no SQL required:

**1. Export it back to files.** The round trip is lossless (bar sub-second
timestamps, which SQLite has never stored):

```bash
fss export --db --out-dir ./data -o json,csv   # every tracked studio
fss export --db --out-dir ./data <studio-url>  # just one
```

**2. List what is tracked.**

```bash
fss list-studios --db
```

**3. Export CSV and open it in a spreadsheet.** `-o csv` gives one row per
scene with every field as a column — the most approachable view there is.

**4. Use a viewer if you want one.** The database is a single ordinary file:

- [DB Browser for SQLite](https://sqlitebrowser.org/) — free GUI, all platforms
- `sqlite3 ~/.local/share/fss/fss.db` — the official CLI
- [Datasette](https://datasette.io/) — browse and query in a web UI

The schema is small and readable: `scenes`, `studios`, `price_history`,
`scene_external_ids`, plus `performers`/`tags`/`categories` and their junction
tables. See [usage.md](usage.md#sqlite).

---

## What blocks the switch

Two things must land before SQLite can reasonably become the default. Neither is
about the store itself.

### 1. The consumers are JSON-only

`fss stash import` and `fss identify` do **not** accept `--db`. Both call
`match.LoadJSONFiles` / `match.LoadJSONDir` and can only read `.json` files.

So today, if you scrape with `--db`, you still need JSON to do anything with the
result — which is why `fss scrape --db -o json` writes both. **JSON is not
optional right now; it is a mandatory intermediate.**

This is a small fix, not a redesign. `match.BuildIndex` already takes a plain
`[]models.Scene`, so the change is loading that slice from a `store.Store`
instead of from disk. Until it lands, "switch to the database" is not a real
option — it just adds a step.

### 2. ~~`Save` must become diff-aware~~ — done

`Save` now fingerprints each scene into `content_hash` and skips the expensive
write path (row upsert plus three relation syncs plus price-history diffing) for
any scene whose stored fingerprint matches. Measured on the 59k-scene catalogue,
`Save` went from **6.7 s to 0.5 s**.

Two properties make it correct:

- The hash is computed over the **stored** representation, using the same
  `timeStr` conversions as the writer. Hashing the in-memory values would make a
  freshly scraped scene (nanosecond timestamps) differ from the same scene
  loaded back (second precision), and nothing would ever be skipped.
- `ScrapedAt` and `FirstSeenAt` are excluded. `ScrapedAt` changes on every
  scrape by definition, so including it would defeat the mechanism; when it is
  the only difference, `Save` issues one narrow `UPDATE` instead of the full
  path. `FirstSeenAt` is store-owned and never changes once set.

**Invariant for future work:** anything that writes scene state outside
`upsertScene` must invalidate `content_hash`. `MarkDeleted` sets it to `''`
for exactly this reason — without that, re-saving a soft-deleted scene with
`DeletedAt == nil` would be skipped and the delete would never lift.

`Load` was optimised alongside it (3.2 s → 2.0 s) by dropping two `ORDER BY`
clauses that made SQLite build temp B-trees over every row and every junction
row in the studio. The ordering now happens in Go, on slices that are already
materialised, and is identical.

---

## The flag question

The natural instinct is to flip `--db` to `--no-db`. Recommended: **don't.**

- `--no-db` is a double negative, and it reads as "disable the database" when
  what you mean is "use the other store."
- It strands the existing spelling. Everyone's scripts and cron jobs pass
  `--db` or set `db:` in config today.
- It couples the *default* to a *flag rename*. Those are separate decisions and
  should ship separately.

Preferred shape, when the blockers above are cleared:

```bash
fss scrape <url>                       # uses whatever the config default is
fss scrape --store flat <url>          # explicit opt-out
fss scrape --db=/custom/path.db <url>  # unchanged, still works
```

- Add `--store flat|sqlite` as the canonical selector. No negation, and it reads
  correctly in both directions.
- Keep `--db[=PATH]` exactly as it is — it stays sugar for
  `--store sqlite` plus a path. **Nothing anyone has written breaks.**
- Flip the *config default* (`db:`) in a release that says so loudly, with a
  note pointing at `fss import` for moving existing JSON in.

Changing a default silently reorganises where people's data lives. The migration
path has to exist and be documented before the default moves, which is what
`fss import` and `fss export` are for.

---

## Moving between stores

Both directions already work and are lossless:

```bash
fss import --db ./data/                  # JSON files → database (merges)
fss import --db --replace ./data/        # JSON files → database (authoritative)
fss export --db --out-dir ./data -o json # database → JSON files
```

`import` keys on each file's own `studioUrl`, never the filename — `Slugify` is
lossy and its hash suffix is not reversible. It also derives the `studios` row,
which JSON does not carry, from the scenes themselves.

Nothing in the database schema needed to change to support this: it is a strict
superset of the JSON layout.
