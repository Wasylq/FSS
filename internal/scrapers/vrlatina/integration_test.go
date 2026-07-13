//go:build integration

package vrlatina

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveVRLatina(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://vrlatina.com/most-recent/", 3)
}

func TestLiveVRLatinaModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://vrlatina.com/models/bianca-still-111.html", 3)
}
