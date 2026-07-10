// Package xxcel registers the XX-Cel network sites (xx-cel.com and its child
// Heavy On Hotties), which share the CMS handled by xxcelutil.
package xxcel

import (
	"github.com/Wasylq/FSS/internal/scrapers/xxcelutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []xxcelutil.SiteConfig{
	{SiteID: "xxcel", Domain: "xx-cel.com", Host: "https://xx-cel.com", StudioName: "XX-Cel"},
	{SiteID: "heavyonhotties", Domain: "heavyonhotties.com", Host: "https://www.heavyonhotties.com", StudioName: "Heavy On Hotties"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(xxcelutil.New(cfg))
	}
}
