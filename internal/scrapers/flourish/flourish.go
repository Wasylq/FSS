package flourish

import (
	"github.com/Wasylq/FSS/internal/scrapers/flourishutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []flourishutil.SiteConfig{
	{SiteID: "theflourishxxx", Domain: "theflourishxxx.com", StudioName: "The Flourish XXX"},
	{SiteID: "theflourishpov", Domain: "theflourishpov.com", StudioName: "The Flourish POV"},
	{SiteID: "theflourishfetish", Domain: "theflourishfetish.com", StudioName: "The Flourish Fetish"},
	{SiteID: "theflourishamateurs", Domain: "theflourishamateurs.com", StudioName: "The Flourish Amateurs"},
	{SiteID: "milfcandy", Domain: "milfcandy.com", StudioName: "MILF Candy"},
	{SiteID: "curvyculturexxx", Domain: "curvyculturexxx.com", StudioName: "Curvy Culture XXX"},
	{SiteID: "gilfgasms", Domain: "gilfgasms.com", StudioName: "GILF Gasms"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(flourishutil.New(cfg))
	}
}
