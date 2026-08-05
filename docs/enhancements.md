# Potential Enhancements

## Diff-aware `Save` — the storage layer's biggest win

`Store.Save` is authoritative over the full scene set, so neither implementation
is incremental on write: the flat store rewrites the whole JSON file, and SQLite
upserts every scene plus its relations and price history — roughly **five
statements per scene**.

Measured on a 59,254-scene studio (104 MB JSON), one incremental round trip that
adds a single new scene:

| | Flat | SQLite |
|---|---|---|
| Load + Save | 2.0 s | 9.9 s |
| Peak RSS | 964 MB | 532 MB |
| On disk | 104 MB | 248 MB |

That is ~296,000 SQL statements to record one scene, which is why the database
is currently *slower* than brute-force JSON marshalling.

**Solution**: have `Save` skip scenes whose stored content is unchanged. The cmd
layer already loads the existing set to merge against, so the comparison is
available; a per-scene content hash (or a field-by-field equality check against
the loaded scene) would do. SQLite's `Save` drops from 59k upserts to ~3, and the
flat/SQLite comparison inverts decisively.

**Scope**: contained to `internal/store`. The `Store` contract does not change —
`Save` remains authoritative, it just stops doing redundant writes. Existing
contract tests already pin the observable behaviour.

**Why it matters**: this is the prerequisite for making SQLite the default. See
[storage.md](storage.md).

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

