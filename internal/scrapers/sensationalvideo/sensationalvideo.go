package sensationalvideo

import (
	"github.com/Wasylq/FSS/internal/scrapers/masutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []masutil.SiteConfig{
	{SiteID: "plumperpass", Domain: "plumperpass.com", PageID: "584", Base: "https://www.plumperpass.com"},
	{SiteID: "bbwdreams", Domain: "bbwdreams.com", PageID: "788", Base: "https://www.bbwdreams.com"},
	{SiteID: "hotsexyplumpers", Domain: "hotsexyplumpers.com", PageID: "786", Base: "https://www.hotsexyplumpers.com"},
	{SiteID: "bbwsgoneblack", Domain: "bbwsgoneblack.com", PageID: "787", Base: "https://www.bbwsgoneblack.com"},
	{SiteID: "bbwland", Domain: "bbwland.com", PageID: "809", Base: "https://www.bbwland.com"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(masutil.New(cfg))
	}
}
