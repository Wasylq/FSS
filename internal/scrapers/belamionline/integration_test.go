//go:build integration

package belamionline

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveScrape(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://belamionline.com/latestsexscenes.aspx", 5)
}

func TestLiveScrapeModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://newtour.belamionline.com/modelsindex.aspx?ModelID=2722", 5)
}
