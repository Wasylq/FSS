//go:build integration

package amateurallureclassics

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveAmateurAllureClassics(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.amateurallureclassics.com/categories/movies_1_d.html", 3)
}
