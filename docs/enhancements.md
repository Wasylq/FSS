# Potential Enhancements

## uTLS — Browser TLS Fingerprint Impersonation

Some sites (e.g., ladyfyre.com) use Wordfence or Cloudflare WAFs that detect automated requests via **TLS fingerprinting** (JA3/JA4). Go's `net/http` TLS stack has a distinctive fingerprint that doesn't match any real browser, so these WAFs block requests regardless of User-Agent or HTTP headers.

**Solution**: Integrate [uTLS](https://github.com/refraction-networking/utls) (`github.com/refraction-networking/utls`) into `internal/httpx`. uTLS replaces `crypto/tls` at the transport level and can impersonate a real browser's TLS ClientHello (Chrome, Firefox, Safari).

**Scope**: ~20-line change in `httpx.NewClient()` to swap the default transport for a uTLS-based one. All scrapers would benefit automatically.

**Blocked sites**: ladyfyre.com (WordPress + Wordfence WAF).
