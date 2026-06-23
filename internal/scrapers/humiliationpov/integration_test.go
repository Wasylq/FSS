//go:build integration

package humiliationpov

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveHumiliationPOV(t *testing.T) {
	s := fotoroutil.New(fotoroutil.SiteConfig{ID: "humiliationpov", Studio: "Humiliation POV", SiteBase: "https://www.humiliationpov.com/blog", TagsAsTags: true, MatchRe: regexp.MustCompile(`humiliationpov`)})
	testutil.RunLiveScrape(t, s, "https://www.humiliationpov.com/", 2)
}
