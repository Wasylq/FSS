package main

import (
	"github.com/Wasylq/FSS/cmd"
	_ "github.com/Wasylq/FSS/internal/scrapers/clips4sale"
	_ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
	_ "github.com/Wasylq/FSS/internal/scrapers/mydirtyhobby"
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
