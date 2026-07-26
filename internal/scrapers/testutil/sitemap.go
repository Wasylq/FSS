package testutil

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SitemapServer starts an offline test server for a sitemap-driven scraper and
// returns it alongside the rewritten sitemap body.
//
// Sitemap fixtures are captured verbatim, so their <loc> entries are absolute
// URLs on the live site. Scrapers that fetch those URLs as given (rather than
// rebuilding them from a base) will therefore walk straight out of an
// "offline" test and scrape production: overriding the scraper's base URL only
// redirects the sitemap request itself, not the detail requests it yields.
//
// Such a test grades live markup instead of the fixture. It passes on a
// developer machine with network access and fails on a sandboxed CI runner, on
// a page nobody touched — which is exactly how arx and kristenbjorn broke.
//
// liveHost is the scheme+host to strip (e.g. "https://honeytrans.com"). The
// server is registered with t.Cleanup, and its handler serves sitemapBody for
// any path ending in "sitemap.xml" and detail for everything else.
func SitemapServer(t *testing.T, liveHost string, sitemap, detail []byte) *httptest.Server {
	t.Helper()

	// Unstarted, so the listener address is known before the body is rewritten
	// and no request can observe the pre-rewrite bytes.
	srv := httptest.NewUnstartedServer(nil)
	local, err := rewriteSitemapHost(sitemap, liveHost, "http://"+srv.Listener.Addr().String())
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}

	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Any "sitemap"-ish path: callers use /sitemap.xml, /sitemap_video.xml,
		// and per-language variants. Scene paths are slugs and never match.
		if strings.Contains(r.URL.Path, "sitemap") {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write(local)
			return
		}
		_, _ = w.Write(detail)
	})
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

// rewriteSitemapHost replaces every occurrence of liveHost in the fixture with
// localHost. It errors rather than returning the body unchanged when liveHost
// is absent: a silent no-op would leave the detail fetches pointing at the live
// site, which is the exact failure this helper exists to prevent. Split out
// from SitemapServer so the tripwire is unit-testable.
func rewriteSitemapHost(sitemap []byte, liveHost, localHost string) ([]byte, error) {
	if !bytes.Contains(sitemap, []byte(liveHost)) {
		return nil, fmt.Errorf("sitemap fixture contains no %q — the host rewrite would be a no-op "+
			"and detail fetches would hit the live site", liveHost)
	}
	return bytes.ReplaceAll(sitemap, []byte(liveHost), []byte(localHost)), nil
}
