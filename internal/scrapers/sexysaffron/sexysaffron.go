// Package sexysaffron scrapes Sexy Saffron (sexysaffron.com), the site of
// Saffron Bacchus.
//
// It is a plain WordPress install — not the "video-elements" theme the
// videoelements package covers — but the transport is the same open WP REST
// posts endpoint, so it delegates to veutil. Category 3 ("videos") is the
// parent of clips/free/shows and yields the whole ~695-scene catalogue.
//
// Two site quirks worth knowing, neither of which veutil can fix generically:
//
//   - Every post has featured_media = 0, so scenes carry no thumbnail. Videos
//     embed as third-party iframes (redgifs, pornhub) that would need external
//     resolution to turn into images.
//   - Duration exists only as prose in the body ("Length: 25 minutes"), so it
//     is not extracted.
//
// Performers are not structured either: this is a single-model site, and guest
// performers appear as ordinary tags, which veutil already collects.
package sexysaffron

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/veutil"
	"github.com/Wasylq/FSS/scraper"
)

// videosCategoryID is the "videos" category, parent of clips/free/shows.
const videosCategoryID = 3

func init() {
	scraper.Register(veutil.New(veutil.SiteConfig{
		ID:             "sexysaffron",
		Studio:         "Saffron Bacchus",
		SiteBase:       "https://sexysaffron.com",
		MainCategoryID: videosCategoryID,
		Patterns:       []string{"sexysaffron.com", "sexysaffron.com/videos/{slug}/"},
		MatchRe:        regexp.MustCompile(`^https?://(?:www\.)?sexysaffron\.com(?:/|$)`),
	}))
}
