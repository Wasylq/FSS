//go:build integration

package purecfnm

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePureCFNM(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.purecfnm.com/categories/movies_1_d.html", 2)
}

func TestLivePureCFNMModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.purecfnm.com/models/summer-foxy.html", 2)
}
