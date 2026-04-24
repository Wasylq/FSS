package main

import (
	"github.com/Wasylq/FSS/cmd"
	_ "github.com/Wasylq/FSS/internal/scrapers/apovstory"
	_ "github.com/Wasylq/FSS/internal/scrapers/babes"
	_ "github.com/Wasylq/FSS/internal/scrapers/brazzers"
	_ "github.com/Wasylq/FSS/internal/scrapers/clips4sale"
	_ "github.com/Wasylq/FSS/internal/scrapers/digitalplayground"
	_ "github.com/Wasylq/FSS/internal/scrapers/iwantclips"
	_ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
	_ "github.com/Wasylq/FSS/internal/scrapers/missax"
	_ "github.com/Wasylq/FSS/internal/scrapers/mofos"
	_ "github.com/Wasylq/FSS/internal/scrapers/naughtyamerica"
	_ "github.com/Wasylq/FSS/internal/scrapers/momcomesfirst"
	_ "github.com/Wasylq/FSS/internal/scrapers/mydirtyhobby"
	_ "github.com/Wasylq/FSS/internal/scrapers/pornhub"
	_ "github.com/Wasylq/FSS/internal/scrapers/puretaboo"
	_ "github.com/Wasylq/FSS/internal/scrapers/rachelsteele"
	_ "github.com/Wasylq/FSS/internal/scrapers/realitykings"
	_ "github.com/Wasylq/FSS/internal/scrapers/tabooheat"
	_ "github.com/Wasylq/FSS/internal/scrapers/taratainton"
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
