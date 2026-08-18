//go:build integration

package gasm

import (
	"context"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// gasm has two URL modes and 13 patterns: the gasm.com studio-profile path, and
// sibling domains resolved through domainToSlug. Only the profile mode was
// covered, leaving the whole domain-mapping path untested — a wrong or missing
// domainToSlug entry would have gone unnoticed.
//
// All three are gated on a reachability probe. From a jurisdiction with
// age-verification laws the network 302s every domain to `sfw.gasm.com`, a
// preview page with no catalogue on it, so the tests would fail through no
// fault of the scraper. The probe looks for that page specifically: any other
// failure still fails the test, and the moment the block lifts the tests run
// again on their own.

func TestLiveScrape(t *testing.T) {
	skipIfSFWGated(t, "https://www.gasm.com/studio/profile/cosplaybabes")
	testutil.RunLiveScrape(t, New(), "https://www.gasm.com/studio/profile/cosplaybabes", 3)
}

// A second profile, so the mode is not proven by a single studio.
func TestLiveScrapeSecondProfile(t *testing.T) {
	skipIfSFWGated(t, "https://www.gasm.com/studio/profile/mmvfilms")
	testutil.RunLiveScrape(t, New(), "https://www.gasm.com/studio/profile/mmvfilms", 2)
}

// The sibling-domain mode: resolved via domainToSlug rather than the URL path.
func TestLiveSiblingDomain(t *testing.T) {
	skipIfSFWGated(t, "https://mmvfilms.com/")
	testutil.RunLiveScrape(t, New(), "https://mmvfilms.com/", 2)
}

func skipIfSFWGated(t *testing.T, u string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := httpx.Do(ctx, httpx.NewClient(15*time.Second), httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		// Not the gate — let the real test report whatever this is.
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return
	}
	if sfwGateRe.Match(body) {
		t.Skipf("%s served the SFW preview (%s) — this network is age-verification "+
			"geo-blocked, not a scraper failure", u, resp.Request.URL)
	}
}
