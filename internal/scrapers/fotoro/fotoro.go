// Package fotoro registers the Fotoro-network fetish sites, all running
// WordPress with an open REST API. See fotoroutil for the shared logic.
package fotoro

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/fotoroutil"
	"github.com/Wasylq/FSS/scraper"
)

type site struct {
	id     string
	domain string
	studio string
}

var sites = []site{
	{"chastitybabes", "chastitybabes.com", "Chastity Babes"},
	{"metalbondage", "metalbondage.com", "Metal Bondage"},
	{"hucows", "hucows.com", "HuCows"},
	{"shockchallenge", "shockchallenge.com", "Shock Challenge"},
	{"tieable", "tieable.com", "Tieable"},
	{"sybian1", "sybian1.com", "Sybian1"},
	{"girlasylum", "girlasylum.com", "Girl Asylum"},
}

func init() {
	for _, s := range sites {
		escaped := strings.ReplaceAll(s.domain, ".", `\.`)
		scraper.Register(fotoroutil.New(fotoroutil.SiteConfig{
			ID:       s.id,
			Studio:   s.studio,
			SiteBase: "https://www." + s.domain,
			Patterns: []string{s.domain, s.domain + "/tag/{slug}"},
			MatchRe:  regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped)),
		}))
	}
}
