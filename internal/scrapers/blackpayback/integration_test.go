//go:build integration

package blackpayback

import (
	"testing"

	"github.com/Anastylosis/FSS/internal/scrapers/adultdoorwayclassicutil"
	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
)

func TestLiveBlackPayback(t *testing.T) {
	testutil.RunLiveScrape(t, adultdoorwayclassicutil.New(sites[0]), "https://blackpayback.com/", 2)
}

func TestLiveBlackPaybackCategory(t *testing.T) {
	testutil.RunLiveScrape(t, adultdoorwayclassicutil.New(sites[0]), "https://blackpayback.com/tour/categories/blondes/1/latest/", 2)
}
