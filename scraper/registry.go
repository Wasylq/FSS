package scraper

import "fmt"

var registered []StudioScraper

// Register adds a scraper to the global registry.
// Call this from an init() function in each scraper package.
func Register(s StudioScraper) {
	registered = append(registered, s)
}

// All returns every registered scraper.
func All() []StudioScraper {
	return registered
}

// ForURL returns the first registered scraper that matches the given URL,
// or an error if none match.
func ForURL(url string) (StudioScraper, error) {
	for _, s := range registered {
		if s.MatchesURL(url) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no scraper registered for URL: %s", url)
}
