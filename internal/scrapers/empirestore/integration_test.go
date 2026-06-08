//go:build integration

package empirestore

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/empirestoreutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
)

func newSiteScraper(cfg siteConfig) *siteScraper {
	escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
	return &siteScraper{
		es: empirestoreutil.New(empirestoreutil.SiteConfig{
			SiteID:     cfg.SiteID,
			Domain:     cfg.Domain,
			StudioName: cfg.StudioName,
			ListingURL: cfg.ListingURL,
		}),
		config: cfg,
		re:     regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(?:/|$)`, escaped)),
	}
}

func TestLiveVouyerMedia(t *testing.T) {
	const u = "https://vouyermedia.com/watch-newest-vouyer-media-clips-and-scenes.html"
	testutil.SkipIfPlaceholder(t, u)
	cfg := siteConfig{"vouyermedia", "vouyermedia.com", "Vouyer Media", "/watch-newest-vouyer-media-clips-and-scenes.html"}
	testutil.RunLiveScrape(t, newSiteScraper(cfg), u, 2)
}

func TestLiveReaganFoxx(t *testing.T) {
	const u = "https://www.reaganfoxx.com/scenes/673608/reagan-foxx-streaming-pornstar-videos.html"
	testutil.SkipIfPlaceholder(t, u)
	cfg := siteConfig{"reaganfoxx", "reaganfoxx.com", "Reagan Foxx", "/scenes/673608/reagan-foxx-streaming-pornstar-videos.html"}
	testutil.RunLiveScrape(t, newSiteScraper(cfg), u, 2)
}

func TestLiveBobsVideos(t *testing.T) {
	const u = "https://bobsvideos.empirestores.co/shop-streaming-video-by-scene.html"
	testutil.SkipIfPlaceholder(t, u)
	cfg := siteConfig{"bobsvideos", "bobsvideos.empirestores.co", "Bob's Videos", "/shop-streaming-video-by-scene.html"}
	testutil.RunLiveScrape(t, newSiteScraper(cfg), u, 2)
}
