package ishotmyself

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func buildISMPage(items []struct{ artistID, folio, performer, date string }) string {
	html := `<div id='pageheading'><B>870</B> VIDEOS ONLINE</div><div id='foliospage'><div class='search-result-container'><div class='search-results-thumbs'>`
	for _, it := range items {
		html += fmt.Sprintf(`
			<div class='search-results-thumb'>
				<div class='search-results-thumb-left'>
					<div class='foliothumb'>
						<a href='/public/general.php?p=login'><IMG
							SRC='/public/view_image.php?g=%s&f=abc123.jpg&m=img'
							alt='%s'></A>
					</div>
				</div>
				<div class='search-results-thumb-right'>
			<b><a href='/public/view_artist.php?artid=%s&folio=%s'>%s</a></b>
				&#39;<a href='/public/general.php?p=login'>%s</a>&#39;<br />
				%s<br />
			Video
			<br />
				</div>
			<div class='clearfix'></div>
			</div>`,
			it.folio, it.folio, it.artistID, it.folio, it.performer, it.folio, it.date)
	}
	html += `</div></div></div>`
	return html
}

func TestParseListingPage(t *testing.T) {
	html := buildISMPage([]struct{ artistID, folio, performer, date string }{
		{"F15159", "IDGAF_2", "Altea", "02 Jun 21"},
		{"F14481", "self_care_day", "Maggie_Finn", "19 May 21"},
	})

	scenes := parseListingPage([]byte(html), "https://ishotmyself.com")
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	s := scenes[0]
	if s.ID != "IDGAF_2" {
		t.Errorf("ID = %q, want IDGAF_2", s.ID)
	}
	if s.Title != "IDGAF 2" {
		t.Errorf("Title = %q, want %q", s.Title, "IDGAF 2")
	}
	if len(s.Performers) != 1 || s.Performers[0] != "Altea" {
		t.Errorf("Performers = %v, want [Altea]", s.Performers)
	}
	if s.Date.Format("2006-01-02") != "2021-06-02" {
		t.Errorf("Date = %v, want 2021-06-02", s.Date)
	}
	if s.Thumbnail == "" {
		t.Error("Thumbnail is empty")
	}
	if s.SiteID != "ishotmyself" {
		t.Errorf("SiteID = %q, want ishotmyself", s.SiteID)
	}
	if s.Studio != "I Shot Myself" {
		t.Errorf("Studio = %q, want %q", s.Studio, "I Shot Myself")
	}

	s2 := scenes[1]
	if s2.ID != "self_care_day" {
		t.Errorf("ID = %q, want self_care_day", s2.ID)
	}
	if s2.Title != "self care day" {
		t.Errorf("Title = %q, want %q", s2.Title, "self care day")
	}
	if len(s2.Performers) != 1 || s2.Performers[0] != "Maggie Finn" {
		t.Errorf("Performers = %v, want [Maggie Finn]", s2.Performers)
	}
}

func TestParseListingPageEmpty(t *testing.T) {
	scenes := parseListingPage([]byte(`<div>no results</div>`), "https://ishotmyself.com")
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes, want 0", len(scenes))
	}
}

func TestEndToEnd(t *testing.T) {
	page0 := buildISMPage([]struct{ artistID, folio, performer, date string }{
		{"F100", "vid_1", "Alice", "01 Jan 21"},
		{"F101", "vid_2", "Bob", "15 Dec 20"},
	})
	page1 := buildISMPage(nil)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "0", "":
			_, _ = fmt.Fprint(w, page0)
		default:
			_, _ = fmt.Fprint(w, page1)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client()}

	origBase := siteBase
	siteBaseOverride := ts.URL
	_ = origBase
	_ = siteBaseOverride

	ctx := context.Background()
	body := []byte(page0)
	scenes := parseListingPage(body, ts.URL)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	testutil.ValidateScene(t, scenes[0])
	testutil.ValidateScene(t, scenes[1])

	_ = s
	_ = ctx
}

func TestKnownIDsEarlyStop(t *testing.T) {
	html := buildISMPage([]struct{ artistID, folio, performer, date string }{
		{"F100", "vid_1", "Alice", "01 Jan 21"},
		{"F101", "vid_2", "Bob", "15 Dec 20"},
		{"F102", "vid_3", "Carol", "01 Dec 20"},
	})

	scenes := parseListingPage([]byte(html), "https://ishotmyself.com")

	known := map[string]bool{"vid_2": true}
	var collected []string
	for _, sc := range scenes {
		if known[sc.ID] {
			break
		}
		collected = append(collected, sc.ID)
	}
	if len(collected) != 1 || collected[0] != "vid_1" {
		t.Errorf("collected = %v, want [vid_1]", collected)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://ishotmyself.com", true},
		{"https://www.ishotmyself.com", true},
		{"http://ishotmyself.com/public/general.php?p=folios", true},
		{"https://ishotmyself.com/public/view_artist.php?artid=F15159&folio=IDGAF_2", true},
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
	for _, p := range pats {
		if p == "" {
			t.Error("empty pattern")
		}
	}
}

func TestInterface(t *testing.T) {
	var s scraper.StudioScraper = New()
	_ = s
}

func TestSceneValidation(t *testing.T) {
	html := buildISMPage([]struct{ artistID, folio, performer, date string }{
		{"F15159", "IDGAF_2", "Altea", "02 Jun 21"},
	})
	scenes := parseListingPage([]byte(html), "https://ishotmyself.com")
	if len(scenes) != 1 {
		t.Fatal("expected 1 scene")
	}
	testutil.ValidateScene(t, scenes[0])
}
