package missax

import (
	"github.com/Anastylosis/FSS/internal/scrapers/missaxutil"
	"github.com/Anastylosis/FSS/scraper"
)

var sites = []missaxutil.SiteConfig{
	{SiteID: "missax", Domain: "missax.com", Studio: "MissaX"},
	{SiteID: "allherluv", Domain: "allherluv.com", Studio: "All Her Luv"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(missaxutil.New(cfg))
	}
}
