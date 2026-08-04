# Metadata model & storage

How FSS represents scene metadata, what it guarantees across re-scrapes, and how
the two stores relate.

## Stores

| | Flat (default) | SQLite (`--db`) |
|---|---|---|
| Layout | one `<slug>.json` per studio | one database, all studios |
| Read cost | full parse of the whole file | indexed query per studio |
| Write cost | full rewrite of the whole file | upsert of changed rows only |
| Scales to | tens of thousands of scenes | millions |

Both implement `store.Store` and are covered by the same contract tests
(`internal/store/contract_test.go`), so behaviour is identical apart from cost.

**The flat store rewrites everything on every save.** A studio with 59k scenes is
a ~100 MB JSON file; an incremental run that finds three new scenes still parses
and rewrites all 100 MB. Use `--db` for large catalogues and treat JSON as an
export format (`fss export`).

## Schema versioning

`models.StoreSchemaVersion` is stamped into every `StudioFile` as
`"schemaVersion"`. It exists so a reader can *refuse* a file it does not
understand: `Save` is authoritative and rewrites the whole file, so parsing a
newer layout on a guess would silently drop whatever that layout added.

- Bump it only when a change alters how an **existing** file must be interpreted.
- Do **not** bump for additive optional fields — old readers ignore them, new
  readers see zero values.
- Version `0` means "written before versioning existed". Those files load
  normally and are stamped on the next save.
- A version **greater** than this build's is a hard error naming the file.

SQLite has had the equivalent since v0: a `schema_version` table plus numbered
migrations in `internal/store/sqlite.go`.

## Scraping multiple studios

Studios never overwrite each other:

- SQLite keys scenes on `(id, site_id, studio_url)`, and `Save` deletes only
  rows matching the studio URL being saved. Junction and price-history rows are
  studio-qualified too.
- The flat store gives each studio its own file; `Slugify` appends a hash of the
  raw URL so two URLs that sanitize alike still get distinct filenames, and a
  stored-URL mismatch is an error rather than a silent overwrite.
- `performers`, `tags` and `categories` are global tables with `UNIQUE(name)`.
  Sharing them across studios is deliberate deduplication.

The one cross-studio effect is **duplication, not loss**: a scene reachable from
two studio URLs is stored once per URL.
