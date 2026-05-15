package wankz

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/wankzutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

var sites = []siteConfig{
	{"wankz", "wankz.com", "Wankz"},
	{"lethalpass", "lethalpass.com", "Lethal Pass"},
}

type siteScraper struct {
	wankz    *wankzutil.Scraper
	config   siteConfig
	matchRe  *regexp.Regexp
	patterns []string
}

func (s *siteScraper) ID() string               { return s.config.SiteID }
func (s *siteScraper) Patterns() []string       { return s.patterns }
func (s *siteScraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.wankz.Run(ctx, studioURL, opts, out)
	return out, nil
}

func init() {
	for _, cfg := range sites {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped))

		s := &siteScraper{
			wankz: wankzutil.NewScraper(wankzutil.SiteConfig{
				SiteID:     cfg.SiteID,
				SiteBase:   "https://www." + cfg.Domain,
				StudioName: cfg.StudioName,
			}),
			config:  cfg,
			matchRe: re,
			patterns: []string{
				cfg.Domain,
				cfg.Domain + "/channels/{slug}",
			},
		}
		scraper.Register(s)
	}
}
