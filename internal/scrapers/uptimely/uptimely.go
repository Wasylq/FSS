package uptimely

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/uptimelyutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

var sites = []siteConfig{
	{"dasdas", "dasdas.jp", "DAS!"},
	{"ideapocket", "ideapocket.com", "Idea Pocket"},
	{"madonna", "madonna-av.com", "Madonna"},
	{"moodyz", "moodyz.com", "MOODYZ"},
	{"s1no1style", "s1s1s1.com", "S1 NO.1 STYLE"},
}

func init() {
	for _, cfg := range sites {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s/(?:works/list/|actress/detail/)`, escaped))

		s := uptimelyutil.New(uptimelyutil.SiteConfig{
			ID:     cfg.SiteID,
			Studio: cfg.StudioName,
			Domain: cfg.Domain,
			Patterns: []string{
				cfg.Domain + "/works/list/series/{id}",
				cfg.Domain + "/works/list/release",
				cfg.Domain + "/works/list/date/{date}",
				cfg.Domain + "/works/list/genre/{id}",
				cfg.Domain + "/works/list/label/{id}",
				cfg.Domain + "/actress/detail/{id}",
			},
			MatchRe: re,
		})
		scraper.Register(s)
	}
}
