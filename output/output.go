package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/models"
)

// WriteJSON writes a StudioFile as indented JSON, using atomic file replacement
// to prevent corruption on crash.
func WriteJSON(sf models.StudioFile, path string) error {
	return atomicWriteFile(path, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sf)
	})
}

// CSVHeaders defines the column order for CSV exports. Multi-value fields use
// | as separator. PriceHistory is serialised as a JSON string.
var CSVHeaders = []string{
	"id", "siteId", "studioUrl",
	"title", "url", "date", "description",
	"thumbnail", "preview",
	"performers", "director", "studio",
	"tags", "categories",
	"series", "seriesPart",
	"duration", "resolution", "width", "height", "format",
	"views", "likes", "comments",
	"lowestPrice", "lowestPriceDate", "priceHistory",
	"scrapedAt", "deletedAt",
}

// WriteCSV writes scenes as CSV with a header row, using atomic file replacement.
func WriteCSV(scenes []models.Scene, path string) error {
	return atomicWriteFile(path, func(out io.Writer) error {
		w := csv.NewWriter(out)
		if err := w.Write(CSVHeaders); err != nil {
			return err
		}
		for _, s := range scenes {
			row, err := sceneToRow(s)
			if err != nil {
				return err
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
		w.Flush()
		return w.Error()
	})
}

// Slugify turns a studio URL into a safe, human-readable filename stem.
// e.g. "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos"
//
//	→ "www-manyvids-com-profile-590705-bettie-bondage-store-videos"
func Slugify(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return sanitize(rawURL)
	}
	return sanitize(u.Hostname() + u.Path)
}

func atomicWriteFile(path string, writeFn func(io.Writer) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".fss-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	if err := writeFn(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming %s → %s: %w", tmpPath, path, err)
	}
	return nil
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

func sceneToRow(s models.Scene) ([]string, error) {
	ph, err := json.Marshal(s.PriceHistory)
	if err != nil {
		return nil, fmt.Errorf("marshalling price history: %w", err)
	}
	return []string{
		s.ID,
		s.SiteID,
		s.StudioURL,
		s.Title,
		s.URL,
		formatTime(s.Date),
		s.Description,
		s.Thumbnail,
		s.Preview,
		strings.Join(s.Performers, "|"),
		s.Director,
		s.Studio,
		strings.Join(s.Tags, "|"),
		strings.Join(s.Categories, "|"),
		s.Series,
		strconv.Itoa(s.SeriesPart),
		strconv.Itoa(s.Duration),
		s.Resolution,
		strconv.Itoa(s.Width),
		strconv.Itoa(s.Height),
		s.Format,
		strconv.Itoa(s.Views),
		strconv.Itoa(s.Likes),
		strconv.Itoa(s.Comments),
		strconv.FormatFloat(s.LowestPrice, 'f', 2, 64),
		formatTimePtr(s.LowestPriceDate),
		string(ph),
		s.ScrapedAt.Format(time.RFC3339),
		formatTimePtr(s.DeletedAt),
	}, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
