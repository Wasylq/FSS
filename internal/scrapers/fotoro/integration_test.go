//go:build integration

package fotoro

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func newTestScraper(s site) *fotoroutil.Scraper {
	escaped := strings.ReplaceAll(s.domain, ".", `\.`)
	return fotoroutil.New(fotoroutil.SiteConfig{
		ID:       s.id,
		Studio:   s.studio,
		SiteBase: "https://www." + s.domain,
		MatchRe:  regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped)),
	})
}

func TestLiveHuCows(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[2]), "https://www.hucows.com/", 2)
}

func TestLiveChastityBabes(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[0]), "https://www.chastitybabes.com/", 2)
}

func TestLiveGirlAsylum(t *testing.T) {
	testutil.RunLiveScrape(t, newTestScraper(sites[6]), "https://www.girlasylum.com/", 2)
}
