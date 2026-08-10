// Package mediafetch downloads scraped image assets with SSRF and size guards,
// and detects signed URLs whose expiry has already passed.
package mediafetch

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
)

// MaxBytes caps how much of a response is read, so a hostile or misconfigured
// host cannot exhaust memory.
const MaxBytes = 10 * 1024 * 1024

// Asset is a downloaded image.
type Asset struct {
	Data        []byte
	ContentType string
}

// Fetch downloads rawURL after validating it. allowPrivate permits hosts that
// resolve to private or loopback addresses, for local media servers.
func Fetch(ctx context.Context, client *http.Client, rawURL string, allowPrivate bool) (Asset, error) {
	if err := ValidateURL(rawURL, allowPrivate); err != nil {
		return Asset{}, fmt.Errorf("rejecting URL: %w", err)
	}

	resp, err := httpx.Do(ctx, client, httpx.Request{
		URL:     rawURL,
		Headers: map[string]string{"User-Agent": httpx.UserAgentFirefox},
	})
	if err != nil {
		return Asset{}, fmt.Errorf("downloading: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := httpx.ReadBodyN(resp.Body, MaxBytes+1)
	if err != nil {
		return Asset{}, fmt.Errorf("reading: %w", err)
	}
	if len(data) > MaxBytes {
		return Asset{}, fmt.Errorf("image exceeds %d bytes", MaxBytes)
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	return Asset{Data: data, ContentType: ct}, nil
}

// DataURI fetches an image and encodes it as a data URI, the form Stash's
// cover_image field takes.
//
// A URL carrying an already-passed signed expiry is rejected without a request:
// scraped CDN thumbnails are often dead within hours of the scrape.
func DataURI(ctx context.Context, client *http.Client, rawURL string, allowPrivate bool) (string, error) {
	if Expired(rawURL, time.Now()) {
		return "", fmt.Errorf("URL signature expired — re-scrape the studio for a fresh URL")
	}
	asset, err := Fetch(ctx, client, rawURL, allowPrivate)
	if err != nil {
		return "", err
	}
	return "data:" + asset.ContentType + ";base64," + base64.StdEncoding.EncodeToString(asset.Data), nil
}

// ValidateURL enforces the SSRF defense: http(s) only, and unless allowPrivate
// is set, no host that resolves to a private or loopback address.
//
// This resolves DNS once, before the request, so DNS rebinding is not
// mitigated. For the threat model here — consuming someone else's scraped JSON
// — the dump author would also need to control DNS for a domain the importer
// resolves.
func ValidateURL(rawURL string, allowPrivate bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme %q not allowed (only http/https)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}
	if allowPrivate {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLocal(ip) {
			return fmt.Errorf("host %s is a private/loopback address", ip)
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolving host %q: %w", host, err)
	}
	for _, ip := range ips {
		if isPrivateOrLocal(ip) {
			return fmt.Errorf("host %q resolves to private/loopback IP %s", host, ip)
		}
	}
	return nil
}

func isPrivateOrLocal(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified()
}

// plausible bounds for a signed-URL expiry, so a random numeric query parameter
// is not mistaken for a timestamp.
var (
	expiryFloor   = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	expiryCeiling = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
)

// absoluteExpiryParams are query keys (matched case-insensitively) whose value
// is a Unix timestamp after which the URL stops working.
var absoluteExpiryParams = []string{"expires", "exp", "expiry", "valid_until"}

// Expired reports whether rawURL carries a signed expiry that has already
// passed at time now.
//
// Many CDNs hand out short-lived signed URLs — `?expires=…&token=…` — so a
// stored thumbnail is often dead within hours. Detecting that offline lets a
// caller skip a request it knows will 403, and lets the NFO writer omit a
// `<thumb>` rather than bake in a broken link.
//
// It is deliberately conservative: an unrecognised or unparseable expiry
// returns false. A false negative costs one failed request; a false positive
// would discard a working URL.
func Expired(rawURL string, now time.Time) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	q := u.Query()

	for key, values := range q {
		if !matchesAny(key, absoluteExpiryParams) {
			continue
		}
		for _, v := range values {
			if t, ok := parseUnixTimestamp(v); ok {
				return t.Before(now)
			}
		}
	}

	// AWS SigV4 presigned: X-Amz-Date is the signing time, X-Amz-Expires the
	// lifetime in seconds.
	if signed, ok := lookupFold(q, "X-Amz-Date"); ok {
		if lifetime, ok2 := lookupFold(q, "X-Amz-Expires"); ok2 {
			base, err := time.Parse("20060102T150405Z", signed)
			if err != nil {
				return false
			}
			secs, err := strconv.ParseInt(lifetime, 10, 64)
			if err != nil || secs <= 0 {
				return false
			}
			return base.Add(time.Duration(secs) * time.Second).Before(now)
		}
	}
	return false
}

func matchesAny(key string, candidates []string) bool {
	for _, c := range candidates {
		if strings.EqualFold(key, c) {
			return true
		}
	}
	return false
}

func lookupFold(q url.Values, want string) (string, bool) {
	for key, values := range q {
		if strings.EqualFold(key, want) && len(values) > 0 {
			return values[0], true
		}
	}
	return "", false
}

// parseUnixTimestamp accepts seconds or milliseconds since the epoch, rejecting
// values outside a plausible date range.
func parseUnixTimestamp(s string) (time.Time, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}, false
	}
	t := time.Unix(n, 0).UTC()
	if t.After(expiryCeiling) {
		t = time.UnixMilli(n).UTC() // the value was milliseconds
	}
	if t.Before(expiryFloor) || t.After(expiryCeiling) {
		return time.Time{}, false
	}
	return t, true
}
