// Package digitaljmedia registers every site of the Digital J Media / JapanHDV
// English-JAV network. All ten sites share one backend (and the
// digitaljmediautil scraper) but use per-site HTML markup; the util's Configs()
// supplies one SiteConfig per site.
package digitaljmedia

import (
	"github.com/Wasylq/FSS/internal/scrapers/digitaljmediautil"
	"github.com/Wasylq/FSS/scraper"
)

func init() {
	for _, cfg := range digitaljmediautil.Configs() {
		scraper.Register(digitaljmediautil.New(cfg))
	}
}
