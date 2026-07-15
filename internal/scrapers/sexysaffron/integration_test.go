//go:build integration

package sexysaffron

import (
	"regexp"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/internal/scrapers/veutil"
)

func TestLiveSexySaffron(t *testing.T) {
	s := veutil.New(veutil.SiteConfig{
		ID:             "sexysaffron",
		Studio:         "Saffron Bacchus",
		SiteBase:       "https://sexysaffron.com",
		MainCategoryID: videosCategoryID,
		Patterns:       []string{"sexysaffron.com"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?sexysaffron\.com(?:/|$)`),
	})
	testutil.RunLiveScrape(t, s, "https://sexysaffron.com", 3)
}
