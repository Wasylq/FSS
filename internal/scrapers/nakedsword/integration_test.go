//go:build integration

package nakedsword

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLive_NakedSword(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.nakedsword.com/", 5)
}

// A sub-studio page filters the feed via studios_id and names the scenes after
// the sub-studio rather than the parent.
func TestLive_NakedSwordStudioPage(t *testing.T) {
	testutil.RunLiveScrape(t, New(),
		"https://www.nakedsword.com/studios/23749/nakedsword-x-rhyheim", 5)
}
