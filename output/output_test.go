package output

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
)

const testURL = "https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos"

func TestWriteJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	now := time.Now().UTC().Truncate(time.Second)

	sf := models.StudioFile{
		StudioURL:  testURL,
		ScrapedAt:  now,
		SceneCount: 1,
		Scenes: []models.Scene{{
			ID: "1", SiteID: "test", Title: "Test Scene",
			URL: "https://example.com/1", ScrapedAt: now,
		}},
	}

	if err := WriteJSON(sf, path); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got models.StudioFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.StudioURL != testURL {
		t.Errorf("studioUrl = %q", got.StudioURL)
	}
	if got.SceneCount != 1 {
		t.Errorf("sceneCount = %d", got.SceneCount)
	}
	if got.Scenes[0].Title != "Test Scene" {
		t.Errorf("title = %q", got.Scenes[0].Title)
	}
}

func TestWriteJSONSpecialChars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "special.json")
	now := time.Now().UTC().Truncate(time.Second)

	sf := models.StudioFile{
		StudioURL:  testURL,
		ScrapedAt:  now,
		SceneCount: 1,
		Scenes: []models.Scene{{
			ID: "1", SiteID: "test", ScrapedAt: now,
			Title:       `Title with "quotes" & <brackets>`,
			Description: "Line 1\nLine 2\tTabbed",
			Performers:  []string{"Performer O'Neil"},
		}},
	}

	if err := WriteJSON(sf, path); err != nil {
		t.Fatal(err)
	}

	var got models.StudioFile
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON after special chars: %v", err)
	}
	if got.Scenes[0].Title != `Title with "quotes" & <brackets>` {
		t.Errorf("title = %q", got.Scenes[0].Title)
	}
}

func TestWriteJSONAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.json")
	now := time.Now().UTC().Truncate(time.Second)

	sf := models.StudioFile{
		StudioURL: testURL, ScrapedAt: now, SceneCount: 1,
		Scenes: []models.Scene{{
			ID: "1", SiteID: "test", Title: "First", ScrapedAt: now,
		}},
	}
	if err := WriteJSON(sf, path); err != nil {
		t.Fatal(err)
	}

	sf.Scenes[0].Title = "Second"
	if err := WriteJSON(sf, path); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	var got models.StudioFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Scenes[0].Title != "Second" {
		t.Errorf("atomic overwrite failed, got %q", got.Scenes[0].Title)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".fss-tmp-") {
			t.Errorf("temp file not cleaned up: %s", e.Name())
		}
	}
}

func TestWriteCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	now := time.Now().UTC().Truncate(time.Second)

	scenes := []models.Scene{
		{
			ID:         "1",
			SiteID:     "test",
			StudioURL:  testURL,
			Title:      "Test Scene",
			URL:        "https://example.com/1",
			Date:       now,
			Performers: []string{"Alice", "Bob"},
			Tags:       []string{"tag1", "tag2"},
			Duration:   3600,
			ScrapedAt:  now,
		},
	}

	if err := WriteCSV(scenes, path); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("got %d rows (including header), want 2", len(records))
	}

	header := records[0]
	if header[0] != "id" {
		t.Errorf("header[0] = %q, want 'id'", header[0])
	}
	if len(header) != len(CSVHeaders) {
		t.Errorf("got %d columns, want %d", len(header), len(CSVHeaders))
	}

	row := records[1]
	if row[0] != "1" {
		t.Errorf("id = %q", row[0])
	}
	if row[3] != "Test Scene" {
		t.Errorf("title = %q", row[3])
	}

	perfIdx := indexOf(header, "performers")
	if perfIdx < 0 {
		t.Fatal("performers column not found")
	}
	if row[perfIdx] != "Alice|Bob" {
		t.Errorf("performers = %q, want 'Alice|Bob'", row[perfIdx])
	}

	tagIdx := indexOf(header, "tags")
	if tagIdx < 0 {
		t.Fatal("tags column not found")
	}
	if row[tagIdx] != "tag1|tag2" {
		t.Errorf("tags = %q", row[tagIdx])
	}

	durIdx := indexOf(header, "duration")
	if durIdx < 0 {
		t.Fatal("duration column not found")
	}
	if row[durIdx] != "3600" {
		t.Errorf("duration = %q", row[durIdx])
	}
}

func TestWriteCSVEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.csv")

	if err := WriteCSV(nil, path); err != nil {
		t.Fatal(err)
	}

	f, _ := os.Open(path)
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	records, _ := r.ReadAll()
	if len(records) != 1 {
		t.Errorf("expected header only, got %d rows", len(records))
	}
}

func TestWriteCSVSpecialChars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "special.csv")
	now := time.Now().UTC().Truncate(time.Second)

	scenes := []models.Scene{{
		ID: "1", SiteID: "test", StudioURL: testURL,
		Title:     `Scene with "quotes", commas, and newlines` + "\n" + "second line",
		ScrapedAt: now,
	}}

	if err := WriteCSV(scenes, path); err != nil {
		t.Fatal(err)
	}

	f, _ := os.Open(path)
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV should handle special chars: %v", err)
	}
	if records[1][3] != scenes[0].Title {
		t.Errorf("title roundtrip failed: got %q", records[1][3])
	}
}

func TestFormatTime(t *testing.T) {
	if got := formatTime(time.Time{}); got != "" {
		t.Errorf("zero time = %q, want empty", got)
	}
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	if got := formatTime(now); got != "2026-05-06T12:00:00Z" {
		t.Errorf("formatTime = %q", got)
	}
}

func TestFormatTimePtr(t *testing.T) {
	if got := formatTimePtr(nil); got != "" {
		t.Errorf("nil = %q, want empty", got)
	}
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	if got := formatTimePtr(&now); got != "2026-05-06T12:00:00Z" {
		t.Errorf("formatTimePtr = %q", got)
	}
}

func TestSceneToRow(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	scene := models.Scene{
		ID: "42", SiteID: "test", StudioURL: testURL,
		Title: "Test", URL: "https://example.com/42",
		Performers: []string{"A", "B"}, Tags: []string{"t1"},
		Duration: 120, LowestPrice: 9.99,
		ScrapedAt: now,
	}
	row, err := sceneToRow(scene)
	if err != nil {
		t.Fatal(err)
	}
	if len(row) != len(CSVHeaders) {
		t.Fatalf("row has %d fields, want %d", len(row), len(CSVHeaders))
	}
	if row[0] != "42" {
		t.Errorf("id = %q", row[0])
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{testURL, "www-manyvids-com-profile-590705-bettie-bondage-store-videos"},
		{"https://www.clips4sale.com/studio/12345/some-store", "www-clips4sale-com-studio-12345-some-store"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugifySpecialChars(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/path/with spaces/", "example-com-path-with-spaces"},
		{"https://example.com/UPPER/Case", "example-com-upper-case"},
		{"https://example.com/path///multiple///slashes", "example-com-path-multiple-slashes"},
		{"https://example.com/special!@#$%chars", "https-example-com-special-chars"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugifyEmpty(t *testing.T) {
	got := Slugify("")
	if got != "" {
		t.Errorf("Slugify('') = %q, want empty", got)
	}
}

func TestSlugifyPathTraversal(t *testing.T) {
	got := Slugify("https://evil.com/../../etc/passwd")
	if strings.Contains(got, "..") || strings.Contains(got, "/") {
		t.Errorf("Slugify path traversal = %q, want no dots/slashes", got)
	}
}

func indexOf(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}
