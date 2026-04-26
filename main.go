package main

import (
	"github.com/Wasylq/FSS/cmd"
	_ "github.com/Wasylq/FSS/internal/scrapers/apovstory"
	_ "github.com/Wasylq/FSS/internal/scrapers/babes"
	_ "github.com/Wasylq/FSS/internal/scrapers/brasilvr"
	_ "github.com/Wasylq/FSS/internal/scrapers/brazzers"
	_ "github.com/Wasylq/FSS/internal/scrapers/clips4sale"
	_ "github.com/Wasylq/FSS/internal/scrapers/analtherapy"
	_ "github.com/Wasylq/FSS/internal/scrapers/exposedlatinas"
	_ "github.com/Wasylq/FSS/internal/scrapers/digitalplayground"
	_ "github.com/Wasylq/FSS/internal/scrapers/fakings"
	_ "github.com/Wasylq/FSS/internal/scrapers/gloryholesecrets"
	_ "github.com/Wasylq/FSS/internal/scrapers/familytherapy"
	_ "github.com/Wasylq/FSS/internal/scrapers/iwantclips"
	_ "github.com/Wasylq/FSS/internal/scrapers/kink"
	_ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
	_ "github.com/Wasylq/FSS/internal/scrapers/milfvr"
	_ "github.com/Wasylq/FSS/internal/scrapers/missax"
	_ "github.com/Wasylq/FSS/internal/scrapers/mofos"
	_ "github.com/Wasylq/FSS/internal/scrapers/momcomesfirst"
	_ "github.com/Wasylq/FSS/internal/scrapers/mydirtyhobby"
	_ "github.com/Wasylq/FSS/internal/scrapers/naughtyamerica"
	_ "github.com/Wasylq/FSS/internal/scrapers/nubiles"
	_ "github.com/Wasylq/FSS/internal/scrapers/perfectgirlfriend"
	_ "github.com/Wasylq/FSS/internal/scrapers/pornhub"
	_ "github.com/Wasylq/FSS/internal/scrapers/puretaboo"
	_ "github.com/Wasylq/FSS/internal/scrapers/rachelsteele"
	_ "github.com/Wasylq/FSS/internal/scrapers/realitykings"
	_ "github.com/Wasylq/FSS/internal/scrapers/sexmex"
	_ "github.com/Wasylq/FSS/internal/scrapers/tabooheat"
	_ "github.com/Wasylq/FSS/internal/scrapers/taratainton"
	_ "github.com/Wasylq/FSS/internal/scrapers/wankzvr"
	_ "github.com/Wasylq/FSS/internal/scrapers/transqueens"
	_ "github.com/Wasylq/FSS/internal/scrapers/tranzvr"
	_ "github.com/Wasylq/FSS/internal/scrapers/yourvids"
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
