package ftvmilfs

import (
	"github.com/Wasylq/FSS/internal/scrapers/ftvutil"
	"github.com/Wasylq/FSS/scraper"
)

var s = ftvutil.New(ftvutil.SiteConfig{
	SiteID:    "ftvmilfs",
	Domain:    "ftvmilfs.com",
	Studio:    "FTV MILFs",
	TitleSite: "FTVMilfs.com",
})

func New() *ftvutil.Scraper { return s }

func init() { scraper.Register(s) }
