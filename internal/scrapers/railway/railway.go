package railway

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/railwayutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID     string
	SiteCode   string
	Domain     string
	StudioName string
}

var sites = []siteConfig{
	{"smokingerotica", "SE", "smokingerotica.com", "Smoking Erotica"},
	{"smokingmodels", "SM", "smokingmodels.com", "Smoking Models"},
	{"spankingglamour", "SPG", "spankingglamour.com", "Spanking Glamour"},
}

func init() {
	for _, cfg := range sites {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped))

		s := railwayutil.New(railwayutil.SiteConfig{
			ID:       cfg.SiteID,
			SiteCode: cfg.SiteCode,
			Studio:   cfg.StudioName,
			SiteBase: "https://" + cfg.Domain,
			Patterns: []string{
				cfg.Domain + "/#/models",
				cfg.Domain + "/#/models/{name}",
			},
			MatchRe: re,
		})
		scraper.Register(s)
	}
}
