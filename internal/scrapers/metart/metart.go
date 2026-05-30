package metart

import (
	"github.com/Wasylq/FSS/internal/scrapers/metartutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []metartutil.SiteConfig{
	{SiteID: "alsscan", Domain: "alsscan.com", StudioName: "ALS Scan"},
	{SiteID: "erroticaarchives", Domain: "errotica-archives.com", StudioName: "Errotica Archives", MoviesOnly: true},
	{SiteID: "eternaldesire", Domain: "eternaldesire.com", StudioName: "Eternal Desire"},
	{SiteID: "lovehairy", Domain: "lovehairy.com", StudioName: "Love Hairy"},
	{SiteID: "metart", Domain: "metart.com", StudioName: "MetArt"},
	{SiteID: "metartnetwork", Domain: "metartnetwork.com", StudioName: "MetArt Network"},
	{SiteID: "metartx", Domain: "metartx.com", StudioName: "MetArt X"},
	{SiteID: "rylskyart", Domain: "rylskyart.com", StudioName: "Rylsky Art"},
	{SiteID: "sexart", Domain: "sexart.com", StudioName: "SexArt"},
	{SiteID: "straplez", Domain: "straplez.com", StudioName: "Straplez"},
	{SiteID: "stunning18", Domain: "stunning18.com", StudioName: "Stunning 18"},
	{SiteID: "thelifeerotic", Domain: "thelifeerotic.com", StudioName: "The Life Erotic"},
	{SiteID: "vivthomas", Domain: "vivthomas.com", StudioName: "Viv Thomas"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(metartutil.New(cfg))
	}
}
