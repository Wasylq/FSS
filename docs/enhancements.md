# Potential Enhancements

## ~~Diff-aware `Save`~~ — implemented

`Store.Save` is authoritative over the full scene set, and SQLite used to honour
that by rewriting every row: roughly five statements per scene, about **296,000
statements to record one new scene** in a 59k-scene studio. That made the
database several times slower than brute-force JSON marshalling.

`Save` now fingerprints each scene into a `content_hash` column (migration 6) and
skips the row upsert, all three relation syncs, and price-history diffing for any
scene whose stored fingerprint matches. When only `ScrapedAt` moved, it issues a
single narrow `UPDATE`. `Load` was optimised alongside it by moving two
`ORDER BY` clauses out of SQL, where they forced temp B-trees over every row.

Measured on a 59,254-scene studio (104 MB JSON):

| | before | after | flat, for reference |
|---|---|---|---|
| Save | 6.7 s | **0.5 s** | 1.3 s |
| Load | 3.2 s | **2.0 s** | 0.7 s |
| Peak RSS | 532 MB | 634 MB | 964 MB |

See [storage.md](storage.md) for the correctness properties and the
`content_hash` invalidation invariant.

## Read the store from `stash import` and `identify`

Both commands are JSON-only: they call `match.LoadJSONFiles` / `LoadJSONDir` and
have no `--db` flag. So a `--db` scrape still has to export JSON before anything
downstream can use it, which makes the database an extra step rather than an
alternative.

**Solution**: accept `--db` on both and build the index from `store.Store.Load`
instead of from disk. `match.BuildIndex` already takes a plain `[]models.Scene`,
so the loading function is the only thing that changes.

**Scope**: small — a shared "load scenes from JSON *or* store" helper in the cmd
layer, used by `stash import`, `identify`, and any future consumer.

## uTLS — Browser TLS Fingerprint Impersonation

Some sites use Wordfence or Cloudflare WAFs that detect automated requests via **TLS fingerprinting** (JA3/JA4). Go's `net/http` TLS stack has a distinctive fingerprint that doesn't match any real browser, so these WAFs block requests regardless of User-Agent or HTTP headers.

**Solution**: Integrate [uTLS](https://github.com/refraction-networking/utls) (`github.com/refraction-networking/utls`) into `internal/httpx`. uTLS replaces `crypto/tls` at the transport level and can impersonate a real browser's TLS ClientHello (Chrome, Firefox, Safari).

**Scope**: ~20-line change in `httpx.NewClient()` to swap the default transport for a uTLS-based one. All scrapers would benefit automatically.

**Blocked sites**: ladyfyre.com (WordPress + Wordfence WAF).

## Packagecloud / Cloudsmith — APT and DNF Auto-Updates

GoReleaser produces `.deb` and `.rpm` packages attached to each GitHub Release, but they're manual one-off installs. For `apt upgrade` / `dnf upgrade` to pick up new versions automatically, the packages need to be hosted in a proper repository.

**Options**: [Packagecloud](https://packagecloud.io/) (free for open-source), [Cloudsmith](https://cloudsmith.com/) (free tier), or [Gemfury](https://gemfury.com/). All provide APT and YUM/DNF repos with a stable URL users add once.

**Scope**: Add a post-release GitHub Actions step that pushes `.deb`/`.rpm` artifacts to the hosted repo. ~10-line workflow addition + one-time account setup. GoReleaser's `publishers` feature can do this natively with Packagecloud.

## `fss identify` — Future Improvements

`fss identify` is implemented — see [identify.md](identify.md) for full documentation. Potential future additions:

- **`--nfo-dir`**: Write `.nfo` files to a `.nfo/` subdirectory instead of next to the video, keeping the video folder clean. The Stash NFO scraper supports both locations.
- ~~**ffprobe duration**: Use `ffprobe` (if available) to read video file durations for better matching accuracy.~~ **Implemented** — `identify.probeDuration()` calls `ffprobe` automatically and falls back to 0 if not installed.

