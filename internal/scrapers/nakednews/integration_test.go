//go:build integration

package nakednews

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveArchives(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.nakednews.com/archives", 2)
}

func TestLiveAnchor(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.nakednews.com/naked-news-anchor-alana-blaire-a104", 2)
}

func TestLiveAuditions(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.nakednews.com/auditions", 2)
}
