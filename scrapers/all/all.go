// Package all registers every site scraper with scraper.Register().
// Importing this package wires up the full scraper catalog.
//
// Usage from external modules:
//
//	import _ "github.com/Wasylq/FSS/scrapers/all"
package all

import (
	// Blank import for side effects only: the internal package's init()
	// functions call scraper.Register() for every site. This package is the
	// public re-export of that registration.
	_ "github.com/Wasylq/FSS/internal/scrapers/all"
)
