//go:build integration

package hmp

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

// maintenanceMarker is the notice h.m.p serves from every path while the shop
// is down for updates ("we are temporarily suspending the site").
var maintenanceMarker = []byte("メンテナンス中")

// TestLiveHMP is gated on a maintenance probe. h.m.p online has been serving a
// whole-site maintenance notice from every path since at least August 2026, so
// there is no catalogue to scrape and the failure says nothing about the
// scraper. Any other empty response still fails, and the test resumes on its
// own once the shop is back.
func TestLiveHMP(t *testing.T) {
	const u = "https://www.hmp.jp/"
	testutil.SkipIfPlaceholder(t, u)
	skipIfUnderMaintenance(t)
	testutil.RunLiveScrape(t, New(), u, 2)
}

func skipIfUnderMaintenance(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s := New()
	body, err := s.fetchPage(ctx, s.siteBase+"/portal/catalog/?scd=10&p=1")
	if err != nil {
		// No HTTP response at all means the origin is unreachable, not that
		// the scraper is wrong. hmp.jp went from serving the maintenance
		// notice to refusing connections outright — DNS still resolves, the
		// TCP connect just never completes — so the marker probe below can no
		// longer see anything, and the test used to spend ~198s discovering
		// that. A StatusError means the server did answer, so that still falls
		// through and the real test reports it.
		var status *httpx.StatusError
		if !errors.As(err, &status) {
			t.Skipf("h.m.p online is unreachable (%v) — no catalogue to scrape, "+
				"and this is not a scraper failure", err)
		}
		// Not the maintenance page — let the real test report whatever this is.
		return
	}
	if bytes.Contains(body, maintenanceMarker) {
		t.Skip("h.m.p online is serving its whole-site maintenance notice — " +
			"there is no catalogue to scrape, and this is not a scraper failure")
	}
}
