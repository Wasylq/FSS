package gamma

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/gammautil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

var sites = []siteConfig{
	{"burningangel", "burningangel.com", "Burning Angel"},
	{"evilangel", "evilangel.com", "Evil Angel"},
	{"filthykings", "filthykings.com", "Filthy Kings"},
	{"gangbangcreampie", "gangbangcreampie.com", "Gangbang Creampie"},
	{"girlfriendsfilms", "girlfriendsfilms.com", "Girlfriends Films"},
	{"gloryholesecrets", "gloryholesecrets.com", "Gloryhole Secrets"},
	{"lethalhardcore", "lethalhardcore.com", "Lethal Hardcore"},
	{"mommyblowsbest", "mommyblowsbest.com", "Mommy Blows Best"},
	{"puretaboo", "puretaboo.com", "Pure Taboo"},
	{"roccosiffredi", "roccosiffredi.com", "Rocco Siffredi"},
	{"tabooheat", "tabooheat.com", "Taboo Heat"},
	{"wicked", "wicked.com", "Wicked"},
}

type siteScraper struct {
	gamma   *gammautil.Scraper
	config  siteConfig
	matchRe *regexp.Regexp
}

func (s *siteScraper) ID() string              { return s.config.SiteID }
func (s *siteScraper) Patterns() []string      { return []string{s.config.Domain} }
func (s *siteScraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.gamma.Run(ctx, studioURL, opts, out)
	return out, nil
}

func init() {
	for _, cfg := range sites {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped))

		gammaCfg := gammautil.SiteConfig{
			SiteID:     cfg.SiteID,
			SiteBase:   "https://www." + cfg.Domain,
			StudioName: cfg.StudioName,
			SiteName:   cfg.SiteID,
		}

		s := &siteScraper{
			gamma:   gammautil.NewScraper(gammaCfg),
			config:  cfg,
			matchRe: re,
		}
		scraper.Register(s)
	}
}
