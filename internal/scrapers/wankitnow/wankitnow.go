package wankitnow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/wankitnowutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

var sites = []siteConfig{
	{"wankitnow", "wankitnow.com", "Wank It Now"},
	{"boppingbabes", "boppingbabes.com", "Bopping Babes"},
	{"downblousejerk", "downblousejerk.com", "Downblouse Jerk"},
	{"lingerietales", "lingerietales.com", "Lingerie Tales"},
	{"realbikinigirls", "realbikinigirls.com", "Real Bikini Girls"},
	{"upskirtjerk", "upskirtjerk.com", "Upskirt Jerk"},
}

func init() {
	for _, cfg := range sites {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped))

		s := wankitnowutil.New(wankitnowutil.SiteConfig{
			ID:       cfg.SiteID,
			Domain:   cfg.Domain,
			Studio:   cfg.StudioName,
			Patterns: []string{cfg.Domain},
			MatchRe:  re,
		})
		scraper.Register(s)
	}
}
