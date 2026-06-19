//go:build integration

package clubdom

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveClubDom(t *testing.T) {
	const u = "https://www.clubdom.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("clubdom"), u, 2)
}

func TestLiveSubbyHubby(t *testing.T) {
	const u = "https://www.subbyhubby.com/"
	testutil.SkipIfPlaceholder(t, u)
	testutil.RunLiveScrape(t, newFor("subbyhubby"), u, 2)
}
