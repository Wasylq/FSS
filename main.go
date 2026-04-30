// FSS (FullStudioScraper) scrapes scene metadata from studio URLs.
//
// # CLI
//
// Install the binary and run:
//
//	fss scrape <studio-url>
//	fss list-scrapers
//	fss stash import --dir ./data
//
// See https://github.com/Wasylq/FSS for full CLI documentation.
//
// # Library
//
// FSS can be imported as a Go module. The public API lives in two packages:
//
//   - [github.com/Wasylq/FSS/scraper] — scraper registry and streaming interface
//   - [github.com/Wasylq/FSS/models] — Scene and PriceSnapshot types
//
// Blank-import the scraper packages you need to register them, then look up
// by URL or ID:
//
//	import (
//	    "github.com/Wasylq/FSS/scraper"
//	    _ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
//	)
//
//	s, err := scraper.ForURL("https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos")
//	ch, err := s.ListScenes(ctx, url, scraper.ListOpts{})
//	for r := range ch {
//	    fmt.Println(r.Scene.Title)
//	}
//
// See [docs/library.md] in the repository for the full guide.
package main

import (
	"github.com/Wasylq/FSS/cmd"
	_ "github.com/Wasylq/FSS/internal/scrapers/analtherapy"
	_ "github.com/Wasylq/FSS/internal/scrapers/apclips"
	_ "github.com/Wasylq/FSS/internal/scrapers/apovstory"
	_ "github.com/Wasylq/FSS/internal/scrapers/auntjudys"
	_ "github.com/Wasylq/FSS/internal/scrapers/babes"
	_ "github.com/Wasylq/FSS/internal/scrapers/bangbros"
	_ "github.com/Wasylq/FSS/internal/scrapers/brasilvr"
	_ "github.com/Wasylq/FSS/internal/scrapers/brazzers"
	_ "github.com/Wasylq/FSS/internal/scrapers/clips4sale"
	_ "github.com/Wasylq/FSS/internal/scrapers/digitalplayground"
	_ "github.com/Wasylq/FSS/internal/scrapers/evilangel"
	_ "github.com/Wasylq/FSS/internal/scrapers/exposedlatinas"
	_ "github.com/Wasylq/FSS/internal/scrapers/fakings"
	_ "github.com/Wasylq/FSS/internal/scrapers/familytherapy"
	_ "github.com/Wasylq/FSS/internal/scrapers/fancentro"
	_ "github.com/Wasylq/FSS/internal/scrapers/faphouse"
	_ "github.com/Wasylq/FSS/internal/scrapers/fiftyplus"
	_ "github.com/Wasylq/FSS/internal/scrapers/ftvmilfs"
	_ "github.com/Wasylq/FSS/internal/scrapers/gloryholesecrets"
	_ "github.com/Wasylq/FSS/internal/scrapers/gloryquest"
	_ "github.com/Wasylq/FSS/internal/scrapers/houseofyre"
	_ "github.com/Wasylq/FSS/internal/scrapers/ideapocket"
	_ "github.com/Wasylq/FSS/internal/scrapers/iwantclips"
	_ "github.com/Wasylq/FSS/internal/scrapers/kink"
	_ "github.com/Wasylq/FSS/internal/scrapers/ladysonia"
	_ "github.com/Wasylq/FSS/internal/scrapers/loyalfans"
	_ "github.com/Wasylq/FSS/internal/scrapers/madonna"
	_ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
	_ "github.com/Wasylq/FSS/internal/scrapers/milfvr"
	_ "github.com/Wasylq/FSS/internal/scrapers/missax"
	_ "github.com/Wasylq/FSS/internal/scrapers/mofos"
	_ "github.com/Wasylq/FSS/internal/scrapers/momcomesfirst"
	_ "github.com/Wasylq/FSS/internal/scrapers/mommyblowsbest"
	_ "github.com/Wasylq/FSS/internal/scrapers/moodyz"
	_ "github.com/Wasylq/FSS/internal/scrapers/mydirtyhobby"
	_ "github.com/Wasylq/FSS/internal/scrapers/naughtyamerica"
	_ "github.com/Wasylq/FSS/internal/scrapers/nubiles"
	_ "github.com/Wasylq/FSS/internal/scrapers/oopsfamily"
	_ "github.com/Wasylq/FSS/internal/scrapers/over40handjobs"
	_ "github.com/Wasylq/FSS/internal/scrapers/pennybarber"
	_ "github.com/Wasylq/FSS/internal/scrapers/perfectgirlfriend"
	_ "github.com/Wasylq/FSS/internal/scrapers/pornhub"
	_ "github.com/Wasylq/FSS/internal/scrapers/propertysex"
	_ "github.com/Wasylq/FSS/internal/scrapers/puretaboo"
	_ "github.com/Wasylq/FSS/internal/scrapers/queensnake"
	_ "github.com/Wasylq/FSS/internal/scrapers/rachelsteele"
	_ "github.com/Wasylq/FSS/internal/scrapers/reaganfoxx"
	_ "github.com/Wasylq/FSS/internal/scrapers/realitykings"
	_ "github.com/Wasylq/FSS/internal/scrapers/rocketinc"
	_ "github.com/Wasylq/FSS/internal/scrapers/seemomsuck"
	_ "github.com/Wasylq/FSS/internal/scrapers/sexmex"
	_ "github.com/Wasylq/FSS/internal/scrapers/tabooheat"
	_ "github.com/Wasylq/FSS/internal/scrapers/taratainton"
	_ "github.com/Wasylq/FSS/internal/scrapers/transangels"
	_ "github.com/Wasylq/FSS/internal/scrapers/transqueens"
	_ "github.com/Wasylq/FSS/internal/scrapers/tranzvr"
	_ "github.com/Wasylq/FSS/internal/scrapers/twistys"
	_ "github.com/Wasylq/FSS/internal/scrapers/visitx"
	_ "github.com/Wasylq/FSS/internal/scrapers/wankzvr"
	_ "github.com/Wasylq/FSS/internal/scrapers/xespl"
	_ "github.com/Wasylq/FSS/internal/scrapers/xevbellringer"
	_ "github.com/Wasylq/FSS/internal/scrapers/yourvids"

	_ "github.com/Wasylq/FSS/internal/scrapers/boyfriendsharing"
	_ "github.com/Wasylq/FSS/internal/scrapers/brattyfamily"
	_ "github.com/Wasylq/FSS/internal/scrapers/gostuckyourself"
	_ "github.com/Wasylq/FSS/internal/scrapers/hugecockbreak"
	_ "github.com/Wasylq/FSS/internal/scrapers/littlefromasia"
	_ "github.com/Wasylq/FSS/internal/scrapers/mommysboy"
	_ "github.com/Wasylq/FSS/internal/scrapers/momxxx"
	_ "github.com/Wasylq/FSS/internal/scrapers/mybadmilfs"
	_ "github.com/Wasylq/FSS/internal/scrapers/mydaughterswap"
	_ "github.com/Wasylq/FSS/internal/scrapers/mypervmom"
	_ "github.com/Wasylq/FSS/internal/scrapers/mysislovesme"
	_ "github.com/Wasylq/FSS/internal/scrapers/youngerloverofmine"

	_ "github.com/Wasylq/FSS/internal/scrapers/jerkoffinstructions"
	_ "github.com/Wasylq/FSS/internal/scrapers/maturenl"
	_ "github.com/Wasylq/FSS/internal/scrapers/mylf"
	_ "github.com/Wasylq/FSS/internal/scrapers/puremature"
	_ "github.com/Wasylq/FSS/internal/scrapers/sofiemarie"

	_ "github.com/Wasylq/FSS/internal/scrapers/charleechase"
	_ "github.com/Wasylq/FSS/internal/scrapers/deeps"
	_ "github.com/Wasylq/FSS/internal/scrapers/kmproduce"
	_ "github.com/Wasylq/FSS/internal/scrapers/takaratv"
	_ "github.com/Wasylq/FSS/internal/scrapers/venusav"

	_ "github.com/Wasylq/FSS/internal/scrapers/burningangel"
	_ "github.com/Wasylq/FSS/internal/scrapers/filthykings"
	_ "github.com/Wasylq/FSS/internal/scrapers/gangbangcreampie"
	_ "github.com/Wasylq/FSS/internal/scrapers/girlfriendsfilms"
	_ "github.com/Wasylq/FSS/internal/scrapers/lethalhardcore"
	_ "github.com/Wasylq/FSS/internal/scrapers/roccosiffredi"
	_ "github.com/Wasylq/FSS/internal/scrapers/wicked"
)

// Set by -ldflags at release build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersion(version, commit, date)
	cmd.Execute()
}
