package scraper

import (
	"context"
	"fmt"
	"time"

	"github.com/Wasylq/FSS/models"
)

// PageResult is returned by a FetchPage callback to the pagination loop.
type PageResult struct {
	Scenes []models.Scene
	Total  int
	Done   bool
}

// Paginate runs a page-numbered pagination loop, handling delay, context
// cancellation, progress reporting, KnownIDs early-stop, and scene
// sending. The caller provides a FetchPage callback that fetches and
// parses a single page; everything else is handled by the loop.
//
// FetchPage receives the 1-based page number and returns a PageResult.
// Set PageResult.Total on the first page for progress display. Set
// PageResult.Done to true when there are no more pages (e.g. items <
// pageSize, page >= totalPages). An empty Scenes slice also stops the
// loop.
//
// The siteID string is used only for debug log messages.
//
// Paginate does NOT call defer close(out) — the caller's run() must
// still do that, since some scrapers do additional work after the
// pagination loop returns (e.g. worker pool teardown).
func Paginate(ctx context.Context, opts ListOpts, siteID string, out chan<- SceneResult,
	fetchPage func(ctx context.Context, page int) (PageResult, error),
) {
	progressSent := false
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}
		Debugf(1, "%s: fetching page %d", siteID, page)

		result, err := fetchPage(ctx, page)
		if err != nil {
			select {
			case out <- Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		if len(result.Scenes) == 0 {
			return
		}

		if !progressSent && result.Total > 0 {
			progressSent = true
			Debugf(1, "%s: %d total scenes", siteID, result.Total)
			select {
			case out <- Progress(result.Total):
			case <-ctx.Done():
				return
			}
		}

		for _, sc := range result.Scenes {
			if opts.KnownIDs[sc.ID] {
				Debugf(1, "%s: hit known ID, stopping early", siteID)
				select {
				case out <- StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- Scene(sc):
			case <-ctx.Done():
				return
			}
		}

		if result.Done {
			return
		}
	}
}
