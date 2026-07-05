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
	{SiteID: "familystrokes", Domain: "familystrokes.com", SiteBase: "https://www.familystrokes.com", Index: "familybundle", NickName: "family-strokes"},
	{SiteID: "freeuse", Domain: "freeuse.com", SiteBase: "https://www.freeuse.com", Index: "freeusebundle"},
	{SiteID: "pervz", Domain: "pervz.com", SiteBase: "https://www.pervz.com", Index: "pervbundle"},
	{SiteID: "shoplyfter", Domain: "shoplyfter.com", SiteBase: "https://www.shoplyfter.com", Index: "pervbundle", NickName: "shoplyfter"},
	{SiteID: "swappz", Domain: "swappz.com", SiteBase: "https://www.swappz.com", Index: "swap_bundle"},
	{SiteID: "dadcrush", Domain: "dadcrush.com", SiteBase: "https://www.dadcrush.com", Index: "ts_network", NickName: "dadcrush"},
	{SiteID: "sislovesme", Domain: "sislovesme.com", SiteBase: "https://www.sislovesme.com", Index: "ts_network", NickName: "sislovesme"},
	{SiteID: "exxxtrasmall", Domain: "exxxtrasmall.com", SiteBase: "https://www.exxxtrasmall.com", Index: "ts_network", NickName: "exxxtrasmall"},
	{SiteID: "bffs", Domain: "bffs.com", SiteBase: "https://www.bffs.com", Index: "ts_network", NickName: "bffs"},
	{SiteID: "innocenthigh", Domain: "innocenthigh.com", SiteBase: "https://www.innocenthigh.com", Index: "ts_network", NickName: "innocenthigh"},
	{SiteID: "dyked", Domain: "dyked.com", SiteBase: "https://www.dyked.com", Index: "ts_network", NickName: "dyked"},
	{SiteID: "oyeloca", Domain: "oyeloca.com", SiteBase: "https://www.oyeloca.com", Index: "ts_network", NickName: "oyeloca"},
	{SiteID: "daughterswap", Domain: "daughterswap.com", SiteBase: "https://www.daughterswap.com", Index: "ts_network", NickName: "daughterswap"},
	{SiteID: "teenpies", Domain: "teenpies.com", SiteBase: "https://www.teenpies.com", Index: "ts_network", NickName: "teenpies"},
	{SiteID: "povlife", Domain: "povlife.com", SiteBase: "https://www.povlife.com", Index: "ts_network", NickName: "povlife"},
	{SiteID: "therealworkout", Domain: "therealworkout.com", SiteBase: "https://www.therealworkout.com", Index: "ts_network", NickName: "therealworkout"},
	{SiteID: "tittyattack", Domain: "tittyattack.com", SiteBase: "https://www.tittyattack.com", Index: "ts_network", NickName: "titty-attack"},
	{SiteID: "teencurves", Domain: "teencurves.com", SiteBase: "https://www.teencurves.com", Index: "ts_network", NickName: "teen-curves"},
	{SiteID: "badmilfs", Domain: "badmilfs.com", SiteBase: "https://www.badmilfs.com", Index: "ts_network", NickName: "bad-milfs"},
	{SiteID: "teensloveblackcocks", Domain: "teensloveblackcocks.com", SiteBase: "https://www.teensloveblackcocks.com", Index: "ts_network", NickName: "teens-love-black-cocks"},
	{SiteID: "teensloveanal", Domain: "teensloveanal.com", SiteBase: "https://www.teensloveanal.com", Index: "ts_network", NickName: "teens-love-anal"},
	{SiteID: "mybabysittersclub", Domain: "mybabysittersclub.com", SiteBase: "https://www.mybabysittersclub.com", Index: "ts_network", NickName: "my-babysitters-club"},
	{SiteID: "blackvalleygirls", Domain: "blackvalleygirls.com", SiteBase: "https://www.blackvalleygirls.com", Index: "ts_network", NickName: "black-valley-girls"},
	{SiteID: "bracefaced", Domain: "bracefaced.com", SiteBase: "https://www.bracefaced.com", Index: "ts_network", NickName: "bracefaced"},
	{SiteID: "gingerpatch", Domain: "gingerpatch.com", SiteBase: "https://www.gingerpatch.com", Index: "ts_network", NickName: "gingerpatch"},
	{SiteID: "hijabhookup", Domain: "hijabhookup.com", SiteBase: "https://www.hijabhookup.com", Index: "ts_network", NickName: "hijab-hookup"},
	{SiteID: "mormongirlz", Domain: "mormongirlz.com", SiteBase: "https://www.mormongirlz.com", Index: "ts_network", NickName: "mormon-girlz"},
	{SiteID: "littleasians", Domain: "littleasians.com", SiteBase: "https://www.littleasians.com", Index: "ts_network", NickName: "little-asians"},
	{SiteID: "shesnew", Domain: "shesnew.com", SiteBase: "https://www.shesnew.com", Index: "ts_network", NickName: "shesnew"},
	{SiteID: "thickumz", Domain: "thickumz.com", SiteBase: "https://www.thickumz.com", Index: "ts_network", NickName: "thickumz"},
	{SiteID: "cfnmteens", Domain: "cfnmteens.com", SiteBase: "https://www.cfnmteens.com", Index: "ts_network", NickName: "cfnm-teens"},
	{SiteID: "breedingmaterial", Domain: "breedingmaterial.com", SiteBase: "https://www.breedingmaterial.com", Index: "ts_network", NickName: "breeding-material"},
	{SiteID: "stayhomepov", Domain: "stayhomepov.com", SiteBase: "https://www.stayhomepov.com", Index: "ts_network", NickName: "stay-home-pov"},
	{SiteID: "submissived", Domain: "submissived.com", SiteBase: "https://www.submissived.com", Index: "ts_network"},
	{SiteID: "fostertapes", Domain: "fostertapes.com", SiteBase: "https://www.fostertapes.com", Index: "ts_network", NickName: "fostertapes"},
	{SiteID: "punishteens", Domain: "punishteens.com", SiteBase: "https://www.punishteens.com", Index: "ts_network"},
	{SiteID: "pervmom", Domain: "pervmom.com", SiteBase: "https://www.pervmom.com", Index: "ts_network", NickName: "pervmom"},
	{SiteID: "pervtherapy", Domain: "pervtherapy.com", SiteBase: "https://www.pervtherapy.com", Index: "ts_network", NickName: "pervtherapy"},
	{SiteID: "pervdoctor", Domain: "pervdoctor.com", SiteBase: "https://www.pervdoctor.com", Index: "ts_network", NickName: "pervdoctor"},
	{SiteID: "notmygrandpa", Domain: "notmygrandpa.com", SiteBase: "https://www.notmygrandpa.com", Index: "ts_network", NickName: "not-my-grandpa"},
	{SiteID: "freakyfembots", Domain: "freakyfembots.com", SiteBase: "https://www.freakyfembots.com", Index: "ts_network", NickName: "freakyfembots"},
	{SiteID: "sisswap", Domain: "sisswap.com", SiteBase: "https://www.sisswap.com", Index: "ts_network", NickName: "sis-swap"},
	{SiteID: "tinysis", Domain: "tinysis.com", SiteBase: "https://www.tinysis.com", Index: "ts_network", NickName: "tiny-sis"},
	{SiteID: "freeusefantasy", Domain: "freeusefantasy.com", SiteBase: "https://www.freeusefantasy.com", Index: "ts_network", NickName: "freeuse-fantasy"},

	// SayUncle network
	{SiteID: "sayuncle", Domain: "sayuncle.com", SiteBase: "https://www.sayuncle.com", Index: "sau_network", ScenePath: "/movies/"},
}

type siteScraper struct {
	config  teamskeetutil.SiteConfig
	matchRe *regexp.Regexp
	inner   *teamskeetutil.Scraper
}

var _ scraper.StudioScraper = (*siteScraper)(nil)

func newSiteScraper(cfg teamskeetutil.SiteConfig) *siteScraper {
	re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(?:/|$)`, strings.ReplaceAll(cfg.Domain, ".", `\.`)))
	return &siteScraper{
		config:  cfg,
		matchRe: re,
		inner:   teamskeetutil.New(cfg),
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
