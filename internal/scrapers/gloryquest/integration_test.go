//go:build integration

package gloryquest

import (
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func TestLiveGloryQuestAll(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.gloryquest.tv/", 2)
}

func TestLiveGloryQuestSearch(t *testing.T) {
	testutil.RunLiveScrape(t, New(), "https://www.gloryquest.tv/search.php?KeyWord=黒川すみれ", 2)
}
