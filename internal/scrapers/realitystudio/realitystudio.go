// Package realitystudio registers the Reality Studio LLC fetish sites, all
// driven by a static /js/clips.js catalog. See realitystudioutil.
package realitystudio

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/realitystudioutil"
	"github.com/Wasylq/FSS/scraper"
)

type site struct {
	id     string
	domain string
	studio string
}

var sites = []site{
	{"subbygirls", "subbygirls.com", "Subby Girls"},
	{"femaleworship", "femaleworship.com", "Female Worship"},
	{"menareslaves", "menareslaves.com", "Men Are Slaves"},
	{"cumcountdown", "cumcountdown.com", "Cum Countdown"},
}

func init() {
	for _, s := range sites {
		escaped := strings.ReplaceAll(s.domain, ".", `\.`)
		scraper.Register(realitystudioutil.New(realitystudioutil.SiteConfig{
			ID:       s.id,
			Studio:   s.studio,
			SiteBase: "https://www." + s.domain,
			Patterns: []string{s.domain, s.domain + "/main.html"},
			MatchRe:  regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped)),
		}))
	}
}
