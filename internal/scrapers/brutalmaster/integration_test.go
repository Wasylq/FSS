//go:build integration

package brutalmaster

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBrutalMaster(t *testing.T) {
	s := fotoroutil.New(fotoroutil.SiteConfig{ID: "brutalmaster", Studio: "Brutal Master", SiteBase: "https://brutalmaster.com", TagsAsTags: true, MatchRe: regexp.MustCompile(`brutalmaster`)})
	testutil.RunLiveScrape(t, s, "https://brutalmaster.com/", 2)
}
