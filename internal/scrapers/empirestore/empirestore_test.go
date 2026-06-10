package empirestore

import (
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestSitesTable(t *testing.T) {
	if len(sites) != 6 {
		t.Errorf("sites count = %d, want 6", len(sites))
	}
}

func TestMatchesURL(t *testing.T) {
	all := scraper.All()

	cases := []struct {
		url    string
		siteID string
	}{
		{"https://www.elegantangel.com/", "elegantangel"},
		{"https://elegantangel.com/watch-newest-elegant-angel-clips-and-scenes.html", "elegantangel"},
		{"https://www.elegantangel.com/93560/studio/club-59-elegant-angel-studios.html", "elegantangel"},
		{"https://vouyermedia.com/watch-newest-vouyer-media-clips-and-scenes.html", "vouyermedia"},
		{"https://www.vouyermedia.com/", "vouyermedia"},
		{"https://vouyermedia.com/157946/vouyer-media-test-streaming-scene-video.html", "vouyermedia"},
		{"https://www.reaganfoxx.com/", "reaganfoxx"},
		{"https://reaganfoxx.com/scenes/673608/reagan-foxx-streaming-pornstar-videos.html", "reaganfoxx"},
		{"https://bobsvideos.empirestores.co/shop-streaming-video-by-scene.html", "bobsvideos"},
		{"https://jsi.empirestores.co/", "justinslayer"},
		{"https://spungygunkfilms.empirestores.co/", "spungygunkfilms"},
	}

	for _, c := range cases {
		found := false
		for _, s := range all {
			if s.ID() == c.siteID && s.MatchesURL(c.url) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no scraper with ID %q matched %q", c.siteID, c.url)
		}
	}
}
