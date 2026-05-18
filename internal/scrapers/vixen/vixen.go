package vixen

import (
	"github.com/Wasylq/FSS/internal/scrapers/vixenutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []vixenutil.SiteConfig{
	{SiteID: "vixen", Domain: "vixen.com", StudioName: "Vixen"},
	{SiteID: "blacked", Domain: "blacked.com", StudioName: "Blacked"},
	{SiteID: "blackedraw", Domain: "blackedraw.com", StudioName: "Blacked Raw"},
	{SiteID: "tushy", Domain: "tushy.com", StudioName: "Tushy"},
	{SiteID: "tushyraw", Domain: "tushyraw.com", StudioName: "Tushy Raw"},
	{SiteID: "deeper", Domain: "deeper.com", StudioName: "Deeper"},
	{SiteID: "slayed", Domain: "slayed.com", StudioName: "Slayed"},
	{SiteID: "milfy", Domain: "milfy.com", StudioName: "Milfy"},
	{SiteID: "wifey", Domain: "wifey.com", StudioName: "Wifey"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(vixenutil.New(cfg))
	}
}
