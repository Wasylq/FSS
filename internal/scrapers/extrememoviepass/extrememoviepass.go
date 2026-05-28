// Package extrememoviepass registers scrapers for the Extreme Movie Pass
// affiliate network — 19 sister sites running the NATS modelfeature template
// rooted at /tour/. Parser logic lives in extrememoviepassutil.
//
// Out-of-scope:
//   - extrememoviepass.com itself: portal landing page, no /tour/ catalog.
//   - goldwinpass.com: same network but older ElevatedX template variant
//     (`data-setid` cards, `/tour/updates/page_N.html` pagination) — needs a
//     separate parser.
package extrememoviepass

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/extrememoviepassutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []extrememoviepassutil.SiteConfig{
	{
		ID:       "amateur18",
		SiteBase: "https://www.amateur18.tv",
		Studio:   "Amateur 18",
		Patterns: []string{"amateur18.tv", "amateur18.tv/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?amateur18\.tv`),
	},
	{
		ID:       "bigbreasttv",
		SiteBase: "https://www.bigbreast.tv",
		Studio:   "Big Breast TV",
		Patterns: []string{"bigbreast.tv", "bigbreast.tv/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?bigbreast\.tv`),
	},
	{
		ID:       "bosslesson",
		SiteBase: "https://www.bosslesson.com",
		Studio:   "Boss Lesson",
		Patterns: []string{"bosslesson.com", "bosslesson.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?bosslesson\.com`),
	},
	{
		ID:       "brazilpartyorgy",
		SiteBase: "https://www.brazilpartyorgy.com",
		Studio:   "Brazil Party Orgy",
		Patterns: []string{"brazilpartyorgy.com", "brazilpartyorgy.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?brazilpartyorgy\.com`),
	},
	{
		ID:       "bukkakeorgy",
		SiteBase: "https://www.bukkakeorgy.com",
		Studio:   "Bukkake Orgy",
		Patterns: []string{"bukkakeorgy.com", "bukkakeorgy.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?bukkakeorgy\.com`),
	},
	{
		ID:       "flexidolls",
		SiteBase: "https://www.flexidolls.com",
		Studio:   "Flexi Dolls",
		Patterns: []string{"flexidolls.com", "flexidolls.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?flexidolls\.com`),
	},
	{
		ID:       "fuckthosemoms",
		SiteBase: "https://www.fuckthosemoms.com",
		Studio:   "Fuck Those Moms",
		Patterns: []string{"fuckthosemoms.com", "fuckthosemoms.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?fuckthosemoms\.com`),
	},
	{
		ID:       "grannyguide",
		SiteBase: "https://www.grannyguide.com",
		Studio:   "Granny Guide",
		Patterns: []string{"grannyguide.com", "grannyguide.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?grannyguide\.com`),
	},
	{
		ID:       "mountainfuckfest",
		SiteBase: "https://www.mountainfuckfest.com",
		Studio:   "Mountain Fuck Fest",
		Patterns: []string{"mountainfuckfest.com", "mountainfuckfest.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?mountainfuckfest\.com`),
	},
	{
		ID:       "mybangvan",
		SiteBase: "https://www.mybangvan.com",
		Studio:   "MyBangVan",
		Patterns: []string{"mybangvan.com", "mybangvan.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?mybangvan\.com`),
	},
	{
		ID:       "pornonstage",
		SiteBase: "https://www.pornonstage.com",
		Studio:   "Porn On Stage",
		Patterns: []string{"pornonstage.com", "pornonstage.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?pornonstage\.com`),
	},
	{
		ID:       "realgangbangs",
		SiteBase: "https://www.realgangbangs.com",
		Studio:   "Real Gang Bang",
		Patterns: []string{"realgangbangs.com", "realgangbangs.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?realgangbangs\.com`),
	},
	{
		ID:       "scandalonstage",
		SiteBase: "https://www.scandalonstage.com",
		Studio:   "Scandal On Stage",
		Patterns: []string{"scandalonstage.com", "scandalonstage.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?scandalonstage\.com`),
	},
	{
		ID:       "sexflexvideo",
		SiteBase: "https://www.sexflexvideo.com",
		Studio:   "Sex Flex Video",
		Patterns: []string{"sexflexvideo.com", "sexflexvideo.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?sexflexvideo\.com`),
	},
	{
		ID:       "sexycuckold",
		SiteBase: "https://www.sexycuckold.com",
		Studio:   "SexyCuckold",
		Patterns: []string{"sexycuckold.com", "sexycuckold.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?sexycuckold\.com`),
	},
	{
		ID:       "slipperymassage",
		SiteBase: "https://www.slipperymassage.com",
		Studio:   "Slippery Massage",
		Patterns: []string{"slipperymassage.com", "slipperymassage.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?slipperymassage\.com`),
	},
	{
		ID:       "spandexporn",
		SiteBase: "https://www.spandexporn.com",
		Studio:   "Spandex Porn",
		Patterns: []string{"spandexporn.com", "spandexporn.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?spandexporn\.com`),
	},
	{
		ID:       "virtualxporn",
		SiteBase: "https://www.virtualxporn.com",
		Studio:   "Virtual X Porn",
		Patterns: []string{"virtualxporn.com", "virtualxporn.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?virtualxporn\.com`),
	},
	{
		ID:       "voyeurpapy",
		SiteBase: "https://www.voyeurpapy.com",
		Studio:   "Voyeur Papy",
		Patterns: []string{"voyeurpapy.com", "voyeurpapy.com/tour/"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?voyeurpapy\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(extrememoviepassutil.New(cfg))
	}
}
