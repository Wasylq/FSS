package teamskeet

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/teamskeetutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []teamskeetutil.SiteConfig{
	{SiteID: "teamskeet", Domain: "teamskeet.com", SiteBase: "https://www.teamskeet.com", Index: "ts_network"},
	{SiteID: "mylf", Domain: "mylf.com", SiteBase: "https://www.mylf.com", Index: "mylf_bundle"},
	{SiteID: "familystrokes", Domain: "familystrokes.com", SiteBase: "https://www.familystrokes.com", Index: "familybundle"},
	{SiteID: "freeuse", Domain: "freeuse.com", SiteBase: "https://www.freeuse.com", Index: "freeusebundle"},
	{SiteID: "pervz", Domain: "pervz.com", SiteBase: "https://www.pervz.com", Index: "pervbundle"},
	{SiteID: "shoplyfter", Domain: "shoplyfter.com", SiteBase: "https://www.shoplyfter.com", Index: "pervbundle"},
	{SiteID: "swappz", Domain: "swappz.com", SiteBase: "https://www.swappz.com", Index: "swap_bundle"},
	{SiteID: "dadcrush", Domain: "dadcrush.com", SiteBase: "https://www.dadcrush.com", Index: "ts_network"},
	{SiteID: "sislovesme", Domain: "sislovesme.com", SiteBase: "https://www.sislovesme.com", Index: "ts_network"},
	{SiteID: "exxxtrasmall", Domain: "exxxtrasmall.com", SiteBase: "https://www.exxxtrasmall.com", Index: "ts_network"},
	{SiteID: "bffs", Domain: "bffs.com", SiteBase: "https://www.bffs.com", Index: "ts_network"},
	{SiteID: "innocenthigh", Domain: "innocenthigh.com", SiteBase: "https://www.innocenthigh.com", Index: "ts_network"},
	{SiteID: "dyked", Domain: "dyked.com", SiteBase: "https://www.dyked.com", Index: "ts_network"},
	{SiteID: "oyeloca", Domain: "oyeloca.com", SiteBase: "https://www.oyeloca.com", Index: "ts_network"},
	{SiteID: "daughterswap", Domain: "daughterswap.com", SiteBase: "https://www.daughterswap.com", Index: "ts_network"},
	{SiteID: "teenpies", Domain: "teenpies.com", SiteBase: "https://www.teenpies.com", Index: "ts_network"},
	{SiteID: "povlife", Domain: "povlife.com", SiteBase: "https://www.povlife.com", Index: "ts_network"},
	{SiteID: "therealworkout", Domain: "therealworkout.com", SiteBase: "https://www.therealworkout.com", Index: "ts_network"},
	{SiteID: "tittyattack", Domain: "tittyattack.com", SiteBase: "https://www.tittyattack.com", Index: "ts_network"},
	{SiteID: "teencurves", Domain: "teencurves.com", SiteBase: "https://www.teencurves.com", Index: "ts_network"},
	{SiteID: "badmilfs", Domain: "badmilfs.com", SiteBase: "https://www.badmilfs.com", Index: "ts_network"},
	{SiteID: "teensloveblackcocks", Domain: "teensloveblackcocks.com", SiteBase: "https://www.teensloveblackcocks.com", Index: "ts_network"},
	{SiteID: "teensloveanal", Domain: "teensloveanal.com", SiteBase: "https://www.teensloveanal.com", Index: "ts_network"},
	{SiteID: "mybabysittersclub", Domain: "mybabysittersclub.com", SiteBase: "https://www.mybabysittersclub.com", Index: "ts_network"},
	{SiteID: "blackvalleygirls", Domain: "blackvalleygirls.com", SiteBase: "https://www.blackvalleygirls.com", Index: "ts_network"},
	{SiteID: "bracefaced", Domain: "bracefaced.com", SiteBase: "https://www.bracefaced.com", Index: "ts_network"},
	{SiteID: "gingerpatch", Domain: "gingerpatch.com", SiteBase: "https://www.gingerpatch.com", Index: "ts_network"},
	{SiteID: "hijabhookup", Domain: "hijabhookup.com", SiteBase: "https://www.hijabhookup.com", Index: "ts_network"},
	{SiteID: "mormongirlz", Domain: "mormongirlz.com", SiteBase: "https://www.mormongirlz.com", Index: "ts_network"},
	{SiteID: "littleasians", Domain: "littleasians.com", SiteBase: "https://www.littleasians.com", Index: "ts_network"},
	{SiteID: "shesnew", Domain: "shesnew.com", SiteBase: "https://www.shesnew.com", Index: "ts_network"},
	{SiteID: "thickumz", Domain: "thickumz.com", SiteBase: "https://www.thickumz.com", Index: "ts_network"},
	{SiteID: "cfnmteens", Domain: "cfnmteens.com", SiteBase: "https://www.cfnmteens.com", Index: "ts_network"},
	{SiteID: "breedingmaterial", Domain: "breedingmaterial.com", SiteBase: "https://www.breedingmaterial.com", Index: "ts_network"},
	{SiteID: "stayhomepov", Domain: "stayhomepov.com", SiteBase: "https://www.stayhomepov.com", Index: "ts_network"},
	{SiteID: "submissived", Domain: "submissived.com", SiteBase: "https://www.submissived.com", Index: "ts_network"},
	{SiteID: "fostertapes", Domain: "fostertapes.com", SiteBase: "https://www.fostertapes.com", Index: "ts_network"},
	{SiteID: "punishteens", Domain: "punishteens.com", SiteBase: "https://www.punishteens.com", Index: "ts_network"},
	{SiteID: "pervmom", Domain: "pervmom.com", SiteBase: "https://www.pervmom.com", Index: "ts_network"},
	{SiteID: "pervtherapy", Domain: "pervtherapy.com", SiteBase: "https://www.pervtherapy.com", Index: "ts_network"},
	{SiteID: "pervdoctor", Domain: "pervdoctor.com", SiteBase: "https://www.pervdoctor.com", Index: "ts_network"},
	{SiteID: "notmygrandpa", Domain: "notmygrandpa.com", SiteBase: "https://www.notmygrandpa.com", Index: "ts_network"},
	{SiteID: "freakyfembots", Domain: "freakyfembots.com", SiteBase: "https://www.freakyfembots.com", Index: "ts_network"},
	{SiteID: "sisswap", Domain: "sisswap.com", SiteBase: "https://www.sisswap.com", Index: "ts_network"},
	{SiteID: "tinysis", Domain: "tinysis.com", SiteBase: "https://www.tinysis.com", Index: "ts_network"},
	{SiteID: "freeusefantasy", Domain: "freeusefantasy.com", SiteBase: "https://www.freeusefantasy.com", Index: "ts_network"},
}

type siteScraper struct {
	config  teamskeetutil.SiteConfig
	matchRe *regexp.Regexp
	inner   *teamskeetutil.Scraper
}

func newSiteScraper(cfg teamskeetutil.SiteConfig) *siteScraper {
	re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, strings.ReplaceAll(cfg.Domain, ".", `\.`)))
	return &siteScraper{
		config:  cfg,
		matchRe: re,
		inner:   teamskeetutil.NewScraper(cfg),
	}
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newSiteScraper(cfg))
	}
}

func (s *siteScraper) ID() string { return s.config.SiteID }

func (s *siteScraper) Patterns() []string {
	return []string{
		s.config.Domain,
		s.config.Domain + "/models/{slug}",
		s.config.Domain + "/series/{slug}",
		s.config.Domain + "/categories/{name}",
	}
}

func (s *siteScraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.inner.Run(ctx, studioURL, opts, out)
	return out, nil
}
