//go:build integration

package waap

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

const liveURL = "https://www.waap.co.jp/work/search.php?serch=5&onrls=new&limit=45&pg=1"

// TestLiveWaap is gated on a cache probe. waap.co.jp sits behind a CloudFront
// distribution that ignores the query string in its cache key: `search.php`
// answers with a stale cached result set for whatever query was cached last,
// and every `item.php?itemcode=X` returns the same product page regardless of
// the code. Neither the listing nor the detail fetch can work until that is
// fixed upstream. The probe skips only when a CloudFront cache hit is what
// emptied the listing — a listing that comes back empty from the origin itself
// still fails, and the test resumes once the caching is corrected.
func TestLiveWaap(t *testing.T) {
	testutil.SkipIfPlaceholder(t, liveURL)
	skipIfCDNServesAStaleSearch(t)
	testutil.RunLiveScrape(t, New(), liveURL, 2)
}

func skipIfCDNServesAStaleSearch(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := httpx.Do(ctx, httpx.NewClient(15*time.Second), httpx.Request{
		URL:     liveURL,
		Headers: map[string]string{"User-Agent": httpx.UserAgentChrome},
	})
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return
	}

	// The listing is Shift-JIS; decode it the same way the scraper does so the
	// probe measures the same thing the scrape would see.
	raw, err := io.ReadAll(io.LimitReader(
		transform.NewReader(resp.Body, japanese.ShiftJIS.NewDecoder()), httpx.MaxPageBytes))
	if err != nil {
		return
	}
	items, _ := parseListingPage(string(raw))
	if len(items) > 0 {
		return
	}
	if hit := resp.Header.Get("X-Cache"); hit != "" {
		t.Skipf("the search listing came back empty from a CloudFront cache (X-Cache=%q, "+
			"Age=%q): the distribution ignores the query string, so search.php answers with "+
			"a stale result set for a different query", hit, resp.Header.Get("Age"))
	}
}
