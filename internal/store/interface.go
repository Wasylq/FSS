package store

import "github.com/Wasylq/FSS/models"

// Store is the persistence layer. The default implementation uses flat JSON/CSV files.
// An optional SQLite-backed implementation is selected with the --db flag.
type Store interface {
	// Load returns all scenes previously scraped for this studio URL.
	Load(studioURL string) ([]models.Scene, error)

	// Save persists the full scene list for a studio URL, replacing any prior data.
	Save(studioURL string, scenes []models.Scene) error

	// MarkDeleted soft-deletes scenes by ID — sets DeletedAt, does not remove records.
	MarkDeleted(studioURL string, ids []string) error

	// Export writes the store contents to a file in the given format ("json" or "csv").
	// Used when SQLite is the source of truth and flat files are requested as output.
	Export(format, path string) error
}
