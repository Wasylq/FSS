package store

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/models"
)

// csvHeaders defines column order. Multi-value fields use | as separator.
// PriceHistory is serialised as a JSON string — use a JSON-aware tool to query it.
var csvHeaders = []string{
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

func WriteCSV(scenes []models.Scene, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	w := csv.NewWriter(f)
	if err := w.Write(csvHeaders); err != nil {
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
		s.Date.Format(time.RFC3339),
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

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
