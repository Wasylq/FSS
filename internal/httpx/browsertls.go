package httpx

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// A handful of WAFs classify clients by the shape of the TLS ClientHello
// (JA3/JA4) and the HTTP/2 SETTINGS that follow, before any request header is
// read. Go's crypto/tls emits a hello nothing else emits, so such a host
// answers every Go request identically — NaughtyAmerica's AWS load balancer
// returns a bare 403 to the homepage, with or without browser headers, over
// HTTP/1.1 or HTTP/2, from any address. A browser on the same machine and the
// same address loads the site fine.
//
// utls replays a recorded browser hello byte for byte, which is enough to be
// served normally. This is emphatically NOT a way around a login, a paywall,
// or a rate limit: it makes an ordinary anonymous request look like the
// browser the site already serves, and everything it reaches is public.
//
// Certificates are verified exactly as elsewhere — utls performs the same
// verification against the same root pool. What changes is the hello, not the
// trust decision.

// browserHello is the fingerprint to present. HelloChrome_Auto tracks the
// newest Chrome profile the pinned utls version carries, so bumping utls
// refreshes it. That is also the maintenance cost: fingerprints drift as
// browsers move, and a preset that falls far behind starts looking anomalous
// in its own right. If a site that worked starts returning 403 again, bump
// utls before suspecting the scraper.
var browserHello = utls.HelloChrome_Auto

// dialBrowserTLS completes a handshake presenting a browser ClientHello.
func dialBrowserTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	raw, err := (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	// ServerName drives both SNI and verification; MinVersion guards against a
	// downgrade, since the hello preset advertises a browser's full range.
	conn := utls.UClient(raw, &utls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}, browserHello)
	if err := conn.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return conn, nil
}

var (
	browserTLSOnce      sync.Once
	browserTLSTransport http.RoundTripper
)

// browserTransport builds the shared browser-fingerprint transport.
//
// It is HTTP/2 only, deliberately. A browser hello advertises h2 in ALPN, so
// every host that accepts it negotiates HTTP/2 anyway; wiring an HTTP/1.1
// fallback that could never be selected would only add a path no test covers.
// A host that refuses h2 will surface as a handshake error rather than
// silently falling back to Go's own fingerprint, which would defeat the point.
func browserTransport() http.RoundTripper {
	browserTLSOnce.Do(func() {
		browserTLSTransport = &http2.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return dialBrowserTLS(ctx, network, addr)
			},
		}
	})
	return browserTLSTransport
}

// NewBrowserTLSClient returns a client that presents a browser's TLS
// fingerprint instead of Go's.
//
// Reach for it only after a scraper has been shown to fail without it — the
// signature is a WAF answering an identical 403 to every request including the
// site's own homepage, while a browser on the same machine loads it. It is the
// mirror image of NewLegacyTLSClient: that one widens what Go will accept from
// an old server, this one changes what Go presents to a picky one. Both verify
// certificates.
//
// The connection pool is separate from NewClient's, so using it for one site
// does not change how every other site is reached.
func NewBrowserTLSClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: browserTransport(),
	}
}
