package beautifulagony

import (
	"fmt"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func buildBAPage(items []struct {
	id, date, thumb string
	hd              bool
}) string {
	html := ""
	for _, it := range items {
		hdTag := ""
		if it.hd {
			hdTag = `<font class="hdtext">HD</font>`
		}
		thumbTag := ""
		if it.thumb != "" {
			thumbTag = fmt.Sprintf(`<img src="%s">`, it.thumb)
		}
		html += fmt.Sprintf(`<div class="vid">
			<font class="agonyid">#%s</font>
			<div class="thumb_release_date_div">%s</div>
			%s
			%s
		</div>`, it.id, it.date, thumbTag, hdTag)
	}
	return html
}

func TestParseListingPage(t *testing.T) {
	html := buildBAPage([]struct {
		id, date, thumb string
		hd              bool
	}{
		{"5864", "02 Jun 2021", "https://bcdn.beautifulagony.com/img/5864.jpg", true},
		{"5863", "01 Jun 2021", "https://bcdn.beautifulagony.com/img/5863.jpg", false},
	})

	scenes := parseListingPage([]byte(html), "https://beautifulagony.com")
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	s := scenes[0]
	if s.ID != "5864" {
		t.Errorf("ID = %q, want 5864", s.ID)
	}
	if s.Title != "Agony #5864" {
		t.Errorf("Title = %q, want %q", s.Title, "Agony #5864")
	}
	if s.Date.Format("2006-01-02") != "2021-06-02" {
		t.Errorf("Date = %v, want 2021-06-02", s.Date)
	}
	if s.Thumbnail != "https://bcdn.beautifulagony.com/img/5864.jpg" {
		t.Errorf("Thumbnail = %q", s.Thumbnail)
	}
	if s.Resolution != "HD" {
		t.Errorf("Resolution = %q, want HD", s.Resolution)
	}
	if s.SiteID != "beautifulagony" {
		t.Errorf("SiteID = %q, want beautifulagony", s.SiteID)
	}
	if s.Studio != "Beautiful Agony" {
		t.Errorf("Studio = %q, want %q", s.Studio, "Beautiful Agony")
	}

	s2 := scenes[1]
	if s2.Resolution != "" {
		t.Errorf("Resolution = %q, want empty", s2.Resolution)
	}
}

func TestParseListingPageEmpty(t *testing.T) {
	scenes := parseListingPage([]byte(`<div>empty</div>`), "https://beautifulagony.com")
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes, want 0", len(scenes))
	}
}

func TestSplitVidBlocks(t *testing.T) {
	html := `<div class="vid">block1</div><div class="vid">block2</div>`
	blocks := splitVidBlocks([]byte(html))
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
}

func TestKnownIDsEarlyStop(t *testing.T) {
	html := buildBAPage([]struct {
		id, date, thumb string
		hd              bool
	}{
		{"100", "01 Jan 2021", "", false},
		{"99", "31 Dec 2020", "", false},
		{"98", "30 Dec 2020", "", false},
	})

	scenes := parseListingPage([]byte(html), "https://beautifulagony.com")
	known := map[string]bool{"99": true}
	var collected []string
	for _, sc := range scenes {
		if known[sc.ID] {
			break
		}
		collected = append(collected, sc.ID)
	}
	if len(collected) != 1 || collected[0] != "100" {
		t.Errorf("collected = %v, want [100]", collected)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://beautifulagony.com", true},
		{"https://www.beautifulagony.com", true},
		{"http://beautifulagony.com/public/main.php", true},
		{"https://ifeelmyself.com", false},
		{"https://example.com", false},
	}
	for _, tc := range tests {
		if got := s.MatchesURL(tc.url); got != tc.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestPatterns(t *testing.T) {
	s := New()
	pats := s.Patterns()
	if len(pats) < 1 {
		t.Fatal("no patterns")
	}
}

func TestInterface(t *testing.T) {
	var s scraper.StudioScraper = New()
	_ = s
}

func TestSceneValidation(t *testing.T) {
	html := buildBAPage([]struct {
		id, date, thumb string
		hd              bool
	}{
		{"5864", "02 Jun 2021", "https://bcdn.beautifulagony.com/img/5864.jpg", true},
	})
	scenes := parseListingPage([]byte(html), "https://beautifulagony.com")
	if len(scenes) != 1 {
		t.Fatal("expected 1 scene")
	}
	testutil.ValidateScene(t, scenes[0])
}
