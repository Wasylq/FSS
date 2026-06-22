//go:build integration

package realitystudio

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/realitystudioutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func newTestScraper(s site) *realitystudioutil.Scraper {
	escaped := strings.ReplaceAll(s.domain, ".", `\.`)
	return realitystudioutil.New(realitystudioutil.SiteConfig{
		ID:       s.id,
		Studio:   s.studio,
		SiteBase: "https://www." + s.domain,
		MatchRe:  regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped)),
	})
}

func TestLiveSubbyGirls(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[0]), "https://www.subbygirls.com/main.html", 3)
}

func TestLiveMenAreSlaves(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[2]), "https://www.menareslaves.com/main.html", 3)
}
