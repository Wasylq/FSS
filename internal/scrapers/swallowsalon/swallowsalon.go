// Package swallowsalon scrapes Swallow Salon (swallowsalon.com), a child site
// of Amateur Allure. It runs on the same ElevatedX "Classic" template as the
// Jules Jordan Network sister sites (update_details cards, /scenes/{slug}_vids.html
// detail pages, /categories/movies_{N}_d.html pagination), but serves the tour
// from the document root rather than the usual "/trial" prefix.
package swallowsalon

import (
	"github.com/Wasylq/FSS/internal/scrapers/julesjordanutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *julesjordanutil.Scraper {
	return julesjordanutil.New(julesjordanutil.SiteConfig{
		SiteID:     "swallowsalon",
		Domain:     "swallowsalon.com",
		StudioName: "Swallow Salon",
		Template:   julesjordanutil.TemplateClassic,
		BasePath:   "/",
	})
}

func init() { scraper.Register(New()) }
