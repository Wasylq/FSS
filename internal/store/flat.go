package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/output"
)

// Flat is the default store backed by a per-studio JSON file on disk.
// JSON is always written — it is the backing format for incremental updates.
// CSV is written as an additional export when included in formats.
type Flat struct {
	dir     string
	formats []string
}

func NewFlat(dir string, formats []string) *Flat {
	return &Flat{dir: dir, formats: formats}
}

func (f *Flat) jsonPath(studioURL string) string {
	return filepath.Join(f.dir, Slugify(studioURL)+".json")
}

// legacyJSONPath is the pre-hash filename for a studio URL, used to migrate
// files written before Slugify gained its hash suffix.
func (f *Flat) legacyJSONPath(studioURL string) string {
	legacy := output.LegacySlugify(studioURL)
	if legacy == "" {
		return ""
	}
	return filepath.Join(f.dir, legacy+".json")
}

func (f *Flat) csvPath(studioURL string) string {
	return filepath.Join(f.dir, Slugify(studioURL)+".csv")
}

func (f *Flat) Lock(studioURL string) (io.Closer, error) {
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir for lock: %w", err)
	}
	return lockFile(filepath.Join(f.dir, Slugify(studioURL)+".lock"))
}

func (f *Flat) Load(studioURL string) ([]models.Scene, error) {
	sf, err := f.loadStudioFile(studioURL)
	if err != nil || sf == nil {
		return nil, err
	}
	return sf.Scenes, nil
}

// loadStudioFile reads the slug-keyed JSON file, parses it, and verifies the
// stored StudioURL matches the requested one. Two distinct studio URLs can
// slug to the same filename (e.g. "/foo-bar" and "/foo/bar"), and silently
// overwriting one with the other was a documented data-loss bug. Returns
// (nil, nil) when no file exists yet.
func (f *Flat) loadStudioFile(studioURL string) (*models.StudioFile, error) {
	path := f.jsonPath(studioURL)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Migrate a pre-hash (legacy) file to the new hashed name, if one
		// exists and belongs to this studio, so existing incremental state
		// (price history, soft-deletes) survives the Slugify change.
		if migrated, mErr := f.migrateLegacy(studioURL, path); mErr != nil {
			return nil, mErr
		} else if migrated {
			data, err = os.ReadFile(path)
		} else {
			return nil, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("reading store: %w", err)
	}
	var sf models.StudioFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing store: %w", err)
	}
	if sf.StudioURL != "" && sf.StudioURL != studioURL {
		return nil, fmt.Errorf(
			"slug collision: %s stores data for %q but %q was requested — rename or move one of the studio files",
			path, sf.StudioURL, studioURL,
		)
	}
	return &sf, nil
}

// migrateLegacy renames a pre-hash studio file to the new hashed name when one
// exists and belongs to studioURL. Returns whether a migration happened. A
// legacy file recording a different StudioURL is left untouched (the studios
// no longer collide once both are hashed).
func (f *Flat) migrateLegacy(studioURL, newPath string) (bool, error) {
	legacy := f.legacyJSONPath(studioURL)
	if legacy == "" || legacy == newPath {
		return false, nil
	}
	data, err := os.ReadFile(legacy)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading legacy store: %w", err)
	}
	var sf models.StudioFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return false, fmt.Errorf("parsing legacy store: %w", err)
	}
	if sf.StudioURL != "" && sf.StudioURL != studioURL {
		return false, nil
	}
	if err := os.Rename(legacy, newPath); err != nil {
		return false, fmt.Errorf("migrating legacy store %s: %w", legacy, err)
	}
	return true, nil
}

func (f *Flat) Save(studioURL string, scenes []models.Scene) error {
	if err := validateScenes(scenes); err != nil {
		return err
	}
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}
	// Refuse to clobber a file that belongs to a different studio URL.
	if _, err := f.loadStudioFile(studioURL); err != nil {
		return err
	}
	sf := models.StudioFile{
		StudioURL:  studioURL,
		ScrapedAt:  time.Now().UTC(),
		SceneCount: len(scenes),
		Scenes:     scenes,
	}
	if err := WriteJSON(sf, f.jsonPath(studioURL)); err != nil {
		return err
	}
	for _, format := range f.formats {
		if format == "csv" {
			if err := WriteCSV(scenes, f.csvPath(studioURL)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *Flat) MarkDeleted(studioURL, siteID string, ids []string) error {
	unlock, err := f.Lock(studioURL)
	if err != nil {
		return fmt.Errorf("locking studio for MarkDeleted: %w", err)
	}
	defer func() { _ = unlock.Close() }()

	scenes, err := f.Load(studioURL)
	if err != nil {
		return err
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	now := time.Now().UTC()
	for i := range scenes {
		// Match the SQLite store: a scene is soft-deleted only when both
		// its ID is in the set and its SiteID matches. Studio files that
		// mix scenes from multiple sites (e.g. cross-site stash merges)
		// can hold overlapping IDs across SiteIDs; without the SiteID
		// filter those collateral scenes would be wiped too.
		if set[scenes[i].ID] && scenes[i].SiteID == siteID && scenes[i].DeletedAt == nil {
			scenes[i].DeletedAt = &now
		}
	}
	return f.Save(studioURL, scenes)
}

// Export is a no-op for the flat store — files are written directly by Save.
func (f *Flat) Export(_, _, _ string) error { return nil }

// UpsertStudio is a no-op for the flat store — studio tracking requires SQLite.
func (f *Flat) UpsertStudio(_ models.Studio) error { return nil }

// ListStudios is a no-op for the flat store.
func (f *Flat) ListStudios() ([]models.Studio, error) { return nil, nil }

func Slugify(rawURL string) string {
	return output.Slugify(rawURL)
}
