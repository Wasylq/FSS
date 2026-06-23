//go:build integration

package pegasproductions

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePegas(t *testing.T) {
	s := fotoroutil.New(fotoroutil.SiteConfig{ID: "pegasproductions", Studio: "Pegas Productions", SiteBase: "https://www.pegasproductions.com", TagsAsTags: true, MatchRe: regexp.MustCompile(`pegas`)})
	testutil.RunLiveScrape(t, s, "https://www.pegasproductions.com/", 2)
}
