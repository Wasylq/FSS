//go:build integration

package maycontaingirl

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveMayContainGirl(t *testing.T) {
	s := fotoroutil.New(fotoroutil.SiteConfig{ID: "maycontaingirl", Studio: "May Contain Girl", SiteBase: "https://maycontaingirl.com", TagsAsTags: true, MatchRe: regexp.MustCompile(`maycontaingirl`)})
	testutil.RunLiveScrape(t, s, "https://maycontaingirl.com/", 2)
}
