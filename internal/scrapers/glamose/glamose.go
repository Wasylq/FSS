package glamose

import (
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/internal/scrapers/utgutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []utgutil.SiteConfig{
	// New template (Tailwind, <article> tags, /updates/videos/{page}/{perPage})
	{SiteID: "hayleyssecrets", Domain: "hayleyssecrets.com", StudioName: "Hayley's Secrets"},
	{SiteID: "morethannylons", Domain: "morethannylons.com", StudioName: "More Than Nylons"},
	{SiteID: "skintightglamour", Domain: "skintightglamour.com", StudioName: "Skin Tight Glamour"},
	{SiteID: "uktickling", Domain: "uktickling.com", StudioName: "UK Tickling"},
	{SiteID: "worshipjasmine", Domain: "worshipjasmine.com", StudioName: "Worship Jasmine"},
	// Legacy template (Bootstrap 3, page-based pagination)
	{SiteID: "allbrookwright", Domain: "allbrookwright.com", StudioName: "All Brook Wright", Legacy: true},
	{SiteID: "bethmorganofficial", Domain: "bethmorganofficial.com", StudioName: "Beth Morgan Official", Legacy: true},
	{SiteID: "sophiassexylegwear", Domain: "sophiassexylegwear.com", StudioName: "Sophia's Sexy Legwear", Legacy: true},
	// Legacy template (Bootstrap 3, year-based pagination)
	{SiteID: "breathtakers", Domain: "breath-takers.com", StudioName: "BreathTakers", Legacy: true, YearBased: true},
	{SiteID: "girlfolio", Domain: "girlfolio.com", StudioName: "Girlfolio", Legacy: true, YearBased: true},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(utgutil.New(cfg))
	}
	scraper.Register(&portalScraper{client: httpx.NewClient(30 * time.Second)})
}
