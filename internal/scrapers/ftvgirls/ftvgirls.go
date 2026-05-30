package ftvgirls

import (
	"github.com/Wasylq/FSS/internal/scrapers/ftvutil"
	"github.com/Wasylq/FSS/scraper"
)

var s = ftvutil.New(ftvutil.SiteConfig{
	SiteID:    "ftvgirls",
	Domain:    "ftvgirls.com",
	Studio:    "FTV Girls",
	TitleSite: "FTVGirls.com",
})

func New() *ftvutil.Scraper { return s }

func init() { scraper.Register(s) }
