//go:build integration

package julesjordan

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveJulesJordan(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.julesjordan.com/trial/categories/movies.html", 2)
}

func TestLiveJulesJordanModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.julesjordan.com/trial/models/kendra-lust.html", 2)
}
