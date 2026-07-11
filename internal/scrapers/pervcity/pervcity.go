// Package pervcity registers the PervCity network sites (pervcity.com,
// analoverdose.com and upherasshole.com), which share the NATS/ElevatedX
// videoBlock tour CMS handled by pervcityutil.
package pervcity

import (
	"github.com/Wasylq/FSS/internal/scrapers/pervcityutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []pervcityutil.SiteConfig{
	{SiteID: "pervcity", Domain: "pervcity.com", Host: "https://pervcity.com", StudioName: "PervCity", PathStem: "updates"},
	{SiteID: "analoverdose", Domain: "analoverdose.com", Host: "https://analoverdose.com", StudioName: "Anal Overdose", PathStem: "movies"},
	{SiteID: "upherasshole", Domain: "upherasshole.com", Host: "https://upherasshole.com", StudioName: "Up Her Asshole", PathStem: "movies"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(pervcityutil.New(cfg))
	}
}
