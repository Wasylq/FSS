//go:build integration

package newsensations

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/newsensationsutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveNewSensations(t *testing.T) {
	const url = "https://www.newsensations.com/tour_ns/categories/movies_1_d.html"
	testutil.SkipIfPlaceholder(t, url)
	s := newsensationsutil.New(sites[0])
	testutil.RunLiveScrape(t, s, url, 2)
}

func TestLiveFamilyXXX(t *testing.T) {
	const url = "https://familyxxx.com/tour_famxxx/categories/movies_1_d.html"
	testutil.SkipIfPlaceholder(t, url)
	s := newsensationsutil.New(sites[1])
	testutil.RunLiveScrape(t, s, url, 2)
}
