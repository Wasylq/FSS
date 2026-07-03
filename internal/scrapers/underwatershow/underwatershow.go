// Package underwatershow registers the RevshareCash "jscroll" tour sites
// (Underwater Show, Anal-Coach), all driven by the load_pics.php AJAX feed.
// See underwatershowutil.
package underwatershow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/underwatershowutil"
	"github.com/Wasylq/FSS/scraper"
)

type site struct {
	id       string
	domain   string
	studio   string
	loadPath string
}

var sites = []site{
	{"underwatershow", "underwatershow.com", "Underwater Show", "load_pics.php"},
	{"analcoach", "anal-coach.com", "Anal-Coach", "modules/load_pics.php"},
}

func newScraper(s site) *underwatershowutil.Scraper {
	escaped := strings.ReplaceAll(s.domain, ".", `\.`)
	return underwatershowutil.New(underwatershowutil.SiteConfig{
		ID:       s.id,
		Studio:   s.studio,
		SiteBase: "https://" + s.domain,
		LoadPath: s.loadPath,
		Patterns: []string{s.domain},
		MatchRe:  regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped)),
	})
}

func init() {
	for _, s := range sites {
		scraper.Register(newScraper(s))
	}
}
