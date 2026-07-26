//go:build integration

package hookuphotshot

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveHookupHotshot(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://hookuphotshot.com/", 3)
}
