package main

import (
	"github.com/Wasylq/FSS/cmd"
	_ "github.com/Wasylq/FSS/internal/scrapers/clips4sale"
	_ "github.com/Wasylq/FSS/internal/scrapers/manyvids"
)

func main() {
	cmd.Execute()
}
