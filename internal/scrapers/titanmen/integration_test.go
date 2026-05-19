//go:build integration

package titanmen

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveTitanMen(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.titanmen.com/category.php?id=5&s=d", 2)
}

func TestLiveTitanMenModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.titanmen.com/sets.php?id=8", 2)
}
