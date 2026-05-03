# Potential Enhancements

## uTLS — Browser TLS Fingerprint Impersonation

Some sites (e.g., ladyfyre.com) use Wordfence or Cloudflare WAFs that detect automated requests via **TLS fingerprinting** (JA3/JA4). Go's `net/http` TLS stack has a distinctive fingerprint that doesn't match any real browser, so these WAFs block requests regardless of User-Agent or HTTP headers.

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
- **ffprobe duration**: Use `ffprobe` (if available) to read video file durations for better matching accuracy, especially for the trailing-number pass which requires duration to disambiguate. Fall back to duration=0 if not installed.
- **Importable package**: Move `internal/identify` to a top-level `identify` package so external Go code can use it as a library. Requires also extracting the matching engine from `internal/stash`.

