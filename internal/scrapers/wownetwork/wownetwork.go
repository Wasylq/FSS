package wownetwork

import (
	"github.com/Wasylq/FSS/internal/scrapers/wownetworkutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []wownetworkutil.SiteConfig{
	{SiteID: "wowgirls", Domain: "wowgirls.com", StudioName: "WowGirls", AltDomains: []string{"wowporn.com"}},
	{SiteID: "ultrafilms", Domain: "ultrafilms.com", StudioName: "Ultra Films"},
	{SiteID: "angelslove", Domain: "angels.love", StudioName: "angels.love"},
	{SiteID: "sensuallove", Domain: "sensual.love", StudioName: "sensual.love"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(wownetworkutil.New(cfg))
	}
}
