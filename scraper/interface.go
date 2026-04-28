// Package scraper defines the interface for site scrapers and a global registry.
//
// Each supported site implements [StudioScraper] and registers itself via
// [Register] in an init() function. Consumers look up scrapers with [ForURL]
// or [ForID], then call [StudioScraper.ListScenes] to stream results.
//
// Scraper packages live under internal/scrapers/ and must be blank-imported
// to trigger registration.
package scraper

import (
	"context"
	"time"

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
	// Workers sets the number of concurrent detail-page fetchers for scrapers
	// that use a worker pool. Zero uses the scraper's default (typically 4).
	Workers int
	// KnownIDs, when non-empty, signals the scraper to stop pagination as soon
	// as it encounters an ID already in the set. Used for incremental runs where
	// content is sorted newest-first and trailing pages are already stored.
	KnownIDs map[string]bool
	// Delay is the duration to sleep between page fetches (and between detail
	// fetches for scrapers that use a worker pool). Zero means no delay.
	Delay time.Duration
}

// SceneResult is a single item sent by ListScenes — either a scene or an error.
type SceneResult struct {
	Scene models.Scene
	Err   error
	// Total, when > 0, carries a hint about the total number of scenes for the
	// studio. Sent at most once (after the first page). Consumers should skip
	// this result and use the value only for progress display.
	Total int
	// StoppedEarly, when true, signals the scraper halted pagination because it
	// hit an ID from opts.KnownIDs. Older scenes beyond that point already exist
	// in storage. Sent once immediately before the channel is closed, instead
	// of a Scene.
	StoppedEarly bool
}
