package spankmonster

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

// TestDelayReachesDetailPool guards a gap that affected ~45 scrapers: the
// listing pages were spaced out by scraper.Paginate, but detail pages were
// fetched through a worker pool that never saw opts.Delay, so --delay had no
// effect on the per-scene requests. Paginate throttles pages and cannot
// throttle a detail pool, so the pool has to apply the delay itself.
//
// spankmonster stands in for the whole family here — every enrich() worker
// pool follows the same shape. Without the delay block in the worker, all
// detail requests fire concurrently within ~1ms of the listing response.
func TestDelayReachesDetailPool(t *testing.T) {
	listing := readFixture(t, "listing.html")
	detail := readFixture(t, "detail.html")

	var mu sync.Mutex
	var detailTimes []time.Time

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/spank-monster-updates.html"):
			if r.URL.Query().Get("page") == "1" {
				_, _ = w.Write(listing)
				return
			}
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
		default:
			mu.Lock()
			detailTimes = append(detailTimes, time.Now())
			mu.Unlock()
			_, _ = w.Write(detail)
		}
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	const delay = 150 * time.Millisecond

	s := New()
	s.Client = srv.Client()

	start := time.Now()
	ch, err := s.ListScenes(context.Background(), srv.URL, scraper.ListOpts{Delay: delay})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)

	mu.Lock()
	times := append([]time.Time(nil), detailTimes...)
	mu.Unlock()

	if len(times) == 0 {
		t.Fatal("no detail requests were made; the test would prove nothing")
	}
	if len(scenes) == 0 {
		t.Fatal("no scenes returned")
	}

	// Every worker waits `delay` before its fetch, so even at full concurrency
	// no detail request can land sooner than `delay` after the scrape started.
	for i, ts := range times {
		if d := ts.Sub(start); d < delay {
			t.Errorf("detail request %d fired %v after start, before the %v delay", i, d, delay)
		}
	}
}
