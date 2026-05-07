package scraper

import "fmt"

var registered []StudioScraper

// Register adds a scraper to the global registry.
// Call this from an init() function in each scraper package.
// Panics if a scraper with the same ID is already registered.
func Register(s StudioScraper) {
	for _, existing := range registered {
		if existing.ID() == s.ID() {
			panic(fmt.Sprintf("duplicate scraper ID: %s", s.ID()))
		}
	}
	registered = append(registered, s)
}

// All returns every registered scraper.
func All() []StudioScraper {
	return registered
}

// ForID returns the registered scraper with the given ID,
// or an error if none match.
func ForID(id string) (StudioScraper, error) {
	for _, s := range registered {
		if s.ID() == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no scraper registered with ID: %s", id)
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
