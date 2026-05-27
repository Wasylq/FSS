package store

import (
	"encoding/json"
	"fmt"
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

func (f *Flat) csvPath(studioURL string) string {
	return filepath.Join(f.dir, Slugify(studioURL)+".csv")
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
		return nil, nil
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

func (f *Flat) Save(studioURL string, scenes []models.Scene) error {
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

func (f *Flat) MarkDeleted(studioURL, _ string, ids []string) error {
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
		if set[scenes[i].ID] && scenes[i].DeletedAt == nil {
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
