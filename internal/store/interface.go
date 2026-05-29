package store

import (
	"fmt"

	"github.com/Wasylq/FSS/models"
)

// validateScenes rejects scenes that would be unaddressable downstream:
// the composite key `(id, site_id)` is used as a primary key in SQLite
// and as a map key in both stores for relation lookups, so an empty
// component would either fail at insert time, collide with other
// empty-keyed scenes, or silently lose its performers/tags/price
// history on Load. Catch it once at the store boundary so neither
// implementation has to.
func validateScenes(scenes []models.Scene) error {
	for i, sc := range scenes {
		if sc.ID == "" {
			return fmt.Errorf("scene[%d]: ID is required (siteID=%q, title=%q)", i, sc.SiteID, sc.Title)
		}
		if sc.SiteID == "" {
			return fmt.Errorf("scene[%d]: SiteID is required (id=%q, title=%q)", i, sc.ID, sc.Title)
		}
	}
	return nil
}

// Store is the persistence layer. The default implementation uses flat JSON/CSV files.
// An optional SQLite-backed implementation is selected with the --db flag.
type Store interface {
	// Load returns all scenes previously scraped for this studio URL.
	Load(studioURL string) ([]models.Scene, error)

	// Save persists the full scene list for a studio URL, replacing any
	// prior data: scenes whose (id, site_id) is absent from `scenes` are
	// hard-deleted from the store (including their join rows and
	// price_history). The cmd layer's `--full` path depends on this — it
	// passes only freshly-scraped scenes and expects everything else to
	// disappear. Incremental and `--refresh` modes pass the merged
	// existing+fresh set so nothing is dropped. Use MarkDeleted for
	// soft-delete semantics that preserve historical data.
	//
	// Soft-delete state is NOT sticky across Save: each scene's
	// `DeletedAt` is written verbatim. A re-emitted scene with
	// `DeletedAt == nil` therefore auto-revives a prior soft-delete —
	// "the site brought the scene back, so the store shouldn't lie
	// about it being gone". `--refresh` mode is what stamps `DeletedAt`
	// on scenes the scraper no longer sees; callers wanting to preserve
	// an existing soft-delete must include the existing scene (with its
	// `DeletedAt` intact) in the `scenes` slice.
	Save(studioURL string, scenes []models.Scene) error

	// MarkDeleted soft-deletes scenes by ID — sets DeletedAt, does not remove records.
	MarkDeleted(studioURL, siteID string, ids []string) error

	// Export writes scenes for a studio URL to a file in the given format ("json" or "csv").
	// Used when SQLite is the source of truth and flat files are requested as output.
	Export(format, path, studioURL string) error

	// UpsertStudio records or updates a studio entry. No-op for the flat store.
	UpsertStudio(studio models.Studio) error

	// ListStudios returns all known studios. Empty for the flat store.
	ListStudios() ([]models.Studio, error)
}
