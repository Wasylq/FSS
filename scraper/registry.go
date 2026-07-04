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

// All returns a copy of every registered scraper.
func All() []StudioScraper {
	out := make([]StudioScraper, len(registered))
	copy(out, registered)
	return out
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
// or an error if none match. Resolution is first-match-wins by registration
// (import) order. When more than one scraper matches — e.g. a broad parent
// regex shadowing a sub-site — the extra matches are reported at debug level
// so the overlap is visible without changing which scraper is chosen.
func ForURL(url string) (StudioScraper, error) {
	var chosen StudioScraper
	var others []string
	for _, s := range registered {
		if s.MatchesURL(url) {
			if chosen == nil {
				chosen = s
			} else {
				others = append(others, s.ID())
			}
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("no scraper registered for URL: %s", url)
	}
	if len(others) > 0 {
		Debugf(1, "registry: %q matched %d scrapers; using %q, also matched by %v",
			url, len(others)+1, chosen.ID(), others)
	}
	return chosen, nil
}
