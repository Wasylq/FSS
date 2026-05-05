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
	AltDomains    []string // additional domains that resolve to the same site
}

var sites = []siteConfig{
	{"babes", "babes.com", "Babes", []string{"babes.com/model/{id}/{slug}"}, nil},
	{"bigstr", "czechhunter.com", "BigStr", nil, nil},
	{"brazzers", "brazzers.com", "Brazzers", nil, nil},
	{"digitalplayground", "digitalplayground.com", "Digital Playground", []string{"digitalplayground.com/modelprofile/{id}/{slug}"}, []string{"digitalplaygroundnetwork.com"}},
	{"erito", "erito.com", "Erito", nil, nil},
	{"hentaipros", "hentaipros.com", "HentaiPros", nil, nil},
	{"killergram", "killergram.com", "Killergram", nil, nil},
	{"letsdoeit", "letsdoeit.com", "LetsDoeIt", nil, nil},
	{"metro", "shewillcheat.com", "Metro", nil, nil},
	{"milehigh", "milfed.com", "MileHigh", nil, nil},
	{"mofos", "mofos.com", "Mofos", []string{"mofos.com/model/{id}/{slug}"}, nil},
	{"propertysex", "propertysex.com", "PropertySex", nil, nil},
	{"realitydudes", "realitydudes.com", "RealityDudes", nil, nil},
	{"realitykings", "realitykings.com", "Reality Kings", []string{"realitykings.com/model/{id}/{slug}"}, []string{"rk.com"}},
	{"seancody", "seancody.com", "Sean Cody", nil, nil},
	{"squirted", "squirted.com", "Squirted", nil, nil},
	{"transangels", "transangels.com", "TransAngels", nil, nil},
	{"twistys", "twistys.com", "Twistys", nil, nil},
	{"whynotbi", "men.com", "WhyNotBi", nil, nil},
}

type siteScraper struct {
	aylo     *ayloutil.Scraper
	config   siteConfig
	matchRe  *regexp.Regexp
	patterns []string
}

func (s *siteScraper) ID() string               { return s.config.SiteID }
func (s *siteScraper) Patterns() []string       { return s.patterns }
func (s *siteScraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.aylo.Run(ctx, studioURL, opts, out)
	return out, nil
}

func init() {
	for _, cfg := range sites {
		allDomains := append([]string{cfg.Domain}, cfg.AltDomains...)
		var reparts []string
		for _, d := range allDomains {
			reparts = append(reparts, strings.ReplaceAll(d, ".", `\.`))
		}
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?(?:%s)`, strings.Join(reparts, "|")))

		patterns := []string{cfg.Domain}
		patterns = append(patterns, cfg.AltDomains...)
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
