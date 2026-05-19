//go:build integration

package virtualtaboo

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func newTestScraper(cfg siteConfig) *Scraper {
	escaped := strings.ReplaceAll(cfg.domain, ".", `\.`)
	return &Scraper{
		cfg:     cfg,
		client:  httpx.NewClient(30 * time.Second),
		matchRe: regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped)),
	}
}

func TestLiveDarkRoomVR(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[0]), "https://darkroomvr.com/videos", 2)
}

func TestLiveOnlyTarts(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[1]), "https://onlytarts.com/videos", 2)
}

func TestLiveVirtualTaboo(t *testing.T) {
	s := newTestScraper(sites[2])
	s.client = &http.Client{Timeout: 60 * time.Second}
	testutil.RunLiveScrape(t, s, "https://virtualtaboo.com/videos", 2)
}
