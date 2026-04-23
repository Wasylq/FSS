package stash

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"MILF JOI Countdown!!!", "milf joi countdown"},
		{"Fostering the Bully", "fostering the bully"},
		{"Scene - Title (4K)", "scene title"},
		{"  spaces  everywhere  ", "spaces everywhere"},
		{"already clean", "already clean"},
		{"", ""},
		{"123-numbers_and-stuff", "123 numbers and stuff"},
		{"SunnyDayAtTheBeach_1080p", "sunny day at the beach 1080p"},
		{"camelCaseWords", "camel case words"},
		{"ALLCAPSword", "allcap sword"},
		{"Scene4KVersion", "scene4k version"},
		{"Some Title (FULL HD)", "some title"},
		{"Some Title (mp4)", "some title"},
		{"Some Title (mov)", "some title"},
		{"Some Title (wmv)", "some title"},
		{"Title With (Real Parens)", "title with real parens"},
	}
	for _, c := range cases {
		got := Normalize(c.input)
		if got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestStripExtension(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"scene.mp4", "scene"},
		{"scene.name.mp4", "scene.name"},
		{"no-extension", "no-extension"},
		{"", ""},
	}
	for _, c := range cases {
		got := stripExtension(c.input)
		if got != c.want {
			t.Errorf("stripExtension(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func scene(id, siteID, title string) models.Scene {
	return models.Scene{
		ID:     id,
		SiteID: siteID,
		Title:  title,
		URL:    "https://example.com/" + id,
	}
}

func sceneWithDuration(id, siteID, title string, duration int) models.Scene {
	s := scene(id, siteID, title)
	s.Duration = duration
	return s
}

func TestMatchExact(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "Fostering the Bully"),
		scene("2", "manyvids", "MILF JOI Countdown"),
	})

	r := idx.Match("Fostering the Bully.mp4", 0)
	if r.Confidence != MatchExact {
		t.Errorf("confidence = %v, want EXACT", r.Confidence)
	}
	if len(r.Scenes) != 1 || r.Scenes[0].ID != "1" {
		t.Errorf("scenes = %v, want [id=1]", r.Scenes)
	}
}

func TestMatchExactCaseInsensitive(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "Fostering the Bully"),
	})

	r := idx.Match("fostering the bully.mp4", 0)
	if r.Confidence != MatchExact {
		t.Errorf("confidence = %v, want EXACT", r.Confidence)
	}
}

func TestMatchSubstring(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "iwantclips", "JOI Countdown"),
	})

	r := idx.Match("Studio Name - JOI Countdown.mp4", 0)
	if r.Confidence != MatchSubstring {
		t.Errorf("confidence = %v, want SUBSTR", r.Confidence)
	}
	if len(r.Scenes) != 1 || r.Scenes[0].ID != "1" {
		t.Errorf("scenes = %v, want [id=1]", r.Scenes)
	}
}

func TestMatchSubstringPicksLongest(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "JOI"),
		scene("2", "iwantclips", "JOI Countdown Special"),
	})

	r := idx.Match("Studio - JOI Countdown Special Edition.mp4", 0)
	if r.Confidence != MatchSubstring {
		t.Errorf("confidence = %v, want SUBSTR", r.Confidence)
	}
	if len(r.Scenes) != 1 || r.Scenes[0].ID != "2" {
		t.Errorf("scenes = %v, want [id=2]", r.Scenes)
	}
}

func TestMatchAmbiguous(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "Hot Scene Alpha"),
		scene("2", "iwantclips", "Hot Scene Bravo"),
	})

	// Both titles are 3 words matching 3 of 4 filename words (75%) — tied length.
	r := idx.Match("Hot Scene Alpha Bravo.mp4", 0)
	if r.Confidence != MatchAmbiguous {
		t.Errorf("confidence = %v, want AMBIGUOUS", r.Confidence)
	}
}

func TestMatchNone(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "Fostering the Bully"),
	})

	r := idx.Match("completely unrelated filename.mp4", 0)
	if r.Confidence != MatchNone {
		t.Errorf("confidence = %v, want SKIP", r.Confidence)
	}
}

func TestMatchShortTitleRejectsLongFilename(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "JOI Custom Video"),
	})

	// "joi custom video" = 3 words, filename has 7 words → 3/7 = 43% < 50%, rejected
	r := idx.Match("yogabella_joi_and_squirting_custom_video_hd.mp4", 0)
	if r.Confidence != MatchNone {
		t.Errorf("confidence = %v, want SKIP (title too short for filename)", r.Confidence)
	}
}

func TestMatchNoPartialWord(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "Neighbor"),
	})

	r := idx.Match("Free Use Neighborhood Cumdump.mp4", 0)
	if r.Confidence != MatchNone {
		t.Errorf("confidence = %v, want SKIP (neighbor != neighborhood)", r.Confidence)
	}
}

func TestMatchCrossSiteSameTitle(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "Fostering the Bully"),
		scene("2", "clips4sale", "Fostering the Bully"),
	})

	r := idx.Match("Fostering the Bully.mp4", 0)
	if r.Confidence != MatchExact {
		t.Errorf("confidence = %v, want EXACT", r.Confidence)
	}
	if len(r.Scenes) != 2 {
		t.Errorf("scenes len = %d, want 2 (cross-site)", len(r.Scenes))
	}
}

func TestMatchStepStripped(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "Step-Sister Blackmail Custom Video"),
	})

	// Filename has "sister" without "step" — should match via sanitized index.
	r := idx.Match("yogabella_sister_blackmail_custom_video.mp4", 0)
	if r.Confidence == MatchNone {
		t.Errorf("confidence = %v, want a match (step stripped)", r.Confidence)
	}
	if len(r.Scenes) != 1 || r.Scenes[0].ID != "1" {
		t.Errorf("scenes = %v, want [id=1]", r.Scenes)
	}
}

func TestMatchStepInBothSides(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "Step-Sister Blackmail"),
	})

	// Filename also has "step" — should match on first pass (exact).
	r := idx.Match("Step-Sister Blackmail.mp4", 0)
	if r.Confidence != MatchExact {
		t.Errorf("confidence = %v, want EXACT", r.Confidence)
	}
}

func TestMatchStepInTitleNotFilename(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		scene("1", "manyvids", "We HAVE to STOP step-Son"),
	})

	r := idx.Match("yogabella_We-HAVE-to-STOP-Son.mp4", 0)
	if r.Confidence == MatchNone {
		t.Errorf("confidence = %v, want a match (step in title, not in filename)", r.Confidence)
	}
	if len(r.Scenes) != 1 || r.Scenes[0].ID != "1" {
		t.Errorf("scenes = %v, want [id=1]", r.Scenes)
	}
}

func TestMatchCamelCaseFilename(t *testing.T) {
	idx := BuildIndex([]models.Scene{scene("1", "tt", "Sunny Day at the Beach")})
	r := idx.Match("SunnyDayAtTheBeach_1080p.mp4", 0)
	if r.Confidence != MatchSubstring {
		t.Errorf("confidence = %v, want SUBSTR", r.Confidence)
	}
	if r.Scenes[0].Title != "Sunny Day at the Beach" {
		t.Errorf("title = %q", r.Scenes[0].Title)
	}
}

func TestMatchEmptyFilename(t *testing.T) {
	idx := BuildIndex([]models.Scene{scene("1", "mv", "Title")})
	r := idx.Match("", 0)
	if r.Confidence != MatchNone {
		t.Errorf("confidence = %v, want SKIP", r.Confidence)
	}
}

// ---- Duration filtering ----

func TestMatchExactDurationMatch(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		sceneWithDuration("1", "manyvids", "Some Title", 600),
	})

	// File is 605s, FSS says 600s — within tolerance
	r := idx.Match("Some Title.mp4", 605)
	if r.Confidence != MatchExact {
		t.Errorf("confidence = %v, want EXACT", r.Confidence)
	}
}

func TestMatchExactDurationMismatch(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		sceneWithDuration("1", "manyvids", "Some Title", 600),
	})

	// File is 900s, FSS says 600s — way off, rejected
	r := idx.Match("Some Title.mp4", 900)
	if r.Confidence != MatchNone {
		t.Errorf("confidence = %v, want SKIP (duration mismatch)", r.Confidence)
	}
}

func TestMatchDurationUnknownPassesThrough(t *testing.T) {
	idx := BuildIndex([]models.Scene{
		sceneWithDuration("1", "manyvids", "Some Title", 600),
	})

	// File duration unknown (0) — should still match
	r := idx.Match("Some Title.mp4", 0)
	if r.Confidence != MatchExact {
		t.Errorf("confidence = %v, want EXACT", r.Confidence)
	}
}

func TestDurationClose(t *testing.T) {
	// tolerance = max(fileDuration * 10%, 30s)
	cases := []struct {
		fss  int
		file float64
		want bool
	}{
		{600, 605, true},   // diff=5, tol=max(60.5,30)=60.5 → ok
		{600, 660, true},   // diff=60, tol=max(66,30)=66 → ok
		{600, 900, false},  // diff=300, tol=max(90,30)=90 → too far
		{100, 125, true},   // diff=25, tol=max(12.5,30)=30 → ok
		{100, 135, false},  // diff=35, tol=max(13.5,30)=30 → too far
		{60, 85, true},     // diff=25, tol=max(8.5,30)=30 → ok
		{60, 95, false},    // diff=35, tol=max(9.5,30)=30 → too far
		{0, 600, true},     // FSS unknown → pass
		{600, 0, true},     // file unknown → pass
	}
	for _, c := range cases {
		got := durationClose(c.fss, c.file)
		if got != c.want {
			t.Errorf("durationClose(%d, %.0f) = %v, want %v", c.fss, c.file, got, c.want)
		}
	}
}

// ---- File loading ----

func TestLoadJSONFiles(t *testing.T) {
	dir := t.TempDir()
	sf := studioFile{
		StudioURL:  "https://example.com/studio/1",
		ScrapedAt:  time.Now().UTC(),
		SceneCount: 2,
		Scenes: []models.Scene{
			scene("1", "manyvids", "Scene One"),
			scene("2", "manyvids", "Scene Two"),
		},
	}
	data, _ := json.MarshalIndent(sf, "", "  ")
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	scenes, err := LoadJSONFiles([]string{path})
	if err != nil {
		t.Fatalf("LoadJSONFiles error: %v", err)
	}
	if len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(scenes))
	}
}

func TestLoadJSONDir(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"studio-a.json", "studio-b.json"} {
		sf := studioFile{
			Scenes: []models.Scene{scene(name, "site", "Title for "+name)},
		}
		data, _ := json.MarshalIndent(sf, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Non-JSON file should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}

	scenes, err := LoadJSONDir(dir)
	if err != nil {
		t.Fatalf("LoadJSONDir error: %v", err)
	}
	if len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(scenes))
	}
}
