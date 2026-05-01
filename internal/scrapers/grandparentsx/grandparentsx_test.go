package grandparentsx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func videoBlock(id, title, thumb, date string, durationMin int) string {
	dateHTML := ""
	if date != "" {
		dateHTML = fmt.Sprintf(`<div class="featuring2">%s</div>`, date)
	}
	durHTML := ""
	if durationMin > 0 {
		durHTML = fmt.Sprintf(`<span class="video-duration"><i class="fa fa-clock-o"></i> %d min</span>`, durationMin)
	}
	return fmt.Sprintf(`<div class='video-wrapper'>
            <div class="ratio-16-9 video-item progressive-load" data-image="%s"><i class="fa fa-play fa-3x thumbnail-play-icon"></i></div>
            <h3 class="video-description">
                <span class="featuring">%s</span>
                %s
                <a href="https://adultprime.com/signup?id=abc&refsite=Grandparentsx&galleryId=%s" class="thumbnail-btn">WATCH NOW</a>
            </h3>
            <a href="https://adultprime.com/signup?id=abc&refsite=Grandparentsx&galleryId=%s" class="absolute"></a>
            %s
        </div>
    </div>
</div>`, thumb, title, dateHTML, id, id, durHTML)
}

func homepageHTML(blocks ...string) string {
	body := `<!DOCTYPE html><html><body>`
	for _, b := range blocks {
		body += `<div class="col-xs-6 col-md-4 mt-10">` + b
	}
	body += `</body></html>`
	return body
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://grandparentsx.com/", true},
		{"https://grandparentsx.com", true},
		{"http://grandparentsx.com/", true},
		{"https://www.grandparentsx.com/", true},
		{"https://grandparentsx.com/some/path", false},
		{"https://example.com/", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestParseScenes(t *testing.T) {
	html := homepageHTML(
		videoBlock("143938", "Test Scene One", "https://cdn.example.com/143938/image.jpg", "April 26, 2026", 22),
		videoBlock("143603", "Test Scene Two", "https://cdn.example.com/143603/image.jpg", "April 05, 2026", 23),
	)
	items := parseScenes([]byte(html))
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].id != "143938" {
		t.Errorf("item[0].id = %q", items[0].id)
	}
	if items[0].title != "Test Scene One" {
		t.Errorf("item[0].title = %q", items[0].title)
	}
	if items[0].thumb != "https://cdn.example.com/143938/image.jpg" {
		t.Errorf("item[0].thumb = %q", items[0].thumb)
	}
	if items[0].date != "April 26, 2026" {
		t.Errorf("item[0].date = %q", items[0].date)
	}
	if items[0].duration != 22*60 {
		t.Errorf("item[0].duration = %d", items[0].duration)
	}
}

func TestParseScenesDedup(t *testing.T) {
	html := homepageHTML(
		videoBlock("143938", "Scene A", "https://cdn.example.com/a.jpg", "April 26, 2026", 22),
		videoBlock("143938", "Scene A Duplicate", "https://cdn.example.com/a.jpg", "April 26, 2026", 22),
		videoBlock("143603", "Scene B", "https://cdn.example.com/b.jpg", "April 05, 2026", 23),
	)
	items := parseScenes([]byte(html))
	if len(items) != 2 {
		t.Errorf("got %d items, want 2 (dedup)", len(items))
	}
}

func TestParseScenesNoDateNoDuration(t *testing.T) {
	html := homepageHTML(
		videoBlock("100334", "OLD SCENE", "https://cdn.example.com/gpx/old.jpg", "", 0),
	)
	items := parseScenes([]byte(html))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].date != "" {
		t.Errorf("date should be empty, got %q", items[0].date)
	}
	if items[0].duration != 0 {
		t.Errorf("duration should be 0, got %d", items[0].duration)
	}
}

func TestToScene(t *testing.T) {
	item := sceneItem{
		id:       "143938",
		title:    "Test Scene",
		thumb:    "https://cdn.example.com/143938/image.jpg",
		date:     "April 26, 2026",
		duration: 22 * 60,
		sceneURL: "https://grandparentsx.com/#143938",
	}
	scene := item.toScene("https://grandparentsx.com/")

	if scene.ID != "143938" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "grandparentsx" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Title != "Test Scene" {
		t.Errorf("Title = %q", scene.Title)
	}
	if scene.Studio != "GrandparentsX" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	wantDate := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}
	if scene.Duration != 1320 {
		t.Errorf("Duration = %d, want 1320", scene.Duration)
	}
	if scene.Thumbnail != "https://cdn.example.com/143938/image.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
}

func TestToSceneNoDate(t *testing.T) {
	item := sceneItem{
		id:       "100334",
		title:    "OLD SCENE",
		sceneURL: "https://grandparentsx.com/#100334",
	}
	scene := item.toScene("https://grandparentsx.com/")
	if !scene.Date.IsZero() {
		t.Errorf("Date should be zero, got %v", scene.Date)
	}
}

func TestListScenes(t *testing.T) {
	html := homepageHTML(
		videoBlock("143938", "Scene One", "https://cdn.example.com/1.jpg", "April 26, 2026", 22),
		videoBlock("143603", "Scene Two", "https://cdn.example.com/2.jpg", "April 05, 2026", 23),
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, html)
	}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	html := homepageHTML(
		videoBlock("143938", "New Scene", "https://cdn.example.com/1.jpg", "April 26, 2026", 22),
		videoBlock("143603", "Old Scene", "https://cdn.example.com/2.jpg", "April 05, 2026", 23),
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, html)
	}))
	defer ts.Close()

	s := New()
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{
		KnownIDs: map[string]bool{"143603": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var scenes, stopped int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
		case scraper.KindStoppedEarly:
			stopped++
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 1 {
		t.Errorf("got %d scenes, want 1", scenes)
	}
	if stopped != 1 {
		t.Errorf("got %d stopped, want 1", stopped)
	}
}
