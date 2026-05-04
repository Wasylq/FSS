package nfo

import (
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/match"
)

func TestFromMergedScene(t *testing.T) {
	m := match.MergedScene{
		Title:       "Test Scene",
		Description: "A test scene.",
		Date:        time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		URLs:        []string{"https://manyvids.com/1", "https://clips4sale.com/2"},
		Studio:      "Test Studio",
		Thumbnail:   "https://example.com/thumb.jpg",
		Performers:  []string{"Performer One", "Performer Two"},
		Tags:        []string{"tag1", "tag2"},
	}

	mov := FromMergedScene(m)

	if mov.Title != "Test Scene" {
		t.Errorf("title = %q, want %q", mov.Title, "Test Scene")
	}
	if mov.Premiered != "2024-03-15" {
		t.Errorf("premiered = %q, want %q", mov.Premiered, "2024-03-15")
	}
	if len(mov.URLs) != 2 {
		t.Errorf("urls = %d, want 2", len(mov.URLs))
	}
	if len(mov.Actors) != 2 {
		t.Errorf("actors = %d, want 2", len(mov.Actors))
	}
	if mov.Actors[0].Name != "Performer One" {
		t.Errorf("actor[0] = %q, want %q", mov.Actors[0].Name, "Performer One")
	}
	if len(mov.Thumbnails) != 1 || mov.Thumbnails[0].Aspect != "poster" {
		t.Errorf("thumbnails = %+v, want 1 poster thumb", mov.Thumbnails)
	}
}

func TestFromMergedSceneNoDate(t *testing.T) {
	m := match.MergedScene{Title: "No Date"}
	mov := FromMergedScene(m)
	if mov.Premiered != "" {
		t.Errorf("premiered = %q, want empty", mov.Premiered)
	}
}

func TestFromMergedSceneNoThumbnail(t *testing.T) {
	m := match.MergedScene{Title: "No Thumb"}
	mov := FromMergedScene(m)
	if len(mov.Thumbnails) != 0 {
		t.Errorf("thumbnails = %d, want 0", len(mov.Thumbnails))
	}
}

func TestMarshal(t *testing.T) {
	mov := Movie{
		Title:      "Test",
		URLs:       []string{"https://example.com"},
		Premiered:  "2024-01-01",
		Plot:       "Description.",
		Studio:     "Studio",
		Thumbnails: []Thumb{{Aspect: "poster", URL: "https://example.com/img.jpg"}},
		Actors:     []Actor{{Name: "Actor"}},
		Tags:       []string{"tag1"},
	}

	data, err := Marshal(mov)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	s := string(data)
	if !strings.HasPrefix(s, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("missing XML declaration")
	}
	if !strings.Contains(s, "<movie>") {
		t.Errorf("missing <movie> root element")
	}
	if !strings.Contains(s, "<title>Test</title>") {
		t.Errorf("missing title")
	}
	if !strings.Contains(s, `<thumb aspect="poster">https://example.com/img.jpg</thumb>`) {
		t.Errorf("missing thumb with aspect attribute")
	}
	if !strings.Contains(s, "<name>Actor</name>") {
		t.Errorf("missing actor name")
	}
}

func TestMarshalXMLEscaping(t *testing.T) {
	mov := Movie{
		Title:  `Scene with "quotes" & <brackets>`,
		Actors: []Actor{{Name: "O'Malley"}},
	}

	data, err := Marshal(mov)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	s := string(data)
	if !strings.Contains(s, "&amp;") {
		t.Errorf("expected XML-escaped ampersand in: %s", s)
	}
	if !strings.Contains(s, "&#34;") || !strings.Contains(s, "&lt;") {
		t.Errorf("expected XML-escaped special chars in: %s", s)
	}
}
