// Package justdanica scrapes justdanica.com.
//
// It runs the same Elevated X "classic tour" the Trix Video sites do —
// `update_details` cards carrying a `data-setid`, `/tour/updates/page_{N}.html`
// pagination and `/tour/updates/{slug}.html` detail pages — so it reuses that
// package's engine rather than restating the parser. The template is a vendor
// one; the Trix Video name is only where FSS first met it.
package justdanica

import (
	"github.com/Anastylosis/FSS/internal/scrapers/trixvideo"
	"github.com/Anastylosis/FSS/scraper"
)

func New() *trixvideo.Scraper {
	return trixvideo.New(trixvideo.SiteConfig{
		SiteID:     "justdanica",
		Domain:     "justdanica.com",
		StudioName: "Just Danica",
		// This tour writes DD/MM/YYYY — its `update_date` values run to 30 in
		// the first position and never past 12 in the second, the opposite of
		// every other site on the template.
		DateLayout: "02/01/2006",
	})
}

func init() { scraper.Register(New()) }
