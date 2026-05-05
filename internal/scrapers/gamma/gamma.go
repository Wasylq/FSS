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
	SiteID      string
	Domain      string
	StudioName  string
	SiteName    string // Algolia availableOnSite filter; defaults to SiteID if empty
	RefererBase string // override for API key bootstrap (network scrapers)
	MatchRe     string // optional: override the default domain-based match regex
}

var sites = []siteConfig{
	// Adult Time segment — originals (more specific match, must be before adulttime)
	{"adulttimeoriginals", "adulttime.com", "Adult Time Originals", "adulttime", "", `^https?://(?:www\.)?adulttime\.com/en/(?:studio|channel)/adult-time(?:-originals)?(?:/|$)`},

	// Adult Time segment — full catalog (all content in the segment)
	{"adulttime", "adulttime.com", "", "", "", ""},

	// Adult Time segment — individual sites
	{"burningangel", "burningangel.com", "Burning Angel", "", "", ""},
	{"evilangel", "evilangel.com", "Evil Angel", "", "", ""},
	{"tsfactor", "tsfactor.com", "TS Factor", "tsfactor", "", ""},
	{"pansexualx", "pansexualx.com", "PansexualX", "pansexualx", "", ""},
	{"transgressivexxx", "transgressivexxx.com", "TransgressiveXXX", "transgressivexxx", "", ""},

	// Evil Angel Network segment (evilangelnetwork) — director-branded sub-sites
	{"buttman", "buttman.com", "Buttman", "buttman", "", ""},
	{"analtrixxx", "analtrixxx.com", "AnalTriXXX", "analtrixxx", "", ""},
	{"jonnidarkkoxxx", "jonnidarkkoxxx.com", "Jonni Darkko XXX", "jonnidarkkoxxx", "", ""},
	{"latexplaytime", "latexplaytime.com", "Latex Playtime", "latexplaytime", "", ""},
	{"transsexualangel", "transsexualangel.com", "Transsexual Angel", "transsexualangel", "", ""},
	{"filthykings", "filthykings.com", "Filthy Kings", "", "", ""},
	{"gangbangcreampie", "gangbangcreampie.com", "Gangbang Creampie", "", "", ""},
	{"girlfriendsfilms", "girlfriendsfilms.com", "Girlfriends Films", "", "", ""},
	{"gloryholesecrets", "gloryholesecrets.com", "Gloryhole Secrets", "", "", ""},
	{"lethalhardcore", "lethalhardcore.com", "Lethal Hardcore", "", "", ""},
	{"mommyblowsbest", "mommyblowsbest.com", "Mommy Blows Best", "", "", ""},
	{"puretaboo", "puretaboo.com", "Pure Taboo", "", "", ""},
	{"roccosiffredi", "roccosiffredi.com", "Rocco Siffredi", "", "", ""},
	{"tabooheat", "tabooheat.com", "Taboo Heat", "", "", ""},
	{"wicked", "wicked.com", "Wicked", "", "", ""},

	// Dogfart segment (dfxtra) — 17 subsites under dogfartnetwork.com
	{"dogfartnetwork", "dogfartnetwork.com", "", "", "", ""},

	// OpenLife segment — 12 subsites under openlife.com
	{"openlife", "openlife.com", "", "", "", ""},
}

type siteScraper struct {
	gamma   *gammautil.Scraper
	config  siteConfig
	matchRe *regexp.Regexp
}

func (s *siteScraper) ID() string               { return s.config.SiteID }
func (s *siteScraper) Patterns() []string       { return []string{s.config.Domain} }
func (s *siteScraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.gamma.Run(ctx, studioURL, opts, out)
	return out, nil
}

func init() {
	for _, cfg := range sites {
		var re *regexp.Regexp
		if cfg.MatchRe != "" {
			re = regexp.MustCompile(cfg.MatchRe)
		} else {
			escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
			re = regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped))
		}

		siteName := cfg.SiteName
		if siteName == "" && cfg.StudioName != "" {
			siteName = cfg.SiteID
		}

		gammaCfg := gammautil.SiteConfig{
			SiteID:      cfg.SiteID,
			SiteBase:    "https://www." + cfg.Domain,
			StudioName:  cfg.StudioName,
			SiteName:    siteName,
			RefererBase: cfg.RefererBase,
		}

		s := &siteScraper{
			gamma:   gammautil.NewScraper(gammaCfg),
			config:  cfg,
			matchRe: re,
		}
		scraper.Register(s)
	}
}
