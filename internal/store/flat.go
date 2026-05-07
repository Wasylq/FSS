package store

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wasylq/FSS/models"
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
	data, err := os.ReadFile(f.jsonPath(studioURL))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading store: %w", err)
	}
	var sf studioFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing store: %w", err)
	}
	return sf.Scenes, nil
}

func (f *Flat) Save(studioURL string, scenes []models.Scene) error {
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}
	sf := studioFile{
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

// Slugify turns a studio URL into a safe, human-readable filename stem.
// e.g. "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos"
//
//	→ "manyvids.com-profile-590705-bettie-bondage-store-videos"
func Slugify(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return sanitize(rawURL)
	}
	return sanitize(u.Hostname() + u.Path)
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}
