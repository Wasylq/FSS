package ftvmilfs

import (
	"github.com/Anastylosis/FSS/internal/scrapers/ftvutil"
	"github.com/Anastylosis/FSS/scraper"
)

var s = ftvutil.New(ftvutil.SiteConfig{
	SiteID:    "ftvmilfs",
	Domain:    "ftvmilfs.com",
	Studio:    "FTV MILFs",
	TitleSite: "FTVMilfs.com",
})

func New() *ftvutil.Scraper { return s }

func init() { scraper.Register(s) }
