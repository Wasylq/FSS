//go:build integration

package puremedia

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/puremediautil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLivePureTS(t *testing.T) {
	testutil.RunLiveScrape(t, puremediautil.New(sites[0]), "https://pure-ts.com/", 3)
}

func TestLivePureBBW(t *testing.T) {
	testutil.RunLiveScrape(t, puremediautil.New(sites[1]), "https://pure-bbw.com/", 3)
}
