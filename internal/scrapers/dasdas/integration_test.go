//go:build integration

package dasdas

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveDasdasRelease(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://dasdas.jp/works/list/release", 2)
}

func TestLiveDasdasActress(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://dasdas.jp/actress/detail/406467", 2)
}
