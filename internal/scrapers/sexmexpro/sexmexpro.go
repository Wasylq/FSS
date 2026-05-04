package sexmexpro

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/sexmexutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

var sites = []siteConfig{
	{"exposedlatinas", "exposedlatinas.com", "Exposed Latinas"},
	{"sexmex", "sexmex.xxx", "SexMex"},
	{"transqueens", "transqueens.com", "Trans Queens"},
}

func init() {
	for _, cfg := range sites {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(/|$)`, escaped))

		s := sexmexutil.New(sexmexutil.SiteConfig{
			ID:       cfg.SiteID,
			Studio:   cfg.StudioName,
			SiteBase: "https://" + cfg.Domain,
			Patterns: []string{
				cfg.Domain,
				cfg.Domain + "/tour/updates",
				cfg.Domain + "/tour/models/{slug}.html",
				cfg.Domain + "/tour/categories/{slug}.html",
			},
			MatchRe: re,
		})
		scraper.Register(s)
	}
}
