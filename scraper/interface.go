package scraper

import (
	"context"

	"github.com/Wasylq/FSS/models"
)

// StudioScraper is implemented once per supported site.
// Adding a new site means creating internal/scrapers/<site>/<site>.go,
// implementing this interface, and calling scraper.Register in an init().
type StudioScraper interface {
	// ID returns a stable lowercase identifier for this scraper (e.g. "manyvids").
	ID() string

	// Patterns returns the URL patterns this scraper handles.
	// Used by `fss list-scrapers` and as documentation. A scraper may declare
	// multiple patterns (different URL formats, shared-platform sites, etc.).
	Patterns() []string

	// MatchesURL returns true if this scraper can handle the given studio URL.
	MatchesURL(url string) bool

	// ListScenes fetches all scenes for the given studio URL and sends each
	// result down the returned channel. The channel is closed when done.
	// Implementations should respect ctx cancellation.
	ListScenes(ctx context.Context, studioURL string, opts ListOpts) (<-chan SceneResult, error)
}

// ListOpts controls scraping behaviour passed in from the CLI/config.
type ListOpts struct {
	Workers int
	// KnownIDs, when non-empty, signals the scraper to stop pagination as soon
	// as it encounters an ID already in the set. Used for incremental runs where
	// content is sorted newest-first and trailing pages are already stored.
	KnownIDs map[string]bool
}

// SceneResult is a single item sent by ListScenes — either a scene or an error.
type SceneResult struct {
	Scene models.Scene
	Err   error
}
