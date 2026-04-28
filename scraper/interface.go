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

// ResultKind identifies what a SceneResult carries.
type ResultKind int

const (
	// KindScene indicates the result carries a valid Scene.
	KindScene ResultKind = iota
	// KindError indicates a non-fatal error. Log and continue.
	KindError
	// KindTotal is a progress hint sent once after the first page.
	KindTotal
	// KindStoppedEarly signals the scraper hit a known ID and stopped pagination.
	KindStoppedEarly
)

// SceneResult is a single item sent on the channel returned by ListScenes.
// Use the Kind field to determine which other fields are populated.
// Prefer the constructor functions [Scene], [Error], [Total], [StoppedEarly].
type SceneResult struct {
	Kind  ResultKind
	Scene models.Scene
	Err   error
	Total int
}

// Scene constructs a SceneResult carrying a scraped scene.
func Scene(s models.Scene) SceneResult {
	return SceneResult{Kind: KindScene, Scene: s}
}

// Error constructs a SceneResult carrying a non-fatal error.
func Error(err error) SceneResult {
	return SceneResult{Kind: KindError, Err: err}
}

// Progress constructs a SceneResult carrying a total-scenes hint.
func Progress(total int) SceneResult {
	return SceneResult{Kind: KindTotal, Total: total}
}

// StoppedEarly constructs a SceneResult signalling early pagination stop.
func StoppedEarly() SceneResult {
	return SceneResult{Kind: KindStoppedEarly}
}
