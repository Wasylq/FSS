//go:build integration

package latinboyz

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveLatinBoyz(t *testing.T) {
	s := fotoroutil.New(fotoroutil.SiteConfig{ID: "latinboyz", Studio: "LatinBoyz", SiteBase: "https://latinboyz.com", TagsAsTags: true, MatchRe: regexp.MustCompile(`latinboyz`)})
	testutil.RunLiveScrape(t, s, "https://latinboyz.com/", 2)
}
