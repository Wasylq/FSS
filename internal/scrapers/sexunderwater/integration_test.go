//go:build integration

package sexunderwater

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveSexUnderwater(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://sexunderwater.com/categories/SexUnderwater_1_d.html", 3)
}
