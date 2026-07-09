// Package amateurallureclassics scrapes Amateur Allure Classics
// (amateurallureclassics.com), a child site of Amateur Allure. Like its sister
// Swallow Salon it runs on the ElevatedX "Classic" template served from the
// document root (update_details cards, /scenes/{slug}_vids.html detail pages,
// /categories/movies_{N}_d.html pagination).
package amateurallureclassics

import (
	"github.com/Wasylq/FSS/internal/scrapers/julesjordanutil"
	"github.com/Wasylq/FSS/scraper"
)

func New() *julesjordanutil.Scraper {
	return julesjordanutil.New(julesjordanutil.SiteConfig{
		SiteID:     "amateurallureclassics",
		Domain:     "amateurallureclassics.com",
		StudioName: "Amateur Allure Classics",
		Template:   julesjordanutil.TemplateClassic,
		BasePath:   "/",
	})
}

func init() { scraper.Register(New()) }
