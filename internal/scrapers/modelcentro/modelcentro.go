package modelcentro

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/modelcentroutil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
	Performers []string // solo-performer sites
}

var sites = []siteConfig{
	{"backalleytoonz", "backalleytoonzonline.com", "Backalley Toonz", nil},
	{"bigjohnnyxxx", "bigjohnnyxxx.com", "Big Johnny XXX", nil},
	{"blackmoneyerotica", "blackmoneyerotica.com", "Black Money Erotica", nil},
	{"blackpynk", "blackpynk.com", "Black Pynk", nil},
	{"cumtrainer", "cumtrainer.com", "Cum Trainer", nil},
	{"facialcasting", "facialcasting.com", "Facial Casting", nil},
	{"lisariveraxo", "lisariveraxo.com", "Lisa Rivera XO", []string{"Lisa Rivera"}},
	{"monstermalesprod", "monstermalesprod.com", "Monster Males", nil},
	{"mugurporn", "mugurporn.com", "Mugur Porn", nil},
	{"naughtycolombia", "naughtycolombia.com", "Naughty Colombia", nil},
	{"nerdsofporn", "nerdsofporn.com", "Nerds of Porn", nil},
	{"peccatriciproduzioni", "peccatriciproduzioni.com", "Peccatrici Produzioni", nil},
	{"pennybarber", "pennybarber.com", "Penny Barber", []string{"Penny Barber"}},
	{"pervsmilfsnteens", "pervsmilfsnteens.com", "Pervs MILFs n Teens", nil},
	{"porntugal", "porntugal.com", "Porntugal", nil},
	{"pvgirls", "pvgirls.com", "Porn Valley Girls", nil},
	{"ricporter", "ricporter.tv", "Ric Porter", nil},
	{"sexyninarivera", "sexyninarivera.com", "Sexy Nina Rivera", []string{"Nina Rivera"}},
	{"sukmydick", "sukmydick.com", "Suk My Dick", nil},
	{"superhotfilms", "superhotfilms.com", "Super Hot Films", nil},
	{"thejerkygirls", "thejerkygirls.com", "The Jerky Girls", nil},
	{"thiccvision", "thiccvision.com", "Thicc Vision", nil},
	{"thicq", "thicq.com", "THICQ", nil},
	{"throatwars", "throatwars.com", "Throat Wars", nil},
	{"wetwetgirls", "wetwetgirls.com", "Wet Wet Girls", nil},
	{"yungdumbsluts", "yungdumbsluts.com", "Yung Dumb Sluts", nil},
}

type siteScraper struct {
	*modelcentroutil.Scraper
	matchRe *regexp.Regexp
}

func (s *siteScraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func init() {
	for _, cfg := range sites {
		escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
		re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(?:/videos)?/?(?:\?.*)?$`, escaped))

		s := &siteScraper{
			Scraper: modelcentroutil.New(modelcentroutil.SiteConfig{
				SiteID:     cfg.SiteID,
				SiteBase:   "https://" + cfg.Domain,
				StudioName: cfg.StudioName,
				Performers: cfg.Performers,
			}),
			matchRe: re,
		}
		scraper.Register(s)
	}
}
