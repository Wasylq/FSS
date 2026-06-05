package williamhiggins

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/whutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID      string
	Domain      string
	StudioName  string
	BackendSlug string // slug for backend.williamhiggins.com/{slug}/api/v1/; empty = use domain-based API
	DetailPath  string // scene URL path, e.g. "/set/detail/"
}

var sites = []siteConfig{
	{"williamhiggins", "williamhiggins.com", "William Higgins", "wh", "/set/detail/"},
	{"str8hell", "str8hell.com", "Str8 Hell", "str8", "/set/detail/"},
	{"malefeet4u", "malefeet4u.com", "MaleFeet4U", "mf4", "/set/detail/"},
	{"swnude", "swnude.com", "SWNude", "swnude", "/set/detail/"},
	{"cfnmeu", "cfnmeu.com", "CFNM EU", "cfnm", "/set/detail/"},
	{"ambushmassage", "ambushmassage.com", "Ambush Massage", "", "/detail/"},
}

type siteScraper struct {
	wh     *whutil.Scraper
	config siteConfig
	re     *regexp.Regexp
}

var _ scraper.StudioScraper = (*siteScraper)(nil)

func (s *siteScraper) ID() string              { return s.config.SiteID }
func (s *siteScraper) Patterns() []string      { return []string{s.config.Domain} }
func (s *siteScraper) MatchesURL(u string) bool { return s.re.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.wh.Run(ctx, studioURL, opts, out)
	return out, nil
}

func init() {
	for _, cfg := range sites {
		apiBase := fmt.Sprintf("https://backend.williamhiggins.com/%s/api/v1/", cfg.BackendSlug)
		if cfg.BackendSlug == "" {
			apiBase = fmt.Sprintf("https://www.%s/api/", cfg.Domain)
		}

		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(?:/|$)`, escaped))

		s := &siteScraper{
			wh: whutil.New(whutil.SiteConfig{
				SiteID:     cfg.SiteID,
				Domain:     cfg.Domain,
				StudioName: cfg.StudioName,
				APIBase:    apiBase,
				DetailPath: cfg.DetailPath,
			}),
			config: cfg,
			re:     re,
		}
		scraper.Register(s)
	}
}
