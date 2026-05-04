package povr

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/povrutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

var sites = []siteConfig{
	{"brasilvr", "brasilvr.com", "BrasilVR"},
	{"milfvr", "milfvr.com", "MilfVR"},
	{"tranzvr", "tranzvr.com", "TranzVR"},
	{"wankzvr", "wankzvr.com", "WankzVR"},
}

func init() {
	for _, cfg := range sites {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(/|$)`, escaped))

		s := povrutil.New(povrutil.SiteConfig{
			ID:       cfg.SiteID,
			Studio:   cfg.StudioName,
			SiteBase: "https://www." + cfg.Domain,
			Patterns: []string{cfg.Domain},
			MatchRe:  re,
		})
		scraper.Register(s)
	}
}
