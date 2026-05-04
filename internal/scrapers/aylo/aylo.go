package aylo

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/ayloutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID        string
	Domain        string
	StudioName    string
	ExtraPatterns []string // additional patterns before the common ones
}

var sites = []siteConfig{
	{"babes", "babes.com", "Babes", []string{"babes.com/model/{id}/{slug}"}},
	{"brazzers", "brazzers.com", "Brazzers", nil},
	{"digitalplayground", "digitalplayground.com", "Digital Playground", []string{"digitalplayground.com/modelprofile/{id}/{slug}"}},
	{"mofos", "mofos.com", "Mofos", []string{"mofos.com/model/{id}/{slug}"}},
	{"propertysex", "propertysex.com", "PropertySex", nil},
	{"realitykings", "realitykings.com", "Reality Kings", []string{"realitykings.com/model/{id}/{slug}"}},
	{"transangels", "transangels.com", "TransAngels", nil},
	{"twistys", "twistys.com", "Twistys", nil},
}

type siteScraper struct {
	aylo     *ayloutil.Scraper
	config   siteConfig
	matchRe  *regexp.Regexp
	patterns []string
}

func (s *siteScraper) ID() string               { return s.config.SiteID }
func (s *siteScraper) Patterns() []string        { return s.patterns }
func (s *siteScraper) MatchesURL(u string) bool  { return s.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.aylo.Run(ctx, studioURL, opts, out)
	return out, nil
}

func init() {
	for _, cfg := range sites {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped))

		patterns := []string{cfg.Domain}
		patterns = append(patterns, cfg.ExtraPatterns...)
		patterns = append(patterns,
			cfg.Domain+"/pornstar/{id}/{slug}",
			cfg.Domain+"/category/{id}/{slug}",
			cfg.Domain+"/site/{id}/{slug}",
			cfg.Domain+"/series/{id}/{slug}",
		)

		ayloCfg := ayloutil.SiteConfig{
			SiteID:     cfg.SiteID,
			SiteBase:   "https://www." + cfg.Domain,
			StudioName: cfg.StudioName,
		}

		s := &siteScraper{
			aylo:     ayloutil.NewScraper(ayloCfg),
			config:   cfg,
			matchRe:  re,
			patterns: patterns,
		}
		scraper.Register(s)
	}
}
