package stash

import (
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
)

func TestMergeScenesBasic(t *testing.T) {
	s1 := models.Scene{
		ID:          "1",
		SiteID:      "manyvids",
		Title:       "Fostering the Bully",
		URL:         "https://manyvids.com/Video/123/fostering-the-bully",
		Date:        time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		Description: "Short desc",
		Performers:  []string{"Bettie Bondage"},
		Tags:        []string{"JOI", "POV"},
		Studio:      "Bettie Bondage",
		Duration:    600,
		Width:       1920,
		Height:      1080,
	}
	s2 := models.Scene{
		ID:          "456",
		SiteID:      "clips4sale",
		Title:       "Fostering the Bully",
		URL:         "https://clips4sale.com/studio/789/fostering-the-bully",
		Date:        time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Description: "A much longer and more detailed description of the scene",
		Performers:  []string{"Bettie Bondage"},
		Tags:        []string{"JOI", "Taboo"},
		Studio:      "Bettie Bondage",
		Duration:    610,
		Width:       3840,
		Height:      2160,
	}

	m := MergeScenes([]models.Scene{s1, s2}, time.Time{})

	if m.Title != "Fostering the Bully" {
		t.Errorf("Title = %q", m.Title)
	}

	// Earliest date wins.
	wantDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if !m.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", m.Date, wantDate)
	}

	// Longest description wins.
	if m.Description != s2.Description {
		t.Errorf("Description = %q, want s2's longer one", m.Description)
	}

	// URLs are union.
	if len(m.URLs) != 2 {
		t.Errorf("URLs len = %d, want 2", len(m.URLs))
	}

	// Tags are union.
	wantTags := map[string]bool{"JOI": true, "POV": true, "Taboo": true}
	if len(m.Tags) != len(wantTags) {
		t.Errorf("Tags = %v, want %d unique", m.Tags, len(wantTags))
	}
	for _, tag := range m.Tags {
		if !wantTags[tag] {
			t.Errorf("unexpected tag %q", tag)
		}
	}

	// Performers deduplicated.
	if len(m.Performers) != 1 || m.Performers[0] != "Bettie Bondage" {
		t.Errorf("Performers = %v", m.Performers)
	}

	// Max duration.
	if m.Duration != 610 {
		t.Errorf("Duration = %d, want 610", m.Duration)
	}

	// Highest resolution.
	if m.Width != 3840 {
		t.Errorf("Width = %d, want 3840", m.Width)
	}

	// Sites tracked.
	if len(m.Sites) != 2 {
		t.Errorf("Sites = %v, want 2 entries", m.Sites)
	}
}

func TestMergeScenesExistingDateEarlier(t *testing.T) {
	s := models.Scene{
		Title: "Test",
		Date:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	existingDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	m := MergeScenes([]models.Scene{s}, existingDate)
	if !m.Date.Equal(existingDate) {
		t.Errorf("Date = %v, want existing date %v", m.Date, existingDate)
	}
}

func TestMergeScenesExistingDateLater(t *testing.T) {
	s := models.Scene{
		Title: "Test",
		Date:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	existingDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	m := MergeScenes([]models.Scene{s}, existingDate)
	if !m.Date.Equal(s.Date) {
		t.Errorf("Date = %v, want FSS date %v", m.Date, s.Date)
	}
}

func TestMergeScenesSingle(t *testing.T) {
	s := models.Scene{
		ID:     "1",
		SiteID: "manyvids",
		Title:  "Solo Scene",
		URL:    "https://example.com/1",
		Tags:   []string{"A", "B"},
	}

	m := MergeScenes([]models.Scene{s}, time.Time{})
	if m.Title != "Solo Scene" {
		t.Errorf("Title = %q", m.Title)
	}
	if len(m.URLs) != 1 {
		t.Errorf("URLs len = %d", len(m.URLs))
	}
	if len(m.Tags) != 2 {
		t.Errorf("Tags len = %d", len(m.Tags))
	}
}

func TestResolutionTags(t *testing.T) {
	cases := []struct {
		width int
		want  string
	}{
		{3840, "4K Available"},
		{1920, "Full HD Available"},
		{1280, "HD Available"},
		{720, ""},
		{0, ""},
	}
	for _, c := range cases {
		tags := ResolutionTags(c.width)
		if c.want == "" {
			if len(tags) != 0 {
				t.Errorf("ResolutionTags(%d) = %v, want none", c.width, tags)
			}
		} else {
			if len(tags) != 1 || tags[0] != c.want {
				t.Errorf("ResolutionTags(%d) = %v, want [%q]", c.width, tags, c.want)
			}
		}
	}
}

func TestMergeURLs(t *testing.T) {
	existing := []string{"https://a.com", "https://b.com"}
	new := []string{"https://b.com", "https://c.com"}

	result := MergeURLs(existing, new)
	if len(result) != 3 {
		t.Errorf("len = %d, want 3", len(result))
	}
}

func TestMergeTagIDs(t *testing.T) {
	result := MergeTagIDs([]string{"1", "2"}, []string{"2", "3"})
	if len(result) != 3 {
		t.Errorf("len = %d, want 3", len(result))
	}
}
