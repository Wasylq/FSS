package ftvgirls

import (
	"github.com/Anastylosis/FSS/internal/scrapers/ftvutil"
	"github.com/Anastylosis/FSS/scraper"
)

var s = ftvutil.New(ftvutil.SiteConfig{
	SiteID:    "ftvgirls",
	Domain:    "ftvgirls.com",
	Studio:    "FTV Girls",
	TitleSite: "FTVGirls.com",
})

func New() *ftvutil.Scraper { return s }

func init() { scraper.Register(s) }
