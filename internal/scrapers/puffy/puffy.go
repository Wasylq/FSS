// Package puffy registers the eight sites of the Puffy + VIPissy network, all
// of which share the bespoke PHP CMS handled by puffyutil.
package puffy

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/puffyutil"
	"github.com/Wasylq/FSS/scraper"
)

func site(id, studio, host, listingPath, scenePrefix string) puffyutil.SiteConfig {
	return puffyutil.SiteConfig{
		ID:          id,
		Studio:      studio,
		SiteBase:    "https://" + host,
		ListingPath: listingPath,
		ScenePrefix: scenePrefix,
		Patterns: []string{
			host,
			host + "/" + listingPath + "/",
			host + "/" + listingPath + "/page-{N}/",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?` + regexp.QuoteMeta(host)),
	}
}

var sites = []puffyutil.SiteConfig{
	site("wetandpuffy", "Wet and Puffy", "wetandpuffy.com", "videos", ""),
	site("wetandpissy", "Wet and Pissy", "wetandpissy.com", "videos", "video-"),
	site("weliketosuck", "We Like To Suck", "weliketosuck.com", "videos", "video-"),
	site("simplyanal", "Simply Anal", "simplyanal.com", "videos", "video-"),
	site("vipissy", "VIPissy", "vipissy.com", "updates", "video-"),
	site("fistertwister", "Fister Twister", "fistertwister.com", "videos", "video-"),
	site("peeonher", "Pee On Her", "peeonher.com", "updates", ""),
	site("virtualpee", "VirtualPee", "virtualpee.com", "videos", "video-"),
}

func init() {
	for _, cfg := range sites {
		scraper.Register(puffyutil.New(cfg))
	}
}
