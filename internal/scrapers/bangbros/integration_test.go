//go:build integration

package bangbros

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveBangBrosModel(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://bangbros.com/model/395971/lisa-ann", 2)
}

func TestLiveBangBrosWebsite(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://bangbros.com/websites/MomIsHorny", 2)
}

func TestLiveBangBrosCategory(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://bangbros.com/category/brunette", 2)
}
