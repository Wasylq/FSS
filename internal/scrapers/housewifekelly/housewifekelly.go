// Package housewifekelly scrapes housewifekelly.com.
//
// It runs the same Elevated X "classic tour" the Trix Video sites do —
// `update_details` cards carrying a `data-setid`, `/tour/updates/page_{N}.html`
// pagination and `/tour/updates/{slug}.html` detail pages — so it reuses that
// package's engine rather than restating the parser. The template is a vendor
// one; the Trix Video name is only where FSS first met it.
package housewifekelly

import (
	"github.com/Anastylosis/FSS/internal/scrapers/trixvideo"
	"github.com/Anastylosis/FSS/scraper"
)

func New() *trixvideo.Scraper {
	return trixvideo.New(trixvideo.SiteConfig{
		SiteID:     "housewifekelly",
		Domain:     "housewifekelly.com",
		StudioName: "Housewife Kelly",
	})
}

func init() { scraper.Register(New()) }
