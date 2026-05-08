package julesjordan

import (
	"github.com/Wasylq/FSS/internal/scrapers/julesjordanutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []julesjordanutil.SiteConfig{
	{SiteID: "julesjordan", Domain: "julesjordan.com", StudioName: "Jules Jordan", Template: julesjordanutil.TemplateJJ},
	{SiteID: "manuelferrara", Domain: "manuelferrara.com", StudioName: "Manuel Ferrara", Template: julesjordanutil.TemplateClassic},
	{SiteID: "girlgirl", Domain: "girlgirl.com", StudioName: "Girl Girl", Template: julesjordanutil.TemplateClassic},
	{SiteID: "spermswallowers", Domain: "spermswallowers.com", StudioName: "Sperm Swallowers", Template: julesjordanutil.TemplateClassic},
	{SiteID: "theassfactory", Domain: "theassfactory.com", StudioName: "The Ass Factory", Template: julesjordanutil.TemplateModern},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(julesjordanutil.NewScraper(cfg))
	}
}
