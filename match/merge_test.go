package match

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

func TestMergeScenesEmpty(t *testing.T) {
	m := MergeScenes(nil, time.Time{})
	if m.Title != "" {
		t.Errorf("Title = %q, want empty", m.Title)
	}
	if !m.Date.IsZero() {
		t.Errorf("Date = %v, want zero", m.Date)
	}
	if len(m.URLs) != 0 {
		t.Errorf("URLs len = %d, want 0", len(m.URLs))
	}
	if len(m.Tags) != 0 {
		t.Errorf("Tags len = %d, want 0", len(m.Tags))
	}
	if len(m.Performers) != 0 {
		t.Errorf("Performers len = %d, want 0", len(m.Performers))
	}
	if len(m.Sites) != 0 {
		t.Errorf("Sites len = %d, want 0", len(m.Sites))
	}
}

func TestMergeScenesEmptyWithExistingDate(t *testing.T) {
	existing := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	m := MergeScenes(nil, existing)
	if !m.Date.Equal(existing) {
		t.Errorf("Date = %v, want %v", m.Date, existing)
	}
}

func TestCleanDescription(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"triple spaces become newline", "word   word", "word\nword"},
		{"triple blank lines collapse", "a\n\n\nb", "a\n\nb"},
		{"many blank lines collapse", "a\n\n\n\n\nb", "a\n\nb"},
		{"leading/trailing whitespace stripped", "  hello  ", "hello"},
		{"tabs count as space runs", "word\t\t\tword", "word\nword"},
		{"two spaces not enough", "word  word", "word  word"},
		{"normal text unchanged", "A short description.", "A short description."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cleanDescription(c.input)
			if got != c.want {
				t.Errorf("cleanDescription(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestMergeScenesResolutionTie(t *testing.T) {
	s1 := models.Scene{
		ID:     "1",
		SiteID: "siteA",
		Title:  "Scene A",
		Width:  1920,
		Height: 1080,
	}
	s2 := models.Scene{
		ID:     "2",
		SiteID: "siteB",
		Title:  "Scene B",
		Width:  1920,
		Height: 800,
	}

	m := MergeScenes([]models.Scene{s1, s2}, time.Time{})

	// When widths tie, the first scene's values should be kept (> comparison, not >=).
	if m.Width != 1920 {
		t.Errorf("Width = %d, want 1920", m.Width)
	}
	if m.Height != 1080 {
		t.Errorf("Height = %d, want 1080 (first scene wins on tie)", m.Height)
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

func TestMergeStrings(t *testing.T) {
	cases := []struct {
		name             string
		existing, update []string
		want             []string
	}{
		{"both empty", nil, nil, []string{}},
		{"existing only", []string{"a", "b"}, nil, []string{"a", "b"}},
		{"new only", nil, []string{"a", "b"}, []string{"a", "b"}},
		{"disjoint appends in order", []string{"a"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"dedups overlap, keeps existing order", []string{"a", "b"}, []string{"b", "a", "c"}, []string{"a", "b", "c"}},
		{"preserves duplicates already in existing", []string{"a", "a"}, []string{"a"}, []string{"a", "a"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MergeStrings(c.existing, c.update)
			if len(got) != len(c.want) {
				t.Fatalf("MergeStrings(%v, %v) = %v, want %v", c.existing, c.update, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("MergeStrings(%v, %v) = %v, want %v", c.existing, c.update, got, c.want)
				}
			}
		})
	}
}

func TestMergeStrings_doesNotMutateInputs(t *testing.T) {
	existing := []string{"a", "b"}
	update := []string{"c"}
	_ = MergeStrings(existing, update)
	if len(existing) != 2 || existing[0] != "a" || existing[1] != "b" {
		t.Errorf("existing slice was mutated: %v", existing)
	}
	if len(update) != 1 || update[0] != "c" {
		t.Errorf("update slice was mutated: %v", update)
	}
}
