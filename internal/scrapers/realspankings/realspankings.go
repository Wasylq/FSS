package realspankings

import (
	"github.com/Wasylq/FSS/internal/scrapers/realspankingsutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []realspankingsutil.SiteConfig{
	{SiteID: "realspankingsinstitute", Domain: "www.realspankingsinstitute.com", StudioName: "Real Spankings Institute", Type: realspankingsutil.TypeRSI},
	{SiteID: "spankedcoeds", Domain: "spankedcoeds.com", StudioName: "Spanked Coeds", Type: realspankingsutil.TypeSpankedCoeds},
	{SiteID: "spankingteenbrandi", Domain: "spankingteenbrandi.com", StudioName: "Spanking Teen Brandi", Type: realspankingsutil.TypeSTB},
	{SiteID: "spankingteenjessica", Domain: "spankingteenjessica.com", StudioName: "Spanking Teen Jessica", Type: realspankingsutil.TypeSTJ},
	{SiteID: "spankingbailey", Domain: "spankingbailey.com", StudioName: "Spanking Bailey", Type: realspankingsutil.TypeBailey},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(realspankingsutil.NewScraper(cfg))
	}
}
