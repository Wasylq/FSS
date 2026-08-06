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

Every consumer reads either source, so switching stores changes nothing
downstream.

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
| Initial ingest | — | 13 s (one-off) |

The two are now within ~25% on wall clock, and SQLite uses a third less memory.

- **Flat** parses and re-marshals the entire file on every save. `encoding/json`
  is fast at that, but it costs *memory*: ~964 MB peak for a 104 MB file. On a
  small VPS that is an OOM waiting to happen, and it grows with the catalogue.
- **SQLite** writes only what changed. `Save` fingerprints each scene
  (`content_hash`) and skips any whose stored fingerprint matches, so an
  incremental scrape touches a handful of rows instead of all 59,254.

The remaining one-off cost is the **initial ingest** (~13 s for 59k scenes),
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

## How the SQLite store stays fast

Four things carry the performance in the table above. Each is easy to undo by
accident, so they are written down rather than left to be rediscovered.

**`Save` writes only what changed.** Each scene is fingerprinted into
`content_hash`; a scene whose stored fingerprint matches skips the row upsert,
all three relation syncs and the price-history diff. When only `scraped_at`
moved, it issues one narrow `UPDATE`.

Two properties make that correct:

- The hash is computed over the **stored** representation, using the same
  `timeStr` conversions as the writer. Hashing in-memory values would make a
  freshly scraped scene (nanosecond timestamps) differ from the same scene
  loaded back (second precision), and nothing would ever be skipped.
- `ScrapedAt` and `FirstSeenAt` are excluded — the first changes on every scrape
  by definition, and the second is store-owned and never changes once set.

> **Invariant:** anything writing scene state outside `upsertScene` must
> invalidate `content_hash`. `MarkDeleted` sets it to `''` for exactly this
> reason — without that, re-saving a soft-deleted scene with `DeletedAt == nil`
> would be skipped and the delete would never lift.

**`Load` orders in Go, not SQL.** `ORDER BY` made SQLite build temp B-trees over
every scene row and every junction row in the studio, to order lists that are a
handful of entries each. Do not add it back.

**The child tables are indexed by `studio_url`** (migration 7). Their primary
keys start with `scene_id`, so a `studio_url` predicate cannot use them; without
the index every `Load` scans the whole table — the entire database's junction
rows to read one studio's. The indexes are deliberately narrow: covering
variants measured 3 ms faster for 31 MB more on a 300 MB database.

**Large saves share a `saveSession`** (`internal/store/savesession.go`), caching
prepared statements by SQL text and entity name→id lookups. Without it a first
ingest spent a third of its CPU re-parsing SQL and resolved every
performer/tag/category name with its own round trip. Its statements are bound to
the transaction, so the session is closed before `Commit`.

## What is still not optimised

**Single-studio `Load` (2.0 s for 59k scenes)** is still ~3× the flat store's
0.7 s. The query plans are clean; the time is in row scanning and `time.Parse`
(three per scene). It only matters for a single studio far larger than the rest,
and loading one studio out of many is already proportional (50 ms of 40).

## Making SQLite the default

Nothing technical blocks it now: both consumers read either source, and the
database is competitive on the numbers above. What is left is the rollout.

The natural instinct is to flip `--db` to `--no-db`. Recommended: **don't.**

- `--no-db` is a double negative, and it reads as "disable the database" when
  what you mean is "use the other store."
- It strands the existing spelling. Everyone's scripts and cron jobs pass
  `--db` or set `db:` in config today.
- It couples the *default* to a *flag rename*. Those are separate decisions and
  should ship separately.

Preferred shape:

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
